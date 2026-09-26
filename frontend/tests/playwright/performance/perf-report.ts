import type { ProbeSnapshot } from "./perf-helpers";
import {
  scopedLongTasks,
  scenarioDuration,
  extractBaselineMetrics,
} from "./perf-extract";
import { cdpDurationSecondsToMs, type CdpDelta } from "./perf-cdp";
import type { FrameTimingStat } from "./perf-frames";
import type { TraceAnalysis, TraceFunction } from "./perf-trace";

export type PerfResultFile = {
  scenario: string;
  browser: string;
  scale: number;
  timestamp: string;
  metrics: Record<string, unknown>;
  /** Number of iterations aggregated into this result. */
  iteration?: number;
};

export type ScenarioTiming = {
  scenario: string;
  ms: number;
};

function domFrom(metrics: Record<string, unknown>) {
  return metrics.dom as
    | {
        listingItemCount?: number;
        documentElementCount?: number;
        bodyDescendantCount?: number;
      }
    | undefined;
}

function probeFrom(metrics: Record<string, unknown>): ProbeSnapshot | undefined {
  return metrics.probe as ProbeSnapshot | undefined;
}

function cdpFrom(metrics: Record<string, unknown>): CdpDelta | null | undefined {
  return metrics.cdp as CdpDelta | null | undefined;
}

function framesFrom(
  metrics: Record<string, unknown>,
): FrameTimingStat | undefined {
  return metrics.frameStats as FrameTimingStat | undefined;
}

/** Absolute gauge readings from the CDP sample. */
function gaugeFrom(metrics: Record<string, unknown>) {
  return cdpFrom(metrics)?.gauge;
}

/** Cumulative-counter deltas across the scenario window. */
function cdpDeltaFrom(metrics: Record<string, unknown>) {
  return cdpFrom(metrics)?.delta;
}

const TRACE_SYMBOL_SOURCES: Record<
  string,
  { file: string; line?: number; symbol?: string }
> = {
  updateScrollableContent: {
    file: "frontend/src/components/files/Scrollbar.vue",
    line: 219,
    symbol: "updateScrollableContent",
  },
};

function cdpAttributedMs(delta: Record<string, number>): {
  scriptMs: number;
  layoutMs: number;
  styleMs: number;
  totalMs: number;
} {
  const scriptMs = cdpDurationSecondsToMs(delta.ScriptDuration ?? 0);
  const layoutMs = cdpDurationSecondsToMs(delta.LayoutDuration ?? 0);
  const styleMs = cdpDurationSecondsToMs(delta.RecalcStyleDuration ?? 0);
  return {
    scriptMs,
    layoutMs,
    styleMs,
    totalMs: scriptMs + layoutMs + styleMs,
  };
}

function traceContributors(
  trace: TraceAnalysis,
  scenarioMs: number,
): Omit<PerfContributor, "rank">[] {
  const out: Omit<PerfContributor, "rank">[] = [];
  const minSelfMs = Math.max(50, scenarioMs * 0.02);

  for (const fn of trace.topFunctions) {
    if (!fn.category.startsWith("app") || fn.selfMs < minSelfMs) continue;
    if (fn.name === "(program)" || fn.name === "(idle)") continue;

    const source = TRACE_SYMBOL_SOURCES[fn.name];
    out.push({
      factor: `Chrome trace: ${fn.name}`,
      detail:
        `Self time ${fn.selfMs} ms in captured trace` +
        (fn.location ? ` (${fn.location})` : "") +
        ".",
      source,
      severity: fn.selfMs > scenarioMs * 0.15 ? "high" : "medium",
      contributionMs: Math.round(fn.selfMs),
      evidence: {
        traceSelfMs: fn.selfMs,
        traceTotalMs: fn.totalMs,
        traceLocation: fn.location,
      },
    });
    if (out.length >= 3) break;
  }

  if (trace.gcMs >= 100) {
    out.push({
      factor: "Garbage collection (trace)",
      detail: `${trace.gcEvents} GC event(s) totalling ${trace.gcMs} ms in the chromium trace.`,
      severity: trace.gcMs > scenarioMs * 0.1 ? "high" : "medium",
      contributionMs: Math.round(trace.gcMs),
      evidence: {
        gcEvents: trace.gcEvents,
        gcMs: trace.gcMs,
      },
    });
  }

  if (trace.forcedLayoutMs >= 50) {
    out.push({
      factor: "Forced synchronous layout (trace)",
      detail:
        `${trace.forcedLayoutEvents} forced layout(s), ${trace.forcedLayoutMs} ms — possible layout thrashing.`,
      severity: trace.forcedLayoutMs > scenarioMs * 0.1 ? "high" : "medium",
      contributionMs: Math.round(trace.forcedLayoutMs),
      evidence: {
        forcedLayoutEvents: trace.forcedLayoutEvents,
        forcedLayoutMs: trace.forcedLayoutMs,
      },
    });
  }

  return out;
}

function hottestAppTraceFunction(
  trace: TraceAnalysis | undefined,
): TraceFunction | undefined {
  if (!trace) return undefined;
  return trace.topFunctions.find(
    (fn) =>
      fn.category.startsWith("app") &&
      fn.name !== "(program)" &&
      fn.name !== "(idle)" &&
      fn.selfMs > 0,
  );
}

export type PerfContributor = {
  rank: number;
  factor: string;
  detail: string;
  severity: "high" | "medium" | "low";
  /** Estimated share of the scenario duration attributable to this factor. */
  contributionMs?: number;
  /** Where in the source this originates, when known. */
  source?: { file: string; line?: number; symbol?: string };
  evidence: Record<string, number | string | boolean | null>;
};

export type RunProfiling = {
  interactionMs: number;
  dom: ReturnType<typeof domFrom>;
  probe: ProbeSnapshot | undefined;
  cdp: CdpDelta | null | undefined;
  /** Long tasks inside the scenario window only. */
  topLongTasks: { duration: number; startTime: number }[];
  frames?: FrameTimingStat;
  chromeTracePath: string | null;
  listenerDelta?: number;
};

export type AnalysisTableRow = {
  browser: string;
  scale: number;
  scenario: string;
  metric: string;
  /** Typed so consumers can compare numerically. */
  value: number | string | boolean | null;
};

export function buildRunProfiling(result: PerfResultFile): RunProfiling {
  const { scenario, metrics } = result;
  const probe = probeFrom(metrics);
  const scoped = scopedLongTasks(metrics);
  const longTasks = [...scoped].sort((a, b) => b.duration - a.duration);
  const trace = metrics.chromeTracePath;
  return {
    interactionMs: scenarioDuration(scenario, metrics),
    dom: domFrom(metrics),
    probe,
    cdp: cdpFrom(metrics),
    topLongTasks: longTasks.slice(0, 10),
    frames: framesFrom(metrics),
    chromeTracePath: typeof trace === "string" ? trace : null,
    listenerDelta:
      metrics.listenerDelta !== undefined
        ? Number(metrics.listenerDelta)
        : undefined,
  };
}

/**
 * Ranked contributors to the scenario's cost.
 *
 * Each contributor now carries an estimated millisecond contribution where one
 * can be derived, so the report ranks by magnitude rather than severity label
 * alone. `source` points at the responsible code where the harness can prove it.
 */
export function buildContributors(
  result: PerfResultFile,
  traceAnalysis?: TraceAnalysis,
): PerfContributor[] {
  const { scenario, metrics } = result;
  const ms = scenarioDuration(scenario, metrics);
  const dom = domFrom(metrics);
  const probe = probeFrom(metrics);
  const gauge = gaugeFrom(metrics);
  const delta = cdpDeltaFrom(metrics);
  const frames = framesFrom(metrics);
  const listingItems = dom?.listingItemCount ?? 0;
  const listeners = probe?.addListenerCalls ?? gauge?.JSEventListeners ?? 0;
  const io = probe?.intersectionObservers ?? 0;
  const scopedTasks = scopedLongTasks(metrics);
  const longMs = scopedTasks.reduce((s, t) => s + t.duration, 0);
  const out: Omit<PerfContributor, "rank">[] = [];

  // --- One IntersectionObserver per row (known ListingItem.vue behaviour) --
  if (listingItems > 0 && io > 0 && Math.abs(io - listingItems) / listingItems < 0.15) {
    out.push({
      factor: "IntersectionObserver per listing row",
      detail:
        "ListingItem.vue constructs one IntersectionObserver per row in mounted(); " +
        "cost grows linearly with row count and observer callbacks fire on every scroll frame.",
      source: { file: "frontend/src/components/files/ListingItem.vue", line: 373, symbol: "mounted()" },
      severity: listingItems >= 5000 ? "high" : listingItems >= 500 ? "medium" : "low",
      contributionMs: Math.round(io * 0.05),
      evidence: {
        listingItemCount: listingItems,
        intersectionObservers: io,
        perRow: Number((io / listingItems).toFixed(3)),
      },
    });
  }

  // --- Listener volume ----------------------------------------------------
  if (listingItems > 0 && listeners / listingItems > 5) {
    out.push({
      factor: "High addEventListener volume",
      detail:
        "Listener registrations are high relative to mounted rows; check for per-row handlers that could be delegated.",
      severity: listeners / listingItems > 15 ? "high" : "medium",
      evidence: {
        addListenerCalls: listeners,
        perRow: Number((listeners / listingItems).toFixed(2)),
      },
    });
  }

  // --- Main-thread long tasks (scenario-scoped) ---------------------------
  if (scopedTasks.length > 0 && longMs > Math.max(ms * 0.2, 50)) {
    const top = [...scopedTasks].sort((a, b) => b.duration - a.duration)[0];
    out.push({
      factor: "Main-thread long tasks",
      detail:
        `PerformanceObserver recorded ${scopedTasks.length} long task(s) inside this ` +
        `scenario totalling ${Math.round(longMs)} ms; largest single task ` +
        `${Math.round(top?.duration ?? 0)} ms.`,
      severity: longMs > ms * 0.5 ? "high" : "medium",
      contributionMs: Math.round(longMs),
      evidence: {
        longTaskCount: scopedTasks.length,
        longTaskMsTotal: Math.round(longMs),
        largestLongTaskMs: Math.round(top?.duration ?? 0),
        shareOfScenarioPct: ms > 0 ? Math.round((longMs / ms) * 100) : null,
      },
    });
  }

  // --- CDP attribution (chromium) ----------------------------------------
  if (delta) {
    const { scriptMs, layoutMs, styleMs, totalMs } = cdpAttributedMs(delta);
    if (totalMs > 0) {
      const cdpWindowMs = cdpFrom(metrics)?.windowMs;
      out.push({
        factor: "Renderer main-thread attribution",
        detail:
          `Script ${scriptMs} ms, layout ${layoutMs} ms, ` +
          `style recalc ${styleMs} ms across the scenario window` +
          (cdpWindowMs !== undefined ? ` (${cdpWindowMs} ms wall)` : "") +
          ` (${delta.LayoutCount ?? 0} layouts).`,
        severity: totalMs > ms * 0.5 ? "high" : "medium",
        contributionMs: totalMs,
        evidence: {
          scriptDurationMs: scriptMs,
          layoutDurationMs: layoutMs,
          recalcStyleDurationMs: styleMs,
          layoutCount: delta.LayoutCount ?? 0,
          recalcStyleCount: delta.RecalcStyleCount ?? 0,
          cdpWindowMs: cdpWindowMs ?? null,
        },
      });
    }
  }

  if (traceAnalysis) {
    out.push(...traceContributors(traceAnalysis, ms));
  }

  // --- Frame health -------------------------------------------------------
  if (frames && frames.frames > 0) {
    if (frames.p95 > 32) {
      out.push({
        factor: "Dropped frames / janky rendering",
        detail:
          `Frame time p95 ${frames.p95} ms, p99 ${frames.p99} ms across ` +
          `${frames.frames} frames (source: ${frames.source}); ` +
          `${frames.droppedFrames} frame(s) exceeded 1.5x the median.`,
        severity: frames.p95 > 100 ? "high" : "medium",
        evidence: {
          frameP50: frames.p50,
          frameP95: frames.p95,
          frameP99: frames.p99,
          droppedFrames: frames.droppedFrames,
          effectiveFps: frames.effectiveFps,
          samplingSource: frames.source,
        },
      });
    }
  } else if (scenario === "scroll" || scenario === "resize") {
    out.push({
      factor: "Frame timing unavailable",
      detail:
        "No frame samples were captured for this scenario, so frame health could " +
        "not be assessed. Treat timing comparisons for this run as unreliable.",
      severity: "low",
      evidence: { scenario },
    });
  }

  // --- Layout tree size ---------------------------------------------------
  if (scenario === "resize") {
    const docNodes = dom?.documentElementCount ?? gauge?.Nodes ?? 0;
    if (docNodes > 5000) {
      out.push({
        factor: "Large layout tree",
        detail: `Resize cycles force layout across ~${Math.round(docNodes)} document nodes.`,
        severity: docNodes > 15000 ? "high" : "medium",
        evidence: { documentElementCount: Math.round(docNodes), resizeDurationMs: ms },
      });
    }
  }

  // --- Selection reactivity ----------------------------------------------
  if (scenario === "select") {
    const selected = Number(metrics.selectedCount ?? 0);
    out.push({
      factor: "Multi-select reactivity",
      detail: `${selected} Ctrl/Meta clicks update selection state and status bar across mounted rows.`,
      severity: ms > 5000 ? "high" : ms > 1500 ? "medium" : "low",
      evidence: { selectedCount: selected, selectMs: ms },
    });
  }

  // --- Memory -------------------------------------------------------------
  if (gauge?.JSHeapUsedSize !== undefined) {
    const heapMb = gauge.JSHeapUsedSize / (1024 * 1024);
    out.push({
      factor: "JS heap footprint",
      detail: `Heap in use after scenario: ${heapMb.toFixed(1)} MB.`,
      severity: heapMb > 400 ? "high" : heapMb > 150 ? "medium" : "low",
      evidence: {
        jsHeapUsedMb: Math.round(heapMb * 10) / 10,
        nodes: gauge.Nodes !== undefined ? Math.round(gauge.Nodes) : null,
        listeners: gauge.JSEventListeners !== undefined ? Math.round(gauge.JSEventListeners) : null,
      },
    });
  }

  if (typeof metrics.chromeTracePath === "string") {
    out.push({
      factor: "Chrome trace captured",
      detail:
        "A chromium trace was written; see traceAnalyses in the report for hot functions and forced layouts.",
      severity: "low",
      evidence: { chromeTracePath: metrics.chromeTracePath },
    });
  }

  const severityOrder = { high: 0, medium: 1, low: 2 };
  out.sort(
    (a, b) =>
      (b.contributionMs ?? 0) - (a.contributionMs ?? 0) ||
      severityOrder[a.severity] - severityOrder[b.severity],
  );

  return out.map((c, i) => ({ rank: i + 1, ...c }));
}

export function buildAnalysisTable(parsed: PerfResultFile[]): AnalysisTableRow[] {
  const rows: AnalysisTableRow[] = [];
  for (const raw of parsed) {
    const m = raw.metrics;
    const { browser, scenario, scale } = raw;

    // Typed scalar metrics from the registry-normalized bag.
    const normalized = extractBaselineMetrics(raw) as unknown as Record<string, number>;
    for (const [metric, value] of Object.entries(normalized)) {
      rows.push({ browser, scale, scenario, metric, value });
    }

    const dom = domFrom(m);
    if (dom) {
      for (const [k, v] of Object.entries(dom)) {
        rows.push({
          browser,
          scale,
          scenario,
          metric: `dom.${k}`,
          value: typeof v === "number" ? v : String(v),
        });
      }
    }

    const frames = framesFrom(m);
    if (frames) {
      const frameMetrics: Record<string, number | string> = {
        "frames.frames": frames.frames,
        "frames.p50": frames.p50,
        "frames.p95": frames.p95,
        "frames.p99": frames.p99,
        "frames.droppedFrames": frames.droppedFrames,
        "frames.effectiveFps": frames.effectiveFps,
        "frames.source": frames.source,
      };
      for (const [k, v] of Object.entries(frameMetrics)) {
        rows.push({ browser, scale, scenario, metric: k, value: v });
      }
    }
  }
  return rows;
}

export function buildRootCauseParagraph(
  result: PerfResultFile,
  traceAnalysis?: TraceAnalysis,
): string {
  const { scenario, browser, scale, metrics } = result;
  const ms = scenarioDuration(scenario, metrics);
  const dom = domFrom(metrics);
  const probe = probeFrom(metrics);
  const gauge = gaugeFrom(metrics);
  const cdp = cdpFrom(metrics);
  const delta = cdp?.delta;
  const frames = framesFrom(metrics);
  const totalItems = (result.scale ?? 0) * 2;
  const listingItems = dom?.listingItemCount ?? 0;
  const docNodes = dom?.documentElementCount ?? gauge?.Nodes ?? 0;
  const listeners = probe?.addListenerCalls ?? gauge?.JSEventListeners ?? 0;
  const io = probe?.intersectionObservers ?? 0;
  const scopedTasks = scopedLongTasks(metrics);
  const longMs = scopedTasks.reduce((s, t) => s + t.duration, 0);
  const perItemListeners =
    listingItems > 0 ? (listeners / listingItems).toFixed(2) : "n/a";
  const perItemIo = listingItems > 0 ? (io / listingItems).toFixed(2) : "n/a";

  const lines: string[] = [
    `**${browser} / scale ${scale} / ${scenario}** (${Math.round(ms)} ms):`,
  ];

  if (scenario === "load") {
    lines.push(
      `- Listing mounts ~${listingItems} \`.listing-item\` rows (${totalItems} mock folders+files). Each row creates an IntersectionObserver in ListingItem.vue → ~${io} observers (${perItemIo} per row).`,
    );
    lines.push(
      `- Probed \`addEventListener\` calls: ${listeners} (~${perItemListeners} per row).`,
    );
  } else if (scenario === "scroll") {
    const frameText = frames
      ? `frame p95 ${frames.p95} ms, p99 ${frames.p99} ms, ${frames.droppedFrames} dropped (${frames.source})`
      : "frame timing unavailable";
    lines.push(
      `- Scroll stress on ${listingItems} DOM rows; ${frameText}. Listener delta during scroll: ${metrics.listenerDelta ?? 0}.`,
    );
  } else if (scenario === "resize") {
    const frameText = frames
      ? `frame p95 ${frames.p95} ms, ${frames.droppedFrames} dropped`
      : "frame timing unavailable";
    lines.push(
      `- Resize forces layout across ~${docNodes} document nodes and ${listingItems} listing rows; ${frameText}.`,
    );
  } else if (scenario === "select") {
    lines.push(
      `- ${metrics.selectedCount ?? 0} Ctrl-clicks update selection state across ${listingItems} mounted rows (Vue reactivity + status bar).`,
    );
  }

  lines.push(
    `- Document nodes: ~${Math.round(docNodes)}; JS listeners: ${Math.round(listeners)}; ` +
      `long tasks in scenario: ${scopedTasks.length} (${Math.round(longMs)} ms).`,
  );

  if (delta && cdp) {
    const { scriptMs, layoutMs, styleMs } = cdpAttributedMs(delta);
    lines.push(
      `- CDP window (${cdp.windowMs} ms): script ${scriptMs} ms, ` +
        `layout ${layoutMs} ms, style ${styleMs} ms, ` +
        `${delta.LayoutCount ?? 0} layouts.`,
    );
  }

  const hotApp = hottestAppTraceFunction(traceAnalysis);
  if (longMs > ms * 0.3 && scopedTasks.length > 0) {
    lines.push(
      `- **Likely dominant cost:** main-thread long tasks (${Math.round(longMs)} ms) during this scenario.`,
    );
  } else if (hotApp && hotApp.selfMs >= Math.max(50, ms * 0.05)) {
    const hint = TRACE_SYMBOL_SOURCES[hotApp.name];
    const where = hint
      ? ` (${hint.file}${hint.line ? `:${hint.line}` : ""})`
      : hotApp.location
        ? ` (${hotApp.location})`
        : "";
    lines.push(
      `- **Likely dominant cost:** \`${hotApp.name}\` — ${hotApp.selfMs} ms self in chromium trace${where}.`,
    );
  } else if (io > 0 && listingItems > 0 && Math.abs(io - listingItems) / listingItems < 0.15) {
    lines.push(
      `- **Likely dominant cost:** one IntersectionObserver per listing item scales linearly with row count.`,
    );
  } else if (listeners > listingItems * 5) {
    lines.push(
      `- **Likely dominant cost:** high listener registration rate relative to row count (check duplicate bindings).`,
    );
  }

  return lines.join("\n");
}

export function findSlowestInteraction(
  results: PerfResultFile[],
  browser: string,
  scale: number,
): ScenarioTiming | null {
  const subset = results.filter((r) => r.browser === browser && r.scale === scale);
  let best: ScenarioTiming | null = null;
  for (const r of subset) {
    const ms = scenarioDuration(r.scenario, r.metrics);
    if (!best || ms > best.ms) {
      best = { scenario: r.scenario, ms };
    }
  }
  return best;
}

export function renderAsciiBarChart(
  label: string,
  scales: number[],
  values: number[],
): string {
  const max = Math.max(...values, 1);
  const width = 40;
  const lines = [`### ${label}`, ""];
  for (let i = 0; i < scales.length; i++) {
    const v = values[i] ?? 0;
    const barLen = Math.round((v / max) * width);
    const bar = "█".repeat(barLen) + "░".repeat(width - barLen);
    lines.push(`${String(scales[i]).padStart(5)} |${bar}| ${v} ms`);
  }
  return lines.join("\n");
}
