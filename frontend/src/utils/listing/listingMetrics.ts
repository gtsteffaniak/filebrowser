
export interface LayoutMetrics {
  columns: number;
  /** Distance between the tops of two consecutive rows (item height + vertical margins and gaps) */
  rowPitch: number;
  /** Distance between the left edges of two consecutive columns */
  colPitch: number;
  itemWidth: number;
  itemHeight: number;
  offsetX: number;
  offsetY: number;
  insetTop: number;
  rowGap: number;
}

const MIN_ROW_TOLERANCE = 4; // Rows of the same grid line up, anything further apart than this is another row
const MAX_SCANNED_ITEMS = 300; // to never walk the whole list, one full row and the start of the next is all we need

export function px(value: string | undefined): number {
  const parsed = Number.parseFloat(value ?? "");
  return Number.isFinite(parsed) ? parsed : 0;
}

/** Null when nothing loaded yet */
export function measureGroup(group: HTMLElement, itemClass = "listing-item"): LayoutMetrics | null {
  const children = group.children;
  let first: HTMLElement | null = null;
  for (let i = 0; i < children.length; i++) {
    const child = children[i] as HTMLElement;
    if (child.classList.contains(itemClass)) {
      first = child;
      break;
    }
  }
  if (!first) return null;

  const firstRect = first.getBoundingClientRect();
  if (firstRect.width === 0 && firstRect.height === 0) return null;

  const groupRect = group.getBoundingClientRect();
  const groupStyle = getComputedStyle(group);
  const itemStyle = getComputedStyle(first);
  const insetTop = px(groupStyle.borderTopWidth) + px(groupStyle.paddingTop);

  const tolerance = Math.max(MIN_ROW_TOLERANCE, firstRect.height / 2);
  let columns = 1;
  let secondColumnLeft: number | null = null;
  let onFirstRow = true;
  const rowTops: number[] = [firstRect.top];
  let seen = 0;
  for (let i = 0; i < children.length && seen < MAX_SCANNED_ITEMS; i++) {
    const child = children[i] as HTMLElement;
    if (child === first || !child.classList.contains(itemClass)) continue;
    seen++;
    const rect = child.getBoundingClientRect();
    if (onFirstRow && Math.abs(rect.top - firstRect.top) <= tolerance) {
      columns++;
      if (secondColumnLeft === null) secondColumnLeft = rect.left;
      continue;
    }
    onFirstRow = false;
    if (rect.top - (rowTops.at(-1) as number) > tolerance) rowTops.push(rect.top);
  }

  const rowGap = px(groupStyle.rowGap);
  const columnGap = px(groupStyle.columnGap);
  const rowPitch =
    rowTops.length > 1
      ? ((rowTops.at(-1) as number) - firstRect.top) / (rowTops.length - 1)
      : firstRect.height + px(itemStyle.marginTop) + px(itemStyle.marginBottom) + rowGap;
  const colPitch =
    secondColumnLeft !== null
      ? secondColumnLeft - firstRect.left
      : firstRect.width + px(itemStyle.marginLeft) + px(itemStyle.marginRight) + columnGap;

  if (!(rowPitch > 0)) return null;

  return {
    columns,
    rowPitch,
    colPitch,
    itemWidth: firstRect.width,
    itemHeight: firstRect.height,
    offsetX: firstRect.left - groupRect.left,
    offsetY: firstRect.top - (groupRect.top + insetTop),
    insetTop,
    rowGap,
  };
}

/** Best of the measurements of several groups sharing the same CSS */
export function pickMetrics(measured: Array<LayoutMetrics | null>): LayoutMetrics | null {
  let best: LayoutMetrics | null = null;
  for (const metrics of measured) {
    if (metrics && (!best || metrics.columns > best.columns)) best = metrics;
  }
  return best;
}

export function sameMetrics(a: LayoutMetrics | null, b: LayoutMetrics | null): boolean {
  if (!a || !b) return a === b;
  return (
    a.columns === b.columns &&
    Math.abs(a.rowPitch - b.rowPitch) < 0.5 &&
    Math.abs(a.colPitch - b.colPitch) < 0.5 &&
    Math.abs(a.itemWidth - b.itemWidth) < 0.5 &&
    Math.abs(a.itemHeight - b.itemHeight) < 0.5 &&
    Math.abs(a.offsetX - b.offsetX) < 0.5 &&
    Math.abs(a.offsetY - b.offsetY) < 0.5 &&
    Math.abs(a.insetTop - b.insetTop) < 0.5 &&
    Math.abs(a.rowGap - b.rowGap) < 0.5
  );
}
