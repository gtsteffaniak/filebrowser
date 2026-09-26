import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { cpus, platform, release, arch } from "node:os";

/**
 * Chromium CI baseline.
 *
 * CI runs chromium only (see `Dockerfile.playwright-performance`, `ci` target),
 * so the baseline is a chromium-only artifact. Local runs cover all three
 * browsers for comparison, but firefox/webkit results are marked
 * `baselineEligible: false` and never gate.
 *
 * The file is committed so it is reviewable and diffable in pull requests, and
 * so the CI image already contains it via the existing `COPY ./frontend/tests/`.
 */

export type BaselineMetricEntry = {
  value: number;
  tolerancePct: number;
  noiseFloor: number;
  direction: "lower-is-better" | "higher-is-better";
  toleranceClass: "shape" | "timing" | "health";
  samples: number;
  /**
   * Observed min/max across the baseline's own iterations, when N>1.
   *
   * Recorded so the report can show how noisy a metric actually is rather than
   * implying every number is equally trustworthy.
   */
  min?: number;
  max?: number;
  spreadPct?: number;
};

export type BaselineRunEntry = {
  scale: number;
  scenario: string;
  metrics: Record<string, BaselineMetricEntry>;
};

export type EnvironmentFingerprint = {
  os: string;
  platform: string;
  release: string;
  arch: string;
  cpuModel: string;
  cpuCount: number;
  nodeVersion: string;
  playwrightVersion: string;
  browserVersion: string;
  browser: string;
  scales: number[];
  workers: number;
  deviceScaleFactor: number;
  /** Set when running inside the CI container image. */
  imageTag: string | null;
};

export type PerfBaseline = {
  schemaVersion: number;
  createdAt: string;
  gitSha: string | null;
  gitDirty: boolean | null;
  /** Baseline is only meaningful for comparisons in a compatible environment. */
  environment: EnvironmentFingerprint;
  metrics: Record<string, BaselineRunEntry>;
  notes?: string;
};

export const BASELINE_SCHEMA_VERSION = 1;

/**
 * Default tolerance per class.
 *
 * Derived from measured spread rather than guesswork. Across a baseline capture,
 * structural counts showed 0% spread while wall-clock durations reached 75% and
 * main-thread health metrics exceeded 200%. The tolerances below sit above the
 * observed noise so the gate fires on code changes, not on scheduling luck.
 */
export const DEFAULT_TOLERANCE_PCT: Record<string, number> = {
  shape: 10,
  timing: 100,
  health: 100,
};

export function baselineKey(scale: number, scenario: string): string {
  return `chromium@${scale}:${scenario}`;
}

export async function readPlaywrightVersion(): Promise<string> {
  try {
    const pkg = JSON.parse(
      await readFile(
        path.resolve(process.cwd(), "node_modules/@playwright/test/package.json"),
        "utf8",
      ),
    ) as { version?: string };
    return pkg.version ?? "unknown";
  } catch {
    return "unknown";
  }
}

export function environmentFingerprint(opts: {
  scales: number[];
  workers: number;
  browserVersion: string;
  playwrightVersion: string;
  deviceScaleFactor?: number;
}): EnvironmentFingerprint {
  const cpuList = cpus();
  return {
    os: platform(),
    platform: platform(),
    release: release(),
    arch: arch(),
    cpuModel: cpuList[0]?.model?.trim() ?? "unknown",
    cpuCount: cpuList.length,
    nodeVersion: process.version,
    playwrightVersion: opts.playwrightVersion,
    browserVersion: opts.browserVersion,
    browser: "chromium",
    scales: [...opts.scales].sort((a, b) => a - b),
    workers: opts.workers,
    deviceScaleFactor: opts.deviceScaleFactor ?? 1,
    imageTag: process.env.PERF_IMAGE_TAG ?? null,
  };
}

/**
 * Two fingerprints are comparable when the shape of the run matches. We
 * deliberately do not compare CPU model string: dev laptops vs CI runners are
 * covered by the explicit `imageTag`/worker check, and a strict CPU match would
 * make local dry-runs impossible. A mismatch produces a warning, not a
 * hard failure, so a genuine regression is never masked by noise.
 */
export function compareEnvironments(
  a: EnvironmentFingerprint,
  b: EnvironmentFingerprint,
): string[] {
  const diffs: string[] = [];
  if (a.browser !== b.browser) {
    diffs.push(`browser ${a.browser} != ${b.browser}`);
  }
  if (a.workers !== b.workers) {
    diffs.push(`workers ${a.workers} != ${b.workers}`);
  }
  if (a.scales.join(",") !== b.scales.join(",")) {
    diffs.push(`scales [${a.scales}] != [${b.scales}]`);
  }
  if (a.playwrightVersion !== b.playwrightVersion) {
    diffs.push(
      `playwright ${a.playwrightVersion} != ${b.playwrightVersion}`,
    );
  }
  if (a.cpuCount !== b.cpuCount) {
    diffs.push(`cpuCount ${a.cpuCount} != ${b.cpuCount}`);
  }
  if (a.imageTag !== b.imageTag) {
    diffs.push(
      `image ${a.imageTag ?? "local"} != ${b.imageTag ?? "local"}`,
    );
  }
  return diffs;
}

export function fingerprintHash(fp: EnvironmentFingerprint): string {
  const material = [
    fp.browser,
    fp.workers,
    fp.scales.join(","),
    fp.playwrightVersion,
    fp.cpuCount,
    fp.imageTag ?? "local",
  ].join("|");
  return createHash("sha256").update(material).digest("hex").slice(0, 12);
}

export async function loadBaseline(
  filePath: string,
): Promise<PerfBaseline | null> {
  try {
    const raw = await readFile(filePath, "utf8");
    const parsed = JSON.parse(raw) as PerfBaseline;
    if (parsed.schemaVersion !== BASELINE_SCHEMA_VERSION) {
      console.warn(
        `[perf] baseline schema ${parsed.schemaVersion} != ${BASELINE_SCHEMA_VERSION}; regenerate with PERF_UPDATE_BASELINE=1`,
      );
      return null;
    }
    return parsed;
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") return null;
    throw err;
  }
}

export async function writeBaseline(
  filePath: string,
  baseline: PerfBaseline,
): Promise<void> {
  await writeFile(filePath, `${JSON.stringify(baseline, null, 2)}\n`);
}

/** Median of numeric samples; used for N=3 local runs. */
export function median(values: number[]): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 === 0
    ? (sorted[mid - 1] + sorted[mid]) / 2
    : sorted[mid];
}

/**
 * Aggregate N samples of the same metric into a single baseline value.
 *
 * Uses the median so a single outlier iteration cannot skew the stored value.
 * Also reports spread so a noisy metric is visibly noisy rather than silently
 * unreliable.
 */
export function aggregateSamples(values: number[]): {
  value: number;
  min: number;
  max: number;
  spreadPct: number;
} {
  if (values.length === 0) {
    return { value: 0, min: 0, max: 0, spreadPct: 0 };
  }
  const value = median(values);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const spreadPct = value > 0 ? ((max - min) / value) * 100 : 0;
  return {
    value: round(value),
    min: round(min),
    max: round(max),
    spreadPct: round(spreadPct),
  };
}

function round(n: number): number {
  return Math.round(n * 100) / 100;
}
