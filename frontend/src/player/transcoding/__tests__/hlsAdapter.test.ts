import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import { transcodeSession } from "@/composables/player/__tests__/mediaTestUtils.ts";

/** The fields of an hls error event that the tests do */
interface FakeHlsErrorData {
  type: string;
  details: string;
  fatal: boolean;
  response?: { code: number };
  frag?: { sn: number; start: number; duration: number };
}

type FakeHlsHandler = (event: string, data: FakeHlsErrorData) => void;

const { FakeHls } = vi.hoisted(() => {
  class HlsStub {
    static instances: HlsStub[] = [];
    static isSupported(): boolean {
      return true;
    }
    static Events = {
      MEDIA_ATTACHED: "mediaattached",
      MANIFEST_LOADING: "manifestloading",
      MANIFEST_LOADED: "manifestloaded",
      MANIFEST_PARSED: "manifestparsed",
      LEVEL_LOADING: "levelloading",
      FRAG_LOADING: "fragloading",
      FRAG_LOADED: "fragloaded",
      FRAG_BUFFERED: "fragbuffered",
      ERROR: "error",
    };

    static ErrorTypes = { NETWORK_ERROR: "networkError" };

    handlers = new Map<string, FakeHlsHandler[]>();
    startLoad: Mock = vi.fn();
    stopLoad: Mock = vi.fn();
    detachMedia: Mock = vi.fn();
    destroy: Mock = vi.fn();
    attachMedia: Mock = vi.fn();
    loadSource: Mock = vi.fn();
    // A VOD playlist still yields a live-edge guess from the latency controller
    liveSyncPosition = 18.593;
    startPosition = 10.508291;

    constructor() {
      HlsStub.instances.push(this);
    }

    on(event: string, cb: FakeHlsHandler): void {
      this.handlers.set(event, [...(this.handlers.get(event) ?? []), cb]);
    }

    emit(event: string, data: FakeHlsErrorData): void {
      for (const cb of this.handlers.get(event) ?? []) {
        cb(event, data);
      }
    }
  }

  return { FakeHls: HlsStub };
});

vi.mock("hls.js", () => ({ default: FakeHls }));

vi.mock("@/store", () => ({
  state: { sessionId: "test-session" },
}));

import { attachDeadlineMs, attachHlsToVideo, waitUntilVideoRenderable } from "@/player/transcoding/hlsAdapter.ts";

const sessionInfo = transcodeSession({ id: "sess-1", deliveryBaseUrl: "/api/media/transcode/sessions/sess-1" });

describe("attachDeadlineMs", () => {
  it("uses the base budget for a start at 0", () => {
    expect(attachDeadlineMs(-1)).toBe(45_000);
    expect(attachDeadlineMs(0)).toBe(45_000);
  });

  it("scales the budget with the seek offset", () => {
    // datasaver resumed at 10.508291s: budget grows by the seek offset.
    expect(attachDeadlineMs(10.508291)).toBeCloseTo(45_000 + 10.508291 * 1_000, 5);
  });

  it("caps the seek warm-up allowance", () => {
    // Long seeks must not grow the budget without bound.
    expect(attachDeadlineMs(10_000)).toBe(45_000 + 45_000);
  });

  it("ignores non-finite or negative start positions", () => {
    expect(attachDeadlineMs(undefined)).toBe(45_000);
    expect(attachDeadlineMs(-5)).toBe(45_000);
  });
});

describe("attachHlsToVideo", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("waitUntilVideoRenderable resolves when video has dimensions", async () => {
    const video = document.createElement("video");
    Object.defineProperty(video, "videoWidth", { configurable: true, get: () => 640 });
    Object.defineProperty(video, "videoHeight", { configurable: true, get: () => 360 });
    Object.defineProperty(video, "readyState", { configurable: true, get: () => HTMLMediaElement.HAVE_CURRENT_DATA });

    await expect(waitUntilVideoRenderable(video, undefined, 500)).resolves.toBe(true);
  });

  it("rejects immediately when signal is already aborted", async () => {
    const video = document.createElement("video");
    const controller = new AbortController();
    controller.abort();

    await expect(attachHlsToVideo({
      video,
      session: sessionInfo,
      signal: controller.signal,
    })).rejects.toMatchObject({ name: "AbortError" });
  });

  it("resumes a 503 retry at the pending fragment, not at liveSyncPosition", async () => {
    const video = document.createElement("video");
    const session = sessionInfo;
    const controller = new AbortController();

    const attach = attachHlsToVideo({
      video,
      session,
      startPosition: 10.508291,
      signal: controller.signal,
    });
    // The promise stays pending until a fragment buffers; we only inspect the
    // error path, then abort so the attach does not leak timers.
    const hls = FakeHls.instances.at(-1);
    if (!hls) throw new Error("no Hls instance was created");

    hls.emit(FakeHls.Events.ERROR, {
      type: FakeHls.ErrorTypes.NETWORK_ERROR,
      details: "fragLoadError",
      fatal: false,
      response: { code: 503 },
      frag: { sn: 3, start: 12, duration: 4 },
    });

    expect(hls.startLoad).toHaveBeenCalledWith(12);
    expect(hls.startLoad).not.toHaveBeenCalledWith(18.593);

    controller.abort();
    await expect(attach).rejects.toMatchObject({ name: "AbortError" });
  });

  it("honors an explicit attach budget over the seek-scaled default", async () => {
    vi.useFakeTimers();
    try {
      const video = document.createElement("video");
      const session = sessionInfo;
      const controller = new AbortController();
      const attach = attachHlsToVideo({
        video,
        session,
        startPosition: 10.508291,
        budgetMs: 1_000,
        signal: controller.signal,
      });
      attach.catch(() => {});
      vi.advanceTimersByTime(1_500);
      await expect(attach).rejects.toMatchObject({ name: "AttachTimeoutError" });
      controller.abort();
    } finally {
      vi.useRealTimers();
    }
  });
});
