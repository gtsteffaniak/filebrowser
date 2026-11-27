import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/store", () => ({ state: { mediaSettings: null } }));
vi.mock("@/utils/constants", () => ({
  globalVars: { ffmpegAvailable: true },
}));

import { isTranscodeAvailable } from "@/player/transcodeMode.ts";
import { state } from "@/store/index.ts";
import type { StoreState } from "@/store/types.ts";
import { globalVars } from "@/utils/constants.ts";

type MediaSettings = NonNullable<StoreState["mediaSettings"]>;

const mediaSettings = (enabled: boolean): MediaSettings => ({
  enabled,
  profiles: ["quality", "balanced", "datasaver"],
  maxConcurrentPerUser: 2,
  imagePreview: {},
});

describe("isTranscodeAvailable", () => {
  beforeEach(() => {
    state.mediaSettings = null;
    globalVars.ffmpegAvailable = true;
  });

  it("stays off until live media settings load", () => {
    expect(isTranscodeAvailable()).toBe(false);
  });

  it("follows the live media settings", () => {
    state.mediaSettings = mediaSettings(false);
    expect(isTranscodeAvailable()).toBe(false);

    state.mediaSettings = mediaSettings(true);
    expect(isTranscodeAvailable()).toBe(true);
  });

  it("stays off when the server has no ffmpeg", () => {
    globalVars.ffmpegAvailable = false;
    state.mediaSettings = mediaSettings(true);
    expect(isTranscodeAvailable()).toBe(false);
  });

  it("stays off when ffmpeg is missing even if settings claim enabled", () => {
    globalVars.ffmpegAvailable = false;
    state.mediaSettings = mediaSettings(true);
    expect(isTranscodeAvailable()).toBe(false);
  });
});
