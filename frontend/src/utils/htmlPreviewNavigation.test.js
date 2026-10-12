import { describe, expect, it } from "vitest";
import { previewNavigationFile } from "./htmlPreviewNavigation.ts";

const frame = {};
const origin = "https://files.example.test";
const path = "/api/resources/view";
const href = `${origin}${path}?source=folder&file=%2Farchive%2Fpost.html&viewToken=opaque`;
const event = (url = href, source = frame, senderOrigin = "null") => ({
  source, origin: senderOrigin,
  data: { type: "filebrowser-html-navigation", href: url },
});

describe("sandboxed HTML navigation", () => {
  it("accepts a local page from the current opaque iframe", () => {
    expect(previewNavigationFile(event(), frame, origin, path, "folder")).toBe("/archive/post.html");
  });
  it("rejects another window even when its origin is also null", () => {
    expect(previewNavigationFile(event(href, {}), frame, origin, path, "folder")).toBeNull();
    expect(previewNavigationFile(event(href, frame, origin), frame, origin, path, "folder")).toBeNull();
  });
  it.each([
    href.replace(origin, "https://evil.example.test"),
    href.replace(path, "/api/users"),
    href.replace("source=folder", "source=private"),
    href + "&file=/second.html",
    href.replace("%2Farchive%2Fpost.html", "%2F..%2Fsecret.html"),
    href.replace("%2Farchive%2Fpost.html", "%2Farchive%2Fmovie.mp4"),
    "javascript:alert(1)",
  ])("rejects a forged destination: %s", (url) => {
    expect(previewNavigationFile(event(url), frame, origin, path, "folder")).toBeNull();
  });
  it("supports a matching share and rejects other shares", () => {
    const sharePath = "/prefix/public/api/resources/view";
    const shareURL = `${origin}${sharePath}?hash=share-a&file=%2Fpost.html`;
    expect(previewNavigationFile(event(shareURL), frame, origin, sharePath, "", "share-a")).toBe("/post.html");
    expect(previewNavigationFile(event(shareURL), frame, origin, sharePath, "", "share-b")).toBeNull();
  });
});
