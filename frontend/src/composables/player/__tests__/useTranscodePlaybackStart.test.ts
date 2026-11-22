import type Plyr from "plyr";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ref, shallowRef } from "vue";
import { getTranscodeSessions } from "@/api/transcode.js";
import { router } from "@/router/index.ts";
import { useTranscodePlayback } from "../useTranscodePlayback.ts";
import { withSetup } from "./withSetup.ts";

const hoisted = vi.hoisted(() => ({
  needsTranscode: { value: false },
  results: [] as Array<{ ok: boolean; reason?: string }>,
  starts: [] as Array<Record<string, (...args: unknown[]) => unknown>>,
}));

vi.mock("@/api/transcode.js", () => ({ getTranscodeSessions: vi.fn() }));
vi.mock("@/api/viewToken.js", () => ({ getCachedViewToken: () => "" }));
vi.mock("@/i18n/index.ts", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
vi.mock("@/notify/index.ts", () => ({ notify: { showError: vi.fn() } }));
vi.mock("@/router/index.ts", () => ({
  router: { currentRoute: { value: { query: {}, path: "/files/a.mp4", hash: "" } }, replace: vi.fn() },
}));
vi.mock("@/store/index.ts", () => ({
  state: { mediaSettings: { enabled: true, profiles: [] } },
  getters: { isShare: () => false, isLoggedIn: () => true },
  mutations: { setPlaybackState: vi.fn() },
}));
vi.mock("@/utils/canBrowserPlayNative.js", () => ({ needsTranscodeFirst: () => hoisted.needsTranscode.value }));
vi.mock("@/player/transcodeMode.js", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/player/transcodeMode.js")>()),
  isTranscodeAvailable: () => true,
}));
vi.mock("@/player/transcodePlaybackController.js", () => ({
  TranscodePlaybackController: class {
    disposed = false;
    active = false;
    startInProgress = false;
    sessionController = null;
    start = vi.fn(async (params: Record<string, (...args: unknown[]) => unknown>) => {
      hoisted.starts.push(params);
      return hoisted.results.shift() ?? { ok: true };
    });
    teardownLocal = vi.fn();
    dispose = vi.fn(() => {
      this.disposed = true;
    });
  },
}));

const getSessions = vi.mocked(getTranscodeSessions);
const replace = vi.mocked(router.replace);

const REQ = { source: "src", path: "/a.mp4", name: "a.mp4", type: "video/mp4", viewToken: "token" };

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

function setup({ query = {}, player = null, video = null }: { query?: Record<string, string>; player?: unknown; video?: HTMLVideoElement | null } = {}) {
  const hooks = { onOfferShown: vi.fn(), onPlaybackStart: vi.fn(), onModeMenuChange: vi.fn() };
  const { result } = withSetup(() =>
    useTranscodePlayback({
      player: shallowRef(player as Plyr | null),
      shouldAutoplay: ref<boolean>(false),
      getReq: () => REQ,
      getRaw: () => "/raw/a.mp4",
      getRoute: () => ({ query, path: "/files/a.mp4", hash: "" }) as never,
      getVideoElement: () => video,
      getPlaybackMedia: () => video,
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

describe("useTranscodePlayback start and offer ownership", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    hoisted.needsTranscode.value = false;
    hoisted.results = [];
    hoisted.starts = [];
  });

  describe("offer after a confirmed playback failure", () => {
    it("offers even when the probe predicts native support, but only for a confirmed failure", async () => {
      getSessions.mockResolvedValue({ enabled: true, canStart: true, sessions: [] });
      const proactive = setup();
      await proactive.transcode.offerTranscodePlayback();
      expect(proactive.transcode.transcodeOffer.value).toBeNull();

      const confirmed = setup();
      await confirmed.transcode.offerTranscodePlayback({ confirmedFailure: true });
      expect(confirmed.transcode.transcodeOffer.value?.mode).toBe("offer");
    });
  });

  describe("obsolete offer lookups ", () => {
    it("does not publish an offer when the file was reset while the lookup was pending", async () => {
      hoisted.needsTranscode.value = true;
      const lookup = deferred<unknown>();
      getSessions.mockReturnValue(lookup.promise as never);
      const { transcode, hooks } = setup();
      const pending = transcode.offerTranscodePlayback();
      transcode.resetFileState();
      lookup.resolve({ enabled: true, canStart: true, sessions: [] });
      await pending;
      expect(transcode.transcodeOffer.value).toBeNull();
      expect(hooks.onOfferShown).not.toHaveBeenCalled();
    });

    it("does not publish an offer after the lookup failed for an obsolete request", async () => {
      hoisted.needsTranscode.value = true;
      const lookup = deferred<unknown>();
      getSessions.mockReturnValue(lookup.promise as never);
      const { transcode, hooks } = setup();
      const pending = transcode.offerTranscodePlayback();
      transcode.teardownTranscodePlayback({ stopSession: true });
      lookup.resolve(Promise.reject(new Error("503")));
      await pending;
      expect(hooks.onOfferShown).not.toHaveBeenCalled();
    });
  });

  describe("start preparation", () => {
    it("drops a start whose session lookup finishes after teardown", async () => {
      const lookup = deferred<unknown>();
      getSessions.mockReturnValue(lookup.promise as never);
      const { transcode } = setup();
      const pending = transcode.startTranscodePlayback({ mode: "balanced" });
      transcode.teardownTranscodePlayback({ stopSession: true });
      lookup.resolve({ sessions: [] });
      const result = await pending;
      expect(result).toEqual({ ok: false, reason: "cancelled" });
      expect(hoisted.starts).toHaveLength(0);
      expect(transcode.transcodeMode.value).toBe("native");
      expect(transcode.transcodeSwitchInProgress.value).toBe(false);
    });
  });

  describe("recovery result", () => {
    async function recoverWith(second: { ok: boolean; reason?: string }) {
      const { transcode } = setup();
      hoisted.results = [{ ok: true }, second];
      await transcode.startTranscodePlayback({ mode: "balanced", replaceSessionId: "s1" });
      const recover = hoisted.starts[0]?.onRecoverPlayback;
      if (!recover) throw new Error("no recover callback");
      return (await recover({ startSec: 5, sessionId: "s1", mode: "balanced" })) as boolean;
    }

    it("reports recovery only when the replacement attachment succeeded", async () => {
      expect(await recoverWith({ ok: true })).toBe(true);
      expect(await recoverWith({ ok: false, reason: "fallback" })).toBe(false);
      expect(await recoverWith({ ok: false, reason: "error" })).toBe(false);
    });

    it("does not report a superseded or torn-down replacement as a failure", async () => {
      expect(await recoverWith({ ok: false, reason: "cancelled" })).toBe(true);
      expect(await recoverWith({ ok: false, reason: "recovered" })).toBe(true);
    });
  });

  describe("terminal attachment failure", () => {
    it("restores native mode for a natively playable source", async () => {
      hoisted.results = [{ ok: false, reason: "fallback" }];
      const { transcode } = setup();
      const result = await transcode.startTranscodePlayback({ mode: "balanced" });
      expect(result).toEqual({ ok: false, reason: "fallback" });
      expect(transcode.transcodeMode.value).toBe("native");
    });

    it("offers transcoding again when native playback is not possible either", async () => {
      hoisted.needsTranscode.value = true;
      getSessions.mockResolvedValue({ enabled: true, canStart: true, sessions: [] });
      hoisted.results = [{ ok: false, reason: "fallback" }];
      const { transcode, hooks } = setup();
      await transcode.startTranscodePlayback({ mode: "balanced" });
      expect(transcode.transcodeMode.value).toBe("native");
      expect(transcode.transcodeOffer.value?.mode).toBe("offer");
      expect(hooks.onOfferShown).toHaveBeenCalledTimes(1);
    });
  });

  describe("playback URL after a managed switch", () => {
    it("writes the HLS mode to the URL after a successful start", async () => {
      hoisted.results = [{ ok: true }];
      const { transcode } = setup();
      await transcode.startTranscodePlayback({ mode: "balanced" });
      expect(replace).toHaveBeenCalledTimes(1);
      expect(replace.mock.calls[0]?.[0]).toMatchObject({ query: { transcode: "balanced" } });
    });

    it("removes the rejected HLS mode from the URL after a capacity fallback", async () => {
      hoisted.results = [{ ok: false, reason: "capacity" }];
      const { transcode } = setup({ query: { transcode: "balanced" } });
      await transcode.startTranscodePlayback({ mode: "balanced" });
      expect(transcode.transcodeMode.value).toBe("native");
      expect(replace).toHaveBeenCalled();
      expect(replace.mock.calls.at(-1)?.[0]).toMatchObject({ query: {} });
    });
  });

  describe("pause at zero when switching to native", () => {
    it("does not start playback of a paused player at 0:00", async () => {
      const video = document.createElement("video");
      video.play = vi.fn(() => Promise.resolve());
      const player = { currentTime: 0, playing: false, play: vi.fn(() => Promise.resolve()) };
      const { transcode } = setup({ query: { transcode: "balanced" }, player, video });
      transcode.initTranscodeModeFromRoute();
      expect(transcode.transcodeMode.value).toBe("balanced");
      await transcode.switchTranscodeMode("native");
      expect(video.play).not.toHaveBeenCalled();
      expect(player.play).not.toHaveBeenCalled();
    });
  });
});
