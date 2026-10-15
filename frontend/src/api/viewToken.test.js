import { describe, expect, it, vi, beforeEach } from "vitest";

const shareState = vi.hoisted(() => ({
  isShare: false,
  shareHash: "",
}));

vi.mock("@/store", () => ({
  getters: {
    isShare: () => shareState.isShare,
  },
  state: {
    sessionId: "test-session",
    shareInfo: {
      get hash() {
        return shareState.shareHash;
      },
      token: "share-token",
    },
  },
}));

vi.mock("@/utils/url.js", () => ({
  getPublicApiPath: (path) => `/public/${path}`,
  getApiPath: (path) => `/api/${path}`,
}));

import {
  requestViewIdentity,
  refreshViewToken,
  setActiveViewGrantScope,
  getCachedViewToken,
} from "./viewToken.js";

describe("requestViewIdentity", () => {
  it("returns empty string for missing request", () => {
    expect(requestViewIdentity(null)).toBe("");
    expect(requestViewIdentity(undefined)).toBe("");
  });

  it("serializes source, share hash, and path", () => {
    shareState.isShare = false;
    shareState.shareHash = "";
    expect(requestViewIdentity({ source: "default", path: "/books/epub.epub" })).toBe(
      JSON.stringify(["default", "", "/books/epub.epub"]),
    );
  });

  it("includes share hash when viewing a share", () => {
    shareState.isShare = true;
    shareState.shareHash = "abc123";
    expect(requestViewIdentity({ source: "srv", path: "/file.epub" })).toBe(
      JSON.stringify(["srv", "abc123", "/file.epub"]),
    );
    shareState.isShare = false;
    shareState.shareHash = "";
  });

  it("does not collide when values contain delimiter-like characters", () => {
    shareState.isShare = false;
    shareState.shareHash = "";
    const splitAcrossFields = requestViewIdentity({
      source: "foo",
      path: "bar|baz",
    });
    const pipeInSource = requestViewIdentity({
      source: "foo|bar",
      path: "baz",
    });
    expect(splitAcrossFields).not.toBe(pipeInSource);
    expect(splitAcrossFields).toBe(JSON.stringify(["foo", "", "bar|baz"]));
    expect(pipeInSource).toBe(JSON.stringify(["foo|bar", "", "baz"]));
  });
});

describe("refreshViewToken", () => {
  beforeEach(() => {
    sessionStorage.clear();
    shareState.isShare = true;
    shareState.shareHash = "share-a";
    setActiveViewGrantScope("src");
    vi.stubGlobal("fetch", vi.fn());
  });

  it("does not cache a token under a new share when scope changes during refresh", async () => {
    let resolveFetch;
    fetch.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveFetch = () =>
            resolve({
              ok: true,
              json: async () => ({
                viewToken: "token-a",
                expiresAt: Math.floor(Date.now() / 1000) + 3600,
              }),
            });
        }),
    );

    const refreshPromise = refreshViewToken("src", undefined, "share-a");

    shareState.shareHash = "share-b";
    setActiveViewGrantScope("src");

    resolveFetch();
    await refreshPromise;

    expect(sessionStorage.getItem("viewToken:share-b")).toBeNull();
    expect(getCachedViewToken("src")).toBeUndefined();
  });

  it("caches the token when the request scope is still active", async () => {
    fetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        viewToken: "token-a",
        expiresAt: Math.floor(Date.now() / 1000) + 3600,
      }),
    });

    await refreshViewToken("src", undefined, "share-a");

    expect(getCachedViewToken("src")).toBe("token-a");
  });
});
