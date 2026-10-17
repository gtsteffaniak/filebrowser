import { describe, expect, it, vi, beforeEach } from 'vitest';

vi.mock('hls.js', () => ({
  default: {
    isSupported: () => true,
    Events: {
      MEDIA_ATTACHED: 'mediaattached',
      MANIFEST_LOADING: 'manifestloading',
      MANIFEST_LOADED: 'manifestloaded',
      MANIFEST_PARSED: 'manifestparsed',
      LEVEL_LOADING: 'levelloading',
      FRAG_LOADING: 'fragloading',
      FRAG_LOADED: 'fragloaded',
      FRAG_BUFFERED: 'fragbuffered',
      ERROR: 'error',
    },
  },
}));

vi.mock('@/store', () => ({
  state: { sessionId: 'test-session' },
}));

import { attachHlsToVideo, waitUntilVideoRenderable } from '@/plyr/hlsAdapter.js';

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
});
