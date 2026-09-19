import { getters } from "@/store";

/** Chromium/Android OOPIF PDF behavior (#2081); Firefox/Safari excluded. */
export function isChromiumBasedBrowser() {
  const ua = navigator.userAgent;
  if (/Firefox|FxiOS/i.test(ua)) {
    return false;
  }
  if (/Safari/i.test(ua) && !/Chrome|CriOS|Chromium|EdgA|EdgiOS/i.test(ua)) {
    return false;
  }
  return /Chrome|Chromium|EdgA|EdgiOS|OPR|SamsungBrowser/i.test(ua);
}

/** Skip in-app PDF iframe on mobile Chromium; show open/download fallback instead. */
export function shouldUsePdfPreviewFallback() {
  return getters.isMobile() && isChromiumBasedBrowser();
}
