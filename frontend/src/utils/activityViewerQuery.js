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
  return str
    .split(",")
    .map((part) => part.trim())
    .filter((part) => part !== "");
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
