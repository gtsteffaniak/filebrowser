import { beforeEach, describe, expect, it, vi } from 'vitest';

const { stateMock } = vi.hoisted(() => ({
  stateMock: {
    shareInfo: null,
    route: { path: '/files/' },
    req: {},
    user: null,
    sources: { current: '', info: {} },
  },
}));

vi.mock('./state', () => ({ state: stateMock }));
vi.mock('./mutations', () => ({ mutations: {} }));
vi.mock('@/utils/constants', () => ({ globalVars: {}, previewViews: [], tools: () => [] }));
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

const allowAll = { view: true, download: true, modify: true, create: true, delete: true };

describe('getters.sourcePermissions on read-only sources', () => {
  beforeEach(() => {
    stateMock.shareInfo = null;
    stateMock.route = { path: '/files/' };
    stateMock.req = {};
    stateMock.user = {
      username: 'admin',
      scopes: [
        { name: 'archive', permissions: { ...allowAll } },
        { name: 'inbox', permissions: { ...allowAll } },
      ],
    };
    stateMock.sources = {
      current: 'archive',
      info: {
        archive: { name: 'archive', readOnly: true },
        inbox: { name: 'inbox', readOnly: false },
      },
    };
  });

  it('withholds modify, create and delete in a read-only source', () => {
    expect(getters.sourcePermissions('archive')).toEqual({
      view: true,
      download: true,
      modify: false,
      create: false,
      delete: false,
    });
    expect(getters.canCreateInSource('archive')).toBe(false);
  });

  it('uses the current source when none is given', () => {
    expect(getters.sourcePermissions().create).toBe(false);
    stateMock.sources.current = 'inbox';
    expect(getters.sourcePermissions().create).toBe(true);
  });

  it('keeps the scope permissions of a writable source', () => {
    expect(getters.sourcePermissions('inbox')).toEqual(allowAll);
  });

  it('does not mutate the stored scope permissions', () => {
    getters.sourcePermissions('archive');
    expect(stateMock.user.scopes[0].permissions).toEqual(allowAll);
  });

  it('falls back to scope permissions when source info is missing', () => {
    stateMock.sources.info = {};
    expect(getters.sourcePermissions('archive')).toEqual(allowAll);
  });

  it('leaves share permissions to the share, even over a read-only source', () => {
    stateMock.route = { path: '/public/share/abc/' };
    stateMock.shareInfo = { allowModify: true, allowCreate: true, allowDelete: true };
    expect(getters.sourcePermissions('archive')).toEqual(allowAll);
  });
});
