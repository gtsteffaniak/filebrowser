import type Plyr from "plyr";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ref, shallowRef } from "vue";
import { getTranscodeSessions } from "@/api/transcode.js";
import { useTranscodePlayback } from "../useTranscodePlayback.ts";
import { withSetup } from "./withSetup.ts";

const hoisted = vi.hoisted(() => ({
  isShare: { value: false },
  needsTranscode: { value: true },
  available: { value: true },
}));

vi.mock("@/api/transcode.js", () => ({ getTranscodeSessions: vi.fn() }));
vi.mock("@/api/viewToken.js", () => ({ getCachedViewToken: () => "" }));
vi.mock("@/i18n/index.ts", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
vi.mock("@/notify/index.ts", () => ({ notify: { showError: vi.fn() } }));
vi.mock("@/router/index.ts", () => ({
  router: { currentRoute: { value: { query: {}, path: "/files/a.wmv", hash: "" } }, replace: vi.fn() },
}));
vi.mock("@/store/index.ts", () => ({
  state: { mediaSettings: { enabled: true, profiles: [] } },
  getters: { isShare: () => hoisted.isShare.value, isLoggedIn: () => true },
  mutations: { setPlaybackState: vi.fn() },
}));
vi.mock("@/utils/canBrowserPlayNative.js", () => ({ needsTranscodeFirst: () => hoisted.needsTranscode.value }));
vi.mock("@/player/transcodeMode.js", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/player/transcodeMode.js")>()),
  isTranscodeAvailable: () => hoisted.available.value,
}));

const getSessions = vi.mocked(getTranscodeSessions);

const REQ = { source: "src", path: "/a.wmv", name: "a.wmv", type: "video/x-ms-wmv" };

function setup(query: Record<string, string> = {}) {
  const hooks = {
    onOfferShown: vi.fn(),
    onPlaybackStart: vi.fn(),
    onModeMenuChange: vi.fn(),
  };
  const { result } = withSetup(() =>
    useTranscodePlayback({
      player: shallowRef<Plyr | null>(null),
      shouldAutoplay: ref<boolean>(false),
      getReq: () => REQ,
      getRaw: () => "/raw/a.wmv",
      getRoute: () => ({ query, path: "/files/a.wmv", hash: "" }) as never,
      getVideoElement: () => null,
      getPlaybackMedia: () => null,
      getNativePlay: () => null,
      isTornDown: () => false,
      isPlaybackLoading: () => false,
      onStreamAttach: vi.fn(),
      ensurePlaybackModeApplied: vi.fn(),
      ...hooks,
    }),
  );
  return { transcode: result, hooks };
}

describe("useTranscodePlayback", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    hoisted.isShare.value = false;
    hoisted.needsTranscode.value = true;
    hoisted.available.value = true;
  });

  describe("resolveReuseSessionId", () => {
    it("prefers a live session of the same file over a completed one", async () => {
      getSessions.mockResolvedValue({
        sessions: [
          { id: "other-file", source: "src", path: "/b.mkv", state: "running" },
          { id: "done", source: "src", path: "/a.wmv", state: "completed" },
          { id: "live", source: "src", path: "/a.wmv", state: "running" },
        ],
      });
      const { transcode } = setup();
      expect(await transcode.resolveReuseSessionId()).toBe("live");
    });

    it("falls back to the first session of the file, and to undefined when there is none or the call fails", async () => {
      const { transcode } = setup();
      getSessions.mockResolvedValueOnce({ sessions: [{ id: "done", source: "src", path: "/a.wmv", state: "completed" }] });
      expect(await transcode.resolveReuseSessionId()).toBe("done");
      getSessions.mockResolvedValueOnce({ sessions: [{ id: "x", source: "src", path: "/b.mkv" }] });
      expect(await transcode.resolveReuseSessionId()).toBeUndefined();
      getSessions.mockRejectedValueOnce(new Error("503"));
      expect(await transcode.resolveReuseSessionId()).toBeUndefined();
    });
  });

  describe("offer", () => {
    it("offers transcoding when a session can start, releases the player, and asks only once", async () => {
      getSessions.mockResolvedValue({ enabled: true, canStart: true, sessions: [] });
      const { transcode, hooks } = setup();
      await transcode.offerTranscodePlayback();
      expect(transcode.transcodeOffer.value?.mode).toBe("offer");
      expect(hooks.onOfferShown).toHaveBeenCalledTimes(1);
      await transcode.offerTranscodePlayback();
      expect(getSessions).toHaveBeenCalledTimes(1);
    });

    it("shows the unsupported notice without asking for sessions when transcoding is off", async () => {
      hoisted.available.value = false;
      const { transcode } = setup();
      await transcode.offerTranscodePlayback();
      expect(getSessions).not.toHaveBeenCalled();
      expect(transcode.transcodeOffer.value?.mode).toBe("unsupported");
    });

    it("maps limits to their modes, but a user limit hit by this very file still offers", async () => {
      getSessions.mockResolvedValue({ canStart: false, blockReason: "user_limit", sessions: [] });
      const first = setup();
      await first.transcode.offerTranscodePlayback();
      expect(first.transcode.transcodeOffer.value?.mode).toBe("user_limit");

      getSessions.mockResolvedValue({ canStart: false, blockReason: "global_limit", globalLimit: 3, sessions: [] });
      const second = setup();
      await second.transcode.offerTranscodePlayback();
      expect(second.transcode.transcodeOffer.value).toMatchObject({ mode: "system_limit", systemLimit: 3 });

      getSessions.mockResolvedValue({
        canStart: false,
        blockReason: "user_limit",
        sessions: [{ id: "mine", source: "src", path: "/a.wmv" }],
      });
      const third = setup();
      await third.transcode.offerTranscodePlayback();
      expect(third.transcode.transcodeOffer.value?.mode).toBe("offer");
    });

    it("never offers in a share, or for formats the browser plays natively", async () => {
      hoisted.isShare.value = true;
      const share = setup();
      await share.transcode.offerTranscodePlayback();
      expect(share.transcode.transcodeOffer.value).toBeNull();

      hoisted.isShare.value = false;
      hoisted.needsTranscode.value = false;
      const native = setup();
      await native.transcode.offerTranscodePlayback();
      expect(native.transcode.transcodeOffer.value).toBeNull();
      expect(getSessions).not.toHaveBeenCalled();
    });

    it("startVideoTranscode hides the offer and requests playback, or keeps the offer when a limit is hit", async () => {
      getSessions.mockResolvedValue({ canStart: true, sessions: [] });
      const { transcode, hooks } = setup();
      await transcode.offerTranscodePlayback();
      expect(await transcode.startVideoTranscode()).toBe(true);
      expect(transcode.transcodeOffer.value).toBeNull();
      expect(transcode.startRequested.value).toBe(true);
      expect(hooks.onPlaybackStart).toHaveBeenCalledTimes(1);
      expect(transcode.shouldProactivelyOfferTranscode.value).toBe(false);

      getSessions.mockResolvedValue({ canStart: false, blockReason: "global_limit", globalLimit: 1, sessions: [] });
      const limited = setup();
      expect(await limited.transcode.startVideoTranscode()).toBe(false);
      expect(limited.transcode.transcodeOffer.value?.mode).toBe("system_limit");
      expect(limited.hooks.onPlaybackStart).not.toHaveBeenCalled();
    });
  });

  describe("initTranscodeModeFromRoute", () => {
    it("starts native for formats that need transcoding first, ignoring the URL mode", () => {
      const { transcode } = setup({ transcode: "balanced" });
      transcode.initTranscodeModeFromRoute();
      expect(transcode.transcodeMode.value).toBe("native");
    });

    it("takes the mode from the URL for natively playable files, and is native without one", () => {
      hoisted.needsTranscode.value = false;
      const withMode = setup({ transcode: "balanced" });
      withMode.transcode.initTranscodeModeFromRoute();
      expect(withMode.transcode.transcodeMode.value).toBe("balanced");
      expect(withMode.transcode.isTranscodePlaybackMode.value).toBe(true);

      const without = setup();
      without.transcode.initTranscodeModeFromRoute();
      expect(without.transcode.transcodeMode.value).toBe("native");
    });
  });

  describe("controller ownership", () => {
    it("reuses one controller across teardowns and creates a new one only after dispose", () => {
      const { transcode } = setup();
      const first = transcode.ensureTranscodePlaybackController();
      transcode.teardownTranscodePlayback({ stopSession: true });
      expect(transcode.ensureTranscodePlaybackController()).toBe(first);
      expect(first.disposed).toBe(false);

      transcode.disposeTranscodeController({ stopSession: true });
      expect(first.disposed).toBe(true);
      expect(transcode.transcodePlayback.value).toBeNull();
      expect(transcode.ensureTranscodePlaybackController()).not.toBe(first);
    });
  });
});
