export default function deepClone<T>(obj: T): T {
  // Base case: primitives and null are returned as-is
  if (obj === null || typeof obj !== 'object') {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map(item => deepClone(item)) as T;
  }

  const clone = Object.create(null) as Record<string, unknown>;
  for (const key in obj) {
    if (key === '__proto__' || key === 'constructor' || key === 'prototype') {
      continue;
    }
    if (Object.prototype.hasOwnProperty.call(obj, key)) {
      clone[key] = deepClone((obj as Record<string, unknown>)[key]);
    }
  }
  return clone as T;
}
