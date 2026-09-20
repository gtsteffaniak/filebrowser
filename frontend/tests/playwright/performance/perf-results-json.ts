import type { PerfConfig } from "./perf-config";
import { checkResultThresholds, runtimeEnvSnapshot } from "./perf-config";
import {
  buildContributors,
  buildRootCauseParagraph,
  buildRunProfiling,
  type AnalysisTableRow,
  type PerfContributor,
  type PerfResultFile,
} from "./perf-report";
import type { ComparisonReport } from "./perf-compare";
import type { EnvironmentFingerprint } from "./perf-baseline";
import type { TraceAnalysis } from "./perf-trace";
import { extractBaselineMetrics, scopedLongTasks } from "./perf-extract";
import { SCENARIOS } from "./perf-metrics";

export type HeadlineFinding = {
  severity: "critical" | "high" | "medium" | "info";
  category: "cross-browser" | "scaling" | "regression" | "structure" | "trace";
  summary: string;
  evidence: Record<string, number | string | null>;
};

export type PerfResultsDocument = {
  schemaVersion: number;
  generatedAt: string;
  /** Reproducibility header. */
  git: { sha: string | null; dirty: boolean | null };
  environment: EnvironmentFingerprint;
  config: PerfConfig;
  runtimeEnv: Record<string, string>;
  baselineInfo: {
    updated: boolean;
    path: string;
    createdAt: string | null;
    gitSha: string | null;
    environment: EnvironmentFingerprint | null;
  };
  browsers: string[];
  /** Browsers whose results can feed a CI baseline (chromium only). */
  baselineEligibleBrowsers: string[];
  scales: number[];

  /** Ranked, cross-browser findings — the top of the report. */
  headlines: HeadlineFinding[];

  runs: {
    browser: string;
    scale: number;
    scenario: string;
    timestamp: string;
    iterations: number;
    durationMs: number;
    baselineEligible: boolean;
    metrics: Record<string, unknown>;
    normalized: Record<string, number>;
    thresholds: { passed: boolean; warnings: string[] };
    rootCause: string;
    rootCauseLines: string[];
    contributors: PerfContributor[];
    profiling: ReturnType<typeof buildRunProfiling>;
  }[];

  /** Per (browser, scale): the slowest scenario and its explanation. */
  summary: {
    browser: string;
    scale: number;
    slowestScenario: string;
    slowestMs: number;
    rootCause: string;
    contributors: PerfContributor[];
  }[];

  analysisTable: AnalysisTableRow[];
  comparison: ComparisonReport;
  traceAnalyses: Record<string, TraceAnalysis>;
  charts: {
    interactionMsByScale: Record<string, Record<string, number[]>>;
    domAtLoad: Record<
      string,
      {
        scales: number[];
        listingItems: number[];
        listeners: number[];
        intersectionObservers: number[];
      }
    >;
    /** Scenario duration vs scale, plus a superlinearity verdict. */
    scaling: {
      browser: string;
      scenario: string;
      scales: number[];
      durations: number[];
      /** Ratio of the final step's growth to a linear expectation. */
      superlinearity: number | null;
    }[];
  };
};

export const REPORT_SCHEMA_VERSION = 2;

function durationMs(metrics: Record<string, unknown>, scenario: string): number {
  switch (scenario) {
    case "load":
      return Number(metrics.loadListingMs) || 0;
    case "scroll":
      return Number(metrics.scrollDurationMs) || 0;
    case "resize":
      return Number(metrics.resizeDurationMs) || 0;
    case "select":
      return Number(metrics.select20Ms) || 0;
    default:
      return Number(metrics.scenarioMs) || 0;
  }
}

/**
 * Detect superlinear scaling.
 *
 * If a scenario's duration grows faster than the data (e.g. 10x rows → >10x
 * time), the ratio exceeds 1 and the implementation has a scaling defect. This
 * turns "it got slower" into "it is O(n^2)", which is the actionable statement.
 */
export function superlinearity(
  scales: number[],
  durations: number[],
): number | null {
  if (scales.length < 2 || durations.length < 2) return null;
  const i = scales.length - 1;
  const scaleRatio = scales[i] / scales[0];
  const timeRatio = durations[i] / Math.max(durations[0], 1);
  if (!Number.isFinite(timeRatio) || scaleRatio <= 0) return null;
  return Math.round((timeRatio / scaleRatio) * 100) / 100;
}

export function buildHeadlines(input: {
  parsed: PerfResultFile[];
  browsers: string[];
  scales: number[];
  comparison: ComparisonReport;
  traceAnalyses: Record<string, TraceAnalysis>;
}): HeadlineFinding[] {
  const { parsed, browsers, scales, comparison, traceAnalyses } = input;
  const findings: HeadlineFinding[] = [];

  const durationOf = (browser: string, scale: number, scenario: string) => {
    const r = parsed.find(
      (x) =>
        x.browser === browser && x.scale === scale && x.scenario === scenario,
    );
    return r ? durationMs(r.metrics, scenario) : null;
  };

  // --- Cross-browser comparison at the largest scale ----------------------
  const maxScale = scales[scales.length - 1];
  for (const scenario of SCENARIOS) {
    const values = browsers
      .map((b) => ({ browser: b, ms: durationOf(b, maxScale, scenario) }))
      .filter((v): v is { browser: string; ms: number } => v.ms !== null && v.ms > 0);
    if (values.length < 2) continue;

    const fastest = values.reduce((a, b) => (b.ms < a.ms ? b : a));
    const slowest = values.reduce((a, b) => (b.ms > a.ms ? b : a));
    const ratio = slowest.ms / Math.max(fastest.ms, 1);

    if (ratio >= 1.5) {
      findings.push({
        severity: ratio >= 3 ? "high" : "medium",
        category: "cross-browser",
        summary:
          `${scenario} at ${maxScale} is ${ratio.toFixed(2)}x slower on ` +
          `${slowest.browser} (${Math.round(slowest.ms)} ms) than ${fastest.browser} ` +
          `(${Math.round(fastest.ms)} ms)`,
        evidence: {
          scenario,
          scale: maxScale,
          slowestBrowser: slowest.browser,
          slowestMs: Math.round(slowest.ms),
          fastestBrowser: fastest.browser,
          fastestMs: Math.round(fastest.ms),
          ratio: Math.round(ratio * 100) / 100,
        },
      });
    }
  }

  // --- Superlinear scaling ------------------------------------------------
  for (const browser of browsers) {
    for (const scenario of SCENARIOS) {
      const durations = scales.map((s) => durationOf(browser, s, scenario) ?? 0);
      if (durations.some((d) => d <= 0)) continue;
      const ratio = superlinearity(scales, durations);
      if (ratio !== null && ratio > 1.5) {
        findings.push({
          severity: ratio > 3 ? "high" : "medium",
          category: "scaling",
          summary:
            `${browser} ${scenario} scales superlinearly: ${scales[0]}→${maxScale} rows ` +
            `(${scales[scales.length - 1] / scales[0]}x) costs ` +
            `${(durations[durations.length - 1] / Math.max(durations[0], 1)).toFixed(1)}x time`,
          evidence: {
            browser,
            scenario,
            scales: scales.join(","),
            durations: durations.map((d) => Math.round(d)).join(","),
            superlinearity: ratio,
          },
        });
      }
    }
  }

  // --- Regressions --------------------------------------------------------
  for (const r of comparison.regressions.slice(0, 10)) {
    findings.push({
      severity: r.deltaPct > r.allowedPct * 2 ? "critical" : "high",
      category: "regression",
      summary:
        `${r.label} regressed ${r.deltaPct > 0 ? "+" : ""}${r.deltaPct}% ` +
        `(baseline ${r.baseline} → ${r.current}, limit +${r.allowedPct}%)`,
      evidence: {
        metric: r.key,
        baseline: r.baseline,
        current: r.current,
        deltaPct: r.deltaPct,
        allowedPct: r.allowedPct,
      },
    });
  }

  // --- Structural: observers and listeners per row ------------------------
  const loadRuns = parsed.filter(
    (r) => r.scenario === "load" && r.scale === maxScale,
  );
  for (const run of loadRuns) {
    const probe = run.metrics.probe as
      | { intersectionObservers?: number; addListenerCalls?: number }
      | undefined;
    const dom = run.metrics.dom as { listingItemCount?: number } | undefined;
    const rows = dom?.listingItemCount ?? 0;
    const io = probe?.intersectionObservers ?? 0;
    if (rows > 0 && io > 0 && Math.abs(io - rows) / rows < 0.15) {
      findings.push({
        severity: rows >= 5000 ? "high" : "medium",
        category: "structure",
        summary:
          `${run.browser}: one IntersectionObserver per listing row ` +
          `(${io} observers for ${rows} rows) — scales linearly with row count`,
        evidence: {
          browser: run.browser,
          rows,
          intersectionObservers: io,
          observersPerRow: Math.round((io / rows) * 1000) / 1000,
        },
      });
    }
  }

  // --- Trace findings -----------------------------------------------------
  for (const [key, analysis] of Object.entries(traceAnalyses)) {
    for (const finding of analysis.findings) {
      findings.push({
        severity: analysis.forcedLayoutEvents > 0 ? "high" : "info",
        category: "trace",
        summary: `${key}: ${finding}`,
        evidence: {
          trace: key,
          forcedLayoutEvents: analysis.forcedLayoutEvents,
          forcedLayoutMs: analysis.forcedLayoutMs,
        },
      });
    }
  }

  const order = { critical: 0, high: 1, medium: 2, info: 3 };
  return findings.sort((a, b) => order[a.severity] - order[b.severity]);
}

export function buildPerfResultsDocument(input: {
  parsed: PerfResultFile[];
  config: PerfConfig;
  analysisTable: AnalysisTableRow[];
  comparison: ComparisonReport;
  baseline: {
    createdAt: string;
    gitSha: string | null;
    environment: EnvironmentFingerprint;
  } | null;
  environment: EnvironmentFingerprint;
  traceAnalyses: Record<string, TraceAnalysis>;
  git: { sha: string | null; dirty: boolean | null };
  baselineUpdated: boolean;
}): PerfResultsDocument {
  const {
    parsed,
    config,
    analysisTable,
    comparison,
    baseline,
    environment,
    traceAnalyses,
    git,
    baselineUpdated,
  } = input;

  const scales = [...new Set(parsed.map((r) => r.scale))].sort((a, b) => a - b);
  const browsers = [...new Set(parsed.map((r) => r.browser))].sort();

  const traceForRun = (r: PerfResultFile) =>
    traceAnalyses[`${r.browser}@${r.scale}:${r.scenario}`];

  const runs = parsed
    .map((r) => {
      const thresholds = checkResultThresholds(r.scenario, r.metrics, config);
      const trace = traceForRun(r);
      const rootCause = buildRootCauseParagraph(r, trace);
      const iteration =
        (r as PerfResultFile & { iteration?: number }).iteration ?? 1;
      return {
        browser: r.browser,
        scale: r.scale,
        scenario: r.scenario,
        timestamp: r.timestamp,
        iterations: iteration,
        durationMs: durationMs(r.metrics, r.scenario),
        baselineEligible: r.browser === "chromium",
        metrics: r.metrics,
        normalized: extractBaselineMetrics(r) as unknown as Record<string, number>,
        thresholds,
        rootCause,
        rootCauseLines: rootCause.split("\n"),
        contributors: buildContributors(r, trace),
        profiling: buildRunProfiling(r),
      };
    })
    .sort(
      (a, b) =>
        a.browser.localeCompare(b.browser) ||
        a.scale - b.scale ||
        SCENARIOS.indexOf(a.scenario as (typeof SCENARIOS)[number]) -
          SCENARIOS.indexOf(b.scenario as (typeof SCENARIOS)[number]),
    );

  const summary: PerfResultsDocument["summary"] = [];
  for (const browser of browsers) {
    for (const scale of scales) {
      const subset = parsed.filter(
        (r) => r.browser === browser && r.scale === scale,
      );
      if (subset.length === 0) continue;
      let slowest = subset[0];
      for (const r of subset) {
        if (durationMs(r.metrics, r.scenario) > durationMs(slowest.metrics, slowest.scenario)) {
          slowest = r;
        }
      }
      summary.push({
        browser,
        scale,
        slowestScenario: slowest.scenario,
        slowestMs: durationMs(slowest.metrics, slowest.scenario),
        rootCause: buildRootCauseParagraph(slowest, traceForRun(slowest)),
        contributors: buildContributors(slowest, traceForRun(slowest)),
      });
    }
  }

  const interactionMsByScale: PerfResultsDocument["charts"]["interactionMsByScale"] =
    {};
  for (const browser of browsers) {
    interactionMsByScale[browser] = {};
    for (const scenario of SCENARIOS) {
      interactionMsByScale[browser][scenario] = scales.map((s) => {
        const r = parsed.find(
          (x) => x.browser === browser && x.scale === s && x.scenario === scenario,
        );
        return r ? durationMs(r.metrics, scenario) : 0;
      });
    }
  }

  const domAtLoad: PerfResultsDocument["charts"]["domAtLoad"] = {};
  for (const browser of browsers) {
    const listingItems: number[] = [];
    const listeners: number[] = [];
    const intersectionObservers: number[] = [];
    for (const s of scales) {
      const r = parsed.find(
        (x) => x.browser === browser && x.scale === s && x.scenario === "load",
      );
      const m = r?.metrics ?? {};
      const dom = m.dom as { listingItemCount?: number } | undefined;
      const probe = m.probe as
        | { addListenerCalls?: number; intersectionObservers?: number }
        | undefined;
      const cdp = m.cdp as { gauge?: { JSEventListeners?: number } } | undefined;
      listingItems.push(dom?.listingItemCount ?? s * 2);
      listeners.push(probe?.addListenerCalls ?? cdp?.gauge?.JSEventListeners ?? 0);
      intersectionObservers.push(probe?.intersectionObservers ?? 0);
    }
    domAtLoad[browser] = {
      scales,
      listingItems,
      listeners,
      intersectionObservers,
    };
  }

  const scaling: PerfResultsDocument["charts"]["scaling"] = [];
  for (const browser of browsers) {
    for (const scenario of SCENARIOS) {
      const durations = scales.map((s) => {
        const r = parsed.find(
          (x) => x.browser === browser && x.scale === s && x.scenario === scenario,
        );
        return r ? durationMs(r.metrics, scenario) : 0;
      });
      scaling.push({
        browser,
        scenario,
        scales,
        durations,
        superlinearity: superlinearity(scales, durations),
      });
    }
  }

  return {
    schemaVersion: REPORT_SCHEMA_VERSION,
    generatedAt: new Date().toISOString(),
    git,
    environment,
    config,
    runtimeEnv: runtimeEnvSnapshot(),
    baselineInfo: {
      updated: baselineUpdated,
      path: "tests/playwright/performance/perf-baseline.json",
      createdAt: baseline?.createdAt ?? null,
      gitSha: baseline?.gitSha ?? null,
      environment: baseline?.environment ?? null,
    },
    browsers,
    baselineEligibleBrowsers: ["chromium"],
    scales,
    headlines: buildHeadlines({ parsed, browsers, scales, comparison, traceAnalyses }),
    runs,
    summary,
    analysisTable,
    comparison,
    traceAnalyses,
    charts: { interactionMsByScale, domAtLoad, scaling },
  };
}

/** Retained for the extractor's long-task windowing. */
export { scopedLongTasks };
