import { beforeEach, describe, expect, it } from "vitest";
import {
  getCaptionSizeLabel,
  getStoredCaptionSize,
  mergePlayerStorage,
  readAlbumArtSize,
  readDesktopPanelOpen,
  readMobileLyricsOpen,
  setStoredCaptionSize,
  writeAlbumArtSize,
  writeDesktopPanelOpen,
  writeMobileLyricsOpen,
} from "../../player/playerStorage.ts";

describe("playerStorage", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  it("defaults the caption size to medium and rejects unknown ids", () => {
    expect(getStoredCaptionSize()).toBe("medium");
    setStoredCaptionSize("huge");
    expect(getStoredCaptionSize()).toBe("medium");
  });

  it("stores the caption size under the legacy plyr key without clobbering other fields", () => {
    localStorage.setItem("plyr", JSON.stringify({ captions: true, language: "en" }));
    setStoredCaptionSize("large");
    expect(JSON.parse(localStorage.getItem("plyr") as string)).toEqual({
      captions: true,
      language: "en",
      captionSize: "large",
    });
    expect(getStoredCaptionSize()).toBe("large");
  });

  it("survives corrupt stored JSON without overwriting it", () => {
    localStorage.setItem("plyr", "{not json");
    expect(getStoredCaptionSize()).toBe("medium");
    expect(() => mergePlayerStorage({ captionSize: "small" })).not.toThrow();
    expect(localStorage.getItem("plyr")).toBe("{not json");
  });

  it("labels caption sizes and falls back to Medium", () => {
    expect(getCaptionSizeLabel("xlarge")).toBe("Extra large");
    expect(getCaptionSizeLabel("nope")).toBe("Medium");
  });

  it("keeps the legacy panel and album-art keys", () => {
    expect(readDesktopPanelOpen()).toBe(false);
    writeDesktopPanelOpen(true);
    expect(localStorage.getItem("plyrShowDesktopPanel")).toBe("1");
    expect(readDesktopPanelOpen()).toBe(true);

    expect(readMobileLyricsOpen()).toBe(false);
    writeMobileLyricsOpen(true);
    expect(sessionStorage.getItem("plyrShowMobileLyrics")).toBe("1");

    expect(readAlbumArtSize()).toBe(24);
    writeAlbumArtSize(31.5);
    expect(sessionStorage.getItem("plyrAlbumArtSize")).toBe("31.5");
    expect(readAlbumArtSize()).toBe(31.5);
  });
});
