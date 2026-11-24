import { describe, expect, it, vi, beforeEach } from 'vitest';

vi.mock('hls.js', () => {
  class FakeHls {
    static instances = [];

    static isSupported() {
      return true;
    }

    static Events = {
      MEDIA_ATTACHED: 'mediaattached',
      MANIFEST_LOADING: 'manifestloading',
      MANIFEST_LOADED: 'manifestloaded',
      MANIFEST_PARSED: 'manifestparsed',
      LEVEL_LOADING: 'levelloading',
      FRAG_LOADING: 'fragloading',
      FRAG_LOADED: 'fragloaded',
      FRAG_BUFFERED: 'fragbuffered',
      ERROR: 'error',
    };

    static ErrorTypes = { NETWORK_ERROR: 'networkError' };

    constructor() {
      this.handlers = new Map();
      this.startLoad = vi.fn();
      this.stopLoad = vi.fn();
      this.detachMedia = vi.fn();
      this.destroy = vi.fn();
      this.attachMedia = vi.fn();
      this.loadSource = vi.fn();
      // A VOD playlist still yields a live-edge guess from the latency
      // controller; 18.593s is the real value for the 30.593s WMV clip.
      this.liveSyncPosition = 18.593;
      this.startPosition = 10.508291;
      FakeHls.instances.push(this);
    }

    on(event, cb) {
      if (!this.handlers.has(event)) {
        this.handlers.set(event, []);
      }
      this.handlers.get(event).push(cb);
    }

    emit(event, data) {
      for (const cb of this.handlers.get(event) || []) {
        cb(event, data);
      }
    }
  }

  return { default: FakeHls };
});

vi.mock('@/store', () => ({
  state: { sessionId: 'test-session' },
}));

import { attachHlsToVideo, waitUntilVideoRenderable, attachDeadlineMs, Hls } from '@/plyr/hlsAdapter.js';

describe('attachDeadlineMs', () => {
  it('uses the base budget for a start at 0', () => {
    expect(attachDeadlineMs(-1)).toBe(45_000);
    expect(attachDeadlineMs(0)).toBe(45_000);
  });

  it('scales the budget with the seek offset', () => {
    // datasaver resumed at 10.508291s: budget grows by the seek offset.
    expect(attachDeadlineMs(10.508291)).toBeCloseTo(45_000 + 10.508291 * 1_000, 5);
  });

  it('caps the seek warm-up allowance', () => {
    // Long seeks must not grow the budget without bound.
    expect(attachDeadlineMs(10_000)).toBe(45_000 + 45_000);
  });

  it('ignores non-finite or negative start positions', () => {
    expect(attachDeadlineMs(undefined)).toBe(45_000);
    expect(attachDeadlineMs(-5)).toBe(45_000);
  });
});

describe('attachHlsToVideo', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('waitUntilVideoRenderable resolves when video has dimensions', async () => {
    const video = document.createElement('video');
    Object.defineProperty(video, 'videoWidth', { configurable: true, get: () => 640 });
    Object.defineProperty(video, 'videoHeight', { configurable: true, get: () => 360 });
    Object.defineProperty(video, 'readyState', { configurable: true, get: () => HTMLMediaElement.HAVE_CURRENT_DATA });

    await expect(waitUntilVideoRenderable(video, null, 500)).resolves.toBe(true);
  });

  it('rejects immediately when signal is already aborted', async () => {
    const video = document.createElement('video');
    const controller = new AbortController();
    controller.abort();

    await expect(attachHlsToVideo({
      video,
      session: { id: 'sess-1', deliveryBaseUrl: '/api/media/transcode/sessions/sess-1' },
      signal: controller.signal,
    })).rejects.toMatchObject({ name: 'AbortError' });
  });

  it('resumes a 503 retry at the pending fragment, not at liveSyncPosition', async () => {
    const video = document.createElement('video');
    const session = { id: 'sess-1', deliveryBaseUrl: '/api/media/transcode/sessions/sess-1' };
    const controller = new AbortController();

    const attach = attachHlsToVideo({
      video,
      session,
      startPosition: 10.508291,
      signal: controller.signal,
    });
    // The promise stays pending until a fragment buffers; we only inspect the
    // error path, then abort so the attach does not leak timers.
    const hls = Hls.instances.at(-1);

    hls.emit(Hls.Events.ERROR, {
      type: Hls.ErrorTypes.NETWORK_ERROR,
      details: 'fragLoadError',
      fatal: false,
      response: { code: 503 },
      // Segment 00003 starts at 12s on this playlist.
      frag: { sn: 3, start: 12, duration: 4 },
    });

    expect(hls.startLoad).toHaveBeenCalledWith(12);
    // The old behavior jumped to the VOD live-edge guess (18.593s), which
    // abandons the not-yet-ready fragment hls.js is already retrying.
    expect(hls.startLoad).not.toHaveBeenCalledWith(18.593);

    controller.abort();
    await expect(attach).rejects.toMatchObject({ name: 'AbortError' });
  });

  it('honors an explicit attach budget over the seek-scaled default', async () => {
    vi.useFakeTimers();
    try {
      const video = document.createElement('video');
      const session = { id: 'sess-1', deliveryBaseUrl: '/api/media/transcode/sessions/sess-1' };
      const controller = new AbortController();

      const attach = attachHlsToVideo({
        video,
        session,
        startPosition: 10.508291,
        // The retry loop already spent part of the seek-scaled budget; the
        // remaining budget must bound this attempt, not a fresh 45s+seek.
        budgetMs: 1_000,
        signal: controller.signal,
      });
      attach.catch(() => {});

      // Just past the explicit budget the attach must time out.
      vi.advanceTimersByTime(1_500);
      await expect(attach).rejects.toMatchObject({ name: 'AttachTimeoutError' });
      controller.abort();
    } finally {
      vi.useRealTimers();
    }
  });
});
