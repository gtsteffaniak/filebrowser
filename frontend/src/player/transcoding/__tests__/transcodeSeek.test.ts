import { describe, expect, it } from "vitest";
import { transcodeSession } from "@/composables/player/__tests__/mediaTestUtils.ts";
import { servableCoverage } from "@/player/transcoding/transcodePlaybackController.ts";

describe("servableCoverage", () => {
  it("returns the full media range for any VOD session", () => {
    const session = transcodeSession({ vodPlaylist: true, durationSec: 5112 });
    expect(servableCoverage(session)).toEqual({ start: 0, end: 5112 });
  });

  it("covers the whole file even while the producer is still encoding", () => {
    const session = transcodeSession({ state: "running", durationSec: 5112 });
    expect(servableCoverage(session)).toEqual({ start: 0, end: 5112 });
  });

  it("covers the whole file for a completed session", () => {
    const session = transcodeSession({ state: "completed", durationSec: 600 });
    expect(servableCoverage(session)).toEqual({ start: 0, end: 600 });
  });

  it("returns null without a usable duration", () => {
    expect(servableCoverage(transcodeSession({ durationSec: 0 }))).toBeNull();
    expect(servableCoverage(transcodeSession({ durationSec: Number.NaN }))).toBeNull();
    expect(servableCoverage(null)).toBeNull();
  });
});
