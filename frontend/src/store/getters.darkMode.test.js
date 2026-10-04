import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { stateMock, globalVarsMock } = vi.hoisted(() => ({
  stateMock: {
    user: { username: '', darkMode: true },
    shareInfo: null,
    disableEventThemes: false,
    route: { path: '/login' },
  },
  globalVarsMock: { darkMode: false, noAuth: false, eventBasedThemes: false },
}));

vi.mock('./state', () => ({ state: stateMock }));
vi.mock('./mutations', () => ({
  mutations: {
    updateCurrentUser: vi.fn(),
  },
}));
vi.mock('@/utils/constants', () => ({ globalVars: globalVarsMock, previewViews: [], tools: () => [] }));
vi.mock('@/i18n', () => ({
  default: { global: { t: (key) => key } },
  detectLocale: () => 'en',
}));
vi.mock('@/utils/url.js', () => ({
  buildItemUrl: vi.fn(),
  removeLeadingSlash: vi.fn((value) => value),
  removePrefix: vi.fn((value) => value),
}));
vi.mock('@/utils', () => ({}));
vi.mock('@/utils/files.js', () => ({ getFileExtension: vi.fn() }));
vi.mock('@/utils/mimetype', () => ({
  getTypeInfo: vi.fn(),
  isHtmlMimeType: vi.fn(),
  isRichTextPreviewMimeType: vi.fn(),
}));
vi.mock('@/utils/moment', () => ({ fromNow: vi.fn() }));
vi.mock('@/utils/object.js', () => ({
  getNestedProperty: vi.fn(),
  getObjectProperty: vi.fn(),
}));

import { getters } from './getters.ts';

describe('getters.isDarkMode logged-out behavior', () => {
  beforeEach(() => {
    stateMock.user = { username: '', darkMode: true, locale: 'en' };
    stateMock.shareInfo = null;
    stateMock.disableEventThemes = false;
    stateMock.route = { path: '/login' };
    globalVarsMock.darkMode = false;
    globalVarsMock.noAuth = false;
    globalVarsMock.eventBasedThemes = false;
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('uses userDefaults (globalVars.darkMode) when not logged in', () => {
    globalVarsMock.darkMode = false;
    expect(getters.isDarkMode()).toBe(false);

    globalVarsMock.darkMode = true;
    expect(getters.isDarkMode()).toBe(true);
  });

  it('uses user preference when logged in', () => {
    stateMock.user = { username: 'alice', darkMode: false, locale: 'en' };
    globalVarsMock.darkMode = true;
    expect(getters.isDarkMode()).toBe(false);

    stateMock.user.darkMode = true;
    expect(getters.isDarkMode()).toBe(true);
  });

  it('anonymous user on a non-share page follows userDefaults', () => {
    stateMock.user = { username: 'anonymous', darkMode: true, locale: 'en' };
    globalVarsMock.darkMode = false;
    expect(getters.isDarkMode()).toBe(false);

    stateMock.user.darkMode = false;
    globalVarsMock.darkMode = true;
    expect(getters.isDarkMode()).toBe(true);
  });

  it('anonymous public share initialized light follows preference changes against a dark server default', () => {
    stateMock.route = { path: '/public/share/hash/testdata/' };
    stateMock.shareInfo = { enforceDarkLightMode: '' };
    stateMock.user = { username: 'anonymous', darkMode: false, locale: 'en' };
    globalVarsMock.darkMode = true;
    expect(getters.isDarkMode()).toBe(false);

    stateMock.user.darkMode = true;
    expect(getters.isDarkMode()).toBe(true);
  });

  it('anonymous public share initialized dark follows preference changes against a light server default', () => {
    stateMock.route = { path: '/public/share/hash/1file1.txt' };
    stateMock.shareInfo = { enforceDarkLightMode: '' };
    stateMock.user = { username: 'anonymous', darkMode: true, locale: 'en' };
    globalVarsMock.darkMode = false;
    expect(getters.isDarkMode()).toBe(true);

    stateMock.user.darkMode = false;
    expect(getters.isDarkMode()).toBe(false);
  });

  it('enforced share dark and light settings override the opposite anonymous preference', () => {
    stateMock.route = { path: '/public/share/hash/testdata/' };
    stateMock.user = { username: 'anonymous', darkMode: false, locale: 'en' };
    globalVarsMock.darkMode = false;
    stateMock.shareInfo = { enforceDarkLightMode: 'dark' };
    expect(getters.isDarkMode()).toBe(true);

    stateMock.user.darkMode = true;
    globalVarsMock.darkMode = true;
    stateMock.shareInfo = { enforceDarkLightMode: 'light' };
    expect(getters.isDarkMode()).toBe(false);
  });

  it('an unenforced share follows the anonymous preference', () => {
    stateMock.route = { path: '/public/share/hash/' };
    stateMock.user = { username: 'anonymous', darkMode: true, locale: 'en' };
    globalVarsMock.darkMode = false;
    stateMock.shareInfo = null;
    expect(getters.isDarkMode()).toBe(true);

    stateMock.shareInfo = { enforceDarkLightMode: '' };
    stateMock.user.darkMode = false;
    globalVarsMock.darkMode = true;
    expect(getters.isDarkMode()).toBe(false);
  });

  it('uses the server default until a share user has a boolean darkMode preference', () => {
    stateMock.route = { path: '/public/share/hash/testdata/' };
    globalVarsMock.darkMode = true;
    stateMock.user = null;
    expect(getters.isDarkMode()).toBe(true);

    globalVarsMock.darkMode = false;
    expect(getters.isDarkMode()).toBe(false);

    stateMock.user = { username: 'anonymous', locale: 'en' };
    globalVarsMock.darkMode = true;
    expect(getters.isDarkMode()).toBe(true);

    stateMock.user.darkMode = null;
    expect(getters.isDarkMode()).toBe(true);

    stateMock.user.darkMode = 'true';
    globalVarsMock.darkMode = false;
    expect(getters.isDarkMode()).toBe(false);
  });

  it('does not force dark mode during seasonal themes; the preference still applies', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 9, 31, 12, 0, 0));
    globalVarsMock.eventBasedThemes = true;
    globalVarsMock.darkMode = false;
    stateMock.disableEventThemes = false;
    stateMock.user = { username: 'alice', darkMode: false, locale: 'en' };
    expect(getters.isDarkMode()).toBe(false);

    stateMock.user.darkMode = true;
    expect(getters.isDarkMode()).toBe(true);

    stateMock.user = { username: '', darkMode: true, locale: 'en' };
    expect(getters.isDarkMode()).toBe(false);

    globalVarsMock.darkMode = true;
    expect(getters.isDarkMode()).toBe(true);

    globalVarsMock.darkMode = false;
    stateMock.user = { username: 'alice', darkMode: false, locale: 'en' };
    stateMock.disableEventThemes = true;
    expect(getters.isDarkMode()).toBe(false);

    stateMock.disableEventThemes = false;
    stateMock.route = { path: '/public/share/hash/testdata/' };
    stateMock.user = { username: 'anonymous', darkMode: false, locale: 'en' };
    expect(getters.isDarkMode()).toBe(false);

    stateMock.user.darkMode = true;
    expect(getters.isDarkMode()).toBe(true);
  });
});
