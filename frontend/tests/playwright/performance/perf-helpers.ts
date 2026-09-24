import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import path from "node:path";
import type { Browser, Page, TestInfo } from "@playwright/test";
import { checkResultThresholds, loadPerfConfig } from "./perf-config";
import {
  aggregateSamples,
  BASELINE_SCHEMA_VERSION,
  DEFAULT_TOLERANCE_PCT,
  baselineKey,
  environmentFingerprint,
  median,
  readPlaywrightVersion,
  type BaselineMetricEntry,
  type BaselineRunEntry,
  type PerfBaseline,
} from "./perf-baseline";
import { failIfEmptyRunSet, writeFinalArtifacts } from "./perf-finalize";
import { installFrameTiming } from "./perf-frames";
import { installWebVitals } from "./perf-vitals";
import { extractBaselineMetrics } from "./perf-extract";
import { findMetric } from "./perf-metrics";
import { frontendRoot, performanceDir } from "./perf-paths";
import type { PerfResultFile } from "./perf-report";

/**
 * Results directory.
 *
 * CI does not set `PERF_RESULTS_DIR`, so the default must resolve to a stable
 * absolute path rather than depending on the container CWD. We anchor to the
 * frontend package root, which is correct both in the CI image and locally.
 */
const PERF_DIRECTORY_ROOT = frontendRoot(import.meta.url);
const PERF_DIR =
  process.env.PERF_RESULTS_DIR ??
  path.join(PERF_DIRECTORY_ROOT, "test-results", "listing-performance-perf");

/** Where the committed baseline lives (copied into the CI image already). */
export function baselinePath(): string {
  return (
    process.env.PERF_BASELINE_PATH ??
    path.join(performanceDir(import.meta.url), "perf-baseline.json")
  );
}

export function perfResultsDir(): string {
  return PERF_DIR;
}

export function baselineUpdateRequested(): boolean {
  return process.env.PERF_UPDATE_BASELINE === "1";
}

/**
 * Number of measurement iterations.
 *
 * CI uses a single pass (N=1) to keep PR feedback fast; local runs use N=3 and
 * take the median to average out machine noise. Set `PERF_REPEATS` to override.
 */
export function repeatCount(): number {
  if (process.env.PERF_REPEATS) {
    const n = parseInt(process.env.PERF_REPEATS, 10);
    if (Number.isFinite(n) && n > 0) return n;
  }
  return process.env.CI ? 1 : 3;
}

export function fullTraceEnabled(): boolean {
  return process.env.PERF_FULL_TRACE === "1";
}

export function playwrightTracingEnabled(): boolean {
  return process.env.PERF_PLAYWRIGHT_TRACE === "1";
}

export function isChromiumProject(testInfo: TestInfo): boolean {
  return testInfo.project.name === "chromium";
}

export function mockListingUrl(scale: number, seed?: number): string {
  const base = `/files/mockData?numDirs=${scale}&numFiles=${scale}`;
  return seed === undefined ? base : `${base}&seed=${seed}`;
}

export function expectedItemCount(scale: number): number {
  return scale * 2;
}

export type DomSnapshot = {
  listingItemCount: number;
  documentElementCount: number;
  bodyDescendantCount: number;
};

export async function readDomSnapshot(page: Page): Promise<DomSnapshot> {
  return page.evaluate(() => ({
    listingItemCount: document.querySelectorAll(".listing-items .listing-item")
      .length,
    documentElementCount: document.getElementsByTagName("*").length,
    bodyDescendantCount: document.body
      ? document.body.querySelectorAll("*").length
      : 0,
  }));
}

/**
 * Page probes.
 *
 * Long tasks are recorded with their absolute `startTime` so the scenario
 * runner can select only those that occurred inside its own window. The
 * previous implementation attributed every long task in the page's lifetime to
 * whichever scenario happened to read the probe last, which is why `select`
 * reported load's 23.5s task.
 */
export async function installProbes(page: Page): Promise<void> {
  await installFrameTiming(page);
  await installWebVitals(page);

  await page.addInitScript(() => {
    const perf = {
      addListenerCalls: 0,
      removeListenerCalls: 0,
      intersectionObservers: 0,
      longTasks: [] as { duration: number; startTime: number }[],
      /** performance.now() when the page probe was installed. */
      installedAt: 0,
    };
    (window as unknown as { __perf: typeof perf }).__perf = perf;

    const origAdd = EventTarget.prototype.addEventListener;
    const origRemove = EventTarget.prototype.removeEventListener;
    EventTarget.prototype.addEventListener = function (
      ...args: Parameters<typeof origAdd>
    ) {
      perf.addListenerCalls += 1;
      return origAdd.apply(this, args);
    };
    EventTarget.prototype.removeEventListener = function (
      ...args: Parameters<typeof origRemove>
    ) {
      perf.removeListenerCalls += 1;
      return origRemove.apply(this, args);
    };

    const OrigIO = window.IntersectionObserver;
    window.IntersectionObserver = class extends OrigIO {
      constructor(...args: ConstructorParameters<typeof OrigIO>) {
        super(...args);
        perf.intersectionObservers += 1;
      }
    } as typeof IntersectionObserver;

    try {
      new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          perf.longTasks.push({
            duration: entry.duration,
            startTime: entry.startTime,
          });
        }
      }).observe({ type: "longtask", buffered: true });
    } catch {
      // WebKit: no longtask support
    }
  });
}

export type LongTask = { duration: number; startTime: number };

export type ProbeSnapshot = {
  addListenerCalls: number;
  removeListenerCalls: number;
  intersectionObservers: number;
  longTasks: LongTask[];
};

export async function readProbe(page: Page): Promise<ProbeSnapshot> {
  return page.evaluate(() => {
    const perf = (window as unknown as { __perf?: ProbeSnapshot }).__perf;
    return (
      perf ?? {
        addListenerCalls: 0,
        removeListenerCalls: 0,
        intersectionObservers: 0,
        longTasks: [],
      }
    );
  });
}

/**
 * Long tasks that fall inside a scenario window.
 *
 * `startedAt`/`endedAt` are DOMHighResTimeStamp values from
 * `performance.now()` in the page, matching `LongTask.startTime`.
 */
export function longTasksInWindow(
  tasks: LongTask[],
  startedAt: number,
  endedAt: number,
): LongTask[] {
  return tasks.filter((t) => t.startTime >= startedAt && t.startTime <= endedAt);
}

/** Read a page-relative `performance.now()` timestamp for windowing. */
export async function pageNow(page: Page): Promise<number> {
  return page.evaluate(() => performance.now());
}

export type PerfMetrics = Record<string, unknown>;

/**
 * Persist one iteration's result.
 *
 * Iterations are written as `<browser>-<scale>-<scenario>.iter<N>.json` so N>1
 * runs keep every sample; aggregation happens at finalize time.
 */
export async function savePerfResult(
  testInfo: TestInfo,
  scenario: string,
  scale: number,
  metrics: PerfMetrics,
  iteration = 0,
): Promise<void> {
  await mkdir(PERF_DIR, { recursive: true });
  const suffix = iteration > 0 ? `.iter${iteration}` : "";
  const file = path.join(
    PERF_DIR,
    `${testInfo.project.name}-${scale}-${scenario}${suffix}.json`,
  );
  const enriched = {
    ...metrics,
    scale,
    numDirs: scale,
    numFiles: scale,
    totalMockItems: scale * 2,
  };
  const payload = {
    scenario,
    browser: testInfo.project.name,
    scale,
    iteration,
    fullTrace: fullTraceEnabled() && isChromiumProject(testInfo),
    playwrightTrace: playwrightTracingEnabled(),
    timestamp: new Date().toISOString(),
    metrics: enriched,
  };
  await writeFile(file, `${JSON.stringify(payload, null, 2)}\n`);
  await testInfo.attach(`${scenario}-metrics${suffix}`, {
    body: JSON.stringify(payload, null, 2),
    contentType: "application/json",
  });
  await evaluateThresholds(scenario, enriched, testInfo);
}

async function evaluateThresholds(
  scenario: string,
  metrics: PerfMetrics,
  testInfo: TestInfo,
): Promise<void> {
  const config = await loadPerfConfig();
  const { passed, warnings } = checkResultThresholds(scenario, metrics, config);
  if (passed) {
    return;
  }
  const msg = `[perf baseline] ${warnings.join("; ")}`;
  if (config.enforce) {
    throw new Error(msg);
  }
  console.warn(msg);
  testInfo.annotations.push({ type: "perf-warn", description: msg });
}

/**
 * Run the Chromium tracing path (renderer flame charts).
 *
 * This is `browser.startTracing`, which is a Chromium-only low-level API and is
 * distinct from Playwright Tracing (which works on all engines and is wired up
 * in the Playwright config). Chromium traces are enabled by default for the
 * load scenario because they are the highest-value root-cause artifact; set
 * `PERF_FULL_TRACE=1` to trace every scenario.
 */
export async function runWithOptionalChromeTracing(
  browser: Browser,
  page: Page,
  testInfo: TestInfo,
  scenario: string,
  fn: () => Promise<void>,
  options?: { force?: boolean },
): Promise<string | undefined> {
  const enabled = options?.force || fullTraceEnabled();
  if (!enabled || !isChromiumProject(testInfo)) {
    await fn();
    return undefined;
  }
  const traceDir = path.join(PERF_DIR, "traces");
  await mkdir(traceDir, { recursive: true });
  const tracePath = path.join(
    traceDir,
    `${testInfo.project.name}-${scenario}.json`,
  );
  await browser.startTracing(page, {
    path: tracePath,
    screenshots: false,
    categories: [
      "devtools.timeline",
      "v8.execute",
      "blink.user_timing",
      "disabled-by-default-devtools.timeline",
      "disabled-by-default-v8.cpu_profiler",
    ],
  });
  try {
    await fn();
  } finally {
    await browser.stopTracing();
  }
  return tracePath;
}

/* ------------------------------------------------------------------------- */
/* Aggregation + baseline generation                                          */
/* ------------------------------------------------------------------------- */

const RESULT_FILE_RE =
  /^([a-z]+)-(\d+)-(load|scroll|resize|select)(?:\.iter(\d+))?\.json$/;

export type ParsedResult = PerfResultFile & { iteration: number; file: string };

/** Read every per-run result file. Exported for tests. */
export async function readAllResults(): Promise<ParsedResult[]> {
  let dirEntries: string[];
  try {
    dirEntries = await readdir(PERF_DIR);
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") return [];
    throw err;
  }

  const out: ParsedResult[] = [];
  for (const file of dirEntries.filter((f) => RESULT_FILE_RE.test(f)).sort()) {
    const raw = JSON.parse(await readFile(path.join(PERF_DIR, file), "utf8")) as {
      scenario: string;
      browser: string;
      scale: number;
      iteration?: number;
      timestamp: string;
      metrics: Record<string, unknown>;
    };
    out.push({
      scenario: raw.scenario,
      browser: raw.browser,
      scale: raw.scale,
      iteration: raw.iteration ?? 0,
      timestamp: raw.timestamp,
      metrics: raw.metrics,
      file,
    });
  }
  return out;
}

/**
 * Collapse N iterations of the same (browser, scale, scenario) into one
 * dataset, taking the median of each metric.
 */
export function aggregateIterations(results: ParsedResult[]): ParsedResult[] {
  const groups = new Map<string, ParsedResult[]>();
  for (const r of results) {
    const key = `${r.browser}|${r.scale}|${r.scenario}`;
    const list = groups.get(key) ?? [];
    list.push(r);
    groups.set(key, list);
  }

  const out: ParsedResult[] = [];
  for (const list of groups.values()) {
    list.sort((a, b) => a.iteration - b.iteration);
    if (list.length === 1) {
      out.push({ ...list[0], file: baseFileName(list[0]) });
      continue;
    }
    // Merge metric bags by taking the median of each numeric leaf.
    const merged = medianMerge(list.map((r) => r.metrics));
    // Retain per-iteration normalized values so the baseline can record spread,
    // which lets the report show how noisy each metric actually is.
    // Sample arrays follow iteration order (repeat 1 = iteration 0).
    const samples: Record<string, number[]> = {};
    const keys = Object.keys(extractBaselineMetrics(list[0]));
    for (const key of keys) {
      const values: number[] = [];
      for (const r of list) {
        const bag = extractBaselineMetrics(r) as unknown as Record<string, number>;
        const v = bag[key];
        if (typeof v === "number" && Number.isFinite(v)) values.push(v);
      }
      if (values.length > 1) samples[key] = values;
    }
    out.push({
      ...list[0],
      iteration: list.length,
      metrics: { ...merged, __samples: samples },
      file: baseFileName(list[0]),
    });
  }
  return out;
}

function baseFileName(r: ParsedResult): string {
  return `${r.browser}-${r.scale}-${r.scenario}.json`;
}

/** Recursively take the median of numeric leaves across samples. */
export function medianMerge(
  samples: Record<string, unknown>[],
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  const keys = new Set<string>();
  for (const s of samples) {
    for (const k of Object.keys(s)) keys.add(k);
  }

  for (const key of keys) {
    const values = samples.map((s) => s[key]);
    const first = values.find((v) => v !== undefined && v !== null);

    if (typeof first === "number") {
      const nums = values.filter((v): v is number => typeof v === "number");
      out[key] = nums.length > 0 ? median(nums) : first;
      continue;
    }

    if (Array.isArray(first)) {
      // Long-task arrays: keep the sample with the most tasks so downstream
      // windowing still sees the worst case.
      const arrays = values.filter(Array.isArray) as unknown[][];
      out[key] =
        arrays.length > 0
          ? arrays.reduce((a, b) => (b.length > a.length ? b : a))
          : first;
      continue;
    }

    if (first && typeof first === "object") {
      const objs = values.filter(
        (v): v is Record<string, unknown> => !!v && typeof v === "object",
      );
      out[key] = medianMerge(objs);
      continue;
    }

    out[key] = first ?? null;
  }
  return out;
}

/** Build a chromium baseline from the aggregated runs. */
export async function buildBaselineFromResults(
  results: ParsedResult[],
  opts: {
    scales: number[];
    workers: number;
    browserVersion: string;
    gitSha: string | null;
    gitDirty: boolean | null;
  },
): Promise<PerfBaseline> {
  if (!opts.gitSha) {
    throw new Error(
      "refusing to build a baseline without a git revision; " +
        "the artifact must identify the source state it measured",
    );
  }
  const chromium = results.filter((r) => r.browser === "chromium");
  const metrics: Record<string, BaselineRunEntry> = {};

  for (const r of chromium) {
    const bag = extractBaselineMetrics(r) as unknown as Record<string, number>;
    const samples =
      (r.metrics as { __samples?: Record<string, number[]> }).__samples ?? {};
    const entry: Record<string, BaselineMetricEntry> = {};
    for (const [key, value] of Object.entries(bag)) {
      if (typeof value !== "number" || !Number.isFinite(value)) continue;
      const def = findMetric(key);
      if (!def) continue;
      const agg = samples[key]?.length
        ? aggregateSamples(samples[key])
        : undefined;
      entry[key] = {
        value: agg ? agg.value : value,
        tolerancePct: defaultTolerance(def.toleranceClass),
        noiseFloor: def.noiseFloor,
        direction: def.direction,
        toleranceClass: def.toleranceClass,
        samples: samples[key]?.length ?? (r.iteration > 0 ? r.iteration : 1),
        ...(agg
          ? { min: agg.min, max: agg.max, spreadPct: agg.spreadPct }
          : {}),
      };
    }
    metrics[baselineKey(r.scale, r.scenario)] = {
      scale: r.scale,
      scenario: r.scenario,
      metrics: entry,
    };
  }

  const playwrightVersion = await readPlaywrightVersion();
  return {
    schemaVersion: BASELINE_SCHEMA_VERSION,
    createdAt: new Date().toISOString(),
    gitSha: opts.gitSha,
    gitDirty: opts.gitDirty,
    environment: environmentFingerprint({
      scales: opts.scales,
      workers: opts.workers,
      browserVersion: opts.browserVersion,
      playwrightVersion,
    }),
    metrics,
    notes:
      "Chromium-only CI baseline. Regenerate with: make perf-baseline (PERF_UPDATE_BASELINE=1).",
  };
}

function defaultTolerance(cls: string): number {
  return DEFAULT_TOLERANCE_PCT[cls] ?? 25;
}

export { failIfEmptyRunSet, writeFinalArtifacts };
