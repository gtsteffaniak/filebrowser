import { markRaw, toRaw } from "vue";

/** Marks a list as not reactive */
export function rawList<T>(list: T[]): T[] {
  return markRaw(list);
}

/** The list with `value` added at the end, as a raw array to assign */
export function withEntry<T>(list: readonly T[], value: T): T[] {
  return rawList([...toRaw(list), value]);
}

/** The list without `value` as a raw array to assign, or null when it is not in it. Objects match by raw identity */
export function withoutEntry<T>(list: readonly T[], value: T): T[] | null {
  const plain = toRaw(list) as readonly T[];
  let at = plain.indexOf(value);
  if (at === -1 && typeof value === "object" && value !== null) {
    const target = toRaw(value);
    at = plain.findIndex((entry) => toRaw(entry) === target);
  }
  if (at === -1) return null;
  return rawList([...plain.slice(0, at), ...plain.slice(at + 1)]);
}
