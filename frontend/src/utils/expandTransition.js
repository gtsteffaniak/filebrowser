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
 */
export function expandEnter(el, done, durationMs = 300) {
  el.style.transition = "";
  el.style.height = "0";
  el.style.opacity = "0";
  void el.offsetHeight;
  el.style.height = "auto";
  el.style.visibility = "hidden";
  void el.offsetHeight;
  const fullHeight = el.scrollHeight;
  el.style.height = "0";
  el.style.visibility = "visible";
  el.style.transition = `height ${durationMs}ms, opacity ${durationMs}ms`;
  void el.offsetHeight;
  el.style.height = `${fullHeight}px`;
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
