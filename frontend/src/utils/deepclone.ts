export default function deepClone<T>(obj: T): T {
  if (obj === null || typeof obj !== 'object') {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map(item => deepClone(item)) as T;
  }

  const entries: [string, unknown][] = [];
  for (const key in obj) {
    if (key === '__proto__' || key === 'constructor' || key === 'prototype') {
      continue;
    }
    if (Object.prototype.hasOwnProperty.call(obj, key)) {
      entries.push([key, deepClone((obj as Record<string, unknown>)[key])]);
    }
  }
  return Object.fromEntries(entries) as T;
}
