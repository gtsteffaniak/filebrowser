import type Plyr from "plyr";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { shallowRef } from "vue";
import { useMediaSession } from "../useMediaSession.ts";
import { type FakeMediaSession, installMediaSession, removeMediaSession } from "./mediaTestUtils.ts";

vi.mock("@/store/index.ts", () => ({
  state: { playbackQueue: { queue: [], currentIndex: -1, loop: "off" } },
}));
vi.mock("@/utils/constants.ts", () => ({ globalVars: { loginIcon: "/icon.png" } }));
vi.mock("@/utils/playbackQueue.js", () => ({ getNextItem: vi.fn(), navigatePlaybackQueue: vi.fn() }));

const INFO = { title: "T", artist: "A", album: "B", artworkSrc: "blob:art" };

describe("useMediaSession", () => {
  let session: FakeMediaSession;

  beforeEach(() => {
    session = installMediaSession();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    removeMediaSession();
  });

  function setup() {
    const player = shallowRef({ playing: false, duration: 0, speed: 1, currentTime: 0 } as Plyr);
    return useMediaSession(player);
  }

  it("registers the handlers once and updates only the metadata per file", () => {
    const media = setup();
    media.setupMediaSession(INFO);
    const registered = session.setActionHandler.mock.calls.length;
    expect(registered).toBeGreaterThan(0);
    expect(session.metadata).toMatchObject({ title: "T", artist: "A", artwork: [{ src: "blob:art" }] });

    media.updateMediaSessionMetadata({ title: "T2", artist: "A2", album: "B2" });
    expect(session.setActionHandler).toHaveBeenCalledTimes(registered);
    expect(session.metadata).toMatchObject({ title: "T2", artist: "A2", album: "B2" });
    expect(session.metadata?.artwork?.[0]?.src).toContain("/icon.png");
  });

  it("does not touch the session when it does not own it", () => {
    const media = setup();
    media.updateMediaSessionMetadata(INFO);
    expect(session.metadata).toBeNull();
  });

  it("stops updating after the session is cleared", () => {
    const media = setup();
    media.setupMediaSession(INFO);
    media.clearMediaSession();
    expect(session.metadata).toBeNull();
    media.updateMediaSessionMetadata(INFO);
    expect(session.metadata).toBeNull();
  });
});
