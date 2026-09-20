import { describe, expect, it } from "vitest";
import {
  extractBaselineMetrics,
  scenarioDuration,
  scopedLongTasks,
} from "./perf-extract";
import { superlinearity } from "./perf-results-json";

describe("scopedLongTasks", () => {
  const metrics = {
    windowStart: 1000,
    windowEnd: 2000,
    probe: {
      longTasks: [
        { duration: 23536, startTime: 500 }, // before the window (load phase)
        { duration: 120, startTime: 1100 }, // inside
        { duration: 300, startTime: 1900 }, // inside
        { duration: 90, startTime: 2500 }, // after
      ],
    },
  };

  it("keeps only tasks inside the scenario window", () => {
    const scoped = scopedLongTasks(metrics);
    expect(scoped).toHaveLength(2);
    expect(scoped.map((t) => t.duration)).toEqual([120, 300]);
  });

  it("does not attribute the load-phase task to a later scenario", () => {
    // This is the regression that made `select` report load's 23.5s task.
    const scoped = scopedLongTasks(metrics);
    expect(scoped.some((t) => t.duration === 23536)).toBe(false);
  });

  it("falls back to the full list when window bounds are unavailable", () => {
    const scoped = scopedLongTasks({
      probe: { longTasks: [{ duration: 50, startTime: 1 }] },
    });
    expect(scoped).toHaveLength(1);
  });
});

describe("scenarioDuration", () => {
  it("maps each scenario to its duration key", () => {
    expect(scenarioDuration("load", { loadListingMs: 111 })).toBe(111);
    expect(scenarioDuration("scroll", { scrollDurationMs: 222 })).toBe(222);
    expect(scenarioDuration("resize", { resizeDurationMs: 333 })).toBe(333);
    expect(scenarioDuration("select", { select20Ms: 444 })).toBe(444);
  });
});

describe("extractBaselineMetrics", () => {
  const run = {
    scenario: "scroll",
    metrics: {
      scrollDurationMs: 5000,
      windowStart: 0,
      windowEnd: 10000,
      listenerDelta: 13,
      probe: {
        addListenerCalls: 20134,
        removeListenerCalls: 1,
        intersectionObservers: 2000,
        longTasks: [
          { duration: 1000, startTime: 500 },
          { duration: 500, startTime: 1500 },
        ],
      },
      dom: { listingItemCount: 2000, documentElementCount: 20153 },
      frameStats: {
        frames: 60,
        p50: 16,
        p95: 40,
        p99: 90,
        max: 120,
        droppedFrames: 5,
        longFrameMs: 200,
        effectiveFps: 55,
        windowMs: 1000,
        source: "long-animation-frame",
      },
      cdp: {
        delta: {
          ScriptDuration: 1.5,
          LayoutDuration: 0.25,
          RecalcStyleDuration: 0.1,
          LayoutCount: 42,
        },
        gauge: { JSEventListeners: 20148, JSHeapUsedSize: 82173568, Nodes: 38347 },
        before: {},
        after: {},
        windowMs: 5000,
      },
    },
  };

  it("produces the registry metric keys", () => {
    const bag = extractBaselineMetrics(run);
    expect(bag.scenarioMs).toBe(5000);
    expect(bag.domNodes).toBe(20153);
    expect(bag.listingItems).toBe(2000);
    expect(bag.addListenerCalls).toBe(20134);
    expect(bag.intersectionObservers).toBe(2000);
    expect(bag.jSEventListeners).toBe(20148);
    expect(bag.jSHeapUsedSize).toBe(82173568);
  });

  it("reads CDP deltas rather than the zeroed absolutes", () => {
    const bag = extractBaselineMetrics(run);
    expect(bag.scriptDurationDelta).toBe(1500);
    expect(bag.layoutDurationDelta).toBe(250);
    expect(bag.recalcStyleDurationDelta).toBe(100);
    expect(bag.layoutCountDelta).toBe(42);
  });

  it("sums only scenario-scoped long tasks", () => {
    const bag = extractBaselineMetrics(run);
    expect(bag.longTaskMs).toBe(1500);
    expect(bag.longTaskCount).toBe(2);
  });

  it("surfaces real frame percentiles", () => {
    const bag = extractBaselineMetrics(run);
    expect(bag.frameP95).toBe(40);
    expect(bag.frameP99).toBe(90);
    expect(bag.droppedFrames).toBe(5);
    expect(bag.effectiveFps).toBe(55);
  });

  it("omits interaction p95 when Event Timing captured no samples", () => {
    const bag = extractBaselineMetrics(run);
    expect(bag.interactionP95).toBeUndefined();
  });

  it("keeps interaction p95 when Event Timing captured clicks", () => {
    const bag = extractBaselineMetrics({
      scenario: "select",
      metrics: {
        select20Ms: 400,
        interaction: { samples: 12, p50: 20, p95: 48, max: 90, processingP95: 10 },
        probe: {},
        dom: { listingItemCount: 200, documentElementCount: 2154 },
        cdp: {
          delta: {},
          gauge: { JSEventListeners: 10, JSHeapUsedSize: 1000 },
        },
      },
    });
    expect(bag.interactionP95).toBe(48);
    expect(bag.scenarioMs).toBe(400);
  });

  it("omits CDP keys when cdp is null (firefox/webkit)", () => {
    const bag = extractBaselineMetrics({
      scenario: "load",
      metrics: { loadListingMs: 100, cdp: null, dom: {}, probe: {} },
    });
    expect(bag.scriptDurationDelta).toBeUndefined();
    expect(bag.layoutDurationDelta).toBeUndefined();
    expect(Number.isFinite(bag.scenarioMs)).toBe(true);
  });

  it("omits only the CDP keys whose backing field is absent", () => {
    // A present `cdp` object must not imply every delta/gauge field was sampled;
    // missing fields used to be coerced to a false zero.
    const bag = extractBaselineMetrics({
      ...run,
      metrics: {
        ...run.metrics,
        cdp: {
          delta: { ScriptDuration: 1.5 },
          gauge: { JSHeapUsedSize: 1000 },
        },
      },
    });
    expect(bag.scriptDurationDelta).toBe(1500);
    expect(bag.jSHeapUsedSize).toBe(1000);
    expect(bag.layoutDurationDelta).toBeUndefined();
    expect(bag.recalcStyleDurationDelta).toBeUndefined();
    expect(bag.layoutCountDelta).toBeUndefined();
    expect(bag.jSEventListeners).toBeUndefined();
  });

  it("omits frame metrics when no frame window ran", () => {
    const bag = extractBaselineMetrics({
      scenario: "load",
      metrics: { loadListingMs: 100, cdp: null, dom: {}, probe: {} },
    });
    expect(bag.effectiveFps).toBeUndefined();
    expect(bag.frameP95).toBeUndefined();
  });
});

describe("superlinearity", () => {
  it("returns 1 for linear scaling", () => {
    // 10x the data for 10x the time.
    expect(superlinearity([100, 1000, 10000], [100, 1000, 10000])).toBe(1);
  });

  it("detects quadratic scaling", () => {
    // 100x the rows (100 -> 10000) costing 10000x the time is quadratic.
    expect(superlinearity([100, 1000, 10000], [10, 100, 100000])).toBe(100);
  });

  it("flags moderately superlinear growth", () => {
    // 100x rows costing 300x time -> 3x worse than linear.
    expect(superlinearity([100, 1000, 10000], [10, 300, 3000])).toBe(3);
  });

  it("returns null when there are too few points", () => {
    expect(superlinearity([100], [10])).toBeNull();
  });

  it("does not divide by zero on a zero first sample", () => {
    expect(Number.isFinite(superlinearity([100, 1000], [0, 500]) as number)).toBe(
      true,
    );
  });
});
