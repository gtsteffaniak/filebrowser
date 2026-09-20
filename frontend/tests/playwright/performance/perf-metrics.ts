/**
 * Central metric registry.
 *
 * Every measured number that can be baselined is declared here exactly once,
 * with the direction and tolerance class that govern how a regression against
 * it is judged. The baseline writer, the comparison engine and the report all
 * read this table, so adding a metric in one place makes it flow everywhere.
 *
 * Tolerance classes:
 *  - `shape`    deterministic structural counts (DOM nodes, listeners). These
 *               are stable run to run, so a small move is a real regression.
 *  - `timing`   wall-clock durations. Inherently noisy on shared CI runners.
 *  - `health`   frame/memory health where lower is better but variance is high.
 */

export type MetricDirection = "lower-is-better" | "higher-is-better";

export type MetricToleranceClass = "shape" | "timing" | "health";

export type MetricDefinition = {
  /** Key path inside a scenario's flat metric bag. */
  key: string;
  label: string;
  unit: "ms" | "count" | "bytes" | "ratio";
  direction: MetricDirection;
  toleranceClass: MetricToleranceClass;
  /** Absolute floor below which a delta is ignored, in the metric's unit. */
  noiseFloor: number;
  /**
   * Whether this metric may fail a build.
   *
   * Metrics whose absolute magnitude is small or highly variable run to run are
   * marked `false`: they are still reported, but a flaky gate is worse than no
   * gate because it teaches people to ignore failures.
   */
  gating: boolean;
  /** Scenarios this metric is expected to appear in ("*" = all). */
  scenarios: string[];
};

export const SCENARIOS = ["load", "scroll", "resize", "select"] as const;
export type ScenarioName = (typeof SCENARIOS)[number];

export const METRIC_REGISTRY: MetricDefinition[] = [
  // ---- Scenario durations -------------------------------------------------
  {
    key: "scenarioMs",
    label: "Scenario duration",
    unit: "ms",
    direction: "lower-is-better",
    toleranceClass: "timing",
    noiseFloor: 50,
    scenarios: [...SCENARIOS],
    gating: true,
  },

  // ---- Structural shape (deterministic → tight tolerance) -----------------
  {
    key: "domNodes",
    label: "Document nodes",
    unit: "count",
    direction: "lower-is-better",
    toleranceClass: "shape",
    noiseFloor: 20,
    scenarios: [...SCENARIOS],
    gating: true,
  },
  {
    key: "listingItems",
    label: "Listing rows in DOM",
    unit: "count",
    direction: "lower-is-better",
    toleranceClass: "shape",
    noiseFloor: 5,
    scenarios: [...SCENARIOS],
    gating: true,
  },
  {
    key: "jSEventListeners",
    label: "JS event listeners",
    unit: "count",
    direction: "lower-is-better",
    toleranceClass: "shape",
    noiseFloor: 50,
    scenarios: [...SCENARIOS],
    gating: true,
  },
  {
    key: "intersectionObservers",
    label: "IntersectionObservers created",
    unit: "count",
    direction: "lower-is-better",
    toleranceClass: "shape",
    noiseFloor: 5,
    scenarios: [...SCENARIOS],
    gating: true,
  },
  {
    key: "addListenerCalls",
    label: "addEventListener calls",
    unit: "count",
    direction: "lower-is-better",
    toleranceClass: "shape",
    noiseFloor: 50,
    scenarios: [...SCENARIOS],
    gating: true,
  },

  // ---- Main-thread cost (scenario-scoped) ---------------------------------
  // Long-task totals are bimodal at N=1: a run either trips the browser's 50ms
  // long-task threshold or it does not, so identical code can report 0 ms one
  // run and several seconds the next. Measured spread inside a single baseline
  // capture was ~38%. Gating on that fires on scheduling luck rather than on
  // code, so both long-task metrics are reported but advisory only. The
  // structural metrics above are the reliable gate.
  {
    key: "longTaskMs",
    label: "Long-task time in scenario",
    unit: "ms",
    direction: "lower-is-better",
    toleranceClass: "health",
    noiseFloor: 500,
    scenarios: [...SCENARIOS],
    gating: false,
  },
  {
    key: "longTaskCount",
    label: "Long tasks in scenario",
    unit: "count",
    direction: "lower-is-better",
    toleranceClass: "health",
    noiseFloor: 3,
    scenarios: [...SCENARIOS],
    gating: false,
  },
  {
    key: "layoutDurationDelta",
    label: "Layout time (CDP)",
    unit: "ms",
    direction: "lower-is-better",
    toleranceClass: "health",
    noiseFloor: 20,
    scenarios: [...SCENARIOS],
    gating: false,
  },
  {
    key: "recalcStyleDurationDelta",
    label: "Style recalc time (CDP)",
    unit: "ms",
    direction: "lower-is-better",
    toleranceClass: "health",
    noiseFloor: 20,
    scenarios: [...SCENARIOS],
    gating: false,
  },
  {
    key: "scriptDurationDelta",
    label: "Script time (CDP)",
    unit: "ms",
    direction: "lower-is-better",
    toleranceClass: "health",
    noiseFloor: 20,
    scenarios: [...SCENARIOS],
    gating: false,
  },
  {
    key: "layoutCountDelta",
    label: "Layout count (CDP)",
    unit: "count",
    direction: "lower-is-better",
    toleranceClass: "shape",
    noiseFloor: 20,
    scenarios: [...SCENARIOS],
    gating: true,
  },

  // ---- Frame health -------------------------------------------------------
  {
    key: "frameP95",
    label: "Frame time p95",
    unit: "ms",
    direction: "lower-is-better",
    toleranceClass: "health",
    // ~one vsync interval: a single extra dropped frame is not a regression.
    // Advisory: measured spread reaches 219% at N=1 because frame sampling
    // depends on what the compositor happened to emit during the window.
    noiseFloor: 20,
    scenarios: ["scroll", "resize"],
    gating: false,
  },
  {
    key: "frameP99",
    label: "Frame time p99",
    unit: "ms",
    direction: "lower-is-better",
    toleranceClass: "health",
    // p99 is the tail by definition; allow a wider absolute swing.
    // Advisory, for the same reason as frameP95.
    noiseFloor: 40,
    scenarios: ["scroll", "resize"],
    gating: false,
  },
  {
    key: "droppedFrames",
    label: "Dropped frames",
    unit: "count",
    direction: "lower-is-better",
    toleranceClass: "health",
    noiseFloor: 2,
    scenarios: ["scroll", "resize"],
    gating: false,
  },
  {
    key: "effectiveFps",
    label: "Effective FPS",
    unit: "ratio",
    direction: "higher-is-better",
    toleranceClass: "health",
    noiseFloor: 2,
    scenarios: ["scroll", "resize"],
    gating: false,
  },

  // ---- Memory -------------------------------------------------------------
  {
    key: "jSHeapUsedSize",
    label: "JS heap used",
    unit: "bytes",
    direction: "lower-is-better",
    toleranceClass: "health",
    noiseFloor: 1_000_000,
    scenarios: [...SCENARIOS],
    gating: true,
  },

  // ---- Interaction latency ------------------------------------------------
  {
    key: "interactionP95",
    label: "Interaction duration p95",
    unit: "ms",
    direction: "lower-is-better",
    toleranceClass: "health",
    noiseFloor: 5,
    scenarios: ["select", "scroll"],
    gating: false,
  },
];

/** Metrics eligible for CI gating (chromium only). */
export function gatingMetrics(): MetricDefinition[] {
  return METRIC_REGISTRY.filter((m) => m.gating);
}

export function metricsForScenario(scenario: string): MetricDefinition[] {
  return METRIC_REGISTRY.filter(
    (m) => m.scenarios.includes("*") || m.scenarios.includes(scenario),
  );
}

export function findMetric(key: string): MetricDefinition | undefined {
  return METRIC_REGISTRY.find((m) => m.key === key);
}
