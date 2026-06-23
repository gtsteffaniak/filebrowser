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
 */
export function expandEnter(el, done) {
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
  el.style.transition = "height 0.3s, opacity 0.3s";
  void el.offsetHeight;
  el.style.height = `${fullHeight}px`;
  el.style.opacity = "1";
  setTimeout(done, 300);
}

/**
 * @param {HTMLElement} el
 * @param {() => void} done
 */
export function expandLeave(el, done) {
  el.style.transition = "height 0.3s, opacity 0.3s";
  el.style.height = `${el.scrollHeight}px`;
  void el.offsetHeight;
  el.style.height = "0";
  el.style.opacity = "0";
  setTimeout(done, 300);
}
