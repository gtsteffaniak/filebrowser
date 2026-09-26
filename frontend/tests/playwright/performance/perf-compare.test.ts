import { describe, expect, it } from "vitest";
import {
  compareAgainstBaseline,
  compareMetric,
  renderRegressionTable,
} from "./perf-compare";
import {
  aggregateSamples,
  compareEnvironments,
  median,
  type BaselineMetricEntry,
  type EnvironmentFingerprint,
} from "./perf-baseline";

const entry = (
  value: number,
  overrides: Partial<BaselineMetricEntry> = {},
): BaselineMetricEntry => ({
  value,
  tolerancePct: 25,
  noiseFloor: 0,
  direction: "lower-is-better",
  toleranceClass: "timing",
  samples: 1,
  ...overrides,
});

describe("median", () => {
  it("returns the middle value for odd counts", () => {
    expect(median([5, 1, 3])).toBe(3);
  });

  it("averages the two middle values for even counts", () => {
    expect(median([1, 2, 3, 4])).toBe(2.5);
  });

  it("returns 0 for an empty list", () => {
    expect(median([])).toBe(0);
  });

  it("is robust to a single outlier", () => {
    // The key property for N=3 local runs.
    expect(median([100, 102, 5000])).toBe(102);
  });
});

describe("aggregateSamples", () => {
  it("reports median plus spread", () => {
    const agg = aggregateSamples([100, 110, 120]);
    expect(agg.value).toBe(110);
    expect(agg.min).toBe(100);
    expect(agg.max).toBe(120);
    expect(agg.spreadPct).toBeCloseTo(18.18, 1);
  });

  it("handles empty input", () => {
    expect(aggregateSamples([])).toEqual({
      value: 0,
      min: 0,
      max: 0,
      spreadPct: 0,
    });
  });
});

describe("compareMetric", () => {
  it("passes when within tolerance", () => {
    const r = compareMetric("scenarioMs", entry(1000), 1100);
    expect(r.status).toBe("pass");
    expect(r.deltaPct).toBe(10);
  });

  it("flags a regression past the limit", () => {
    const r = compareMetric("scenarioMs", entry(1000), 1300);
    expect(r.status).toBe("regression");
    expect(r.deltaPct).toBe(30);
    expect(r.detail).toContain("exceeds 25% limit");
  });

  it("does not flag movement inside the noise floor", () => {
    // 30% over, but only 3 units absolute, below the floor of 50.
    const r = compareMetric(
      "scenarioMs",
      entry(10, { noiseFloor: 50 }),
      13,
    );
    expect(r.status).toBe("pass");
    expect(r.detail).toContain("noise floor");
  });

  it("treats a decrease as an improvement", () => {
    const r = compareMetric("scenarioMs", entry(1000), 700);
    expect(r.status).toBe("improvement");
  });

  it("inverts the comparison for higher-is-better metrics", () => {
    const baseline = entry(60, {
      direction: "higher-is-better",
      toleranceClass: "health",
      noiseFloor: 2,
    });
    // A drop to 40 is a 33% regression for FPS.
    expect(compareMetric("effectiveFps", baseline, 40).status).toBe("regression");
    // A rise is good.
    expect(compareMetric("effectiveFps", baseline, 80).status).toBe("improvement");
  });

  it("applies a tighter effective tolerance to shape metrics", () => {
    const baseline = entry(1000, { tolerancePct: 10, toleranceClass: "shape" });
    expect(compareMetric("domNodes", baseline, 1080).status).toBe("pass");
    expect(compareMetric("domNodes", baseline, 1150).status).toBe("regression");
  });

  it("handles a zero baseline without dividing by zero", () => {
    const r = compareMetric("scenarioMs", entry(0), 500);
    expect(Number.isFinite(r.deltaPct)).toBe(true);
    expect(r.status).toBe("regression");
  });
});

const envFixture = (
  overrides: Partial<EnvironmentFingerprint> = {},
): EnvironmentFingerprint => ({
  os: "linux",
  platform: "linux",
  release: "6.0.0",
  arch: "x64",
  cpuModel: "Test CPU",
  cpuCount: 8,
  nodeVersion: "v22.0.0",
  playwrightVersion: "1.54.1",
  browserVersion: "140.0",
  browser: "chromium",
  scales: [100, 1000, 10000],
  workers: 6,
  deviceScaleFactor: 1,
  imageTag: "ci",
  ...overrides,
});

describe("compareEnvironments", () => {
  it("reports no differences for identical environments", () => {
    expect(compareEnvironments(envFixture(), envFixture())).toEqual([]);
  });

  it("detects a worker-count mismatch", () => {
    const diffs = compareEnvironments(envFixture(), envFixture({ workers: 3 }));
    expect(diffs.some((d) => d.startsWith("workers"))).toBe(true);
  });

  it("detects a scale-set mismatch", () => {
    const diffs = compareEnvironments(
      envFixture(),
      envFixture({ scales: [100, 1000] }),
    );
    expect(diffs.some((d) => d.startsWith("scales"))).toBe(true);
  });
});

describe("compareAgainstBaseline", () => {
  const baseline = {
    schemaVersion: 1,
    createdAt: "2026-01-01T00:00:00.000Z",
    gitSha: "abc",
    gitDirty: false,
    environment: envFixture(),
    metrics: {
      "chromium@1000:load": {
        scale: 1000,
        scenario: "load",
        metrics: {
          scenarioMs: entry(1000),
          domNodes: entry(20000, { tolerancePct: 10, toleranceClass: "shape" }),
        },
      },
    },
  };

  it("reports no baseline when null", () => {
    const r = compareAgainstBaseline(null, {
      environment: envFixture(),
      runs: [],
    });
    expect(r.hasBaseline).toBe(false);
    expect(renderRegressionTable(r)).toContain("No baseline found");
  });

  it("passes a run within tolerance", () => {
    const r = compareAgainstBaseline(baseline, {
      environment: envFixture(),
      runs: [
        {
          scale: 1000,
          scenario: "load",
          metrics: { scenarioMs: 1050, domNodes: 20100 },
        },
      ],
    });
    expect(r.totals.regressions).toBe(0);
    expect(r.totals.compared).toBe(2);
  });

  it("detects a regression and renders it", () => {
    const r = compareAgainstBaseline(baseline, {
      environment: envFixture(),
      runs: [
        {
          scale: 1000,
          scenario: "load",
          metrics: { scenarioMs: 1400, domNodes: 20100 },
        },
      ],
    });
    expect(r.totals.regressions).toBe(1);
    expect(r.regressions[0].key).toBe("scenarioMs");
    const table = renderRegressionTable(r);
    expect(table).toContain("REGRESSIONS");
    expect(table).toContain("FAIL");
  });

  it("marks a metric missing from the current run", () => {
    const r = compareAgainstBaseline(baseline, {
      environment: envFixture(),
      runs: [{ scale: 1000, scenario: "load", metrics: { scenarioMs: 1000 } }],
    });
    expect(r.totals.missing).toBe(1);
  });

  it("skips gating when the environment is incomparable", () => {
    const r = compareAgainstBaseline(baseline, {
      environment: envFixture({ workers: 3 }),
      runs: [
        {
          scale: 1000,
          scenario: "load",
          metrics: { scenarioMs: 5000, domNodes: 99999 },
        },
      ],
    });
    expect(r.gatingSkipped).toBe(true);
  });

  it("separates gated regressions from advisory ones", () => {
    const baselineWithAdvisory = {
      ...baseline,
      metrics: {
        "chromium@100:load": {
          scale: 100,
          scenario: "load",
          metrics: {
            scenarioMs: entry(1000),
            // droppedFrames is declared gating:false in the registry.
            droppedFrames: entry(0, { toleranceClass: "health", noiseFloor: 0 }),
          },
        },
      },
    };
    const r = compareAgainstBaseline(baselineWithAdvisory, {
      environment: envFixture(),
      runs: [
        {
          scale: 100,
          scenario: "load",
          metrics: { scenarioMs: 1400, droppedFrames: 9 },
        },
      ],
    });
    expect(r.regressions).toHaveLength(2);
    expect(r.gatedRegressions).toHaveLength(1);
    expect(r.gatedRegressions[0].key).toBe("scenarioMs");
    // The advisory one is still reported, but tagged as non-gating.
    const table = renderRegressionTable(r);
    expect(table).toContain("advisory, not gated");
  });
});
