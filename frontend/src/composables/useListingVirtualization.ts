import { computed, nextTick, onBeforeUnmount, shallowRef, watch, type CSSProperties, type Ref } from "vue";
import { getters, state } from "@/store/index.ts";
import type { fileListItemIndex } from "@/store/types.ts";
import {
  buildGridGroups,
  clampWindow,
  hitTest,
  itemRangeOfRows,
  locateFlat,
  moveVertical,
  rowCount,
  sameWindow,
  visibleRows,
  windowCovers,
  type GridGroup,
  type HitGroup,
  type HitRect,
  type RowWindow,
} from "@/utils/listing/listingGrid.ts";
import { measureGroup, pickMetrics, px, sameMetrics, type LayoutMetrics } from "@/utils/listing/listingMetrics.ts";
import {
  registerVirtualListing,
  type VirtualListingHandle,
  type VirtualScrollOptions,
} from "@/utils/listing/virtualListing.ts";

const DEFAULT_MIN_ITEMS = 100; // Listings with fewer items than this render all
const ESTIMATED_ROW_PITCH = 56; // Row height guess until the first measurement
const MIN_INITIAL_ROWS = 10;
const MAX_INITIAL_ROWS = 30;
const INITIAL_VIEWPORTS = 1.2;

function initialRows(): number {
  const height = typeof window === "undefined" ? 0 : window.innerHeight;
  if (!(height > 0)) return MAX_INITIAL_ROWS;
  const rows = Math.ceil((height * INITIAL_VIEWPORTS) / ESTIMATED_ROW_PITCH);
  return Math.max(MIN_INITIAL_ROWS, Math.min(MAX_INITIAL_ROWS, rows));
}

const OVERSCAN_VIEWPORTS = 0.30; // Screens of rows kept rendered above and below the visible ones
const BUFFER_VIEWPORT_SHARE = 0.15;
const MAX_SETTLE_PASSES = 4; // Rendering and measuring (columns change, so rows change... etc)
const MAX_MEASURE_RETRIES = 2; // A list that cannot be measured is tried again this many times
const PITCH_NOISE_PX = 4; // A new measurement of the row size is ignored when it is this close to the one in use (rows of different heights)
const PITCH_NOISE_SHARE = 0.06;
const MAX_JUMP_PASSES = 2; // scroll to a item that wasn't rendered is repeated this many times

export interface VirtualItem {
  index: number;
}

export type VirtualGroupInput<T extends VirtualItem = fileListItemIndex> = {
  /** Name of the group, matches `data-virtual-group` on the element that holds its items */
  key: string;
} & (
  | { items: T[] }
  | { count: number; at: (position: number) => T }
);

function groupSize<T extends VirtualItem>(group: VirtualGroupInput<T>): number {
  return "items" in group ? group.items.length : group.count;
}

function groupItem<T extends VirtualItem>(group: VirtualGroupInput<T>, position: number): T | undefined {
  return "items" in group ? group.items[position] : group.at(position);
}

function groupSlice<T extends VirtualItem>(group: VirtualGroupInput<T>, start: number, end: number): T[] {
  if ("items" in group) return start === 0 && end >= group.items.length ? group.items : group.items.slice(start, end);
  const items: T[] = [];
  for (let position = start; position < end; position++) items.push(group.at(position));
  return items;
}

export interface VisibleGroup<T extends VirtualItem = fileListItemIndex> {
  /** The part of the group that has to be rendered */
  items: T[];
  /** Position of items in all the groups one after another (for things like the zebra style in list/compact) */
  position: number;
  /** Space standing in for the rows that are not rendered */
  style: CSSProperties | undefined;
}

export interface ListingVirtualizationOptions<T extends VirtualItem = fileListItemIndex> {
  /** The element whose direct children are the `data-virtual-group` elements (`.listing-items` in listings) */
  container: Ref<HTMLElement | null>;
  /** The groups in the order they are displayed, they must not be reordered */
  groups: () => VirtualGroupInput<T>[];
  /** Number of columns to start with, before the layout has been measured */
  columnsHint?: () => number;
  /**
   * Finds the element(s) that may scroll the container, default: the closest `.scroll-wrapper`. With several
   * candidates the one whose content overflows is used. With none, everything is rendered.
   */
  scroller?: (container: HTMLElement) => HTMLElement | HTMLElement[] | null;
  itemClass?: string;
  register?: boolean;
  minItems?: number;
}

/** The ancestor that scrolls */
export function findScroller(el: HTMLElement): HTMLElement | null {
  return pickScroller(findScrollParents(el));
}

/** The first candidate whose content is taller than itself */
function pickScroller(candidates: HTMLElement[]): HTMLElement | null {
  if (candidates.length < 2) return candidates[0] ?? null;
  return candidates.find((candidate) => candidate.scrollHeight > candidate.clientHeight + 1) ?? candidates[0] ?? null;
}

export function findScrollParents(el: HTMLElement, includeSelf = false): HTMLElement[] {
  const parents: HTMLElement[] = [];
  let node: HTMLElement | null = includeSelf ? el : el.parentElement;
  while (node) {
    if (/(auto|scroll|overlay)/.test(getComputedStyle(node).overflowY)) parents.push(node);
    node = node.parentElement;
  }
  return parents;
}

const EMPTY_GROUP: VisibleGroup<never> = { items: [], position: 0, style: undefined };

/**
 * Renders only the rows on (or near) the screen and replaces the rest with empty space of the same size.
 * Selection, arrow keys and rectangle selection work on data (indexes of `state.req.items`) through the
 * grid maths, since most items have no element. The CSS still lays the items out: columns and row size
 * are measured from what is rendered.
 */
export function useListingVirtualization<T extends VirtualItem = fileListItemIndex>(
  options: ListingVirtualizationOptions<T>,
) {
  const container = options.container;
  const itemClass = options.itemClass ?? "listing-item";
  let scrollCandidates: HTMLElement[] = [];
  const metrics = shallowRef<LayoutMetrics | null>(null);
  const windows = shallowRef<Record<string, RowWindow>>({});
  const containerWidth = shallowRef(0);
  const unsupported = shallowRef(false);
  const unmeasurable = shallowRef(false);
  const settling = shallowRef(false);
  let retries = 0;

  const inputs = computed(() => options.groups());
  const gridGroups = computed(() =>
    buildGridGroups(inputs.value.map((group) => ({ key: group.key, count: groupSize(group) }))),
  );
  const minItems = options.minItems ?? DEFAULT_MIN_ITEMS;
  const totalItems = computed(() => gridGroups.value.reduce((total, group) => total + group.count, 0));
  const active = computed(() => !unsupported.value && !unmeasurable.value && totalItems.value >= minItems);
  const settled = computed(() => active.value && metrics.value !== null && !settling.value);

  function currentScroller(): HTMLElement | null {
    return pickScroller(scrollCandidates);
  }
  const columns = computed(() => metrics.value?.columns ?? Math.max(1, Math.round(options.columnsHint?.() ?? 1)));
  const rowPitch = computed(() => metrics.value?.rowPitch ?? ESTIMATED_ROW_PITCH);
  // Anything that changes how the browser lays the items out
  const layoutKey = computed(() => `${getters.viewMode()}|${state.user?.gallerySize}|${containerWidth.value}`);

  const firstRows = initialRows();

  // Rows of the first render, shared by the groups in the order they are displayed: only the first rows of the
  // list are on the screen, so a group below them would mount rows that are thrown away right after measuring.
  const initialEnds = computed(() => {
    let left = firstRows;
    const ends: Record<string, number> = {};
    for (const g of gridGroups.value) {
      const end = Math.min(rowCount(g.count, columns.value), left);
      ends[g.key] = end;
      left -= end;
    }
    return ends;
  });

  function windowFor(group: GridGroup): RowWindow {
    const rows = rowCount(group.count, columns.value);
    const stored = windows.value[group.key];
    return clampWindow(stored ?? { firstRow: 0, endRow: initialEnds.value[group.key] ?? 0 }, rows);
  }

  const visible = computed(() => {
    const result: Record<string, VisibleGroup<T>> = {};
    const grid = gridGroups.value;
    inputs.value.forEach((input, i) => {
      const group = grid[i] as GridGroup;
      if (!active.value) {
        result[input.key] = { items: groupSlice(input, 0, group.count), position: group.start, style: undefined };
        return;
      }
      const rows = rowCount(group.count, columns.value);
      const window = windowFor(group);
      const range = itemRangeOfRows(window, columns.value, group.count);
      let above = window.firstRow * rowPitch.value;
      let below = (rows - window.endRow) * rowPitch.value;

      if (window.endRow <= window.firstRow && rows > 0) {
        const gap = metrics.value?.rowGap ?? 0;
        if (above > 0) above -= gap;
        else below -= gap;
      }
      result[input.key] = {
        items: groupSlice(input, range.start, range.end),
        position: group.start + range.start,
        style: { marginTop: `${above}px`, marginBottom: `${below}px` },
      };
    });
    return result;
  });

  function group(key: string): VisibleGroup<T> {
    return visible.value[key] ?? EMPTY_GROUP;
  }

  function groupElement(key: string): HTMLElement | null {
    return container.value?.querySelector<HTMLElement>(`:scope > [data-virtual-group="${key}"]`) ?? null;
  }

  /** Top of a group first row (rendered or not), measured from the top of the scrolled content */
  function firstRowTop(el: HTMLElement, scrollRect: DOMRect, scrollTop: number, insetTop: number): number {
    return el.getBoundingClientRect().top - scrollRect.top + scrollTop - px(el.style.marginTop) + insetTop;
  }

  let measuredKey = "";

  function hasRenderedItems(): boolean {
    return !!container.value?.querySelector(`.${itemClass}`);
  }

  /** Reads columns and row size from what is rendered. True when something changed. */
  function measure(): boolean {
    const measured = gridGroups.value.map((g) => {
      const el = g.count > 0 ? groupElement(g.key) : null;
      return el ? measureGroup(el, itemClass) : null;
    });
    let next = pickMetrics(measured);
    if (!next) return false;
    const previous = metrics.value;
    // A partial row at the end of a group can show fewer columns than there are
    if (previous && measuredKey === layoutKey.value && previous.columns > next.columns) {
      next = { ...next, columns: previous.columns };
    }
    // Rows of different heights make the average pitch drift with every window, which would shift the spacers
    // (pitch x rows left out) under the scroll position. For the same layout a small change is noise.
    if (
      previous &&
      measuredKey === layoutKey.value &&
      previous.columns === next.columns &&
      Math.abs(previous.rowPitch - next.rowPitch) <= Math.max(PITCH_NOISE_PX, previous.rowPitch * PITCH_NOISE_SHARE)
    ) {
      next = { ...next, rowPitch: previous.rowPitch, itemHeight: previous.itemHeight, offsetY: previous.offsetY };
    }
    measuredKey = layoutKey.value;
    if (sameMetrics(previous, next)) return false;
    metrics.value = next;
    return true;
  }

  /** Moves the rendered rows to follow the scroll position, true when they changed. `force` ignores what is rendered. */
  function updateWindows({ force = false, only }: { force?: boolean; only?: ReadonlySet<string> } = {}): boolean {
    const el = currentScroller();
    const m = metrics.value;
    if (!active.value || !el || !m) return false;

    const scrollRect = el.getBoundingClientRect();
    const viewportTop = el.scrollTop;
    const viewportBottom = viewportTop + el.clientHeight;
    const overscan = el.clientHeight * OVERSCAN_VIEWPORTS;
    const buffer = Math.max(1, Math.floor((el.clientHeight * BUFFER_VIEWPORT_SHARE) / m.rowPitch));
    const lastGroup = [...gridGroups.value].reverse().find((g) => g.count > 0);

    const next = { ...windows.value };
    let changed = false;
    for (const g of gridGroups.value) {
      if (only && !only.has(g.key)) continue;
      const groupEl = groupElement(g.key);
      if (!groupEl || g.count === 0) continue;
      const rows = rowCount(g.count, m.columns);
      const top = firstRowTop(groupEl, scrollRect, viewportTop, m.insetTop);
      const geometry = { firstRowTop: top, rowPitch: m.rowPitch, rows, viewportTop, viewportBottom };
      const needed = visibleRows(geometry);
      const current = windowFor(g);
      const keep =
        !force && needed.endRow > needed.firstRow && windowCovers(current, needed, rows, buffer);
      let target = keep ? current : visibleRows({ ...geometry, overscan });
      if (g === lastGroup && rows > 0 && target.endRow <= target.firstRow && target.firstRow >= rows) {
        const screenRows = Math.ceil((el.clientHeight + overscan * 2) / m.rowPitch);
        target = { firstRow: Math.max(0, rows - screenRows), endRow: rows };
      }
      if (!sameWindow(current, target) || !windows.value[g.key]) {
        next[g.key] = target;
        changed = changed || !sameWindow(current, target);
      }
    }
    if (changed) windows.value = next;
    return changed;
  }

  let settleRun: { promise: Promise<void> } | null = null;
  let settleAgain = false;

  async function runSettle(): Promise<void> {
    do {
      settleAgain = false;
      for (let pass = 0; pass < MAX_SETTLE_PASSES; pass++) {
        await nextTick(); // let what changed get rendered
        if (!active.value || !container.value) break;
        const measured = measure();
        if (!metrics.value && hasRenderedItems()) {
          unmeasurable.value = true;
          break;
        }
        if (metrics.value) retries = 0;
        const moved = updateWindows({ force: measured });
        if (!measured && !moved) break;
      }
    } while (settleAgain);
  }

  function settle(): Promise<void> {
    if (settleRun) {
      settleAgain = true;
      return settleRun.promise;
    }
    const run = { promise: Promise.resolve() };
    settleRun = run;
    settling.value = true;
    run.promise = runSettle().finally(() => {
      settleRun = null;
      settling.value = false;
    });
    return run.promise;
  }

  interface Located {
    group: number;
    local: number;
  }

  /** Items of a group come in the order of req.items, so their index is sorted */
  function locateIndex(index: number): Located | null {
    const all = inputs.value;
    for (let g = 0; g < all.length; g++) {
      const input = all[g] as VirtualGroupInput<T>;
      if (!("items" in input)) {
        if (index >= 0 && index < input.count) return { group: g, local: index };
        continue;
      }
      const items = input.items;
      let low = 0;
      let high = items.length - 1;
      while (low <= high) {
        const mid = (low + high) >>> 1;
        const value = (items[mid] as T).index;
        if (value === index) return { group: g, local: mid };
        if (value < index) low = mid + 1;
        else high = mid - 1;
      }
    }
    return null;
  }

  function itemElement(index: number): HTMLElement | null {
    return container.value?.querySelector<HTMLElement>(`.${itemClass}[data-index="${index}"]`) ?? null;
  }

  /** Puts the row of an item in the middle of the screen so it gets rendered */
  function jumpToRow(located: Located): boolean {
    const el = currentScroller();
    const grid = gridGroups.value[located.group];
    const groupEl = grid ? groupElement(grid.key) : null;
    if (!el || !grid || !groupEl) return false;
    const m = metrics.value;
    const pitch = m?.rowPitch ?? ESTIMATED_ROW_PITCH;
    const cols = m?.columns ?? columns.value;
    const top = firstRowTop(groupEl, el.getBoundingClientRect(), el.scrollTop, m?.insetTop ?? 0);
    const row = Math.floor(located.local / cols);
    el.scrollTop = Math.max(0, top + row * pitch - (el.clientHeight - pitch) / 2);
    updateWindows({ force: true });
    return true;
  }

  function scrollToIndex(index: number, scrollOptions: VirtualScrollOptions = {}): boolean {
    if (!container.value) return false;
    const located = locateIndex(index);
    if (!located) return false;
    const block = scrollOptions.block ?? "nearest";
    const behavior = scrollOptions.behavior ?? "instant";

    const rendered = itemElement(index);
    if (rendered) {
      rendered.scrollIntoView({ behavior, block, inline: "nearest" });
      return true;
    }
    if (!active.value) return false;
    if (!metrics.value && hasRenderedItems()) measure();
    if (!jumpToRow(located)) return false;
    void bringIntoView(index, located, block, behavior);
    return true;
  }

  /** Once the row is rendered the browser aligns it exactly. A jump based on wrong sizes is repeated with the new ones. */
  async function bringIntoView(
    index: number,
    located: Located,
    block: ScrollLogicalPosition,
    behavior: ScrollBehavior,
  ) {
    for (let pass = 0; pass < MAX_JUMP_PASSES; pass++) {
      await settle();
      const el = itemElement(index);
      if (el) {
        el.scrollIntoView({ behavior, block, inline: "nearest" });
        return;
      }
      if (!jumpToRow(located)) return;
    }
  }

  function indexAtFlat(flat: number): number | null {
    const at = locateFlat(gridGroups.value, flat, columns.value);
    if (!at) return null;
    const input = inputs.value[at.group];
    return (input ? groupItem(input, at.local) : undefined)?.index ?? null;
  }

  function moveFromIndex(index: number, direction: -1 | 1): number | null {
    const located = locateIndex(index);
    if (!located) return null;
    const grid = gridGroups.value[located.group] as GridGroup;
    const target = moveVertical(gridGroups.value, columns.value, grid.start + located.local, direction);
    return target === null ? null : indexAtFlat(target);
  }

  function hitTestRect(rect: HitRect, listingRect: DOMRect): number[] | null {
    const m = metrics.value;
    if (!active.value || !m || !(m.colPitch > 0)) return null; // right-to-left layouts use the DOM
    const geometry: HitGroup[] = gridGroups.value.map((g) => {
      const el = g.count > 0 ? groupElement(g.key) : null;
      if (!el) return { count: g.count, firstRowTop: 0, left: 0 };
      const box = el.getBoundingClientRect();
      return {
        count: g.count,
        firstRowTop: box.top - listingRect.top - px(el.style.marginTop) + m.insetTop,
        left: box.left - listingRect.left,
      };
    });
    const indexes: number[] = [];
    for (const flat of hitTest(gridGroups.value, geometry, m, rect)) {
      const index = indexAtFlat(flat);
      if (index !== null) indexes.push(index);
    }
    return indexes;
  }

  const handle: VirtualListingHandle = {
    isActive: () => active.value,
    scrollToIndex,
    moveVertical: moveFromIndex,
    hitTest: hitTestRect,
  };
  const unregister = options.register === false ? () => {} : registerVirtualListing(handle);

  let frame = 0;

  function scheduleUpdate() {
    if (frame) return;
    frame = requestAnimationFrame(() => {
      frame = 0;
      if (updateWindows()) void settle();
    });
  }

  let resizeObserver: ResizeObserver | null = null;

  watch(
    container,
    (el, _previous, onCleanup) => {
      if (!el) return;
      const found = options.scroller ? options.scroller(el) : el.closest<HTMLElement>(".scroll-wrapper");
      const candidates = Array.isArray(found) ? found : found ? [found] : [];
      scrollCandidates = candidates;
      unsupported.value = candidates.length === 0;
      unmeasurable.value = false;
      metrics.value = null;
      containerWidth.value = Math.round(el.clientWidth);

      let lastHeight = el.clientHeight;
      const onResize = () => {
        containerWidth.value = Math.round(el.clientWidth);
        // A list that could not be measured is tried again as soon as it is shown
        if (unmeasurable.value && el.clientWidth > 0) {
          if (retries++ < MAX_MEASURE_RETRIES) unmeasurable.value = false;
          return;
        }
        // The rows can change size after they were measured (fonts, icons and thumbnails loading), then every
        // position worked out from the old size is wrong. Sub pixel noise from the spacers is ignored.
        const height = el.clientHeight;
        const resized = Math.abs(height - lastHeight) > 1;
        lastHeight = height;
        if (resized) void settle();
        else scheduleUpdate();
      };
      resizeObserver = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(onResize);
      resizeObserver?.observe(el);
      for (const candidate of candidates) {
        resizeObserver?.observe(candidate);
        candidate.addEventListener("scroll", scheduleUpdate, { passive: true });
      }
      void settle();

      onCleanup(() => {
        resizeObserver?.disconnect();
        resizeObserver = null;
        for (const candidate of candidates) candidate.removeEventListener("scroll", scheduleUpdate);
        scrollCandidates = [];
      });
    },
    { immediate: true, flush: "post" },
  );

  let knownCounts = new Map(gridGroups.value.map((g) => [g.key, g.count] as const));

  function takeChangedGroups(): Set<string> {
    const changed = new Set<string>();
    const counts = new Map<string, number>();
    for (const g of gridGroups.value) {
      counts.set(g.key, g.count);
      if (knownCounts.get(g.key) !== g.count) changed.add(g.key);
    }
    knownCounts = counts;
    return changed;
  }

  // New items (sorting, deleting, new search)
  watch(
    [gridGroups, active],
    () => {
      const changed = takeChangedGroups();
      if (changed.size > 0) updateWindows({ force: true, only: changed });
      void settle();
    },
    { flush: "post" },
  );

  // Another view mode, gallery size...
  watch(
    layoutKey,
    () => void settle(),
    { flush: "post" },
  );

  onBeforeUnmount(() => {
    unregister();
    if (frame) cancelAnimationFrame(frame);
  });

  return {
    active,
    settled,
    group,
    columns,
    handle,
    scrollToIndex,
    refresh: settle,
  };
}
