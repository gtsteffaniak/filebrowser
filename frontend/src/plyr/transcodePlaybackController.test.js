import { describe, expect, it, vi, beforeEach } from 'vitest';
import { reactive } from 'vue';
import { TranscodePlaybackController } from '@/plyr/transcodePlaybackController.js';

vi.mock('@/plyr/hlsAdapter.js', () => ({
  attachHlsToVideo: vi.fn(async () => ({
    cleanup: vi.fn(),
  })),
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
import { startTranscodeSession } from '@/api/transcode.js';

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

  it('clamps a seek target to the produced edge on a running session', async () => {
    const controller = new TranscodePlaybackController();
    const video = document.createElement('video');
    Object.defineProperty(video, 'seekable', {
      get: () => ({ length: 1, start: () => 2400, end: () => 2440 }),
      configurable: true,
    });
    vi.mocked(startTranscodeSession).mockResolvedValueOnce({
      id: 'sess-2',
      state: 'running',
      startSec: 2400,
      endSec: 2440,
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
    // Production has not reached 2600 yet; clamp to the produced edge and let
    // the precise-seek wait re-apply the target once production catches up.
    expect(video.currentTime).toBeCloseTo(2440, 1);
  });
});
