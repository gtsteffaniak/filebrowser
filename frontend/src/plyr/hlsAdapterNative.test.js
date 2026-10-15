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

import { attachHlsToVideo } from '@/plyr/hlsAdapter.js';

/** Minimal MediaSource-less element that reports native HLS support. */
function nativeHlsVideo() {
  const video = document.createElement('video');
  video.canPlayType = () => 'maybe';
  return video;
}

describe('attachHlsToVideo native HLS fallback', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('reports first-buffered once the element is ready', async () => {
    const video = nativeHlsVideo();
    const onFirstBuffered = vi.fn();

    const handle = await attachHlsToVideo({
      video,
      session: { id: 'sess-1', deliveryBaseUrl: '/api/media/transcode/sessions/sess-1' },
      onFirstBuffered,
    });

    expect(onFirstBuffered).not.toHaveBeenCalled();
    video.dispatchEvent(new Event('loadeddata'));
    expect(onFirstBuffered).toHaveBeenCalledTimes(1);
    expect(onFirstBuffered).toHaveBeenCalledWith(video);

    handle.cleanup();
  });

  it('reports readiness only once when both events fire', async () => {
    const video = nativeHlsVideo();
    const onFirstBuffered = vi.fn();

    const handle = await attachHlsToVideo({
      video,
      session: { id: 'sess-1', deliveryBaseUrl: '/api/media/transcode/sessions/sess-1' },
      onFirstBuffered,
    });

    video.dispatchEvent(new Event('loadeddata'));
    video.dispatchEvent(new Event('canplay'));
    expect(onFirstBuffered).toHaveBeenCalledTimes(1);

    handle.cleanup();
  });

  it('does not report readiness after cleanup', async () => {
    const video = nativeHlsVideo();
    const onFirstBuffered = vi.fn();

    const handle = await attachHlsToVideo({
      video,
      session: { id: 'sess-1', deliveryBaseUrl: '/api/media/transcode/sessions/sess-1' },
      onFirstBuffered,
    });

    handle.cleanup();
    video.dispatchEvent(new Event('loadeddata'));
    video.dispatchEvent(new Event('canplay'));
    expect(onFirstBuffered).not.toHaveBeenCalled();
  });

  it('rejects when the signal is already aborted', async () => {
    const controller = new AbortController();
    controller.abort();

    await expect(attachHlsToVideo({
      video: nativeHlsVideo(),
      session: { id: 'sess-1', deliveryBaseUrl: '/api/media/transcode/sessions/sess-1' },
      signal: controller.signal,
      onFirstBuffered: vi.fn(),
    })).rejects.toMatchObject({ name: 'AbortError' });
  });
});
