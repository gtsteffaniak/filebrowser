import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { stateMock, globalVarsMock } = vi.hoisted(() => ({
  stateMock: {
    shareInfo: null,
    disableEventThemes: false,
    route: { path: '/files/' },
  },
  globalVarsMock: { eventBasedThemes: true },
}));

vi.mock('./state', () => ({ state: stateMock }));
vi.mock('./mutations', () => ({ mutations: {} }));
vi.mock('@/utils/constants', () => ({ globalVars: globalVarsMock, previewViews: [], tools: () => [] }));
vi.mock('@/i18n', () => ({ detectLocale: () => 'en' }));
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
vi.mock('@/utils/theme', () => ({ defaultDarkMode: vi.fn() }));
vi.mock('@/utils/viewport.js', () => ({ isMobileLayout: vi.fn() }));
vi.mock('@/utils/toolAccess', () => ({ hasToolAccess: vi.fn(), toolIdFromPath: vi.fn() }));

import { getters } from './getters.ts';

describe('getters event themes', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    stateMock.shareInfo = null;
    stateMock.disableEventThemes = false;
    stateMock.route = { path: '/files/' };
    globalVarsMock.eventBasedThemes = true;
    localStorage.clear();
  });

  afterEach(() => {
    vi.useRealTimers();
    localStorage.clear();
  });

  it('offers halloween in settings from Oct 9 through Oct 31', () => {
    vi.setSystemTime(new Date(2026, 9, 8, 12));
    expect(getters.eventThemeAvailable()).toBe('');

    vi.setSystemTime(new Date(2026, 9, 9, 12));
    expect(getters.eventThemeAvailable()).toBe('halloween');

    vi.setSystemTime(new Date(2026, 9, 20, 12));
    expect(getters.eventThemeAvailable()).toBe('halloween');

    vi.setSystemTime(new Date(2026, 9, 31, 12));
    expect(getters.eventThemeAvailable()).toBe('halloween');

    vi.setSystemTime(new Date(2026, 10, 1, 12));
    expect(getters.eventThemeAvailable()).toBe('');
  });

  it('auto-applies only on Halloween (Oct 31)', () => {
    vi.setSystemTime(new Date(2026, 9, 20, 12));
    expect(getters.autoEventTheme()).toBe('');
    expect(getters.eventTheme()).toBe('');

    vi.setSystemTime(new Date(2026, 9, 31, 12));
    expect(getters.autoEventTheme()).toBe('halloween');
    expect(getters.eventTheme()).toBe('halloween');
  });

  it('allows manual opt-in before Halloween when in season', () => {
    vi.setSystemTime(new Date(2026, 9, 15, 12));
    expect(getters.eventTheme()).toBe('');

    localStorage.setItem('eventThemeOptIn', 'halloween');
    expect(getters.eventTheme()).toBe('halloween');
  });

  it('respects disableEventThemes even on Halloween', () => {
    vi.setSystemTime(new Date(2026, 9, 31, 12));
    stateMock.disableEventThemes = true;
    expect(getters.eventTheme()).toBe('');
  });
});
