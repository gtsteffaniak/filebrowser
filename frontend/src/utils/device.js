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
  const width = window.innerWidth;
  const height = window.innerHeight;
  const minSide = Math.min(width, height);
  const maxSide = Math.max(width, height);

  if (width <= MOBILE_LAYOUT_MAX_PX) {
    return true;
  }
  // Landscape phones: short side ≤768 but width already >768 (CSS width breakpoint alone misses these).
  if (minSide <= MOBILE_LAYOUT_MAX_PX && maxSide <= TOUCH_MOBILE_LAYOUT_MAX_PX) {
    return true;
  }
  // Touch tablets in landscape (e.g. 1100×900): min side between 769 and 1024.
  if (
    hasCoarsePointer() &&
    minSide > MOBILE_LAYOUT_MAX_PX &&
    minSide <= TOUCH_MOBILE_LAYOUT_MAX_PX
  ) {
    return true;
  }
  return false;
}
