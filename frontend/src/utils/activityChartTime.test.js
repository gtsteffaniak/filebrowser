import { describe, expect, it } from "vitest";
import {
  floorActivityBucket,
  generateActivityTimeline,
  resolveActivityChartInterval,
  maxChartBucketsForWidth,
  activityBucketLookup,
  chartBarLayout,
  clampActivityChartInterval,
  activityChartIntervalAllowed,
  ACTIVITY_MAX_MINUTE_RANGE_SECS,
} from "./activityChartTime";

describe("activityChartTime", () => {
  it("floors timestamps to interval boundaries", () => {
    expect(floorActivityBucket(3661, "hour")).toBe(3600);
    expect(floorActivityBucket(125, "minute")).toBe(120);
  });

  it("generates a complete timeline across the selected range", () => {
    const from = 0;
    const to = 5 * 3600;
    const timeline = generateActivityTimeline(from, to, "hour");
    expect(timeline).toEqual([0, 3600, 7200, 10800, 14400, 18000]);
  });

  it("places a single event on the correct bucket index", () => {
    const from = 0;
    const to = 24 * 3600;
    const timeline = generateActivityTimeline(from, to, "hour");
    const eventBucket = floorActivityBucket(14 * 3600 + 900, "hour");
    const index = timeline.indexOf(eventBucket);
    expect(index).toBe(14);
  });

  it("coarsens minute to hour when the range has too many buckets", () => {
    const from = 0;
    const to = 24 * 3600;
    expect(resolveActivityChartInterval(from, to, "minute", 96)).toBe("hour");
    expect(generateActivityTimeline(from, to, "hour").length).toBeLessThanOrEqual(96);
  });

  it("keeps minute interval for short ranges", () => {
    const from = 0;
    const to = 3600;
    expect(resolveActivityChartInterval(from, to, "minute", 96)).toBe("minute");
  });

  it("limits buckets based on chart width", () => {
    expect(maxChartBucketsForWidth(480)).toBe(80);
    expect(maxChartBucketsForWidth(120)).toBe(24);
  });

  it("uses wider bars for sparse timelines and thin bars for dense ones", () => {
    const sparse = chartBarLayout(12, 960);
    const dense = chartBarLayout(200, 960);
    expect(sparse.categoryPercentage).toBeLessThan(dense.categoryPercentage);
    expect(dense.barRadius).toBeLessThanOrEqual(sparse.barRadius);
  });

  it("keeps minute timeline granularity for a 24h range", () => {
    const from = 0;
    const to = 24 * 3600;
    expect(generateActivityTimeline(from, to, "minute").length).toBeGreaterThan(
      generateActivityTimeline(from, to, "hour").length,
    );
  });

  it("clamps minute to hour when the range exceeds 48 hours", () => {
    expect(clampActivityChartInterval("minute", ACTIVITY_MAX_MINUTE_RANGE_SECS + 1)).toBe("hour");
    expect(clampActivityChartInterval("minute", ACTIVITY_MAX_MINUTE_RANGE_SECS)).toBe("minute");
  });

  it("marks minute interval unavailable beyond 48 hours", () => {
    const allowed = activityChartIntervalAllowed(30 * 86400);
    expect(allowed.minute).toBe(false);
    expect(allowed.hour).toBe(true);
  });

  it("builds bucket lookup keys for timeline mapping", () => {
    const lookup = activityBucketLookup(
      [{ bucket: 3600, count: 2, seriesKey: "login" }],
      (row) => row.seriesKey || row.eventType || "total",
    );
    expect(lookup.get("3600:login")).toBe(2);
  });
});
