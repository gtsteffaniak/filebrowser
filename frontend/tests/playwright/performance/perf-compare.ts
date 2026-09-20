import {
  compareEnvironments,
  DEFAULT_TOLERANCE_PCT,
  type BaselineMetricEntry,
  type EnvironmentFingerprint,
  type PerfBaseline,
} from "./perf-baseline";
import { findMetric } from "./perf-metrics";

/**
 * Baseline comparison.
 *
 * Judgement is tiered rather than a single blanket percentage, because the
 * metrics have very different noise characteristics:
 *
 *  - structural counts (DOM nodes, listener counts) are deterministic, so a
 *    small move is a genuine regression and a tight tolerance is correct;
 *  - wall-clock durations on a shared CI runner are noisy and need headroom;
 *  - higher-is-better metrics (effective FPS) invert the comparison.
 *
 * A delta below the metric's absolute noise floor is ignored entirely, so
 * millisecond jitter cannot fail a build.
 */

export type ComparisonStatus = "pass" | "regression" | "improvement" | "missing";

export type MetricComparison = {
  key: string;
  label: string;
  unit: string;
  baseline: number;
  current: number;
  delta: number;
  deltaPct: number;
  allowedPct: number;
  noiseFloor: number;
  direction: "lower-is-better" | "higher-is-better";
  toleranceClass: string;
  status: ComparisonStatus;
  /**
   * Whether this metric may fail the build. Metrics with high run-to-run
   * variance (dropped frames, FPS) are reported but never gate, because a
   * flaky gate trains people to ignore it.
   */
  gating: boolean;
  /** Human-readable explanation, e.g. "+29.8% exceeds +25% limit". */
  detail: string;
};

export type RunComparison = {
  scale: number;
  scenario: string;
  status: ComparisonStatus;
  metrics: MetricComparison[];
};

export type ComparisonReport = {
  /** False when no baseline file exists yet. */
  hasBaseline: boolean;
  /** Non-fatal environment mismatches; comparisons are still produced. */
  environmentWarnings: string[];
  /** True when the environment mismatch is severe enough that gating is skipped. */
  gatingSkipped: boolean;
  totals: {
    compared: number;
    regressions: number;
    improvements: number;
    missing: number;
  };
  runs: RunComparison[];
  /** All regressions, including advisory (non-gating) ones. */
  regressions: MetricComparison[];
  /** Regressions that are allowed to fail the build. */
  gatedRegressions: MetricComparison[];
};

function toleranceFor(entry: BaselineMetricEntry): number {
  return entry.tolerancePct ?? DEFAULT_TOLERANCE_PCT[entry.toleranceClass] ?? 25;
}

/**
 * Decide whether `current` breaches `baseline`.
 *
 * For lower-is-better: breach when current > baseline * (1 + tol).
 * For higher-is-better: breach when current < baseline * (1 - tol).
 */
export function compareMetric(
  key: string,
  baseline: BaselineMetricEntry,
  current: number,
): MetricComparison {
  const def = findMetric(key);
  const label = def?.label ?? key;
  const unit = def?.unit ?? "count";
  const direction = baseline.direction ?? def?.direction ?? "lower-is-better";
  const allowedPct = toleranceFor(baseline);
  const noiseFloor = baseline.noiseFloor ?? def?.noiseFloor ?? 0;

  const delta = current - baseline.value;
  const deltaPct =
    baseline.value !== 0 ? (delta / baseline.value) * 100 : current === 0 ? 0 : Infinity;

  const base: Omit<MetricComparison, "status" | "detail"> = {
    key,
    label,
    unit,
    baseline: baseline.value,
    current,
    delta: round(delta),
    deltaPct: Number.isFinite(deltaPct) ? round(deltaPct) : 999,
    allowedPct,
    noiseFloor,
    direction,
    toleranceClass: baseline.toleranceClass ?? def?.toleranceClass ?? "timing",
    gating: def?.gating ?? true,
  };

  // Absolute noise floor: movement below this is not signal.
  const absoluteDelta = Math.abs(delta);
  const withinNoise = absoluteDelta < noiseFloor;

  const limit =
    direction === "lower-is-better"
      ? baseline.value * (1 + allowedPct / 100)
      : baseline.value * (1 - allowedPct / 100);

  const breaches =
    direction === "lower-is-better" ? current > limit : current < limit;

  if (withinNoise && breaches) {
    return {
      ...base,
      status: "pass",
      detail: `Δ${formatSigned(delta)}${unit === "ms" ? "ms" : ""} within noise floor (${noiseFloor})`,
    };
  }

  if (breaches) {
    // For an improvement in the "wrong" direction we never flag, so this is
    // always a real regression.
    const excessPct =
      direction === "lower-is-better"
        ? deltaPct - allowedPct
        : -deltaPct - allowedPct;
    return {
      ...base,
      status: "regression",
      detail: `${formatSigned(deltaPct)}% exceeds ${allowedPct}% limit (${formatSigned(excessPct)}% over)`,
    };
  }

  // Improvement: moved favourably beyond the noise floor.
  const favourable = direction === "lower-is-better" ? delta < 0 : delta > 0;
  if (favourable && !withinNoise) {
    return {
      ...base,
      status: "improvement",
      detail: `${formatSigned(deltaPct)}% better than baseline`,
    };
  }

  return {
    ...base,
    status: "pass",
    detail: `within ${allowedPct}% of baseline`,
  };
}

export function compareRun(
  scale: number,
  scenario: string,
  baselineEntry: { metrics: Record<string, BaselineMetricEntry> } | undefined,
  currentMetrics: Record<string, number>,
): RunComparison {
  if (!baselineEntry) {
    return { scale, scenario, status: "missing", metrics: [] };
  }

  const metrics: MetricComparison[] = [];
  for (const [key, entry] of Object.entries(baselineEntry.metrics)) {
    const current = currentMetrics[key];
    if (typeof current !== "number" || !Number.isFinite(current)) {
      metrics.push({
        key,
        label: findMetric(key)?.label ?? key,
        unit: findMetric(key)?.unit ?? "count",
        baseline: entry.value,
        current: Number.NaN,
        delta: Number.NaN,
        deltaPct: Number.NaN,
        allowedPct: toleranceFor(entry),
        noiseFloor: entry.noiseFloor ?? 0,
        direction: entry.direction,
        toleranceClass: entry.toleranceClass,
        gating: findMetric(key)?.gating ?? true,
        status: "missing",
        detail: "metric absent from current run",
      });
      continue;
    }
    metrics.push(compareMetric(key, entry, current));
  }

  const status: ComparisonStatus = metrics.some((m) => m.status === "regression")
    ? "regression"
    : metrics.some((m) => m.status === "missing")
      ? "missing"
      : "pass";

  return { scale, scenario, status, metrics };
}

export function compareAgainstBaseline(
  baseline: PerfBaseline | null,
  current: {
    environment: EnvironmentFingerprint;
    runs: { scale: number; scenario: string; metrics: Record<string, number> }[];
  },
): ComparisonReport {
  if (!baseline) {
    return {
      hasBaseline: false,
      environmentWarnings: [],
      gatingSkipped: false,
      totals: { compared: 0, regressions: 0, improvements: 0, missing: 0 },
      runs: [],
      regressions: [],
      gatedRegressions: [],
    };
  }

  const environmentWarnings = compareEnvironments(
    baseline.environment,
    current.environment,
  );
  // A worker-count or scale-set mismatch changes the measurement conditions
  // enough that gating would produce false regressions.
  const gatingSkipped = environmentWarnings.some(
    (w) => w.startsWith("workers") || w.startsWith("scales"),
  );

  const runs: RunComparison[] = [];
  for (const run of current.runs) {
    const key = `chromium@${run.scale}:${run.scenario}`;
    runs.push(
      compareRun(run.scale, run.scenario, baseline.metrics[key], run.metrics),
    );
  }

  const allMetrics = runs.flatMap((r) => r.metrics);
  const regressions = allMetrics
    .filter((m) => m.status === "regression")
    .sort((a, b) => b.deltaPct - b.allowedPct - (a.deltaPct - a.allowedPct));

  return {
    hasBaseline: true,
    environmentWarnings,
    gatingSkipped,
    totals: {
      compared: allMetrics.filter((m) => m.status !== "missing").length,
      regressions: regressions.length,
      improvements: allMetrics.filter((m) => m.status === "improvement").length,
      missing: allMetrics.filter((m) => m.status === "missing").length,
    },
    runs,
    regressions,
    gatedRegressions: regressions.filter((m) => m.gating),
  };
}

/** Render the regression table used in CI output. */
export function renderRegressionTable(report: ComparisonReport): string {
  const lines: string[] = [];
  if (!report.hasBaseline) {
    lines.push(
      "No baseline found. Create one with: make perf-baseline  (PERF_UPDATE_BASELINE=1)",
    );
    return lines.join("\n");
  }

  if (report.environmentWarnings.length > 0) {
    lines.push("Environment differences vs baseline:");
    for (const w of report.environmentWarnings) {
      lines.push(`  - ${w}`);
    }
    if (report.gatingSkipped) {
      lines.push("  => gating skipped (incomparable environment)");
    }
    lines.push("");
  }

  if (report.regressions.length === 0) {
    lines.push(
      `No regressions. ${report.totals.compared} metrics compared, ` +
        `${report.totals.improvements} improved, ${report.totals.missing} missing.`,
    );
    return lines.join("\n");
  }

  lines.push(`REGRESSIONS (${report.regressions.length}):`);
  for (const r of report.regressions) {
    const tag = r.gating ? "FAIL" : "warn (advisory, not gated)";
    lines.push(
      `  ${r.label} [${r.key}]  ${r.baseline} -> ${r.current}  ` +
        `(${formatSigned(r.deltaPct)}%, limit +${r.allowedPct}%)  ${tag}`,
    );
  }
  return lines.join("\n");
}

function round(n: number): number {
  return Math.round(n * 100) / 100;
}

function formatSigned(n: number): string {
  if (!Number.isFinite(n)) return "n/a";
  return n > 0 ? `+${round(n)}` : `${round(n)}`;
}
