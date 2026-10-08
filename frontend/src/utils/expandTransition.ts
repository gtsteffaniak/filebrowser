/* Height/opacity expand transition hooks */

export interface ExpandEnterOptions {
  /** Return a max height (px) to make the element scrollable */
  getMaxHeight?: (fullHeight: number, fullWidth: number) => number | null | undefined;
  /** Called with the measured size before the animation starts */
  onMeasured?: (fullHeight: number, fullWidth: number) => void;
}

export function expandBeforeEnter(node: Element): void {
  const el = node as HTMLElement;
  el.style.height = "0";
  el.style.opacity = "0";
}

export function expandEnter(
  node: Element,
  done: () => void,
  durationMs = 300,
  options: ExpandEnterOptions = {},
): void {
  const el = node as HTMLElement;
  const { getMaxHeight, onMeasured } = options;
  el.style.transition = "";
  el.style.height = "0";
  el.style.opacity = "0";
  void el.offsetHeight;
  el.style.height = "auto";
  el.style.visibility = "hidden";
  void el.offsetHeight;
  const fullHeight = el.scrollHeight;
  const fullWidth = el.scrollWidth;

  if (onMeasured) onMeasured(fullHeight, fullWidth);

  const result = getMaxHeight ? getMaxHeight(fullHeight, fullWidth) : null;
  const maxHeight = typeof result === "number" ? result : null;
  const scrollable = maxHeight !== null && fullHeight > maxHeight;
  const targetHeight = scrollable ? maxHeight : fullHeight;
  el.style.overflowY = scrollable ? "auto" : "";
  el.style.overflowX = scrollable ? "hidden" : "";
  el.style.justifyContent = scrollable ? "flex-start" : "";

  el.style.height = "0";
  el.style.visibility = "visible";
  el.style.transition = `height ${durationMs}ms, opacity ${durationMs}ms`;
  void el.offsetHeight;
  el.style.height = `${targetHeight}px`;
  el.style.opacity = "1";
  setTimeout(done, durationMs);
}

export function expandLeave(node: Element, done: () => void, durationMs = 300): void {
  const el = node as HTMLElement;
  el.style.transition = `height ${durationMs}ms, opacity ${durationMs}ms`;
  el.style.height = `${el.scrollHeight}px`;
  void el.offsetHeight;
  el.style.height = "0";
  el.style.opacity = "0";
  setTimeout(done, durationMs);
}
