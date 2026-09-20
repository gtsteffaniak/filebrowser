/**
 * Single IntersectionObserver for listing thumbnails.
 * observe() is synchronous in mounted() — do not rAF-batch or flush after
 * Vue has created all rows (both patterns previously blew up load time).
 */

const ROOT_MARGIN = "500px";

let observer = null;
/** @type {Map<Element, { onIntersect: (visible: boolean) => void }>} */
const targets = new Map();

function ensureObserver() {
  if (observer) return observer;
  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (!entry.isIntersecting) continue;
        const rec = targets.get(entry.target);
        if (rec) rec.onIntersect(true);
      }
    },
    { root: null, rootMargin: ROOT_MARGIN, threshold: 0 },
  );
  return observer;
}

/**
 * @param {Element} el
 * @param {(visible: boolean) => void} onIntersect
 */
export function registerListingVisibilityTarget(el, onIntersect) {
  if (!el || !(el instanceof Element)) return;
  targets.set(el, { onIntersect });
  ensureObserver().observe(el);
}

export function unregisterListingVisibilityTarget(el) {
  if (!el) return;
  targets.delete(el);
  if (observer) observer.unobserve(el);
}

export function disconnectListingVisibilityObserver() {
  targets.clear();
  if (observer) {
    observer.disconnect();
    observer = null;
  }
}
