import { beforeEach, describe, expect, it, vi } from 'vitest';

const { stateMock, globalVarsMock } = vi.hoisted(() => ({
  stateMock: {
    user: { username: '', darkMode: true },
    shareInfo: null,
    disableEventThemes: false,
    route: { path: '/login' },
  },
  globalVarsMock: { darkMode: false, noAuth: false },
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
    globalVarsMock.darkMode = false;
    globalVarsMock.noAuth = false;
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

  it('anonymous user follows userDefaults because they are not logged in', () => {
    stateMock.user = { username: 'anonymous', darkMode: true, locale: 'en' };
    globalVarsMock.darkMode = false;
    expect(getters.isDarkMode()).toBe(false);
  });
});
