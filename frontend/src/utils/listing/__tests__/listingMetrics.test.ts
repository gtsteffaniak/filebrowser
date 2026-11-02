import { describe, expect, it } from "vitest";
import { measureGroup, pickMetrics, sameMetrics } from "../listingMetrics.ts";

type Box = { left: number; top: number; width: number; height: number };

function rect(box: Box): DOMRect {
  return {
    ...box,
    x: box.left,
    y: box.top,
    right: box.left + box.width,
    bottom: box.top + box.height,
    toJSON: () => ({}),
  } as DOMRect;
}

function place(el: HTMLElement, box: Box) {
  el.getBoundingClientRect = () => rect(box);
}

function makeGroup(opts: {
  count: number;
  columns: number;
  item?: { width: number; height: number };
  gapX?: number;
  gapY?: number;
  padding?: number;
  origin?: { left: number; top: number };
  style?: Partial<CSSStyleDeclaration>;
  itemStyle?: Partial<CSSStyleDeclaration>;
}) {
  const { count, columns } = opts;
  const item = opts.item ?? { width: 100, height: 40 };
  const gapX = opts.gapX ?? 0;
  const gapY = opts.gapY ?? 0;
  const padding = opts.padding ?? 0;
  const origin = opts.origin ?? { left: 20, top: 300 };

  const group = document.createElement("div");
  Object.assign(group.style, opts.style ?? {});
  group.style.paddingTop = `${padding}px`;
  place(group, { ...origin, width: 800, height: 1000 });
  for (let i = 0; i < count; i++) {
    const el = document.createElement("a");
    el.className = "listing-item";
    Object.assign(el.style, opts.itemStyle ?? {});
    place(el, {
      left: origin.left + padding + (i % columns) * (item.width + gapX),
      top: origin.top + padding + Math.floor(i / columns) * (item.height + gapY),
      ...item,
    });
    group.appendChild(el);
  }
  return group;
}

describe("measureGroup", () => {
  it("measures rows with another class when asked to", () => {
    const group = makeGroup({ count: 4, columns: 1 });
    for (const el of group.querySelectorAll("a")) el.className = "queue-item";
    expect(measureGroup(group)).toBeNull();
    expect(measureGroup(group, "queue-item")?.columns).toBe(1);
  });

  it("reads columns and row pitch of a grid", () => {
    const group = makeGroup({ count: 11, columns: 4, item: { width: 100, height: 40 }, gapX: 10, gapY: 12, padding: 8 });
    expect(measureGroup(group)).toEqual({
      columns: 4,
      rowPitch: 52,
      colPitch: 110,
      itemWidth: 100,
      itemHeight: 40,
      offsetX: 8,
      offsetY: 0,
      insetTop: 8,
      rowGap: 0,
    });
  });

  it("treats a list as a single column", () => {
    const group = makeGroup({ count: 5, columns: 1, item: { width: 700, height: 53 } });
    const metrics = measureGroup(group);
    expect(metrics?.columns).toBe(1);
    expect(metrics?.rowPitch).toBe(53);
  });

  it("counts only the first row, so a partially rendered last row does not matter", () => {
    const group = makeGroup({ count: 9, columns: 4 });
    expect(measureGroup(group)?.columns).toBe(4);
  });

  it("falls back to the item size plus margins and gaps when there is a single row", () => {
    const group = makeGroup({
      count: 3,
      columns: 5,
      item: { width: 100, height: 40 },
      style: { rowGap: "10px", columnGap: "6px" },
      itemStyle: { marginTop: "4px", marginBottom: "4px", marginLeft: "2px", marginRight: "2px" },
    });
    const metrics = measureGroup(group);
    expect(metrics?.columns).toBe(3);
    expect(metrics?.rowPitch).toBe(40 + 8 + 10);
    expect(metrics?.rowGap).toBe(10);
    expect(metrics?.colPitch).toBe(100);
  });

  it("uses the margins and gap for the column pitch of a lone item", () => {
    const group = makeGroup({
      count: 1,
      columns: 1,
      item: { width: 100, height: 40 },
      style: { columnGap: "6px" },
      itemStyle: { marginLeft: "2px", marginRight: "2px" },
    });
    expect(measureGroup(group)?.colPitch).toBe(100 + 4 + 6);
  });

  it("tolerates items of one row that are a few pixels apart", () => {
    const group = makeGroup({ count: 6, columns: 3 });
    const items = group.querySelectorAll<HTMLElement>(".listing-item");
    const second = items[1] as HTMLElement;
    const box = second.getBoundingClientRect();
    place(second, { left: box.left, top: box.top - 2, width: box.width, height: box.height });
    expect(measureGroup(group)?.columns).toBe(3);
  });

  it("averages the pitch over every rendered row when the rows are not the same height", () => {
    // Rows of 40, 100, 40 and 100 pixels one after another: the first two rows alone would say 40
    const heights = [40, 100, 40, 100, 40];
    const group = document.createElement("div");
    place(group, { left: 20, top: 300, width: 800, height: 1000 });
    let top = 300;
    for (const height of heights) {
      for (let col = 0; col < 3; col++) {
        const el = document.createElement("a");
        el.className = "listing-item";
        place(el, { left: 20 + col * 100, top, width: 100, height });
        group.appendChild(el);
      }
      top += height;
    }
    const metrics = measureGroup(group);
    expect(metrics?.columns).toBe(3);
    expect(metrics?.rowPitch).toBe(70);
  });

  it("ignores children that are not items", () => {
    const group = makeGroup({ count: 4, columns: 2 });
    const spacer = document.createElement("div");
    group.insertBefore(spacer, group.firstChild);
    expect(measureGroup(group)?.columns).toBe(2);
  });

  it("returns null when nothing is laid out", () => {
    expect(measureGroup(document.createElement("div"))).toBeNull();
    const group = makeGroup({ count: 3, columns: 3, item: { width: 0, height: 0 } });
    expect(measureGroup(group)).toBeNull();
  });
});

describe("pickMetrics / sameMetrics", () => {
  const base = {
    columns: 3,
    rowPitch: 50,
    colPitch: 100,
    itemWidth: 90,
    itemHeight: 40,
    offsetX: 0,
    offsetY: 0,
    insetTop: 0,
    rowGap: 0,
  };

  it("prefers the group that showed the most columns", () => {
    expect(pickMetrics([null, { ...base, columns: 2 }, { ...base, columns: 5 }])?.columns).toBe(5);
    expect(pickMetrics([null, null])).toBeNull();
  });

  it("ignores sub pixel differences", () => {
    expect(sameMetrics(base, { ...base, rowPitch: 50.2 })).toBe(true);
    expect(sameMetrics(base, { ...base, rowPitch: 51 })).toBe(false);
    expect(sameMetrics(base, { ...base, columns: 4 })).toBe(false);
    expect(sameMetrics(null, null)).toBe(true);
    expect(sameMetrics(base, null)).toBe(false);
  });
});
