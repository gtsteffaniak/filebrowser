import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";

const setRequestViewToken = vi.hoisted(() => vi.fn());
const reqState = vi.hoisted(() => ({ req: null }));

const shareState = vi.hoisted(() => ({
  isShare: false,
  shareHash: "",
}));

vi.mock("@/store", () => ({
  getters: {
    isShare: () => shareState.isShare,
  },
  mutations: { setRequestViewToken },
  state: {
    get req() { return reqState.req; },
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
  ensureViewToken,
  refreshViewGrantAfterSessionRenewal,
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

describe("session renewal preview refresh", () => {
  afterEach(() => {
    shareState.isShare = false;
    setActiveViewGrantScope("");
    reqState.req = null;
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });
  beforeEach(() => {
    vi.useFakeTimers();
    sessionStorage.clear();
    shareState.isShare = false;
    reqState.req = { source: "src", path: "/post.html", viewToken: "old" };
    setActiveViewGrantScope("src");
    setRequestViewToken.mockClear();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ viewToken: "new", expiresAt: Math.floor(Date.now() / 1000) + 3600 }),
    }));
  });

  it("clears old cached grants and updates the active preview", async () => {
    sessionStorage.setItem("viewToken:other", JSON.stringify({ viewToken: "old-other", expiresAt: Math.floor(Date.now() / 1000) + 3600 }));
    await refreshViewGrantAfterSessionRenewal();
    expect(sessionStorage.getItem("viewToken:other")).toBeNull();
    expect(getCachedViewToken("src")).toBe("new");
    expect(setRequestViewToken).toHaveBeenCalledWith("new");
  });

  it("does not apply the result to a different file", async () => {
    let release;
    fetch.mockImplementation(() => new Promise((resolve) => { release = resolve; }));
    const refresh = refreshViewGrantAfterSessionRenewal();
    reqState.req = { source: "src", path: "/other.html" };
    release({ ok: true, json: async () => ({ viewToken: "new", expiresAt: Math.floor(Date.now() / 1000) + 3600 }) });
    await refresh;
    expect(setRequestViewToken).not.toHaveBeenCalled();
  });

  it("leaves public shares unchanged", async () => {
    shareState.isShare = true;
    await refreshViewGrantAfterSessionRenewal();
    expect(fetch).not.toHaveBeenCalled();
    expect(setRequestViewToken).not.toHaveBeenCalled();
  });

  it("discards a refresh started before session renewal", async () => {
    let release;
    fetch.mockImplementationOnce(() => new Promise((resolve) => { release = resolve; }));
    const oldRefresh = ensureViewToken("src");
    const rejected = expect(oldRefresh).rejects.toThrow("previous session");
    await refreshViewGrantAfterSessionRenewal();
    release({ ok: true, json: async () => ({ viewToken: "old", expiresAt: Math.floor(Date.now() / 1000) + 3600 }) });
    await rejected;
    expect(getCachedViewToken("src")).toBe("new");
  });
});

describe("preview grant recovery", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    sessionStorage.clear();
    shareState.isShare = false;
    reqState.req = { source: "src", path: "/post.html", viewToken: "old" };
    setActiveViewGrantScope("src");
    setRequestViewToken.mockClear();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ viewToken: "new", expiresAt: Math.floor(Date.now() / 1000) + 3600 }),
    }));
  });
  afterEach(() => {
    shareState.isShare = false;
    setActiveViewGrantScope("");
    reqState.req = null;
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("retries network failures and updates the active preview", async () => {
    fetch.mockRejectedValueOnce(new Error("offline"));
    await refreshViewGrantAfterSessionRenewal();
    expect(setRequestViewToken).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1000);
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(getCachedViewToken("src")).toBe("new");
    expect(setRequestViewToken).toHaveBeenCalledWith("new");
  });

  it("backs off across repeated transient failures", async () => {
    fetch.mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce({ ok: false, status: 503, text: async () => "unavailable" });
    await refreshViewGrantAfterSessionRenewal();
    await vi.advanceTimersByTimeAsync(1000);
    expect(fetch).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(1999);
    expect(fetch).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(1);
    expect(setRequestViewToken).toHaveBeenCalledWith("new");
  });

  it("cancels a retry on navigation", async () => {
    fetch.mockRejectedValueOnce(new Error("offline"));
    await refreshViewGrantAfterSessionRenewal();
    reqState.req = { source: "src", path: "/other.html" };
    setActiveViewGrantScope("src");
    await vi.advanceTimersByTimeAsync(1000);
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(setRequestViewToken).not.toHaveBeenCalled();
  });

  it("preserves recovery when the same preview scope is set again", async () => {
    fetch.mockRejectedValueOnce(new Error("offline"));
    await refreshViewGrantAfterSessionRenewal();
    reqState.req = { ...reqState.req };
    setActiveViewGrantScope("src");
    await vi.advanceTimersByTimeAsync(1000);
    expect(fetch).toHaveBeenCalledTimes(2);
    expect(setRequestViewToken).toHaveBeenCalledWith("new");
  });

  it("cancels recovery when the storage source changes", async () => {
    fetch.mockRejectedValueOnce(new Error("offline"));
    await refreshViewGrantAfterSessionRenewal();
    reqState.req = { source: "other", path: "/post.html" };
    setActiveViewGrantScope("other");
    await vi.advanceTimersByTimeAsync(1000);
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(setRequestViewToken).not.toHaveBeenCalled();
  });

  it("clears sibling grants immediately even when the first refresh fails", async () => {
    reqState.req.parentDirItems = [
      { path: "/movie.mp4", viewToken: "old-video" },
      { path: "/image.png", viewToken: "old-image" },
    ];
    fetch.mockRejectedValueOnce(new Error("offline"));
    await refreshViewGrantAfterSessionRenewal();
    expect(reqState.req.parentDirItems.every((item) => item.viewToken === undefined)).toBe(true);
    await vi.advanceTimersByTimeAsync(1000);
    expect(setRequestViewToken).toHaveBeenCalledWith("new");
  });

  it("does not retry authorization denials", async () => {
    fetch.mockResolvedValueOnce({ ok: false, status: 403, text: async () => "denied" });
    await refreshViewGrantAfterSessionRenewal();
    await vi.advanceTimersByTimeAsync(30_000);
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("times out stalled requests before retrying", async () => {
    fetch.mockImplementationOnce((_url, { signal }) => new Promise((_resolve, reject) => {
      signal.addEventListener("abort", () => reject(new Error("aborted")), { once: true });
    }));
    const refresh = refreshViewGrantAfterSessionRenewal();
    await vi.advanceTimersByTimeAsync(15_000);
    await refresh;
    await vi.advanceTimersByTimeAsync(1000);
    expect(setRequestViewToken).toHaveBeenCalledWith("new");
  });
});
