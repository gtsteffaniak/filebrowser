import { beforeEach, describe, expect, it, vi } from 'vitest';

const { stateMock, globalVarsMock } = vi.hoisted(() => ({
  stateMock: {
    user: { username: 'alice', disableOnlyOfficeExt: '' },
    route: { path: '/files/srv/docs' },
  },
  globalVarsMock: {},
}));

vi.mock('./state', () => ({ state: stateMock }));
vi.mock('./mutations', () => ({ mutations: {} }));
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
vi.mock('@/utils/files.js', () => ({
  getFileExtension: (name) => name.slice(name.lastIndexOf('.')),
}));
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

describe('getters.isWopiFile', () => {
  beforeEach(() => {
    stateMock.user = { username: 'alice', disableOnlyOfficeExt: '' };
    globalVarsMock.wopiUrl = 'https://office.example';
    globalVarsMock.wopiExtensions = { docx: 'edit', xlsx: 'edit', sxw: 'view' };
  });

  it('routes extensions the editor declares, whatever their case', () => {
    expect(getters.isWopiFile('report.docx')).toBe(true);
    expect(getters.isWopiFile('Budget.XLSX')).toBe(true);
    expect(getters.isWopiFile('legacy.sxw')).toBe(true);
  });

  it('leaves other files to their usual viewer', () => {
    expect(getters.isWopiFile('photo.png')).toBe(false);
    expect(getters.isWopiFile('README')).toBe(false);
  });

  it('is off when no editor is configured', () => {
    globalVarsMock.wopiUrl = '';
    expect(getters.isWopiFile('report.docx')).toBe(false);
  });

  it('respects the user disabling office viewing for an extension', () => {
    stateMock.user.disableOnlyOfficeExt = '.docx';
    expect(getters.isWopiFile('report.docx')).toBe(false);
    expect(getters.isWopiFile('budget.xlsx')).toBe(true);
    stateMock.user.disableOnlyOfficeExt = '*';
    expect(getters.isWopiFile('budget.xlsx')).toBe(false);
  });
});
