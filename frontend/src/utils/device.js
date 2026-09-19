/** Layout breakpoint (matches CSS `max-width: 768px` conventions). */
export const MOBILE_LAYOUT_MAX_PX = 768;

/** Wider cap for touch/coarse-pointer devices in landscape (phones, small tablets). */
const TOUCH_MOBILE_LAYOUT_MAX_PX = 1024;

function hasCoarsePointer() {
  if (typeof window !== "undefined" && typeof window.matchMedia === "function") {
    if (window.matchMedia("(pointer: coarse)").matches) {
      return true;
    }
    if (window.matchMedia("(any-pointer: coarse)").matches) {
      return true;
    }
  }
  return typeof navigator !== "undefined" && navigator.maxTouchPoints > 0;
}

/**
 * Whether the UI should use the mobile layout.
 * Uses the shorter viewport side so landscape phones stay mobile; touch devices
 * get a slightly wider landscape cap so PDF preview fallback stays consistent.
 */
export function computeIsMobileLayout() {
  if (typeof window === "undefined") {
    return false;
  }
  const minSide = Math.min(window.innerWidth, window.innerHeight);
  if (minSide <= MOBILE_LAYOUT_MAX_PX) {
    return true;
  }
  if (hasCoarsePointer() && minSide <= TOUCH_MOBILE_LAYOUT_MAX_PX) {
    return true;
  }
  return false;
}
