import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { globalVarsMock } = vi.hoisted(() => ({
  globalVarsMock: { darkMode: false },
}));

vi.mock('@/utils/constants', () => ({
  globalVars: globalVarsMock,
}));

import { defaultDarkMode, normalizeDarkMode, syncDocumentTheme, systemPrefersDark } from './theme.js';

describe('theme utils', () => {
  beforeEach(() => {
    globalVarsMock.darkMode = false;
    document.documentElement.className = '';
    document.documentElement.style.setProperty('--background', '#f5f5f5');
    document.head.innerHTML = '<meta name="theme-color" content="#000000">';
  });

  afterEach(() => {
    document.documentElement.className = '';
    document.head.innerHTML = '';
  });

  it('defaultDarkMode returns true only when globalVars.darkMode is true', () => {
    globalVarsMock.darkMode = false;
    expect(defaultDarkMode()).toBe(false);

    globalVarsMock.darkMode = true;
    expect(defaultDarkMode()).toBe(true);

    globalVarsMock.darkMode = null;
    expect(defaultDarkMode()).toBe(false);
  });

  it('syncDocumentTheme toggles html class and updates theme-color from --background', () => {
    document.documentElement.style.setProperty('--background', '#141D24');

    syncDocumentTheme(true);
    expect(document.documentElement.classList.contains('dark-mode')).toBe(true);
    expect(document.querySelector('meta[name="theme-color"]').getAttribute('content')).toBe('#141D24');

    document.documentElement.style.setProperty('--background', '#f5f5f5');
    syncDocumentTheme(false);
    expect(document.documentElement.classList.contains('dark-mode')).toBe(false);
    expect(document.querySelector('meta[name="theme-color"]').getAttribute('content')).toBe('#f5f5f5');
  });
});

describe('auto dark mode support', () => {
  beforeEach(() => {
    globalVarsMock.darkMode = false;
  });

  it('defaultDarkMode resolves string values from globalVars', () => {
    globalVarsMock.darkMode = 'dark';
    expect(defaultDarkMode()).toBe(true);

    globalVarsMock.darkMode = 'light';
    expect(defaultDarkMode()).toBe(false);

    globalVarsMock.darkMode = true;
    expect(defaultDarkMode()).toBe(true);

    globalVarsMock.darkMode = false;
    expect(defaultDarkMode()).toBe(false);
  });

  it('defaultDarkMode with "auto" follows matchMedia', () => {
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: true })));
    globalVarsMock.darkMode = 'auto';
    expect(defaultDarkMode()).toBe(true);

    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false })));
    expect(defaultDarkMode()).toBe(false);

    expect(systemPrefersDark()).toBe(false);
    vi.unstubAllGlobals();
  });

  it('systemPrefersDark returns false when matchMedia is unavailable', () => {
    vi.stubGlobal('matchMedia', undefined);
    expect(systemPrefersDark()).toBe(false);
    vi.unstubAllGlobals();
  });

  it('normalizeDarkMode maps legacy booleans and passes through strings', () => {
    expect(normalizeDarkMode(true)).toBe('dark');
    expect(normalizeDarkMode(false)).toBe('light');
    expect(normalizeDarkMode('auto')).toBe('auto');
    expect(normalizeDarkMode('dark')).toBe('dark');
    expect(normalizeDarkMode('light')).toBe('light');
    expect(normalizeDarkMode(undefined)).toBe('dark');
  });
});
