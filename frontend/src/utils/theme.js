import { globalVars } from '@/utils/constants';

function halloweenPatternUrl(filename) {
  const base = globalVars.baseURL || '/';
  const normalized = base.endsWith('/') ? base : `${base}/`;
  return `url("${normalized}public/static/img/${filename}")`;
}

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
  const root = document.documentElement;
  if (active) {
    root.style.setProperty('--halloween-pattern-dark', halloweenPatternUrl('halloween-pattern.svg'));
    root.style.setProperty('--halloween-pattern-light', halloweenPatternUrl('halloween-pattern-light.svg'));
  } else {
    root.style.removeProperty('--halloween-pattern-dark');
    root.style.removeProperty('--halloween-pattern-light');
  }
  const userTheme = document.getElementById('user-selected-theme');
  if (userTheme) {
    userTheme.disabled = active;
  }
}
