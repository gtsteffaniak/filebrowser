import { describe, expect, it, vi, beforeEach } from 'vitest';
import { reactive } from 'vue';
import { TranscodePlaybackController } from '@/plyr/transcodePlaybackController.js';

vi.mock('@/plyr/hlsAdapter.js', () => ({
  attachHlsToVideo: vi.fn(async () => ({
    cleanup: vi.fn(),
  })),
  attachDeadlineMs: vi.fn(() => 45_000),
}));

vi.mock('@/api/transcode.js', () => ({
  getTranscodeSessions: vi.fn(async () => ({ canStart: false, blockReason: 'user_limit' })),
  startTranscodeSession: vi.fn(async () => ({
    id: 'sess-1',
    heartbeatIntervalSec: 12,
    deliveryBaseUrl: '/api/media/transcode/sessions/sess-1',
  })),
  heartbeatTranscodeSession: vi.fn(async () => {}),
  stopTranscodeSession: vi.fn(async () => {}),
}));

import { attachHlsToVideo } from '@/plyr/hlsAdapter.js';
import { startTranscodeSession, stopTranscodeSession } from '@/api/transcode.js';

describe('TranscodePlaybackController', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('cancels stale starts after dispose', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');
    let resolveStart;
    const startPromise = new Promise((resolve) => {
      resolveStart = resolve;
    });
    vi.mocked(attachHlsToVideo).mockReturnValueOnce(startPromise);

    const pending = controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      clientId: 'c1',
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    controller.dispose();
    resolveStart({ cleanup: vi.fn() });

    const result = await pending;
    expect(result.ok).toBe(false);
    expect(['cancelled', 'stale', 'disposed']).toContain(result.reason);
  });

  it('marks playback active after successful attach', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');
    const result = await controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      clientId: 'c1',
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(result.ok).toBe(true);
    expect(controller.active).toBe(true);
    expect(attachHlsToVideo).toHaveBeenCalledOnce();
  });

  it('works when Vue wraps the controller in a reactive proxy', async () => {    const controller = reactive(new TranscodePlaybackController());
    const video = document.createElement('video');
    const result = await controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      clientId: 'c1',
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(result.ok).toBe(true);
    expect(controller.active).toBe(true);
    expect(attachHlsToVideo).toHaveBeenCalledOnce();
  });

  it('is active before first-buffered playback emits play', async () => {
    const controller = reactive(new TranscodePlaybackController());
    const video = document.createElement('video');
    let activeDuringFirstBuffered = false;
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered();
      return { cleanup: vi.fn() };
    });

    const result = await controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      clientId: 'c1',
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

  it('supersedes in-flight start with the latest mode switch', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');
    let resolveFirstAttach;
    vi.mocked(attachHlsToVideo)
      .mockReturnValueOnce(new Promise((resolve) => {
        resolveFirstAttach = resolve;
      }))
      .mockResolvedValueOnce({ cleanup: vi.fn() });

    const first = controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      clientId: 'c1',
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    // Let the first start reach HLS attach before superseding it.
    await new Promise((resolve) => setTimeout(resolve, 0));

    const second = controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'datasaver',
      clientId: 'c1',
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    resolveFirstAttach({ cleanup: vi.fn() });

    const firstResult = await first;
    const secondResult = await second;

    expect(firstResult.ok).toBe(false);
    expect(['cancelled', 'stale']).toContain(firstResult.reason);
    expect(secondResult.ok).toBe(true);
    expect(controller.active).toBe(true);
    expect(attachHlsToVideo).toHaveBeenCalledTimes(2);
  });

  it('clears startInProgress when preconditions fail so a later start still runs', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');

    const skipped = await controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: '',
      mode: 'quality',
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(skipped).toMatchObject({ ok: false, reason: 'preconditions' });
    expect(controller.startInProgress).toBe(false);

    const result = await controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      clientId: 'c1',
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(result.ok).toBe(true);
    expect(attachHlsToVideo).toHaveBeenCalledOnce();
  });

  it('clears startInProgress when disposed', async () => {
    const controller = new TranscodePlaybackController();
    controller.disposed = true;

    const result = await controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      getVideoElement: () => document.createElement('video'),
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(result).toMatchObject({ ok: false, reason: 'disposed' });
    expect(controller.startInProgress).toBe(false);
  });

  it('clears startInProgress when torn down mid-start', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');
    let resolveAttach;
    vi.mocked(attachHlsToVideo).mockReturnValueOnce(new Promise((resolve) => {
      resolveAttach = resolve;
    }));

    const pending = controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      clientId: 'c1',
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    // Let the start reach the HLS attach stage before tearing it down.
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(attachHlsToVideo).toHaveBeenCalledOnce();

    // teardownLocal invalidates the attempt; its finally block skips the reset.
    controller.teardownLocal();
    expect(controller.startInProgress).toBe(false);

    resolveAttach({ cleanup: vi.fn() });
    const result = await pending;
    expect(result).toMatchObject({ ok: false, reason: 'stale' });

    // A later start must not coalesce onto the settled chain.
    const restarted = await controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      clientId: 'c2',
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });
    expect(restarted.ok).toBe(true);
  });

  it('reattaches after an attach timeout instead of falling back', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');
    const timeoutError = new Error('HLS attach timed out');
    timeoutError.name = 'AttachTimeoutError';
    vi.mocked(attachHlsToVideo)
      .mockRejectedValueOnce(timeoutError)
      .mockImplementationOnce(async ({ onFirstBuffered }) => {
        onFirstBuffered();
        return { cleanup: vi.fn() };
      });

    const onAttachNative = vi.fn();
    const result = await controller.start({
      source: 's',
      path: '/v.wmv',
      viewToken: 'token',
      mode: 'datasaver',
      clientId: 'c1',
      startSec: 10.508291,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
      onAttachNative,
    });

    expect(result.ok).toBe(true);
    expect(attachHlsToVideo).toHaveBeenCalledTimes(2);
    // A timed-out attach is a race, not a format problem: never fall back.
    expect(onAttachNative).not.toHaveBeenCalled();
  });

  it('gives up after the bounded reattach retries', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');
    const timeoutError = () => {
      const err = new Error('HLS attach timed out');
      err.name = 'AttachTimeoutError';
      return err;
    };
    vi.mocked(attachHlsToVideo)
      .mockRejectedValueOnce(timeoutError())
      .mockRejectedValueOnce(timeoutError());

    const onAttachNative = vi.fn();
    const result = await controller.start({
      source: 's',
      path: '/v.wmv',
      viewToken: 'token',
      mode: 'datasaver',
      clientId: 'c1',
      startSec: 10.508291,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
      onAttachNative,
    });

    expect(result.reason).toBe('fallback');
    expect(attachHlsToVideo).toHaveBeenCalledTimes(2);
  });

  it('applies a seek target past the buffered window on a reused completed session', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');
    // Only the first fragments are buffered, but a completed generation can
    // serve its whole playlist on demand.
    Object.defineProperty(video, 'seekable', {
      get: () => ({ length: 1, start: () => 0, end: () => 24 }),
      configurable: true,
    });
    vi.mocked(startTranscodeSession).mockResolvedValueOnce({
      id: 'sess-1',
      state: 'completed',
      startSec: 0,
      endSec: 5112,
      durationSec: 5112,
      heartbeatIntervalSec: 12,
      deliveryBaseUrl: '/api/media/transcode/sessions/sess-1',
    });
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered();
      return { cleanup: vi.fn() };
    });

    const result = await controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      clientId: 'c1',
      startSec: 3415.39,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(result.ok).toBe(true);
    // The target must not be clamped to the buffered edge (24s).
    expect(video.currentTime).toBeCloseTo(3415.39, 2);
  });

  it('applies an unproduced seek target on a running VOD session', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');
    Object.defineProperty(video, 'seekable', {
      get: () => ({ length: 1, start: () => 2400, end: () => 2440 }),
      configurable: true,
    });
    vi.mocked(startTranscodeSession).mockResolvedValueOnce({
      id: 'sess-2',
      vodPlaylist: true,
      state: 'running',
      durationSec: 5112,
      heartbeatIntervalSec: 12,
      deliveryBaseUrl: '/api/media/transcode/sessions/sess-2',
    });
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered();
      return { cleanup: vi.fn() };
    });

    const result = await controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      clientId: 'c1',
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

  it('passes the requested seek to the initial attach even without a vodPlaylist flag', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');
    // Production session responses never set vodPlaylist; gating the seek on it
    // forced a warm-up from zero followed by an after-the-fact seek.
    vi.mocked(startTranscodeSession).mockResolvedValueOnce({
      id: 'sess-3',
      state: 'running',
      durationSec: 300,
      heartbeatIntervalSec: 12,
      deliveryBaseUrl: '/api/media/transcode/sessions/sess-3',
    });
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered();
      return { cleanup: vi.fn() };
    });

    await controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'datasaver',
      clientId: 'c1',
      startSec: 120.5,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(attachHlsToVideo).toHaveBeenCalledWith(expect.objectContaining({
      startPosition: 120.5,
    }));
  });

  it('does not request a seek for a start at zero', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered();
      return { cleanup: vi.fn() };
    });

    await controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      clientId: 'c1',
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    expect(attachHlsToVideo).toHaveBeenCalledWith(expect.objectContaining({
      startPosition: -1,
    }));
  });

  it('stops the server session on dispose when asked', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered();
      return { cleanup: vi.fn() };
    });

    await controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      clientId: 'c1',
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    controller.dispose({ stopSession: true });
    await Promise.resolve();
    expect(stopTranscodeSession).toHaveBeenCalledWith('sess-1', { keepalive: false });
  });

  it('forwards keepalive to the session stop on unload', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');
    vi.mocked(attachHlsToVideo).mockImplementationOnce(async ({ onFirstBuffered }) => {
      onFirstBuffered();
      return { cleanup: vi.fn() };
    });

    await controller.start({
      source: 's',
      path: '/v.mp4',
      viewToken: 'token',
      mode: 'quality',
      clientId: 'c1',
      startSec: 0,
      getVideoElement: () => video,
      nextTick: async () => {},
      shouldAutoplay: false,
    });

    controller.dispose({ stopSession: true, keepalive: true });
    await Promise.resolve();
    expect(stopTranscodeSession).toHaveBeenCalledWith('sess-1', { keepalive: true });
  });
});
