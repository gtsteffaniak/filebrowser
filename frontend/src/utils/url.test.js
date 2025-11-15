import { describe, it, expect, vi } from 'vitest';

vi.mock('@/utils/constants', () => {
  return {
    globalVars: {
      baseURL: "/files/",
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
    shareInfo: {
      isShare: false,
      disableThumbnails: false,
      hash: "",
      enforceDarkLightMode: "",
      disableSidebar: false,
      isValid: true,
    },
    logoURL: "test-logo.png",
    origin: "http://localhost",
    settings: [],
  };
});

import { removePrefix, extractSourceFromPath, getApiPath } from './url.js';

describe('testurl', () => {

  it('url prefix', () => {
    let tests = [
      {input: "/files/share/hash", trimArg:"/files/",expected: "/share/hash",},
      {input: "/files/files", trimArg: "/files/",expected: "/files",},
      {input: "/files/share/something/", trimArg: "files", expected:"/share/something/"},
      {input: "test/iscool/", trimArg: "test",expected:"/iscool/"},
      {input: "test",trimArg: "",expected:"/test"},
      {input: "/test", trimArg: "test",expected:"/"},
      {input: "/my/test/file", trimArg: "",expected:"/my/test/file"},
      {input: "/my/test/file", trimArg: "my",expected:"/test/file"},
      {input: "/files/my/test/file", trimArg: "files",expected:"/my/test/file"},
    ]
    for (let test of tests) {
      expect(removePrefix(test.input, test.trimArg)).toEqual(test.expected);
    }
  });

});

describe('getapipath', () => {
  it('url prefix', () => {
    let tests = [
      {input: "/share/to/thing", expected: "/files/share/to/thing",},
      {input: "share/hash", expected: "/files/share/hash",},
    ]
    for (let test of tests) {
      expect(getApiPath(test.input)).toEqual(test.expected);
    }
  });
});

describe('extractSourceFromPath', () => {
  it('single source extract', async () => {
    vi.doMock("@/store", () => {
      return {
        state: {
          sources: {
            current: "default",
          }
        },
      };
    });

    const tests = [
      { url: "/files/root/file1.txt", expected: { source: "default", path: "/root/file1.txt" } },
      { url: "/files/root/folder1/file1.txt", expected: { source: "default", path: "/root/folder1/file1.txt" } },
    ];

    for (const test of tests) {
      const result = extractSourceFromPath(test.url);
      expect(result.source).toEqual(test.expected.source);
      expect(result.path).toEqual(test.expected.path);
    }

    vi.resetModules(); // Reset modules between tests
  });
});

describe('extractSourceFromPath2', () => {
  it('multiple source extract', async () => {
    vi.doMock("@/store", () => {
      return {
        state: {
          sources: {
            current: "first",
            list: [
              { pathPrefix: "first", used: "0 B", total: "0 B", usedPercentage: 0 },
              { pathPrefix: "second", used: "0 B", total: "0 B", usedPercentage: 0 }
            ],
          },
        },
      };
    });
    const { extractSourceFromPath } = await import("@/utils/url"); // Import AFTER mock
    const tests = [
      { url: "/files/first/root/file1.txt", expected: { source: "first", path: "/root/file1.txt" } },
      { url: "/files/second/root/folder1/file1.txt", expected: { source: "second", path: "/root/folder1/file1.txt" } },
    ];
    for (const test of tests) {
      const result = extractSourceFromPath(test.url);
      expect(result.source).toEqual(test.expected.source);
      expect(result.path).toEqual(test.expected.path);
    }
    vi.resetModules(); // Reset modules between tests
  });
});
