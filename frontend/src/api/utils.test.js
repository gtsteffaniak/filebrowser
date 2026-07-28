import { beforeEach, describe, expect, it, vi } from 'vitest';

// Hoisted mocks so the vi.mock factories below can reference them.
const { storeMock, authMock } = vi.hoisted(() => ({
  storeMock: {
    state: { sessionId: 'sess', user: { username: 'bob' } },
    getters: {
      isLoggedIn: vi.fn(() => true),
      isShare: vi.fn(() => false),
    },
  },
  authMock: {
    renew: vi.fn(),
    sessionExpired: vi.fn(),
  },
}));

vi.mock('@/store', () => storeMock);
vi.mock('@/utils/auth', () => authMock);
vi.mock('@/i18n', () => ({ default: { global: { t: (k) => k } } }));

vi.mock('@/utils/constants', () => {
  return {
    globalVars: {
      baseURL: "unit-testing",
      name: "Test App",
      mediaAvailable: true,
      muPdfAvailable: true,
      onlyOfficeUrl: "",
      recaptcha: false,
      recaptchaKey: "",
      darkMode: false,
      oidcAvailable: false,
      passwordAvailable: true,
      externalUrl: "",
      minSearchLength: 1,
      disableNavButtons: false,
      userSelectableThemes: {},
      enableThumbs: true,
      noAuth: false,
      signup: false,
      version: "test",
      commitSHA: "test",
      disableExternal: false,
      externalLinks: [],
      updateAvailable: "",
    },
    logoURL: "test-logo.png",
    origin: "http://localhost",
    settings: [],
  };
});

import { adjustedData, fetchURL } from './utils.js';

const mockFetch = (status, body = '') =>
  vi.fn().mockResolvedValue({
    status,
    headers: { get: () => null }, // no X-Renew-Token
    text: async () => body,
    json: async () => ({}),
  });

describe('fetchURL 401 handling', () => {
  beforeEach(() => {
    authMock.sessionExpired.mockClear();
    storeMock.getters.isLoggedIn.mockReturnValue(true);
    storeMock.getters.isShare.mockReturnValue(false);
  });

  it('clears the session (sessionExpired) on 401 for an authed request while logged in', async () => {
    global.fetch = mockFetch(401, 'token is expired');
    await expect(fetchURL('/api/resources')).rejects.toThrow();
    expect(authMock.sessionExpired).toHaveBeenCalledTimes(1);
  });

  it('does NOT clear the session on 401 for a share', async () => {
    storeMock.getters.isShare.mockReturnValue(true);
    global.fetch = mockFetch(401, 'password required');
    await expect(fetchURL('/api/resources')).rejects.toThrow();
    expect(authMock.sessionExpired).not.toHaveBeenCalled();
  });

  it('does NOT clear the session on 401 when not logged in', async () => {
    storeMock.getters.isLoggedIn.mockReturnValue(false);
    global.fetch = mockFetch(401, 'unauthorized');
    await expect(fetchURL('/api/resources')).rejects.toThrow();
    expect(authMock.sessionExpired).not.toHaveBeenCalled();
  });

  it('does NOT clear the session on non-401 errors or success', async () => {
    global.fetch = mockFetch(403, 'forbidden');
    await expect(fetchURL('/api/resources')).rejects.toThrow();
    global.fetch = mockFetch(200);
    await fetchURL('/api/resources');
    expect(authMock.sessionExpired).not.toHaveBeenCalled();
  });
});

describe('adjustedData', () => {
  it('should append the URL and process directory data correctly', () => {
    const input = {
      type: "directory",
      folders: [
        { name: "folder1", type: "directory" },
        { name: "folder2", type: "directory" },
      ],
      files: [
        { name: "file1.txt", type: "file" },
        { name: "file2.txt", type: "file" },
      ],
      path: "/root/"
    };

    const expected = {
      type: "directory",
      folders: [],
      files: [],
      items: [
        { isShared: false, name: "folder1", path: "/root/folder1/", source: undefined, type: "directory", pinned: false },
        { isShared: false, name: "folder2", path: "/root/folder2/", source: undefined, type: "directory", pinned: false },
        { isShared: false, name: "file1.txt", path: "/root/file1.txt", source: undefined, type: "file", pinned: false },
        { isShared: false, name: "file2.txt", path: "/root/file2.txt", source: undefined, type: "file", pinned: false },
      ],
      path: "/root/",
    };

    expect(adjustedData(input)).toEqual(expected);
  });

  it('should add a trailing slash to the URL if missing for a directory', () => {
    const input = { type: "directory", folders: [], files: [] };

    const expected = {
      type: "directory",
      folders: [],
      files: [],
      items: [],
    };

    expect(adjustedData(input)).toEqual(expected);
  });

  it('should handle non-directory types without modification to items', () => {
    const input = { type: "file", name: "file1.txt" };

    const expected = {
      type: "file",
      name: "file1.txt",
    };

    expect(adjustedData(input)).toEqual(expected);
  });

  it('should handle missing folders and files gracefully', () => {
    const input = { type: "directory" };
    const expected = {
      type: "directory",
      items: [],
    };

    expect(adjustedData(input)).toEqual(expected);
  });

  it('should handle empty input object correctly', () => {
    const input = {};
    const expected = {};
    expect(adjustedData(input)).toEqual(expected);
  });

  it('marks items as pinned from directory pinnedItems names', () => {
    const input = {
      type: "directory",
      path: "/",
      pinnedItems: ["alpha.txt"],
      folders: [{ name: "docs", type: "directory" }],
      files: [
        { name: "alpha.txt", type: "file" },
        { name: "beta.txt", type: "file" },
      ],
      source: "Docs",
    };

    const result = adjustedData(input);

    expect(result.pinnedItems).toBeUndefined();
    expect(result.items).toEqual([
      { isShared: false, name: "docs", path: "/docs/", source: "Docs", type: "directory", pinned: false },
      { isShared: false, name: "alpha.txt", path: "/alpha.txt", source: "Docs", type: "file", pinned: true },
      { isShared: false, name: "beta.txt", path: "/beta.txt", source: "Docs", type: "file", pinned: false },
    ]);
  });

});
