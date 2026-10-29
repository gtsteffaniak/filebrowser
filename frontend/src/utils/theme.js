import { globalVars } from "@/utils/constants.js";

export const DARK_MODE_AUTO = "auto";
export const DARK_MODE_DARK = "dark";
export const DARK_MODE_LIGHT = "light";

/**
 * Resolves the server-provided default (globalVars.darkMode) to a boolean.
 * Accepts legacy booleans as well as the "auto" / "dark" / "light" strings;
 * "auto" follows the client system preference.
 */
export function defaultDarkMode() {
  return resolveDarkMode(globalVars.darkMode, false);
}

/** Resolves a darkMode preference value (boolean or string) to a boolean. */
export function resolveDarkMode(value, fallback = false) {
  if (value === true || value === DARK_MODE_DARK) {
    return true;
  }
  if (value === false || value === DARK_MODE_LIGHT) {
    return false;
  }
  if (value === DARK_MODE_AUTO) {
    return systemPrefersDark();
  }
  return fallback;
}

/** Reports whether the client system currently prefers dark mode. */
export function systemPrefersDark() {
  if (typeof window === "undefined" || !window.matchMedia) {
    return false;
  }
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

/**
 * Watches for system color-scheme changes and invokes the callback with the
 * new boolean preference. Returns a cleanup function.
 */
export function watchSystemTheme(callback) {
  if (typeof window === "undefined" || !window.matchMedia) {
    return () => {};
  }
  const mql = window.matchMedia("(prefers-color-scheme: dark)");
  const handler = (event) => callback(event.matches);
  if (mql.addEventListener) {
    mql.addEventListener("change", handler);
    return () => mql.removeEventListener("change", handler);
  }
  // Safari < 14 fallback
  mql.addListener(handler);
  return () => mql.removeListener(handler);
}

/** Normalizes any accepted darkMode value to the "auto" / "dark" / "light" string form. */
export function normalizeDarkMode(value) {
  if (value === DARK_MODE_AUTO || value === DARK_MODE_DARK || value === DARK_MODE_LIGHT) {
    return value;
  }
  if (value === false) return DARK_MODE_LIGHT;
  // true and unset/unknown values map to the historical default (dark).
  return DARK_MODE_DARK;
}

export function syncDocumentTheme(dark) {
  document.documentElement.classList.toggle('dark-mode', dark);
  document.documentElement.style.colorScheme = dark ? 'dark' : 'light';
  const meta = document.querySelector('meta[name="theme-color"]');
  if (!meta) {
    return;
  }
  const bg = getComputedStyle(document.documentElement).getPropertyValue('--background').trim();
  if (bg) {
    meta.setAttribute('content', bg);
  }
}

export function syncEventTheme(active) {
  document.body.classList.toggle('halloween-theme', active);
  document.documentElement.classList.toggle('halloween-theme', active);
  const userTheme = document.getElementById('user-selected-theme');
  if (userTheme) {
    userTheme.disabled = active;
  }
}
