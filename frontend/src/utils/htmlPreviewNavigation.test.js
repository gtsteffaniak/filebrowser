import { describe, expect, it } from "vitest";
import { previewNavigationFile } from "./htmlPreviewNavigation.ts";

const frame = {};
const origin = "https://files.example.test";
const path = "/files/folder/";
const href = `${origin}${path}archive/post.html`;
const event = (url = href, source = frame, senderOrigin = "null") => ({
  source, origin: senderOrigin,
  data: { type: "filebrowser-html-navigation", href: url },
});

describe("sandboxed HTML navigation", () => {
  it("accepts a local page from the current opaque iframe", () => {
    expect(previewNavigationFile(event(), frame, origin, path)).toBe("/archive/post.html");
  });
  it("rejects another window even when its origin is also null", () => {
    expect(previewNavigationFile(event(href, {}), frame, origin, path)).toBeNull();
    expect(previewNavigationFile(event(href, frame, origin), frame, origin, path)).toBeNull();
  });
  it.each([
    href.replace(origin, "https://evil.example.test"),
    href.replace(path, "/api/users"),
    href.replace("/files/folder/", "/files/private/"),
    `${href}?auth=unexpected`,
    href.replace("archive/post.html", "%2F..%2Fsecret.html"),
    href.replace("archive/post.html", "archive/movie.mp4"),
    "javascript:alert(1)",
  ])("rejects a forged destination: %s", (url) => {
    expect(previewNavigationFile(event(url), frame, origin, path)).toBeNull();
  });
  it("supports a matching share and rejects other shares", () => {
    const sharePath = "/prefix/public/share/share-a/";
    const shareURL = `${origin}${sharePath}post.html`;
    expect(previewNavigationFile(event(shareURL), frame, origin, sharePath)).toBe("/post.html");
    expect(previewNavigationFile(event(shareURL), frame, origin, "/prefix/public/share/share-b/")).toBeNull();
  });

  it("supports encoded filenames and a configured base URL", () => {
    const prefix = "/prefix/files/folder/";
    expect(previewNavigationFile(event(`${origin}${prefix}archive/page%20two.html`), frame, origin, prefix)).toBe("/archive/page two.html");
  });
});
