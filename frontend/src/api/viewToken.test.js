import { describe, expect, it, vi } from "vitest";

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

import { requestViewIdentity } from "./viewToken.js";

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
