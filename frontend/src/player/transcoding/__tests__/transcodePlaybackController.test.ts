import { beforeEach, describe, expect, it, vi } from "vitest";
import { reactive } from "vue";
import { transcodeSession } from "@/composables/player/__tests__/mediaTestUtils.ts";
import { TranscodePlaybackController } from "@/player/transcoding/transcodePlaybackController.ts";

vi.mock("@/player/transcoding/hlsAdapter.ts", () => ({
  attachHlsToVideo: vi.fn(async () => ({
    cleanup: vi.fn(),
  })),
  attachDeadlineMs: vi.fn(() => 45_000),
}));

vi.mock("@/api/transcode.ts", () => ({
  getTranscodeSessions: vi.fn(async () => ({ canStart: false, blockReason: "user_limit" })),
  startTranscodeSession: vi.fn(async () => ({
    id: "sess-1",
    heartbeatIntervalSec: 12,
    deliveryBaseUrl: "/api/media/transcode/sessions/sess-1",
  })),
  heartbeatTranscodeSession: vi.fn(async () => {}),
  stopTranscodeSession: vi.fn(async () => {}),
}));

import { startTranscodeSession, stopTranscodeSession } from "@/api/transcode.ts";
import { attachHlsToVideo, type AttachHlsOptions, type HlsAttachment } from "@/player/transcoding/hlsAdapter.ts";

/** A promise settled from the outside, to hold an attach pending. */
function deferred<T>() {
  let resolve: (value: T) => void = () => {};
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

const attachment = (): HlsAttachment => ({ cleanup: vi.fn() });

describe("TranscodePlaybackController", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("cancels stale starts after dispose", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");
    const cleanup = vi.fn();
    const attach = deferred<HlsAttachment>();
    const attachEntered = deferred<void>();
    vi.mocked(attachHlsToVideo).mockImplementationOnce(() => {
      attachEntered.resolve();
      return attach.promise;
    });

    const pending = controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c1",
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    await attachEntered.promise;
    controller.dispose();
    attach.resolve({ cleanup });

    const result = await pending;
    expect(result).toMatchObject({ ok: false, reason: "stale" });
    expect(controller.active).toBe(false);
    expect(cleanup).toHaveBeenCalledOnce();
  });

  it("marks playback active after successful attach", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");
    const result = await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c1",
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(result.ok).toBe(true);
    expect(controller.active).toBe(true);
    expect(attachHlsToVideo).toHaveBeenCalledOnce();
  });

  it("works when Vue wraps the controller in a reactive proxy", async () => {
    const controller = reactive(new TranscodePlaybackController());
    const video = document.createElement("video");
    const result = await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c1",
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(result.ok).toBe(true);
    expect(controller.active).toBe(true);
    expect(attachHlsToVideo).toHaveBeenCalledOnce();
  });

  it("is active before first-buffered playback emits play", async () => {
    const controller = reactive(new TranscodePlaybackController());
    const video = document.createElement("video");
    let activeDuringFirstBuffered = false;
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered?.(video);
      return attachment();
    });

    const result = await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c1",
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
      onFirstBuffered: () => {
        activeDuringFirstBuffered = controller.active;
      },
    });

    expect(result.ok).toBe(true);
    expect(activeDuringFirstBuffered).toBe(true);
  });

  it("supersedes in-flight start with the latest mode switch", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");
    const firstAttach = deferred<HlsAttachment>();
    vi.mocked(attachHlsToVideo)
      .mockReturnValueOnce(firstAttach.promise)
      .mockResolvedValueOnce(attachment());

    const first = controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c1",
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });
    await new Promise<void>((resolve) => {
      setTimeout(resolve, 0);
    });

    const second = controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "datasaver",
      clientId: "c1",
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    firstAttach.resolve(attachment());

    const firstResult = await first;
    const secondResult = await second;

    expect(firstResult.ok).toBe(false);
    expect(firstResult.reason).toMatch(/^(cancelled|stale)$/);
    expect(secondResult.ok).toBe(true);
    expect(controller.active).toBe(true);
    expect(attachHlsToVideo).toHaveBeenCalledTimes(2);
  });

  it("clears startInProgress when preconditions fail so a later start still runs", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");

    const skipped = await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "",
      mode: "quality",
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(skipped).toMatchObject({ ok: false, reason: "preconditions" });
    expect(controller.startInProgress).toBe(false);

    const result = await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c1",
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(result.ok).toBe(true);
    expect(attachHlsToVideo).toHaveBeenCalledOnce();
  });

  it("clears startInProgress when disposed", async () => {
    const controller = new TranscodePlaybackController();
    controller.disposed = true;

    const result = await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      getVideoElement: () => document.createElement("video"),
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(result).toMatchObject({ ok: false, reason: "disposed" });
    expect(controller.startInProgress).toBe(false);
  });

  it("clears startInProgress when torn down mid-start", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");
    const attach = deferred<HlsAttachment>();
    vi.mocked(attachHlsToVideo).mockReturnValueOnce(attach.promise);

    const pending = controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c1",
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });
    await new Promise<void>((resolve) => {
      setTimeout(resolve, 0);
    });
    expect(attachHlsToVideo).toHaveBeenCalledOnce();

    controller.teardownLocal();
    expect(controller.startInProgress).toBe(false);

    attach.resolve(attachment());
    const result = await pending;
    expect(result).toMatchObject({ ok: false, reason: "stale" });

    // A later start must not coalesce onto the settled chain.
    const restarted = await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c2",
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });
    expect(restarted.ok).toBe(true);
  });

  it("reattaches after an attach timeout instead of falling back", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");
    const timeoutError = new Error("HLS attach timed out");
    timeoutError.name = "AttachTimeoutError";
    vi.mocked(attachHlsToVideo)
      .mockRejectedValueOnce(timeoutError)
      .mockImplementationOnce(async ({ onFirstBuffered }) => {
        onFirstBuffered?.(video);
        return attachment();
      });

    const onAttachNative = vi.fn();
    const result = await controller.start({
      source: "s",
      path: "/v.wmv",
      viewToken: "token",
      mode: "datasaver",
      clientId: "c1",
      startSec: 10.508291,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
      onAttachNative,
    });

    expect(result.ok).toBe(true);
    expect(attachHlsToVideo).toHaveBeenCalledTimes(2);
    expect(onAttachNative).not.toHaveBeenCalled();
  });

  it("gives up after the bounded reattach retries", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");
    const timeoutError = () => {
      const err = new Error("HLS attach timed out");
      err.name = "AttachTimeoutError";
      return err;
    };
    vi.mocked(attachHlsToVideo)
      .mockRejectedValueOnce(timeoutError())
      .mockRejectedValueOnce(timeoutError());

    const onAttachNative = vi.fn();
    const result = await controller.start({
      source: "s",
      path: "/v.wmv",
      viewToken: "token",
      mode: "datasaver",
      clientId: "c1",
      startSec: 10.508291,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
      onAttachNative,
    });

    expect(result.reason).toBe("fallback");
    expect(attachHlsToVideo).toHaveBeenCalledTimes(2);
  });

  it("applies a seek target past the buffered window on a reused completed session", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");
    Object.defineProperty(video, "seekable", {
      get: () => ({ length: 1, start: () => 0, end: () => 24 }),
      configurable: true,
    });
    vi.mocked(startTranscodeSession).mockResolvedValueOnce(
      transcodeSession({
        id: "sess-1",
        state: "completed",
        startSec: 0,
        endSec: 5112,
        durationSec: 5112,
        heartbeatIntervalSec: 12,
        deliveryBaseUrl: "/api/media/transcode/sessions/sess-1",
      }),
    );
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered?.(video);
      return attachment();
    });

    const result = await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c1",
      startSec: 3415.39,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(result.ok).toBe(true);
    expect(video.currentTime).toBeCloseTo(3415.39, 2);
  });

  it("applies an unproduced seek target on a running VOD session", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");
    Object.defineProperty(video, "seekable", {
      get: () => ({ length: 1, start: () => 2400, end: () => 2440 }),
      configurable: true,
    });
    vi.mocked(startTranscodeSession).mockResolvedValueOnce(
      transcodeSession({
        id: "sess-2",
        vodPlaylist: true,
        state: "running",
        durationSec: 5112,
        heartbeatIntervalSec: 12,
        deliveryBaseUrl: "/api/media/transcode/sessions/sess-2",
      }),
    );
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered?.(video);
      return attachment();
    });

    const result = await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c1",
      startSec: 2600,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(result.ok).toBe(true);
    // VOD sessions serve the whole [0, durationSec] playlist; the backend
    // waits/repositions the job for the segment, so the target is never
    // clamped to the produced edge.
    expect(video.currentTime).toBeCloseTo(2600, 1);
  });

  it("passes the requested seek to the initial attach even without a vodPlaylist flag", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");
    // Production session responses never set vodPlaylist
    vi.mocked(startTranscodeSession).mockResolvedValueOnce(
      transcodeSession({
        id: "sess-3",
        state: "running",
        durationSec: 300,
        heartbeatIntervalSec: 12,
        deliveryBaseUrl: "/api/media/transcode/sessions/sess-3",
      }),
    );
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered?.(video);
      return attachment();
    });

    await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "datasaver",
      clientId: "c1",
      startSec: 120.5,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(attachHlsToVideo).toHaveBeenCalledWith(expect.objectContaining({
      startPosition: 120.5,
    }));
  });

  it("does not request a seek for a start at zero", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered?.(video);
      return attachment();
    });

    await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c1",
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(attachHlsToVideo).toHaveBeenCalledWith(expect.objectContaining({
      startPosition: -1,
    }));
  });

  it("stops the server session on dispose when asked", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered?.(video);
      return attachment();
    });

    await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c1",
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    controller.dispose({ stopSession: true });
    await Promise.resolve();
    expect(stopTranscodeSession).toHaveBeenCalledWith("sess-1", { keepalive: false });
  });

  it("forwards keepalive to the session stop on unload", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered?.(video);
      return attachment();
    });

    await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c1",
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    controller.dispose({ stopSession: true, keepalive: true });
    await Promise.resolve();
    expect(stopTranscodeSession).toHaveBeenCalledWith("sess-1", { keepalive: true });
  });

  it("stops reporting active when the adapter fails terminally after attach", async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement("video");
    const onFatal = vi.fn();
    let adapterOnFatal: AttachHlsOptions["onFatal"];
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFatal: fatal }) => {
      adapterOnFatal = fatal;
      return attachment();
    });
    const result = await controller.start({
      source: "s",
      path: "/v.mp4",
      viewToken: "token",
      mode: "quality",
      clientId: "c1",
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
      onFatal,
    });
    expect(result.ok).toBe(true);
    expect(controller.active).toBe(true);
    adapterOnFatal?.({
      fatal: true,
      type: "mediaError",
      details: "nativeHlsError",
      code: undefined,
      reason: "decode failed",
    });
    expect(controller.active).toBe(false);
    expect(onFatal).toHaveBeenCalledOnce();
  });

  it("keeps attempt ids monotonic across reset", () => {
    const controller = new TranscodePlaybackController();
    const before = controller.attempt;
    controller.reset();
    expect(controller.attempt).toBeGreaterThan(before);
    expect(controller.isCurrentAttempt(before)).toBe(false);
  });
});
