type DeepCloneable = object | unknown[];

export default function deepClone<T extends DeepCloneable>(obj: T): T {
  if (obj === null || typeof obj !== 'object') {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map((item) => deepClone(item as DeepCloneable)) as T;
  }

  const entries = Object.entries(obj).map(([key, value]) => [
    key,
    deepClone(value as DeepCloneable),
  ]);
  return Object.fromEntries(entries) as T;
}
