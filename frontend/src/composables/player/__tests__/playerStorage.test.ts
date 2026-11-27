import { describe, expect, it, beforeEach } from "vitest";
import {
  DEFAULT_TRANSCODE_MODE,
  loadPreferredTranscodeMode,
  resolveTranscodeModeForPlayback,
  savePreferredTranscodeMode,
  TRANSCODE_PREFERENCE_STORAGE_KEY,
} from "@/composables/player/playerStorage.ts";

describe("playerStorage transcode preference", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("defaults to quality when nothing stored", () => {
    expect(loadPreferredTranscodeMode()).toBe(DEFAULT_TRANSCODE_MODE);
    expect(resolveTranscodeModeForPlayback({})).toBe("quality");
  });

  it("prefers URL query over localStorage", () => {
    savePreferredTranscodeMode("quality");
    expect(resolveTranscodeModeForPlayback({ transcode: "datasaver" })).toBe("datasaver");
  });

  it("persists preferred mode in localStorage", () => {
    savePreferredTranscodeMode("datasaver");
    expect(localStorage.getItem(TRANSCODE_PREFERENCE_STORAGE_KEY)).toBe("datasaver");
    expect(loadPreferredTranscodeMode()).toBe("datasaver");
  });
});
