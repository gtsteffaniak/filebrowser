/**
 * Normalize an activity viewer `eventType` query value to a list of types.
 * Accepts comma-separated strings, vue-router arrays, or repeated query values.
 * @param {string | string[] | null | undefined} raw
 * @returns {string[]}
 */
export function normalizeEventTypeQueryValue(raw) {
  if (raw === undefined || raw === null) {
    return [];
  }
  if (Array.isArray(raw)) {
    return raw.flatMap((value) => normalizeEventTypeQueryValue(value));
  }
  const str = String(raw).trim();
  if (!str) {
    return [];
  }
  return str.split(",").map((part) => part.trim()).filter(Boolean);
}

/**
 * @param {string[]} types
 * @returns {string}
 */
export function formatEventTypeQueryValue(types) {
  if (!Array.isArray(types) || types.length === 0) {
    return "";
  }
  return types.join(",");
}

/**
 * @param {string[]} types
 * @param {string[]} allowed
 * @returns {string[]}
 */
export function filterEventTypesForScope(types, allowed) {
  if (!Array.isArray(types) || types.length === 0) {
    return [];
  }
  const allowedSet = new Set(allowed);
  return types.filter((type) => allowedSet.has(type));
}

const COMMA_SEPARATED_QUERY_KEYS = new Set(["eventType", "rows"]);

/**
 * Encode a query param value, preserving comma separators for multi-value keys.
 * @param {string} key
 * @param {string} value
 * @returns {string}
 */
export function encodeActivityViewerQueryValue(key, value) {
  const str = String(value);
  if (COMMA_SEPARATED_QUERY_KEYS.has(key)) {
    return str
      .split(",")
      .map((part) => encodeURIComponent(part.trim()))
      .filter(Boolean)
      .join(",");
  }
  return encodeURIComponent(str);
}

/**
 * @param {Record<string, string>} query
 * @returns {string}
 */
export function formatActivityViewerQueryString(query) {
  return Object.entries(query)
    .filter(([, value]) => value !== undefined && value !== null && value !== "")
    .map(([key, value]) => `${encodeURIComponent(key)}=${encodeActivityViewerQueryValue(key, value)}`)
    .join("&");
}

/** @typedef {'all' | 'success' | 'errors'} ActivityStatusOutcome */

export const ACTIVITY_STATUS_RANGES = {
  success: { min: 200, max: 399 },
  errors: { min: 400, max: 599 },
};

/**
 * Map statusMin/statusMax query params to a UI outcome preset.
 * @param {string | string[] | null | undefined} statusMin
 * @param {string | string[] | null | undefined} statusMax
 * @returns {ActivityStatusOutcome}
 */
export function parseStatusOutcomeFromQuery(statusMin, statusMax) {
  const min = statusMin !== undefined && statusMin !== null && String(statusMin).trim() !== ""
    ? parseInt(String(statusMin), 10)
    : null;
  const max = statusMax !== undefined && statusMax !== null && String(statusMax).trim() !== ""
    ? parseInt(String(statusMax), 10)
    : null;
  if (min === ACTIVITY_STATUS_RANGES.success.min && max === ACTIVITY_STATUS_RANGES.success.max) {
    return "success";
  }
  if (min === ACTIVITY_STATUS_RANGES.errors.min && max === ACTIVITY_STATUS_RANGES.errors.max) {
    return "errors";
  }
  return "all";
}

/**
 * @param {ActivityStatusOutcome} outcome
 * @returns {{ statusMin?: number, statusMax?: number }}
 */
export function activityStatusParamsForOutcome(outcome) {
  if (outcome === "success") {
    return {
      statusMin: ACTIVITY_STATUS_RANGES.success.min,
      statusMax: ACTIVITY_STATUS_RANGES.success.max,
    };
  }
  if (outcome === "errors") {
    return {
      statusMin: ACTIVITY_STATUS_RANGES.errors.min,
      statusMax: ACTIVITY_STATUS_RANGES.errors.max,
    };
  }
  return {};
}
