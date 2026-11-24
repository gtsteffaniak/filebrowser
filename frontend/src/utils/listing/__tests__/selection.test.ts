import { computed, isReactive, nextTick, reactive, toRaw } from "vue";
import { describe, expect, it } from "vitest";
import { withEntry, withoutEntry } from "../selection.ts";

describe("withoutEntry", () => {
  it("removes one entry and keeps the order of the others", () => {
    expect(withoutEntry([1, 2, 3, 4], 3)).toEqual([1, 2, 4]);
  });

  it("returns null for an entry that is not in the list, and never changes the list", () => {
    const list = [1, 2];
    expect(withoutEntry(list, 9)).toBeNull();
    expect(withoutEntry(list, 1)).toEqual([2]);
    expect(list).toEqual([1, 2]);
  });

  it("finds an object whether it is given plain or through the reactive state", () => {
    const item = { name: "a.txt" };
    const state = reactive({ selected: [item] as { name: string }[] });
    expect(withoutEntry(state.selected, state.selected[0])).toEqual([]); // the reactive proxy of the object
    expect(withoutEntry(state.selected, item)).toEqual([]); // the plain object
  });

  it("notifies what depends on the list once the result is assigned", async () => {
    const state = reactive({ selected: [1, 2, 3] });
    const set = computed(() => new Set(state.selected));
    const has = (entry: number) => set.value.has(entry);
    expect(has(2)).toBe(true);
    state.selected = withoutEntry(state.selected, 2) as number[];
    await nextTick();
    expect(has(2)).toBe(false);
    expect(has(3)).toBe(true);
  });

  it("stays fast on a reactive list with a very large selection", () => {
    const state = reactive({ selected: Array.from({ length: 50000 }, (_, i) => i) });
    const start = performance.now();
    const next = withoutEntry(state.selected, 5000);
    expect(performance.now() - start).toBeLessThan(100);
    expect(next).toHaveLength(49999);
  });

  it("finds a reactive object that is stored in a raw array", () => {
    const item = { name: "a.txt" };
    const other = { name: "b.txt" };
    const proxy = reactive(item);
    const list = [other, proxy];
    expect(withoutEntry(list, proxy)).toEqual([other]);
    expect(withoutEntry(list, item)).toEqual([other]);
    expect(withoutEntry([proxy], reactive({ ...item }))).toBeNull();
  });

  it("returns a raw array, so it stays raw once it is assigned to the state", () => {
    const state = reactive({ selected: [1, 2, 3, 4] as number[] });
    const next = withoutEntry(state.selected, 2) as number[];
    state.selected = next;
    expect(isReactive(state.selected)).toBe(false);
    expect(state.selected).toBe(toRaw(state.selected));
    state.selected = withoutEntry(state.selected, 3) as number[];
    expect(isReactive(state.selected)).toBe(false);
    expect(state.selected).toEqual([1, 4]);
  });
});

describe("withEntry", () => {
  it("adds one entry at the end without changing the list", () => {
    const list = [1, 2];
    expect(withEntry(list, 3)).toEqual([1, 2, 3]);
    expect(list).toEqual([1, 2]);
  });

  it("returns a raw array, and notifies once it is assigned to the state", async () => {
    const state = reactive({ selected: [] as number[] });
    const count = computed(() => state.selected.length);
    const currentCount = () => count.value;
    expect(currentCount()).toBe(0);
    state.selected = withEntry(state.selected, 7);
    await nextTick();
    expect(isReactive(state.selected)).toBe(false);
    expect(currentCount()).toBe(1);
  });
});
