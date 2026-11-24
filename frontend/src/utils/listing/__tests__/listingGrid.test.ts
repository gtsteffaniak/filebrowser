import { describe, expect, it } from "vitest";
import {
  buildGridGroups,
  hitTest,
  itemRangeOfRows,
  itemsInRow,
  locateFlat,
  moveVertical,
  rowCount,
  visibleRows,
  windowCovers,
} from "../listingGrid.ts";

// pinned: 2 items, folders: 7, files: 10 -> flat 0-1, 2-8, 9-18
const groups = buildGridGroups([
  { key: "pinned", count: 2 },
  { key: "dirs", count: 7 },
  { key: "files", count: 10 },
]);

describe("grid basics", () => {
  it("puts the groups one after another", () => {
    expect(groups.map((g) => g.start)).toEqual([0, 2, 9]);
  });

  it("counts rows and the items of an incomplete last row", () => {
    expect(rowCount(7, 4)).toBe(2);
    expect(rowCount(0, 4)).toBe(0);
    expect(itemsInRow(7, 4, 0)).toBe(4);
    expect(itemsInRow(7, 4, 1)).toBe(3);
    expect(itemsInRow(7, 4, 2)).toBe(0);
  });

  it("locates a flat position inside its own group", () => {
    expect(locateFlat(groups, 8, 4)).toEqual({ group: 1, local: 6, row: 1, col: 2 });
    expect(locateFlat(groups, 9, 4)).toEqual({ group: 2, local: 0, row: 0, col: 0 });
    expect(locateFlat(groups, 19, 4)).toBeNull();
    expect(locateFlat(groups, -1, 4)).toBeNull();
  });
});

describe("moveVertical", () => {
  it("keeps the column inside a group", () => {
    // files: row 0 col 1 (flat 10) -> row 1 col 1 (flat 14)
    expect(moveVertical(groups, 4, 10, 1)).toBe(14);
    expect(moveVertical(groups, 4, 14, -1)).toBe(10);
  });

  it("goes to the last item of a shorter row", () => {
    // folders: row 0 col 3 (flat 5) -> row 1 only has 3 items -> col 2 (flat 8)
    expect(moveVertical(groups, 4, 5, 1)).toBe(8);
  });

  it("does not continue a row into the next group", () => {
    // folders end with a row of 3. Going down from its col 2 enters files at col 2 (flat 11),
    // it never uses index + columns, which would give flat 12 here
    expect(moveVertical(groups, 4, 8, 1)).toBe(11);
  });

  it("crosses into the previous group's last row", () => {
    // files: row 0 col 3 (flat 12) -> folders last row (3 items) -> col 2 (flat 8)
    expect(moveVertical(groups, 4, 12, -1)).toBe(8);
    // folders row 0 col 0 (flat 2) -> pinned (flat 0)
    expect(moveVertical(groups, 4, 2, -1)).toBe(0);
    // pinned only has 2 items: col 3 is clamped
    expect(moveVertical(groups, 4, 5, -1)).toBe(1);
  });

  it("returns null at both edges", () => {
    expect(moveVertical(groups, 4, 0, -1)).toBeNull();
    // last files row (flat 16-18 has 3 items), nothing below
    expect(moveVertical(groups, 4, 18, 1)).toBeNull();
  });

  it("skips empty groups", () => {
    const withEmpty = buildGridGroups([
      { key: "pinned", count: 0 },
      { key: "dirs", count: 3 },
      { key: "files", count: 0 },
      { key: "other", count: 3 },
    ]);
    expect(moveVertical(withEmpty, 5, 1, 1)).toBe(4);
    expect(moveVertical(withEmpty, 5, 4, -1)).toBe(1);
  });

  it("behaves like a list with one column", () => {
    expect(moveVertical(groups, 1, 3, 1)).toBe(4);
    expect(moveVertical(groups, 1, 3, -1)).toBe(2);
    expect(moveVertical(groups, 1, 8, 1)).toBe(9); // folders -> files
    expect(moveVertical(groups, 1, 9, -1)).toBe(8);
  });
});

describe("visibleRows", () => {
  const base = { firstRowTop: 100, rowPitch: 50, rows: 100 };

  it("finds the rows that overlap the viewport", () => {
    // viewport 300-500 -> rows 4 (top 300) .. 7 (top 450)
    expect(visibleRows({ ...base, viewportTop: 300, viewportBottom: 500 })).toEqual({ firstRow: 4, endRow: 8 });
  });

  it("grows by the overscan and clamps to the group", () => {
    expect(visibleRows({ ...base, viewportTop: 0, viewportBottom: 200, overscan: 100 })).toEqual({
      firstRow: 0,
      endRow: 4,
    });
    expect(visibleRows({ ...base, viewportTop: 5000, viewportBottom: 6000, overscan: 500 })).toEqual({
      firstRow: 88,
      endRow: 100,
    });
  });

  it("collapses a group that is entirely below or above the viewport", () => {
    expect(visibleRows({ ...base, viewportTop: 0, viewportBottom: 90 })).toEqual({ firstRow: 0, endRow: 0 });
    expect(visibleRows({ ...base, viewportTop: 9000, viewportBottom: 9500 })).toEqual({ firstRow: 100, endRow: 100 });
  });

  it("handles an empty group", () => {
    expect(visibleRows({ ...base, rows: 0, viewportTop: 0, viewportBottom: 500 })).toEqual({ firstRow: 0, endRow: 0 });
  });
});

describe("windowCovers", () => {
  it("only asks for a new window when the screen gets close to the edge", () => {
    const current = { firstRow: 10, endRow: 40 };
    expect(windowCovers(current, { firstRow: 20, endRow: 30 }, 100, 3)).toBe(true);
    expect(windowCovers(current, { firstRow: 11, endRow: 30 }, 100, 3)).toBe(false);
    expect(windowCovers(current, { firstRow: 20, endRow: 39 }, 100, 3)).toBe(false);
  });

  it("does not need spare rows where the group ends", () => {
    expect(windowCovers({ firstRow: 0, endRow: 40 }, { firstRow: 0, endRow: 10 }, 100, 3)).toBe(true);
    expect(windowCovers({ firstRow: 60, endRow: 100 }, { firstRow: 90, endRow: 100 }, 100, 3)).toBe(true);
  });

  it("is not satisfied by an empty window", () => {
    expect(windowCovers({ firstRow: 0, endRow: 0 }, { firstRow: 0, endRow: 3 }, 100, 3)).toBe(false);
  });
});

describe("itemRangeOfRows", () => {
  it("converts rows to items and stops at the end of the group", () => {
    expect(itemRangeOfRows({ firstRow: 2, endRow: 4 }, 5, 100)).toEqual({ start: 10, end: 20 });
    expect(itemRangeOfRows({ firstRow: 2, endRow: 4 }, 5, 17)).toEqual({ start: 10, end: 17 });
    expect(itemRangeOfRows({ firstRow: 5, endRow: 5 }, 5, 17)).toEqual({ start: 17, end: 17 });
  });
});

describe("hitTest", () => {
  // 2 groups, 3 columns, boxes 90x40 on a 100x50 pitch
  const metrics = { columns: 3, rowPitch: 50, colPitch: 100, itemWidth: 90, itemHeight: 40, offsetX: 5, offsetY: 5 };
  const two = buildGridGroups([
    { key: "dirs", count: 4 },
    { key: "files", count: 3 },
  ]);
  // folders start at y=0, files at y=200 (2 rows = 100px + a heading)
  const where = [
    { count: 4, firstRowTop: 0, left: 0 },
    { count: 3, firstRowTop: 200, left: 0 },
  ];

  it("selects the boxes that overlap the rectangle, including ones that are not rendered", () => {
    // second column of both rows of the folders: the second row only has one item (column 0)
    expect(hitTest(two, where, metrics, { left: 110, top: 0, right: 150, bottom: 100 })).toEqual([1]);
    // first column of both rows
    expect(hitTest(two, where, metrics, { left: 10, top: 0, right: 50, bottom: 100 })).toEqual([0, 3]);
  });

  it("selects across groups", () => {
    const hits = hitTest(two, where, metrics, { left: 0, top: 0, right: 400, bottom: 400 });
    expect(hits).toEqual([0, 1, 2, 3, 4, 5, 6]);
  });

  it("ignores gaps between boxes and cells past the end of the last row", () => {
    // y 46..54 is the gap between the first and second row of the folders
    expect(hitTest(two, where, metrics, { left: 0, top: 46, right: 400, bottom: 54 })).toEqual([]);
    // 4th folder is alone on the second row: columns 2 and 3 of that row are empty
    expect(hitTest(two, where, metrics, { left: 205, top: 55, right: 280, bottom: 80 })).toEqual([]);
  });

  it("treats touching edges as outside, like the DOM based check", () => {
    // box 0 is x 5..95 / y 5..45
    expect(hitTest(two, where, metrics, { left: 95, top: 0, right: 99, bottom: 400 })).toEqual([]);
    expect(hitTest(two, where, metrics, { left: 0, top: 45, right: 99, bottom: 49 })).toEqual([]);
  });
});
