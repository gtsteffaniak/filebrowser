import { expect, type Browser, type Page, type TestInfo } from "@playwright/test";
import { loadPerfConfig, selectCountForScale } from "./perf-config";
import {
  expectedItemCount,
  installProbes,
  mockListingUrl,
  pageNow,
  readDomSnapshot,
  readProbe,
  repeatCount,
  runWithOptionalChromeTracing,
  savePerfResult,
  type DomSnapshot,
} from "./perf-helpers";
import { withCdpDelta, flattenCdp, type CdpDelta } from "./perf-cdp";
import {
  startFrameWindow,
  stopFrameWindow,
  type FrameTimingStat,
} from "./perf-frames";
import {
  readWebVitals,
  startEventWindow,
  stopEventWindow,
  type InteractionTiming,
  type WebVitals,
} from "./perf-vitals";

/**
 * Scenario drivers.
 *
 * `scale` is always passed explicitly. The previous implementation stored it in
 * module-level mutable state while `test.describe.configure({ mode: "parallel" })`
 * ran scales concurrently, so a sibling worker could change the scale between
 * the URL being built and the expected row count being read.
 */

export type ScenarioOptions = {
  /** Listing already open at the correct scale (skip goto + full mount wait). */
  reuseListing?: boolean;
};

export function parsePerfScales(): number[] {
  const raw = process.env.PERF_SCALES ?? "100,1000,10000";
  const scales = raw
    .split(",")
    .map((s) => parseInt(s.trim(), 10))
    .filter((n) => Number.isFinite(n) && n > 0);
  return scales.length > 0 ? scales : [100, 1000, 10000];
}

/** Deterministic seed per scale so payload size is reproducible. */
export function seedForScale(scale: number): number {
  const env = process.env.PERF_MOCK_SEED;
  if (env) {
    const n = parseInt(env, 10);
    if (Number.isFinite(n)) return n + scale;
  }
  return scale * 1_000_003 + scale;
}

async function ensureListing(
  page: Page,
  scale: number,
  options?: ScenarioOptions,
): Promise<void> {
  if (!options?.reuseListing) {
    await openMockListing(page, scale);
  }
}

async function openMockListing(page: Page, scale: number): Promise<void> {
  const count = expectedItemCount(scale);
  const timeout = Math.min(600_000, 120_000 + count * 30);
  await page.goto(mockListingUrl(scale, seedForScale(scale)));
  await page.waitForResponse((r) => r.url().includes("mock-data") && r.ok(), {
    timeout,
  });
  await page
    .locator(".listing-items .listing-item")
    .first()
    .waitFor({ state: "visible", timeout });
  await page.waitForFunction(
    (expected) =>
      document.querySelectorAll(".listing-items .listing-item").length >=
      expected,
    count,
    { timeout },
  );
}

/** Per-scenario payload shared by every scenario runner. */
type ScenarioPayload = {
  loadListingMs?: number;
  scrollDurationMs?: number;
  resizeDurationMs?: number;
  select20Ms?: number;
  selectTarget?: number;
  selectedCount?: number;
  listenerDelta?: number;
  avgFps?: number;
  minFps?: number;
  frames?: number;
  windowStart: number;
  windowEnd: number;
  probe: ReturnType<typeof readProbe> extends Promise<infer T> ? T : never;
  probeBefore?: unknown;
  dom: DomSnapshot;
  cdp: CdpDelta | null;
  cdpFlat: Record<string, number> | null;
  vitals?: WebVitals;
  interaction?: InteractionTiming;
  chromeTracePath?: string;
};

async function save(
  testInfo: TestInfo,
  scenario: string,
  scale: number,
  payload: ScenarioPayload,
  iteration: number,
): Promise<void> {
  await savePerfResult(
    testInfo,
    scenario,
    scale,
    {
      ...payload,
      // Convenience scalars so both old and new consumers find a duration.
      scenarioMs:
        scenario === "load"
          ? payload.loadListingMs
          : scenario === "scroll"
            ? payload.scrollDurationMs
            : scenario === "resize"
              ? payload.resizeDurationMs
              : payload.select20Ms,
    },
    iteration,
  );
}

/**
 * One browser session per scale: a single full listing mount, then
 * load → scroll → resize → select. Repeated `repeatCount()` times.
 *
 * Each repeat runs in a FRESH page. Re-mounting the listing in the same page
 * accumulated state across iterations: JS event listeners and the CDP gauges
 * are page-lifetime measurements, so a second iteration reported exactly double
 * the listeners and heap of the first, and the median then merged two different
 * quantities. A new page makes every iteration an independent sample.
 */
export async function runAllScenariosForScale(
  page: Page,
  browser: Browser,
  testInfo: TestInfo,
  scale: number,
  freshPage?: () => Promise<Page>,
): Promise<void> {
  const iterations = repeatCount();

  for (let iteration = 0; iteration < iterations; iteration++) {
    // First iteration uses the Playwright-provided page; later iterations get a
    // brand new one so no DOM or listener state carries over.
    const activePage =
      iteration === 0 || !freshPage ? page : await freshPage();
    if (iteration > 0) {
      await installProbes(activePage);
    }

    await runLoadScenario(activePage, browser, testInfo, scale, iteration);

    try {
      await runScrollScenario(activePage, browser, testInfo, scale, iteration, {
        reuseListing: true,
      });
      await runResizeScenario(activePage, browser, testInfo, scale, iteration, {
        reuseListing: true,
      });
      await runSelectScenario(activePage, browser, testInfo, scale, iteration, {
        reuseListing: true,
      });
    } catch (err) {
      // A failed interaction should not discard a good load sample.
      console.warn(
        `[perf] iteration ${iteration} interaction failed at scale ${scale}: ${String(err)}`,
      );
      throw err;
    }

    if (iteration > 0 && freshPage) {
      await activePage.close();
    }
  }
}

export async function runLoadScenario(
  page: Page,
  browser: Browser,
  testInfo: TestInfo,
  scale: number,
  iteration = 0,
  options?: ScenarioOptions,
): Promise<DomSnapshot> {
  const before = await readProbe(page).catch(() => null);
  const windowStart = await pageNow(page).catch(() => 0);
  let loadListingMs = 0;

  // Chromium traces are forced on for load: it is the highest-value artifact
  // and the only scenario where the whole mount is attributable.
  const { cdp, result: traceValue } = await withCdpDelta(page, async () => {
    const tracePath = await runWithOptionalChromeTracing(
      browser,
      page,
      testInfo,
      `load-${scale}`,
      async () => {
        const startedAt = Date.now();
        await ensureListing(page, scale, options);
        loadListingMs = Date.now() - startedAt;
      },
      { force: isChromiumProjectWrapper(testInfo) },
    );
    return tracePath;
  });

  const windowEnd = await pageNow(page).catch(() => 0);
  const probe = await readProbe(page);
  const dom = await readDomSnapshot(page);
  const vitals = await safe(() => readWebVitals(page), undefined);

  await save(
    testInfo,
    "load",
    scale,
    {
      loadListingMs,
      windowStart,
      windowEnd,
      probe,
      probeBefore: before,
      dom,
      cdp,
      cdpFlat: flattenCdp(cdp),
      vitals,
      chromeTracePath: traceValue,
    },
    iteration,
  );
  return dom;
}

export async function runScrollScenario(
  page: Page,
  browser: Browser,
  testInfo: TestInfo,
  scale: number,
  iteration = 0,
  options?: ScenarioOptions,
): Promise<void> {
  await ensureListing(page, scale, options);
  const probeBefore = await readProbe(page);
  const windowStart = await pageNow(page);

  await startFrameWindow(page);
  await startEventWindow(page);

  let scrollDurationMs = 0;
  const { cdp, result: traceValue } = await withCdpDelta(page, async () => {
    const startedAt = Date.now();
    const tracePath = await runWithOptionalChromeTracing(
      browser,
      page,
      testInfo,
      `scroll-${scale}`,
      async () => {
        const { scenarios } = await loadPerfConfig();
        const steps =
          scale >= 10000
            ? scenarios.scroll.stepsAtScale10000
            : scenarios.scroll.steps;
        const listing = page.locator(".listing-items");
        for (let i = 0; i < steps; i++) {
          await listing.evaluate((el) => {
            el.scrollTop += 800;
          });
          await page.waitForTimeout(8);
        }
      },
    );
    scrollDurationMs = Date.now() - startedAt;
    return tracePath;
  });

  const frames = await stopFrameWindow(page);
  const interaction = await stopEventWindow(page);
  const windowEnd = await pageNow(page);

  const probe = await readProbe(page);
  const dom = await readDomSnapshot(page);

  await save(
    testInfo,
    "scroll",
    scale,
    {
      scrollDurationMs,
      windowStart,
      windowEnd,
      probe,
      probeBefore,
      listenerDelta: probe.addListenerCalls - probeBefore.addListenerCalls,
      dom,
      cdp,
      cdpFlat: flattenCdp(cdp),
      interaction,
      // Real frame health, replacing the old rAF-interval "FPS".
      avgFps: frames.effectiveFps,
      minFps: frames.p95 > 0 ? 1000 / frames.p95 : 0,
      frames: frames.frames,
      frameStats: frames,
      chromeTracePath: traceValue,
    } as ScenarioPayload,
    iteration,
  );
}

export async function runResizeScenario(
  page: Page,
  browser: Browser,
  testInfo: TestInfo,
  scale: number,
  iteration = 0,
  options?: ScenarioOptions,
): Promise<void> {
  await ensureListing(page, scale, options);
  const windowStart = await pageNow(page);
  const sizes = [
    { width: 1280, height: 720 },
    { width: 800, height: 600 },
    { width: 1280, height: 720 },
  ];

  await startFrameWindow(page);

  let resizeDurationMs = 0;
  const { cdp, result: traceValue } = await withCdpDelta(page, async () => {
    const startedAt = Date.now();
    const tracePath = await runWithOptionalChromeTracing(
      browser,
      page,
      testInfo,
      `resize-${scale}`,
      async () => {
        const { scenarios } = await loadPerfConfig();
        const settleMs =
          scale >= 10000
            ? scenarios.resize.settleMsAtScale10000
            : scenarios.resize.settleMs;
        for (const size of sizes) {
          await page.setViewportSize(size);
          await page.waitForTimeout(settleMs);
        }
      },
    );
    resizeDurationMs = Date.now() - startedAt;
    return tracePath;
  });

  const frames = await stopFrameWindow(page);
  const windowEnd = await pageNow(page);

  const probe = await readProbe(page);
  const dom = await readDomSnapshot(page);

  await save(
    testInfo,
    "resize",
    scale,
    {
      resizeDurationMs,
      windowStart,
      windowEnd,
      probe,
      dom,
      cdp,
      cdpFlat: flattenCdp(cdp),
      frameStats: frames,
      avgFps: frames.effectiveFps,
      frames: frames.frames,
      chromeTracePath: traceValue,
    } as ScenarioPayload,
    iteration,
  );
}

export async function runSelectScenario(
  page: Page,
  browser: Browser,
  testInfo: TestInfo,
  scale: number,
  iteration = 0,
  options?: ScenarioOptions,
): Promise<void> {
  await ensureListing(page, scale, options);
  const modifier = process.platform === "darwin" ? "Meta" : "Control";
  const items = page.locator(".listing-items .listing-item");
  const count = await items.count();
  const selectTarget = selectCountForScale(scale);
  const toSelect = Math.min(selectTarget, count);

  const windowStart = await pageNow(page);
  await startEventWindow(page);

  let select20Ms = 0;
  const { cdp, result: traceValue } = await withCdpDelta(page, async () => {
    const startedAt = Date.now();
    const tracePath = await runWithOptionalChromeTracing(
      browser,
      page,
      testInfo,
      `select-${scale}`,
      async () => {
        for (let i = 0; i < toSelect; i++) {
          await items.nth(i).click({ modifiers: [modifier] });
        }
        await page.locator("#status-bar .status-info .button").waitFor({
          state: "visible",
          timeout: 60_000,
        });
      },
    );
    select20Ms = Date.now() - startedAt;
    return tracePath;
  });

  const interaction = await stopEventWindow(page);
  const windowEnd = await pageNow(page);

  const selected = await page
    .locator('.listing-items .listing-item[aria-selected="true"]')
    .count();
  expect(selected).toBe(toSelect);

  const probe = await readProbe(page);
  const dom = await readDomSnapshot(page);

  await save(
    testInfo,
    "select",
    scale,
    {
      select20Ms,
      selectTarget,
      selectedCount: selected,
      windowStart,
      windowEnd,
      probe,
      dom,
      cdp,
      cdpFlat: flattenCdp(cdp),
      interaction,
      chromeTracePath: traceValue,
    } as ScenarioPayload,
    iteration,
  );
}

function isChromiumProjectWrapper(testInfo: TestInfo): boolean {
  return testInfo.project.name === "chromium";
}

async function safe<T>(
  fn: () => Promise<T>,
  fallback: T,
): Promise<T> {
  try {
    return await fn();
  } catch {
    return fallback;
  }
}

export type { FrameTimingStat };
