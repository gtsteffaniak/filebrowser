/**
 * Trailing-edge debounce: `fn` runs once, `wait` ms after the last call, with the
 * arguments from that last call. The returned function exposes `cancel()` to drop
 * a pending invocation (e.g. when a search is closed before it fires).
 */
export function debounce(fn, wait) {
  let timer = null;
  const debounced = (...args) => {
    if (timer !== null) {
      clearTimeout(timer);
    }
    timer = setTimeout(() => {
      timer = null;
      fn(...args);
    }, wait);
  };
  debounced.cancel = () => {
    if (timer !== null) {
      clearTimeout(timer);
      timer = null;
    }
  };
  return debounced;
}
