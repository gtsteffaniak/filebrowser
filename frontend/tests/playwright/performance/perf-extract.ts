import type { FrameTimingStat } from "./perf-frames";
import type { InteractionTiming, WebVitals } from "./perf-vitals";
import { cdpDurationSecondsToMs, type CdpDelta } from "./perf-cdp";
import type { LongTask, ProbeSnapshot } from "./perf-helpers";
import { findMetric, metricsForScenario } from "./perf-metrics";

/**
 * Projects a raw run result onto the flat metric keys declared in the registry.
 *
 * Keeping this in one place means the baseline writer, the comparison engine
 * and the report all agree on what a metric key means, and adding a metric is a
 * single-line change here plus a registry entry.
 */

export type RunMetricBag = {
  scenarioMs: number;
  domNodes: number;
  listingItems: number;
  addListenerCalls: number;
  intersectionObservers: number;
  jSEventListeners: number;
  jSHeapUsedSize: number;
  longTaskMs: number;
  longTaskCount: number;
  layoutDurationDelta: number;
  recalcStyleDurationDelta: number;
  scriptDurationDelta: number;
  layoutCountDelta: number;
  frameP95: number;
  frameP99: number;
  droppedFrames: number;
  effectiveFps: number;
  interactionP95: number;
};

type RawRun = {
  scenario: string;
  metrics: Record<string, unknown>;
};

function num(v: unknown, fallback = 0): number {
  const n = typeof v === "number" ? v : Number(v);
  return Number.isFinite(n) ? n : fallback;
}

export function scenarioDuration(
  scenario: string,
  metrics: Record<string, unknown>,
): number {
  switch (scenario) {
    case "load":
      return num(metrics.loadListingMs);
    case "scroll":
      return num(metrics.scrollDurationMs);
    case "resize":
      return num(metrics.resizeDurationMs);
    case "select":
      return num(metrics.select20Ms);
    default:
      return num(metrics.scenarioMs);
  }
}

/**
 * Long-task totals limited to the scenario window.
 *
 * Falls back to the full list only when window bounds are unavailable, so an
 * older result file still yields a number instead of silently reporting zero.
 */
export function scopedLongTasks(
  metrics: Record<string, unknown>,
): LongTask[] {
  const probe = metrics.probe as ProbeSnapshot | undefined;
  const tasks = probe?.longTasks ?? [];
  const startedAt = metrics.windowStart as number | undefined;
  const endedAt = metrics.windowEnd as number | undefined;
  if (typeof startedAt !== "number" || typeof endedAt !== "number") {
    return tasks;
  }
  return tasks.filter((t) => t.startTime >= startedAt && t.startTime <= endedAt);
}

export function extractBaselineMetrics(run: RawRun): Record<string, number> {
  const { scenario, metrics } = run;
  const probe = metrics.probe as ProbeSnapshot | undefined;
  const dom = metrics.dom as
    | { listingItemCount?: number; documentElementCount?: number }
    | undefined;
  const cdp = metrics.cdp as CdpDelta | null | undefined;
  const frames = metrics.frameStats as FrameTimingStat | undefined;
  const interaction = metrics.interaction as InteractionTiming | undefined;
  const scoped = scopedLongTasks(metrics);

  const flatCdp = cdp?.delta ?? {};
  const gauge = cdp?.gauge ?? {};

  const full: Record<string, number> = {
    scenarioMs: scenarioDuration(scenario, metrics),
    domNodes: num(dom?.documentElementCount),
    listingItems: num(dom?.listingItemCount),
    addListenerCalls: num(probe?.addListenerCalls),
    intersectionObservers: num(probe?.intersectionObservers),
    jSEventListeners: num(gauge.JSEventListeners ?? metrics.jSEventListeners),
    jSHeapUsedSize: num(gauge.JSHeapUsedSize),
    longTaskMs: Math.round(scoped.reduce((s, t) => s + t.duration, 0)),
    longTaskCount: scoped.length,
    layoutDurationDelta: cdpDurationSecondsToMs(num(flatCdp.LayoutDuration)),
    recalcStyleDurationDelta: cdpDurationSecondsToMs(num(flatCdp.RecalcStyleDuration)),
    scriptDurationDelta: cdpDurationSecondsToMs(num(flatCdp.ScriptDuration)),
    layoutCountDelta: num(flatCdp.LayoutCount),
    frameP95: round(num(frames?.p95)),
    frameP99: round(num(frames?.p99)),
    droppedFrames: num(frames?.droppedFrames),
    effectiveFps: round(num(frames?.effectiveFps)),
    interactionP95: round(num(interaction?.p95)),
  };

  const out: Record<string, number> = {};
  for (const def of metricsForScenario(scenario)) {
    if (!wasMeasured(def.key, { cdp, frames, interaction })) continue;
    const value = full[def.key];
    if (typeof value !== "number" || !Number.isFinite(value)) continue;
    out[def.key] = value;
  }
  return out;
}

const CDP_KEYS = new Set([
  "layoutDurationDelta",
  "recalcStyleDurationDelta",
  "scriptDurationDelta",
  "layoutCountDelta",
  "jSEventListeners",
  "jSHeapUsedSize",
]);

const FRAME_KEYS = new Set([
  "frameP95",
  "frameP99",
  "droppedFrames",
  "effectiveFps",
]);

function wasMeasured(
  key: string,
  probes: {
    cdp: CdpDelta | null | undefined;
    frames: FrameTimingStat | undefined;
    interaction: InteractionTiming | undefined;
  },
): boolean {
  if (CDP_KEYS.has(key) && !probes.cdp) return false;
  if (FRAME_KEYS.has(key)) {
    const src = probes.frames?.source;
    const windowMs = probes.frames?.windowMs ?? 0;
    if (!src || src === "none" || windowMs <= 0) return false;
  }
  if (key === "interactionP95") {
    return (probes.interaction?.samples ?? 0) > 0;
  }
  return findMetric(key) != null;
}

/** Metrics keyed for the report's `metrics` block on each run. */
export function flattenRunMetrics(run: RawRun): Record<string, number> {
  return extractBaselineMetrics(run);
}

function round(n: number): number {
  return Math.round(n * 100) / 100;
}

/** Unused placeholder to keep WebVitals import meaningful for consumers. */
export type { WebVitals };
