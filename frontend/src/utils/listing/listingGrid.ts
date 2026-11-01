// Pure maths for the listing grid without DOM, so it works for items that are not rendered..

export interface GridGroupInput {
  key: string;
  count: number;
}

export interface GridGroup extends GridGroupInput {
  start: number;
}

export interface GridPosition {
  group: number;
  local: number;
  row: number;
  col: number;
}

export interface RowWindow {
  firstRow: number;
  endRow: number;
}

export function buildGridGroups(inputs: GridGroupInput[]): GridGroup[] {
  const groups: GridGroup[] = [];
  let start = 0;
  for (const input of inputs) {
    groups.push({ key: input.key, count: input.count, start });
    start += input.count;
  }
  return groups;
}

export function rowCount(count: number, columns: number): number {
  if (count <= 0) return 0;
  return Math.ceil(count / Math.max(1, columns));
}

/** How many items the given row of a group has */
export function itemsInRow(count: number, columns: number, row: number): number {
  const cols = Math.max(1, columns);
  return Math.max(0, Math.min(cols, count - row * cols));
}

export function locateFlat(groups: GridGroup[], flat: number, columns: number): GridPosition | null {
  if (flat < 0) return null;
  const cols = Math.max(1, columns);
  for (let group = 0; group < groups.length; group++) {
    const g = groups[group] as GridGroup;
    if (flat >= g.start && flat < g.start + g.count) {
      const local = flat - g.start;
      return { group, local, row: Math.floor(local / cols), col: local % cols };
    }
  }
  return null;
}

export function moveVertical(
  groups: GridGroup[],
  columns: number,
  flat: number,
  direction: -1 | 1,
): number | null {
  const cols = Math.max(1, columns);
  const at = locateFlat(groups, flat, cols);
  if (!at) return null;
  const current = groups[at.group] as GridGroup;

  if (direction === 1) {
    const rows = rowCount(current.count, cols);
    if (at.row + 1 < rows) {
      const col = Math.min(at.col, itemsInRow(current.count, cols, at.row + 1) - 1);
      return current.start + (at.row + 1) * cols + col;
    }
    for (let g = at.group + 1; g < groups.length; g++) {
      const next = groups[g] as GridGroup;
      if (next.count === 0) continue;
      return next.start + Math.min(at.col, itemsInRow(next.count, cols, 0) - 1);
    }
    return null;
  }

  if (at.row > 0) {
    return current.start + (at.row - 1) * cols + at.col;
  }
  for (let g = at.group - 1; g >= 0; g--) {
    const previous = groups[g] as GridGroup;
    if (previous.count === 0) continue;
    const lastRow = rowCount(previous.count, cols) - 1;
    const col = Math.min(at.col, itemsInRow(previous.count, cols, lastRow) - 1);
    return previous.start + lastRow * cols + col;
  }
  return null;
}

/** The rows of a group overlapping the viewport grown by `overscan` px. `firstRowTop` shares the viewport's coordinates. */
export function visibleRows(args: {
  firstRowTop: number;
  rowPitch: number;
  rows: number;
  viewportTop: number;
  viewportBottom: number;
  overscan?: number;
}): RowWindow {
  const { firstRowTop, rowPitch, rows } = args;
  const overscan = args.overscan ?? 0;
  if (rows <= 0 || rowPitch <= 0) return { firstRow: 0, endRow: 0 };
  const top = args.viewportTop - overscan;
  const bottom = args.viewportBottom + overscan;
  const first = Math.floor((top - firstRowTop) / rowPitch);
  const end = Math.ceil((bottom - firstRowTop) / rowPitch);
  if (end <= 0) return { firstRow: 0, endRow: 0 }; // everything is below the viewport
  if (first >= rows) return { firstRow: rows, endRow: rows }; // everything is above it
  return {
    firstRow: Math.max(0, first),
    endRow: Math.min(rows, Math.max(end, first + 1)),
  };
}

/** True when `current` covers `needed` with `bufferRows` to spare on each side (unless the group ends), to avoid re-rendering on every scroll */
export function windowCovers(current: RowWindow, needed: RowWindow, rows: number, bufferRows: number): boolean {
  if (current.endRow <= current.firstRow) return false;
  const topOk = current.firstRow === 0 || needed.firstRow - current.firstRow >= bufferRows;
  const bottomOk = current.endRow >= rows || current.endRow - needed.endRow >= bufferRows;
  return topOk && bottomOk;
}

export function clampWindow(window: RowWindow, rows: number): RowWindow {
  const firstRow = Math.max(0, Math.min(window.firstRow, rows));
  const endRow = Math.max(firstRow, Math.min(window.endRow, rows));
  return { firstRow, endRow };
}

export function sameWindow(a: RowWindow | undefined, b: RowWindow | undefined): boolean {
  return !!a && !!b && a.firstRow === b.firstRow && a.endRow === b.endRow;
}

/** The items (as positions inside the group) of a row window */
export function itemRangeOfRows(window: RowWindow, columns: number, count: number): { start: number; end: number } {
  const cols = Math.max(1, columns);
  return {
    start: Math.min(count, window.firstRow * cols),
    end: Math.min(count, window.endRow * cols),
  };
}

export interface HitRect {
  left: number;
  top: number;
  right: number;
  bottom: number;
}

export interface HitGroup {
  count: number;
  firstRowTop: number;
  left: number;
}

export interface HitMetrics {
  columns: number;
  rowPitch: number;
  colPitch: number;
  itemWidth: number;
  itemHeight: number;
  offsetX: number;
  offsetY: number;
}

/** Flat positions of the items whose box overlaps the rectangle selection in order */
export function hitTest(groups: GridGroup[], hitGroups: HitGroup[], metrics: HitMetrics, rect: HitRect): number[] {
  const cols = Math.max(1, metrics.columns);
  const hits: number[] = [];
  for (let g = 0; g < groups.length; g++) {
    const group = groups[g] as GridGroup;
    const geometry = hitGroups[g];
    if (!geometry || group.count === 0) continue;
    const rows = rowCount(group.count, cols);
    const baseTop = geometry.firstRowTop + metrics.offsetY;
    const baseLeft = geometry.left + metrics.offsetX;

    const rowMin = Math.max(0, Math.floor((rect.top - baseTop - metrics.itemHeight) / metrics.rowPitch) + 1);
    const rowMax = Math.min(rows - 1, Math.ceil((rect.bottom - baseTop) / metrics.rowPitch) - 1);
    const colMin = Math.max(0, Math.floor((rect.left - baseLeft - metrics.itemWidth) / metrics.colPitch) + 1);
    const colMax = Math.min(cols - 1, Math.ceil((rect.right - baseLeft) / metrics.colPitch) - 1);

    for (let row = rowMin; row <= rowMax; row++) {
      const top = baseTop + row * metrics.rowPitch;
      if (!(top < rect.bottom && top + metrics.itemHeight > rect.top)) continue;
      for (let col = colMin; col <= colMax; col++) {
        const local = row * cols + col;
        if (local >= group.count) break;
        const left = baseLeft + col * metrics.colPitch;
        if (left < rect.right && left + metrics.itemWidth > rect.left) {
          hits.push(group.start + local);
        }
      }
    }
  }
  return hits;
}
