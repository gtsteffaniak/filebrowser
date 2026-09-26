import { shallowRef } from "vue";

/** Layout breakpoint (matches CSS `max-width: 768px` conventions). */
export const MOBILE_LAYOUT_MAX_PX = 768;

const mobileLayoutMediaQuery = `(max-width: ${MOBILE_LAYOUT_MAX_PX}px)`;

/**
 * Mobile layout flag for JS (sidebar, context menu, etc.).
 * Updated only when matchMedia crosses the breakpoint — not on every resize pixel.
 * Read via getters.isMobile().
 */
export const isMobileLayout = shallowRef(false);

export function isMobileLayoutWidth(width) {
  return width <= MOBILE_LAYOUT_MAX_PX;
}

let viewportListenerBound = false;
let mobileLayoutMql = null;
let onMobileLayoutChange = null;

export function initViewportLayoutListener() {
  if (viewportListenerBound || typeof window === "undefined") {
    return;
  }
  viewportListenerBound = true;

  mobileLayoutMql = window.matchMedia(mobileLayoutMediaQuery);
  isMobileLayout.value = mobileLayoutMql.matches;

  onMobileLayoutChange = (event) => {
    if (event.matches !== isMobileLayout.value) {
      isMobileLayout.value = event.matches;
    }
  };
  mobileLayoutMql.addEventListener("change", onMobileLayoutChange);
}

/** @internal resets listener state between unit tests */
export function resetViewportLayoutForTests() {
  if (mobileLayoutMql && onMobileLayoutChange) {
    mobileLayoutMql.removeEventListener("change", onMobileLayoutChange);
  }
  viewportListenerBound = false;
  mobileLayoutMql = null;
  onMobileLayoutChange = null;
}
