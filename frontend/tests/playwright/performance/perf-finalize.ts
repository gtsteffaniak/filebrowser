import { copyFile, mkdir, writeFile } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import path from "node:path";
import type { Browser } from "@playwright/test";
import { loadPerfConfig, perfConfigPath } from "./perf-config";
import {
  aggregateIterations,
  baselinePath,
  baselineUpdateRequested,
  buildBaselineFromResults,
  perfResultsDir,
  readAllResults,
  type ParsedResult,
} from "./perf-helpers";
import {
  environmentFingerprint,
  loadBaseline,
  writeBaseline,
} from "./perf-baseline";
import { compareAgainstBaseline, renderRegressionTable } from "./perf-compare";
import { buildPerfResultsDocument } from "./perf-results-json";
import { buildAnalysisTable } from "./perf-report";
import { analyzeTrace, type TraceAnalysis } from "./perf-trace";
import { readPlaywrightVersion } from "./perf-baseline";
import { parsePerfScales } from "./perf-scenarios";
import { repoRoot } from "./perf-paths";

const EXPECTED_SCENARIOS = ["load", "scroll", "resize", "select"] as const;
const EXPECTED_BROWSERS = ["chromium", "firefox", "webkit"] as const;

/**
 * Guard against the silent-empty-report failure mode.
 *
 * The harness previously wrote a structurally valid report containing
 * `runs: []` when its filename glob matched nothing, and CI reported success.
 * An empty or partial run set is always a harness bug or an aborted run, never
 * a legitimate result, so it must fail loudly.
 */
export function failIfEmptyRunSet(
  results: ParsedResult[],
  expected: { scales: number[]; browsers: string[] },
): void {
  if (results.length === 0) {
    throw new Error(
      "[perf] No result files were found. The run produced no measurements — " +
        "this is a harness failure, not a passing performance run.",
    );
  }

  const present = new Set(
    results.map((r) => `${r.browser}@${r.scale}:${r.scenario}`),
  );
  const missing: string[] = [];
  for (const browser of expected.browsers) {
    for (const scale of expected.scales) {
      for (const scenario of EXPECTED_SCENARIOS) {
        if (!present.has(`${browser}@${scale}:${scenario}`)) {
          missing.push(`${browser}@${scale}:${scenario}`);
        }
      }
    }
  }

  if (missing.length > 0) {
    throw new Error(
      `[perf] Incomplete result set: ${missing.length} expected run(s) missing:\n  ` +
        missing.slice(0, 20).join("\n  ") +
        (missing.length > 20 ? `\n  ...and ${missing.length - 20} more` : ""),
    );
  }
}

/**
 * Browsers this run is configured to produce.
 *
 * Mirrors the project list in `playwright.performance.config.ts`: CI is
 * chromium-only, and `PERF_BROWSERS` may narrow the set locally.
 */
export function configuredBrowsers(): string[] {
  const raw = process.env.PERF_BROWSERS?.trim();
  if (raw) {
    return raw
      .split(",")
      .map((b) => b.trim().toLowerCase())
      .filter(Boolean);
  }
  if (process.env.CI) return ["chromium"];
  return [...EXPECTED_BROWSERS];
}

function gitInfo(): { sha: string | null; dirty: boolean | null } {  try {
    const sha = execFileSync("git", ["rev-parse", "HEAD"], {
      encoding: "utf8",
      cwd: repoRoot(import.meta.url),
    }).trim();
    const status = execFileSync("git", ["status", "--porcelain"], {
      encoding: "utf8",
      cwd: repoRoot(import.meta.url),
    }).trim();
    return { sha, dirty: status.length > 0 };
  } catch {
    return { sha: null, dirty: null };
  }
}

/** Resolve a browser version string, tolerating an unavailable browser. */
async function resolveBrowserVersion(): Promise<string> {
  return process.env.PERF_BROWSER_VERSION ?? "unknown";
}

/**
 * Finalize: aggregate iterations, build the report, update/compare the
 * baseline, and emit `report.json` + `report.md`.
 */
export async function writeFinalArtifacts(opts: {
  browserVersion?: string;
  workers?: number;
}): Promise<void> {
  const PERF_DIR = perfResultsDir();
  await mkdir(PERF_DIR, { recursive: true });

  const raw = await readAllResults();
  const scales = parsePerfScales();

  // Expected browser set mirrors the Playwright config: CI runs chromium only,
  // and `PERF_BROWSERS` narrows the set locally too. Deriving this from the
  // configured set (rather than assuming all three) keeps the completeness
  // guard honest without producing false failures for intentional subsets.
  const expectedBrowsers = configuredBrowsers();

  failIfEmptyRunSet(raw, { scales, browsers: expectedBrowsers });

  const aggregated = aggregateIterations(raw);
  const config = await loadPerfConfig();
  const git = gitInfo();
  const playwrightVersion = await readPlaywrightVersion();
  const workers = opts.workers ?? Number(process.env.PERF_WORKERS ?? "1");
  const browserVersion = opts.browserVersion ?? (await resolveBrowserVersion());

  const environment = environmentFingerprint({
    scales,
    workers,
    browserVersion,
    playwrightVersion,
  });

  // ---- Baseline update (explicit opt-in) ---------------------------------
  let baselineUpdated = false;
  if (baselineUpdateRequested()) {
    if (!git.sha) {
      throw new Error(
        "[perf] refusing to write a baseline without a git revision; " +
          "run from a git checkout so the artifact records its source state",
      );
    }
    const baseline = await buildBaselineFromResults(aggregated, {
      scales,
      workers,
      browserVersion,
      gitSha: git.sha,
      gitDirty: git.dirty,
    });
    await writeBaseline(baselinePath(), baseline);
    baselineUpdated = true;
    console.log(`[perf] baseline written to ${baselinePath()}`);
  }

  // ---- Baseline comparison ----------------------------------------------
  const { extractBaselineMetrics } = await import("./perf-extract");
  const currentRuns = aggregated
    .filter((r) => r.browser === "chromium")
    .map((r) => ({
      scale: r.scale,
      scenario: r.scenario,
      metrics: extractBaselineMetrics(r) as unknown as Record<string, number>,
    }));

  const baseline = await loadBaseline(baselinePath());
  const comparison = compareAgainstBaseline(baseline, {
    environment,
    runs: currentRuns,
  });

  // ---- Trace analysis (chromium only) ------------------------------------
  const traceAnalyses: Record<string, TraceAnalysis> = {};
  for (const run of aggregated) {
    const tracePath = (run.metrics as { chromeTracePath?: unknown })
      .chromeTracePath;
    if (typeof tracePath === "string") {
      const analysis = await analyzeTrace(tracePath).catch(() => null);
      if (analysis) {
        traceAnalyses[`${run.browser}@${run.scale}:${run.scenario}`] = analysis;
      }
    }
  }

  // ---- Report ------------------------------------------------------------
  const resultsDoc = buildPerfResultsDocument({
    parsed: aggregated,
    config,
    analysisTable: buildAnalysisTable(aggregated),
    comparison,
    baseline: baseline
      ? {
          createdAt: baseline.createdAt,
          gitSha: baseline.gitSha,
          environment: baseline.environment,
        }
      : null,
    environment,
    traceAnalyses,
    git,
    baselineUpdated,
  });

  const resultsJson = `${JSON.stringify(resultsDoc, null, 2)}\n`;
  await writeFile(path.join(PERF_DIR, "report.json"), resultsJson);
  // Backwards-compatible alias for the existing dashboard.
  await writeFile(path.join(PERF_DIR, "results.json"), resultsJson);
  await copyFile(perfConfigPath(), path.join(PERF_DIR, "perf-config.json"));

  const reportOut =
    process.env.PERF_REPORT_DIR ??
    path.resolve(process.cwd(), "tests/playwright/performance");
  await writeFile(path.join(reportOut, "report.json"), resultsJson);

  const markdown = renderMarkdownReport(resultsDoc);
  await writeFile(path.join(PERF_DIR, "report.md"), markdown);
  await writeFile(path.join(reportOut, "report.md"), markdown);

  // ---- CI summary + enforcement ------------------------------------------
  const table = renderRegressionTable(comparison);
  console.log("\n=== performance vs baseline ===");
  console.log(table);
  console.log(`[perf] report: ${path.join(PERF_DIR, "report.json")}`);

  const gated = comparison.gatedRegressions;

  if (comparison.regressions.length > gated.length) {
    const advisory = comparison.regressions.length - gated.length;
    console.warn(
      `[perf] ${advisory} advisory (non-gating) regression(s) reported; ` +
        "these do not fail the build because the metric is too noisy to gate.",
    );
  }

  if (gated.length > 0 && !comparison.gatingSkipped) {
    const enforce = process.env.PERF_ENFORCE_BASELINE
      ? process.env.PERF_ENFORCE_BASELINE === "1"
      : !!process.env.CI;
    if (enforce && !baselineUpdated) {
      throw new Error(
        `[perf] ${gated.length} gated regression(s) vs baseline:\n${table}`,
      );
    }
    if (!enforce) {
      console.warn(
        "[perf] gated regressions detected but not enforced (local run). " +
          "Set PERF_ENFORCE_BASELINE=1 to enforce.",
      );
    }
  }
}

function renderMarkdownReport(doc: ReturnType<typeof buildPerfResultsDocument>): string {
  const lines: string[] = ["# Listing performance report", ""];
  lines.push(`Generated: ${doc.generatedAt}`);
  if (doc.git.sha) {
    lines.push(
      `Revision: \`${doc.git.sha.slice(0, 12)}\`${doc.git.dirty ? " (dirty tree)" : ""}`,
    );
  } else {
    lines.push("Revision: not captured (git unavailable at measurement time)");
  }
  lines.push(
    `Environment: ${doc.environment.browser} ${doc.environment.browserVersion}, ` +
      `playwright ${doc.environment.playwrightVersion}, ${doc.environment.cpuCount} CPUs, ` +
      `workers=${doc.environment.workers}, scales=[${doc.environment.scales.join(", ")}]`,
  );
  lines.push("");

  lines.push("## Headline findings");
  lines.push("");
  if (doc.headlines.length === 0) {
    lines.push("- No cross-browser findings recorded.");
  }
  for (const h of doc.headlines) {
    lines.push(`- **${h.severity}** ${h.summary}`);
  }
  lines.push("");

  lines.push("## Baseline comparison");
  lines.push("");
  if (!doc.comparison.hasBaseline) {
    lines.push(
      "No baseline present. Create one with `make perf-baseline` (chromium, in CI).",
    );
  } else {
    lines.push(
      `${doc.comparison.totals.compared} metrics compared · ` +
        `${doc.comparison.totals.regressions} regressions · ` +
        `${doc.comparison.totals.improvements} improvements · ` +
        `${doc.comparison.totals.missing} missing`,
    );
    if (doc.comparison.environmentWarnings.length > 0) {
      lines.push("");
      lines.push("Environment differences vs baseline:");
      for (const w of doc.comparison.environmentWarnings) {
        lines.push(`- ${w}`);
      }
    }
    const scopedRegressions = doc.comparison.runs.flatMap((run) =>
      run.metrics
        .filter((metric) => metric.status === "regression")
        .map((metric) => ({
          scale: run.scale,
          scenario: run.scenario,
          metric,
        })),
    );
    if (scopedRegressions.length > 0) {
      lines.push("");
      lines.push(
        "| Browser | Scale | Scenario | Metric | Baseline | Current | Δ% | Limit |",
      );
      lines.push("| --- | ---: | --- | --- | ---: | ---: | ---: | ---: |");
      for (const { scale, scenario, metric } of scopedRegressions) {
        lines.push(
          `| ${doc.environment.browser} | ${scale} | ${scenario} | ` +
            `${metric.label} (${metric.key}) | ${metric.baseline} | ${metric.current} | ` +
            `${metric.deltaPct > 0 ? "+" : ""}${metric.deltaPct}% | +${metric.allowedPct}% |`,
        );
      }
    }
  }
  lines.push("");

  lines.push("## Scenario timings by scale");
  lines.push("");
  lines.push("| Browser | Scale | Load | Scroll | Resize | Select |");
  lines.push("| --- | ---: | ---: | ---: | ---: | ---: |");
  for (const browser of doc.browsers) {
    for (const scale of doc.scales) {
      const at = (scenario: string) =>
        doc.runs.find(
          (r) =>
            r.browser === browser &&
            r.scale === scale &&
            r.scenario === scenario,
        );
      const ms = (scenario: string) => {
        const run = at(scenario);
        return run ? `${Math.round(run.durationMs)} ms` : "—";
      };
      lines.push(
        `| ${browser} | ${scale} | ${ms("load")} | ${ms("scroll")} | ${ms("resize")} | ${ms("select")} |`,
      );
    }
  }
  lines.push("");

  const withTrace = Object.entries(doc.traceAnalyses);
  if (withTrace.length > 0) {
    lines.push("## Trace analysis (chromium)");
    lines.push("");
    for (const [key, analysis] of withTrace) {
      lines.push(`### ${key}`);
      lines.push("");
      lines.push(
        `Total trace ${Math.round(analysis.totalMs)} ms · ` +
          `${analysis.eventCount} events · ${analysis.topFunctions.length} hot functions`,
      );
      lines.push("");
      if (analysis.topFunctions.length > 0) {
        lines.push("| Function | Self ms | Total ms | Category |");
        lines.push("| --- | ---: | ---: | --- |");
        for (const fn of analysis.topFunctions.slice(0, 10)) {
          lines.push(
            `| \`${fn.name}\` | ${fn.selfMs} | ${fn.totalMs} | ${fn.category} |`,
          );
        }
        lines.push("");
      }
      if (analysis.forcedLayoutEvents > 0) {
        lines.push(
          `Forced-reflow/layout events: **${analysis.forcedLayoutEvents}**`,
        );
        lines.push("");
      }
    }
  }

  return `${lines.join("\n")}\n`;
}

export type { Browser };
