type DeepCloneable = Record<string, unknown> | unknown[];

export default function deepClone<T extends DeepCloneable>(obj: T): T {
  if (obj === null || typeof obj !== 'object') {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map(item => deepClone(item as DeepCloneable)) as T;
  }

  const clone = {} as Record<string, unknown>;
  for (const key in obj) {
    clone[key] = deepClone(obj[key] as DeepCloneable);
  }
  return clone as T;
}
