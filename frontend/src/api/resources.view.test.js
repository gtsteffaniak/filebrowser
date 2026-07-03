import { describe, expect, it, vi } from "vitest";

vi.mock("@/utils/constants", () => ({
  globalVars: {
    baseURL: "/",
    externalUrl: "",
  },
}));

vi.mock("@/store", () => ({
  getters: { isShare: () => false },
  state: {
    sessionId: "test-session",
    shareInfo: { subPath: "", hash: "", token: "" },
    req: { parentDirItems: [{ name: "app.js", viewToken: "sibling-view-token" }] },
  },
}));

vi.mock("@/api/media", () => ({
  getStreamURL: (_source, path, viewToken) =>
    `http://localhost/api/media/stream?file=${encodeURIComponent(path)}&viewToken=${viewToken}`,
  getStreamURLPublic: () => "http://localhost/public/stream",
}));

import { getViewURL } from "@/api/resources";

describe("getViewURL", () => {
  it("routes audio to media stream with viewToken", () => {
    const url = getViewURL(
      "src",
      "/music/song.mp3",
      "view-tok",
      null,
      false,
      "audio/mpeg",
    );
    expect(url).toContain("/api/media/stream");
    expect(url).toContain("viewToken=view-tok");
  });

  it("routes documents to resources view with viewToken", () => {
    const url = getViewURL(
      "src",
      "/docs/readme.txt",
      "view-tok",
      null,
      false,
      "text/plain",
    );
    expect(url).toContain("/api/resources/view");
    expect(url).toContain("viewToken=view-tok");
  });

  it("returns null without viewToken when fallback disabled", () => {
    expect(getViewURL("src", "/a.txt", null, null, false, "text/plain")).toBeNull();
  });
});
