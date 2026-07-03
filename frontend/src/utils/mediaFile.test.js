import { describe, expect, it } from "vitest";
import { isMediaFile } from "./mediaFile";

describe("isMediaFile", () => {
  it("detects audio and video MIME types", () => {
    expect(isMediaFile("audio/mpeg")).toBe(true);
    expect(isMediaFile("video/mp4")).toBe(true);
  });

  it("detects common media extensions", () => {
    expect(isMediaFile("track.flac")).toBe(true);
    expect(isMediaFile("clip.MP4")).toBe(true);
  });

  it("rejects non-media types", () => {
    expect(isMediaFile("application/pdf")).toBe(false);
    expect(isMediaFile("readme.txt")).toBe(false);
    expect(isMediaFile("model/gltf+json")).toBe(false);
  });
});
