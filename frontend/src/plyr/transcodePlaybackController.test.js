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

  it('works when Vue wraps the controller in a reactive proxy', async () => {
    const controller = reactive(new TranscodePlaybackController());
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

  it('coalesces concurrent duplicate starts', async () => {
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

    expect(firstResult.ok).toBe(true);
    expect(secondResult.ok).toBe(true);
    expect(controller.active).toBe(true);
    expect(attachHlsToVideo).toHaveBeenCalledOnce();
  });
});
