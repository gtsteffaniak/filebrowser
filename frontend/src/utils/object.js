/**
 * Safe property access helpers for plain objects (prototype-pollution / injection-sink safe).
 */

function normalizeKey(key) {
  if (typeof key === 'string' || typeof key === 'number') {
    return String(key);
  }
  return null;
}

/** @returns {unknown} */
export function getObjectProperty(obj, key) {
  const prop = normalizeKey(key);
  if (obj === null || obj === undefined || prop === null || !Object.hasOwn(obj, prop)) {
    return undefined;
  }
  // eslint-disable-next-line security/detect-object-injection
  return obj[prop];
}

/** @returns {Record<string, unknown>} */
export function setObjectProperty(obj, key, value) {
  const prop = normalizeKey(key);
  if (prop === null) {
    return { ...(obj ?? {}) };
  }
  const result = { ...(obj ?? {}) };
  // eslint-disable-next-line security/detect-object-injection
  result[prop] = value;
  return result;
}

/** @returns {Record<string, unknown>} */
export function omitObjectProperty(obj, key) {
  const prop = normalizeKey(key);
  if (obj === null || obj === undefined || prop === null || !Object.hasOwn(obj, prop)) {
    return { ...(obj ?? {}) };
  }
  return Object.fromEntries(
    Object.entries(obj).filter(([entryKey]) => entryKey !== prop)
  );
}

/** @returns {unknown} */
export function getNestedProperty(obj, outerKey, innerKey) {
  const outer = getObjectProperty(obj, outerKey);
  return getObjectProperty(outer, innerKey);
}

/**
 * @param {Record<string, unknown> | null | undefined} obj
 * @param {string | number} outerKey
 * @param {string | number} innerKey
 * @param {unknown} innerValue
 * @param {{ removeInner?: boolean; removeOuterIfEmpty?: boolean }} [options]
 * @returns {Record<string, unknown>}
 */
export function updateNestedProperty(obj, outerKey, innerKey, innerValue, options = {}) {
  const { removeInner = false, removeOuterIfEmpty = false } = options;
  const outerObj = getObjectProperty(obj, outerKey) ?? {};
  let newOuter;
  if (removeInner) {
    newOuter = omitObjectProperty(outerObj, innerKey);
  } else {
    newOuter = setObjectProperty(outerObj, innerKey, innerValue);
  }

  if (removeOuterIfEmpty && Object.keys(newOuter).length === 0) {
    return omitObjectProperty(obj, outerKey);
  }
  return setObjectProperty(obj, outerKey, newOuter);
}
