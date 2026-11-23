import { describe, expect, it, vi, beforeEach } from 'vitest';

vi.mock('hls.js', () => ({
  default: {
    isSupported: () => false,
    Events: {},
  },
}));

vi.mock('@/store', () => ({
  state: { sessionId: 'test-session' },
}));

import { attachHlsToVideo } from '@/player/hlsAdapter.js';

/** Minimal MediaSource-less element that reports native HLS support. */
function nativeHlsVideo() {
  const video = document.createElement('video');
  video.canPlayType = () => 'maybe';
  // jsdom does not implement load(); the cleanup path calls it and jsdom would
  // otherwise print "Not implemented: HTMLMediaElement's load() method".
  video.load = () => {};
  return video;
}

const session = { id: 'sess-1', deliveryBaseUrl: '/api/media/transcode/sessions/sess-1' };

describe('attachHlsToVideo native HLS fallback', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('stays pending until the element is ready, then reports first-buffered and resolves', async () => {
    const video = nativeHlsVideo();
    const onFirstBuffered = vi.fn();

    const attach = attachHlsToVideo({ video, session, onFirstBuffered });
    let resolved = false;
    void attach.then(() => {
      resolved = true;
    });

    await Promise.resolve();
    expect(resolved).toBe(false);
    expect(onFirstBuffered).not.toHaveBeenCalled();

    video.dispatchEvent(new Event('loadeddata'));
    const handle = await attach;

    expect(onFirstBuffered).toHaveBeenCalledTimes(1);
    expect(onFirstBuffered).toHaveBeenCalledWith(video);
    handle.cleanup();
  });

  it('reports readiness only once when both events fire', async () => {
    const video = nativeHlsVideo();
    const onFirstBuffered = vi.fn();

    const attach = attachHlsToVideo({ video, session, onFirstBuffered });
    video.dispatchEvent(new Event('loadeddata'));
    video.dispatchEvent(new Event('canplay'));
    const handle = await attach;

    expect(onFirstBuffered).toHaveBeenCalledTimes(1);
    handle.cleanup();
  });

  it('rejects when the element reports an error before it is ready', async () => {
    const video = nativeHlsVideo();
    const onFirstBuffered = vi.fn();

    const attach = attachHlsToVideo({ video, session, onFirstBuffered });
    video.dispatchEvent(new Event('error'));

    await expect(attach).rejects.toMatchObject({ name: 'NativeHlsError' });
    expect(onFirstBuffered).not.toHaveBeenCalled();
    expect(video.hasAttribute('src')).toBe(false);
    video.dispatchEvent(new Event('loadeddata'));
    expect(onFirstBuffered).not.toHaveBeenCalled();
  });

  it('rejects with an attach timeout when no event arrives in time', async () => {
    vi.useFakeTimers();
    try {
      const video = nativeHlsVideo();
      const attach = attachHlsToVideo({ video, session, onFirstBuffered: vi.fn(), budgetMs: 1_000 });
      attach.catch(() => {});

      await vi.advanceTimersByTimeAsync(1_001);
      await expect(attach).rejects.toMatchObject({ name: 'AttachTimeoutError' });
      expect(video.hasAttribute('src')).toBe(false);
      expect(vi.getTimerCount()).toBe(0);
    } finally {
      vi.useRealTimers();
    }
  });

  it('does not report readiness after cleanup', async () => {
    const video = nativeHlsVideo();
    const onFirstBuffered = vi.fn();

    const attach = attachHlsToVideo({ video, session, onFirstBuffered });
    video.dispatchEvent(new Event('canplay'));
    const handle = await attach;
    onFirstBuffered.mockClear();

    handle.cleanup();
    video.dispatchEvent(new Event('loadeddata'));
    video.dispatchEvent(new Event('canplay'));
    expect(onFirstBuffered).not.toHaveBeenCalled();
    expect(video.hasAttribute('src')).toBe(false);
  });

  it('rejects when the signal is already aborted', async () => {
    const controller = new AbortController();
    controller.abort();

    await expect(attachHlsToVideo({
      video: nativeHlsVideo(),
      session,
      signal: controller.signal,
      onFirstBuffered: vi.fn(),
    })).rejects.toMatchObject({ name: 'AbortError' });
  });
});
