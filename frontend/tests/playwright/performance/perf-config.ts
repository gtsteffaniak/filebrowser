import { readFile } from "node:fs/promises";
import path from "node:path";
import type { ProbeSnapshot } from "./perf-helpers";
import type { PerfResultFile } from "./perf-report";

export type PerfThresholds = {
  loadListingMs: number;
  scrollDurationMs: number;
  resizeDurationMs: number;
  select20Ms: number;
  maxAddEventListenerCount: number;
  maxIntersectionObservers: number;
  maxCdpJsEventListeners: number;
  maxLongTaskCount: number;
  maxLongTaskMs: number;
  maxFrameP95Ms: number;
};

export type PerfConfig = {
  enforce: boolean;
  scales: number[];
  workers: number;
  selectCountByScale: Record<string, number>;
  scenarios: {
    scroll: { steps: number; stepsAtScale10000: number };
    resize: { settleMs: number; settleMsAtScale10000: number };
  };
  thresholds: PerfThresholds;
};

const CONFIG_REL = "tests/playwright/performance/perf-config.json";

/**
 * Absolute ceilings.
 *
 * These are deliberately loose catastrophe limits: the real regression signal
 * comes from the baseline comparison (see perf-compare.ts), which measures
 * movement relative to a known-good run. Keeping both means a first run with no
 * baseline still catches pathological blowups, while the baseline catches
 * subtle drift.
 */
const DEFAULT_CONFIG: PerfConfig = {
  enforce: false,
  scales: [100, 1000, 10000],
  workers: 3,
  selectCountByScale: { "100": 12, "1000": 10, "10000": 8 },
  scenarios: {
    scroll: { steps: 18, stepsAtScale10000: 12 },
    resize: { settleMs: 80, settleMsAtScale10000: 60 },
  },
  thresholds: {
    loadListingMs: 600000,
    scrollDurationMs: 30000,
    resizeDurationMs: 60000,
    select20Ms: 120000,
    maxAddEventListenerCount: 500000,
    maxIntersectionObservers: 25000,
    maxCdpJsEventListeners: 500000,
    maxLongTaskCount: 200,
    maxLongTaskMs: 120000,
    maxFrameP95Ms: 250,
  },
};

let configCache: PerfConfig | null = null;

export function perfConfigPath(): string {
  const override = process.env.PERF_CONFIG_PATH;
  if (override) return path.resolve(process.cwd(), override);
  return path.resolve(process.cwd(), CONFIG_REL);
}

export async function loadPerfConfig(): Promise<PerfConfig> {
  if (configCache) return configCache;
  try {
    const raw = await readFile(perfConfigPath(), "utf8");
    const parsed = JSON.parse(raw) as Partial<PerfConfig>;
    configCache = {
      ...DEFAULT_CONFIG,
      ...parsed,
      scenarios: { ...DEFAULT_CONFIG.scenarios, ...(parsed.scenarios ?? {}) },
      thresholds: { ...DEFAULT_CONFIG.thresholds, ...(parsed.thresholds ?? {}) },
    } as PerfConfig;
  } catch {
    configCache = DEFAULT_CONFIG;
  }
  return configCache;
}

export async function warmPerfConfig(): Promise<PerfConfig> {
  return loadPerfConfig();
}

export function getPerfConfig(): PerfConfig {
  return configCache ?? DEFAULT_CONFIG;
}

export function selectCountForScale(scale: number): number {
  const env = process.env.PERF_SELECT_COUNT;
  if (env) {
    const n = parseInt(env, 10);
    if (Number.isFinite(n) && n > 0) return n;
  }
  const cfg = getPerfConfig();
  return cfg.selectCountByScale[String(scale)] ?? 12;
}

export type ThresholdCheck = {
  passed: boolean;
  warnings: string[];
};

export function checkResultThresholds(
  scenario: string,
  metrics: Record<string, unknown>,
  config: PerfConfig,
): ThresholdCheck {
  const t = config.thresholds;
  const warnings: string[] = [];

  const check = (key: string, value: number, max?: number) => {
    if (max !== undefined && value > max) {
      warnings.push(`${key}=${value} exceeds max ${max}`);
    }
  };

  const duration =
    scenario === "load"
      ? Number(metrics.loadListingMs)
      : scenario === "scroll"
        ? Number(metrics.scrollDurationMs)
        : scenario === "resize"
          ? Number(metrics.resizeDurationMs)
          : Number(metrics.select20Ms);

  if (Number.isFinite(duration)) {
    switch (scenario) {
      case "load":
        check("loadListingMs", duration, t.loadListingMs);
        break;
      case "scroll":
        check("scrollDurationMs", duration, t.scrollDurationMs);
        break;
      case "resize":
        check("resizeDurationMs", duration, t.resizeDurationMs);
        break;
      case "select":
        check("select20Ms", duration, t.select20Ms);
        break;
    }
  }

  const probe = metrics.probe as ProbeSnapshot | undefined;
  if (probe) {
    check("addListenerCalls", probe.addListenerCalls, t.maxAddEventListenerCount);
    check(
      "intersectionObservers",
      probe.intersectionObservers,
      t.maxIntersectionObservers,
    );
  }

  // Scenario-scoped long tasks: the report carries the window bounds, so only
  // tasks that occurred during this scenario are counted.
  const startedAt = metrics.windowStart as number | undefined;
  const endedAt = metrics.windowEnd as number | undefined;
  const allTasks = probe?.longTasks ?? [];
  const tasks =
    typeof startedAt === "number" && typeof endedAt === "number"
      ? allTasks.filter((e) => e.startTime >= startedAt && e.startTime <= endedAt)
      : allTasks;
  const longMs = tasks.reduce((s, e) => s + e.duration, 0);
  check("longTaskCount", tasks.length, t.maxLongTaskCount);
  check("longTaskMs", Math.round(longMs), t.maxLongTaskMs);

  const frames = metrics.frameStats as { p95?: number } | undefined;
  if (frames && typeof frames.p95 === "number" && frames.p95 > 0) {
    check("frameP95", frames.p95, t.maxFrameP95Ms);
  }

  const cdpGauge = (metrics.cdp as { gauge?: { JSEventListeners?: number } } | null)
    ?.gauge;
  if (cdpGauge && typeof cdpGauge.JSEventListeners === "number") {
    check("JSEventListeners", cdpGauge.JSEventListeners, t.maxCdpJsEventListeners);
  }

  return { passed: warnings.length === 0, warnings };
}

export function runtimeEnvSnapshot(): Record<string, string> {
  const keys = [
    "PERF_SCALES",
    "PERF_WORKERS",
    "PERF_SELECT_COUNT",
    "PERF_FULL_TRACE",
    "PERF_PLAYWRIGHT_TRACE",
    "PERF_BASE_URL",
    "PERF_CONFIG_PATH",
    "PERF_BROWSERS",
    "PERF_REPEATS",
    "PERF_UPDATE_BASELINE",
    "PERF_ENFORCE_BASELINE",
    "PERF_MOCK_SEED",
    "PERF_IMAGE_TAG",
    "CI",
  ];
  const out: Record<string, string> = {};
  for (const k of keys) {
    if (process.env[k]) out[k] = process.env[k] as string;
  }
  return out;
}

export type { PerfResultFile };
