import { globalVars } from "@/utils/constants.js";

export function defaultDarkMode() {
  return globalVars.darkMode === true;
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
