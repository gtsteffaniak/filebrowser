import { beforeEach, describe, expect, it, vi } from 'vitest';

const { stateMock, globalVarsMock } = vi.hoisted(() => ({
  stateMock: {
    user: { username: '', darkMode: true },
    shareInfo: null,
    disableEventThemes: false,
    route: { path: '/public/share/abc' },
  },
  globalVarsMock: { darkMode: false, noAuth: false, eventBasedThemes: false, singleClick: false },
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

describe('getters.anonymous singleClick follows userDefaults', () => {
  beforeEach(() => {
    globalVarsMock.singleClick = false;
    globalVarsMock.darkMode = false;
  });

  it('defaults anonymous singleClick to false when userDefaults.listing.singleClick is false', () => {
    globalVarsMock.singleClick = false;
    expect(getters.anonymous().singleClick).toBe(false);
  });

  it('sets anonymous singleClick true when userDefaults.listing.singleClick is true', () => {
    globalVarsMock.singleClick = true;
    expect(getters.anonymous().singleClick).toBe(true);
  });
});
