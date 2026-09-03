/**
 * Height/opacity expand transition hooks (same pattern as ContextMenu).
 * @param {HTMLElement} el
 */
export function expandBeforeEnter(el) {
  el.style.height = "0";
  el.style.opacity = "0";
}

/**
 * @param {HTMLElement} el
 * @param {() => void} done
 * @param {number} [durationMs=300]
 * @param {object} [options]
 * @param {(fullHeight: number, fullWidth: number) => number|null|void} [options.getMaxHeight]
 * @param {(fullHeight: number, fullWidth: number) => void} [options.onMeasured]
 */
export function expandEnter(el, done, durationMs = 300, options = {}) {
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

  const maxHeight = getMaxHeight ? getMaxHeight(fullHeight, fullWidth) : null;
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

/**
 * @param {HTMLElement} el
 * @param {() => void} done
 * @param {number} [durationMs=300]
 */
export function expandLeave(el, done, durationMs = 300) {
  el.style.transition = `height ${durationMs}ms, opacity ${durationMs}ms`;
  el.style.height = `${el.scrollHeight}px`;
  void el.offsetHeight;
  el.style.height = "0";
  el.style.opacity = "0";
  setTimeout(done, durationMs);
}
