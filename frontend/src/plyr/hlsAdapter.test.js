import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';

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

import { attachHlsToVideo, pinMediaTimeline } from '@/plyr/hlsAdapter.js';

describe('attachHlsToVideo', () => {
  beforeEach(() => {
    vi.clearAllMocks();
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

/**
 * Simulates a raw element timeline (what the engine actually appended) so
 * pinMediaTimeline's translation can be verified: `__rawTime` is the raw
 * currentTime and `__rawRanges` the raw seekable/buffered/played ranges.
 */
function stubMediaProto(ranges) {
  const proto = HTMLMediaElement.prototype;
  const saved = {};
  for (const name of ['currentTime', 'seekable', 'buffered', 'played']) {
    saved[name] = Object.getOwnPropertyDescriptor(proto, name);
  }
  const rawRanges = (el) => ({
    length: el.__rawRanges.length,
    start: (i) => el.__rawRanges[i][0],
    end: (i) => el.__rawRanges[i][1],
  });
  Object.defineProperty(proto, 'currentTime', {
    configurable: true,
    get() { return this.__rawTime ?? 0; },
    set(v) { this.__rawTime = v; },
  });
  for (const name of ['seekable', 'buffered', 'played']) {
    Object.defineProperty(proto, name, {
      configurable: true,
      get() { return rawRanges(this); },
    });
  }
  return () => {
    for (const [name, desc] of Object.entries(saved)) {
      if (desc) {
        Object.defineProperty(proto, name, desc);
      } else {
        delete proto[name];
      }
    }
  };
}

describe('pinMediaTimeline', () => {
  let restoreProto;
  beforeEach(() => {
    restoreProto = stubMediaProto();
  });
  afterEach(() => {
    restoreProto?.();
    restoreProto = null;
  });

  it('is a no-op for a generation starting at zero', () => {
    const video = document.createElement('video');
    expect(pinMediaTimeline(video, 0, 0)).toBeNull();
    expect(Object.getOwnPropertyDescriptor(video, 'currentTime')).toBeUndefined();
  });

  it('exposes a source-relative timeline over a 0-based raw origin', () => {
    const video = document.createElement('video');
    video.__rawRanges = [[0, 8]];
    // An engine that rebases the first fragment to raw time 0 gets the
    // generation start as the initial offset.
    const pin = pinMediaTimeline(video, 2400, 2400);

    video.currentTime = 2405;
    expect(video.__rawTime).toBe(5);
    expect(video.currentTime).toBe(2405);
    expect(video.seekable.start(0)).toBe(2400);
    expect(video.seekable.end(0)).toBe(2408);
    expect(video.buffered.end(0)).toBe(2408);

    pin.restore();
    expect(video.__rawTime).toBe(5);
    expect(video.currentTime).toBe(5);
    expect(video.seekable.start(0)).toBe(0);
  });

  it('measures the real raw origin for engines that keep media timestamps', () => {
    const video = document.createElement('video');
    // Native playback keeps tfdt positions — the raw range already starts at
    // the generation start, so the measured offset collapses to ~0.
    video.__rawRanges = [[2400, 2408]];
    const pin = pinMediaTimeline(video, 2400, 0);

    pin.measure();
    expect(video.seekable.start(0)).toBeCloseTo(2400, 5);
    video.currentTime = 2402;
    expect(video.__rawTime).toBeCloseTo(2402, 5);
    pin.restore();
  });

  it('measures once and tolerates an initially empty seekable', () => {
    const video = document.createElement('video');
    video.__rawRanges = [];
    const pin = pinMediaTimeline(video, 2400, 2400);

    pin.measure(); // nothing buffered yet — keeps the assumed offset
    expect(video.seekable.length).toBe(0);

    video.__rawRanges = [[0, 4]];
    pin.measure();
    expect(video.seekable.start(0)).toBe(2400);
    pin.restore();
  });
});
