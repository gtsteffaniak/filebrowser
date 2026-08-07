/** @typedef {'minute' | 'hour' | 'day'} ActivityChartInterval */

export const ACTIVITY_INTERVAL_SECONDS = {
  minute: 60,
  hour: 3600,
  day: 86400,
};

export const ACTIVITY_INTERVAL_ORDER = ["minute", "hour", "day"];

/** Backend limit: minute buckets for at most 48 hours. */
export const ACTIVITY_MAX_MINUTE_RANGE_SECS = 2 * 86400;

/** Backend limit: hour buckets for at most 90 days. */
export const ACTIVITY_MAX_HOUR_RANGE_SECS = 90 * 86400;

export const DEFAULT_MAX_CHART_BUCKETS = 96;

/** Minimum bar width in CSS pixels before coarsening the interval. */
export const MIN_CHART_BAR_PX = 6;

function intervalStepSeconds(interval) {
  switch (interval) {
    case "minute":
      return ACTIVITY_INTERVAL_SECONDS.minute;
    case "hour":
      return ACTIVITY_INTERVAL_SECONDS.hour;
    case "day":
      return ACTIVITY_INTERVAL_SECONDS.day;
    default:
      return ACTIVITY_INTERVAL_SECONDS.hour;
  }
}

/**
 * @param {number} unixSec
 * @param {ActivityChartInterval} interval
 */
export function floorActivityBucket(unixSec, interval) {
  const step = intervalStepSeconds(interval);
  return Math.floor(unixSec / step) * step;
}

/**
 * All bucket start timestamps from range start through range end (inclusive).
 * @param {number} from unix seconds
 * @param {number} to unix seconds
 * @param {ActivityChartInterval} interval
 * @returns {number[]}
 */
export function generateActivityTimeline(from, to, interval) {
  const step = intervalStepSeconds(interval);
  const start = floorActivityBucket(from, interval);
  const end = floorActivityBucket(to, interval);
  if (end < start) {
    return [];
  }
  const buckets = [];
  for (let t = start; t <= end; t += step) {
    buckets.push(t);
  }
  return buckets;
}

/**
 * Clamp the preferred chart interval to what the activity API accepts for this range.
 * @param {ActivityChartInterval|string} interval
 * @param {number} rangeSecs
 * @returns {ActivityChartInterval}
 */
export function clampActivityChartInterval(interval, rangeSecs) {
  let resolved = interval;
  if (resolved === "minute" && rangeSecs > ACTIVITY_MAX_MINUTE_RANGE_SECS) {
    resolved = "hour";
  }
  if (resolved === "hour" && rangeSecs > ACTIVITY_MAX_HOUR_RANGE_SECS) {
    resolved = "day";
  }
  if (!ACTIVITY_INTERVAL_ORDER.includes(resolved)) {
    return "hour";
  }
  return resolved;
}

/**
 * Whether each chart interval is valid for the given range length.
 * @param {number} rangeSecs
 * @returns {Record<ActivityChartInterval, boolean>}
 */
export function activityChartIntervalAllowed(rangeSecs) {
  return {
    minute: rangeSecs <= ACTIVITY_MAX_MINUTE_RANGE_SECS,
    hour: rangeSecs <= ACTIVITY_MAX_HOUR_RANGE_SECS,
    day: true,
  };
}

/**
 * Pick the finest interval at or coarser than preferred that fits maxBuckets.
 * @param {number} from
 * @param {number} to
 * @param {ActivityChartInterval|string} preferred
 * @param {number} [maxBuckets]
 * @returns {ActivityChartInterval}
 */
export function resolveActivityChartInterval(from, to, preferred, maxBuckets = DEFAULT_MAX_CHART_BUCKETS) {
  const preferredIdx = ACTIVITY_INTERVAL_ORDER.indexOf(preferred);
  const startIdx = preferredIdx >= 0 ? preferredIdx : ACTIVITY_INTERVAL_ORDER.indexOf("hour");

  for (let i = startIdx; i < ACTIVITY_INTERVAL_ORDER.length; i += 1) {
    const interval = ACTIVITY_INTERVAL_ORDER.at(i) ?? "day";
    if (generateActivityTimeline(from, to, interval).length <= maxBuckets) {
      return interval;
    }
  }
  return "day";
}

/**
 * @param {number} chartWidthPx
 * @returns {number}
 */
export function maxChartBucketsForWidth(chartWidthPx) {
  const width = Number.isFinite(chartWidthPx) && chartWidthPx > 0 ? chartWidthPx : 960;
  return Math.max(24, Math.min(120, Math.floor(width / MIN_CHART_BAR_PX)));
}

/**
 * @param {Array<{ bucket: number|string, count: number, seriesKey?: string, eventType?: string }>} buckets
 * @param {(row: object) => string} seriesKeyFn
 * @returns {Map<string, number>}
 */
export function activityBucketLookup(buckets, seriesKeyFn) {
  const map = new Map();
  for (const row of buckets || []) {
    const bucket = String(row.bucket);
    const seriesKey = seriesKeyFn(row);
    map.set(`${bucket}:${seriesKey}`, Number(row.count) || 0);
  }
  return map;
}

/**
 * Bar sizing for dense vs sparse timelines (Chart.js category scale).
 * @param {number} bucketCount
 * @param {number} [chartWidthPx]
 * @returns {{ barPercentage: number, categoryPercentage: number, barRadius: number }}
 */
export function chartBarLayout(bucketCount, chartWidthPx) {
  const width = Number.isFinite(chartWidthPx) && chartWidthPx > 0 ? chartWidthPx : 960;
  const pxPerCategory = width / Math.max(bucketCount, 1);

  if (bucketCount <= 24) {
    return { barPercentage: 0.72, categoryPercentage: 0.82, barRadius: 6 };
  }
  if (bucketCount <= 72) {
    return { barPercentage: 0.88, categoryPercentage: 0.94, barRadius: 4 };
  }
  const barRadius = pxPerCategory >= 6 ? 3 : pxPerCategory >= 3 ? 2 : 0;
  return {
    barPercentage: 1,
    categoryPercentage: 0.98,
    barRadius,
  };
}

/**
 * @param {number} bucketCount
 * @returns {number}
 */
export function chartAxisMaxTicks(bucketCount) {
  if (bucketCount <= 12) {
    return bucketCount;
  }
  if (bucketCount <= 48) {
    return 12;
  }
  return 10;
}
