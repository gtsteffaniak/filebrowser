type DeepCloneable = object | Array<any>;

export default function deepClone<T extends DeepCloneable>(obj: T): T {
  if (obj === null || typeof obj !== 'object') {
    return obj;
  }

  if (Array.isArray(obj)) {
    return obj.map(deepClone) as T;
  }

  const clone = {} as T;
  for (const key in obj) {
    clone[key] = deepClone(obj[key] as any);
  }
  return clone;
}