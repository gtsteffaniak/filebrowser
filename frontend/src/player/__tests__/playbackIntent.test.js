import { describe, expect, it, vi } from 'vitest';
import {
  appendMediaFragment,
  continuePlay,
  isPlayBlockedError,
  isPlaybackProgressing,
  isPlayTeardownError,
  primePlay,
  resumeWhenReady,
} from '@/player/playbackIntent.js';

function videoStub({ readyState = 0, paused = true, ended = false, play } = {}) {
  const listeners = new Map();
  return {
    readyState,
    paused,
    ended,
    play: vi.fn(play ?? (() => Promise.resolve())),
    addEventListener: vi.fn((event, fn) => listeners.set(event, fn)),
    removeEventListener: vi.fn((event) => listeners.delete(event)),
    emit(event) {
      const fn = listeners.get(event);
      if (fn) fn();
    },
  };
}

const blockedError = () => Object.assign(new Error('blocked'), { name: 'NotAllowedError' });

describe('playbackIntent', () => {
  it('classifies play errors', () => {
    expect(isPlayBlockedError({ name: 'NotAllowedError' })).toBe(true);
    expect(isPlayTeardownError({ name: 'AbortError' })).toBe(true);
    expect(isPlayTeardownError({ name: 'NotSupportedError' })).toBe(true);
    expect(isPlayBlockedError({ name: 'AbortError' })).toBe(false);
  });

  it('primePlay returns play promise', () => {
    const play = vi.fn(() => Promise.resolve());
    const video = { play };
    expect(primePlay(video)).toBeInstanceOf(Promise);
    expect(play).toHaveBeenCalledOnce();
  });

  it('isPlaybackProgressing requires unpaused, unended and buffered media', () => {
    expect(isPlaybackProgressing(null)).toBe(false);
    // Unpaused but with no buffer is the frozen-frame case after a MediaSource swap.
    expect(isPlaybackProgressing(videoStub({ readyState: 1, paused: false }))).toBe(false);
    expect(isPlaybackProgressing(videoStub({ readyState: 4, paused: true }))).toBe(false);
    expect(isPlaybackProgressing(videoStub({ readyState: 4, paused: false }))).toBe(true);
    expect(isPlaybackProgressing(videoStub({ readyState: 4, paused: false, ended: true }))).toBe(false);
  });

  it('resumeWhenReady calls playCall when canplay already available', () => {
    const playCall = vi.fn(() => Promise.resolve());
    const video = videoStub({ readyState: HTMLMediaElement.HAVE_FUTURE_DATA, paused: true });
    void resumeWhenReady(video, { playCall });
    expect(playCall).toHaveBeenCalledOnce();
  });

  it('resumeWhenReady reports blocked when play is denied', async () => {
    const onPlaying = vi.fn();
    const video = videoStub({ readyState: HTMLMediaElement.HAVE_FUTURE_DATA, paused: true });
    const result = await resumeWhenReady(video, {
      playCall: () => Promise.reject(blockedError()),
      onPlaying,
    });
    expect(result).toEqual({ played: false, blocked: true });
    expect(onPlaying).not.toHaveBeenCalled();
  });

  it('resumeWhenReady skips when already progressing', async () => {
    const playCall = vi.fn();
    const video = videoStub({ readyState: HTMLMediaElement.HAVE_FUTURE_DATA, paused: false });
    const result = await resumeWhenReady(video, { playCall });
    expect(playCall).not.toHaveBeenCalled();
    expect(result.played).toBe(true);
  });

  it('resumeWhenReady does not play after the preview was torn down', async () => {
    const playCall = vi.fn(() => Promise.resolve());
    const video = videoStub({ readyState: HTMLMediaElement.HAVE_FUTURE_DATA, paused: true });
    const result = await resumeWhenReady(video, { playCall, isCancelled: () => true });
    expect(playCall).not.toHaveBeenCalled();
    expect(result).toEqual({ played: false, blocked: false });
  });

  it('continuePlay skips onPlaying when cancelled', async () => {
    const onPlaying = vi.fn();
    const video = videoStub({ readyState: HTMLMediaElement.HAVE_FUTURE_DATA, paused: false });
    const result = await continuePlay(video, Promise.resolve(), {
      onPlaying,
      isCancelled: () => true,
    });
    expect(onPlaying).not.toHaveBeenCalled();
    expect(result).toEqual({ played: false, blocked: false });
  });

  it('continuePlay reports playing when the primed promise resolves and media progresses', async () => {
    const onPlaying = vi.fn();
    const video = videoStub({ readyState: HTMLMediaElement.HAVE_FUTURE_DATA, paused: false });
    const result = await continuePlay(video, Promise.resolve(), { onPlaying });
    expect(onPlaying).toHaveBeenCalledOnce();
    expect(result.played).toBe(true);
  });

  it('continuePlay resumes the current source when a stale primed promise resolves', async () => {
    // A mode switch replaces the source: the primed promise belongs to the old one,
    // so its resolution must not be treated as playback of the new source.
    const playCall = vi.fn(() => Promise.resolve());
    const onPlaying = vi.fn();
    const video = videoStub({ readyState: HTMLMediaElement.HAVE_FUTURE_DATA, paused: true });
    const result = await continuePlay(video, Promise.resolve(), { playCall, onPlaying });
    expect(playCall).toHaveBeenCalledOnce();
    expect(onPlaying).toHaveBeenCalledOnce();
    expect(result.played).toBe(true);
  });

  it('continuePlay falls back when primed promise fails during handoff', async () => {
    const playCall = vi.fn(() => Promise.resolve());
    const video = videoStub({ readyState: HTMLMediaElement.HAVE_FUTURE_DATA, paused: true });
    await continuePlay(
      video,
      Promise.reject(Object.assign(new Error('no source'), { name: 'NotSupportedError' })),
      { playCall },
    );
    expect(playCall).toHaveBeenCalledOnce();
  });

  it('continuePlay does not retry when autoplay is blocked', async () => {
    const playCall = vi.fn(() => Promise.resolve());
    const video = videoStub({ readyState: HTMLMediaElement.HAVE_FUTURE_DATA, paused: true });
    const result = await continuePlay(video, Promise.reject(blockedError()), { playCall });
    expect(playCall).not.toHaveBeenCalled();
    expect(result.blocked).toBe(true);
  });

  it('continuePlay does not report playing without a video element', async () => {
    const onPlaying = vi.fn();
    const result = await continuePlay(null, Promise.resolve(), { onPlaying });
    expect(onPlaying).not.toHaveBeenCalled();
    expect(result.played).toBe(false);
  });
});

describe('playbackIntent cancellation and single-attempt', () => {
  const deferred = () => {
    let resolve;
    let reject;
    const promise = new Promise((res, rej) => {
      resolve = res;
      reject = rej;
    });
    return { promise, resolve, reject };
  };

  it('resumeWhenReady does not report success for a cancelled operation on a playing element', async () => {
    const onPlaying = vi.fn();
    const video = videoStub({ readyState: HTMLMediaElement.HAVE_FUTURE_DATA, paused: false });
    const result = await resumeWhenReady(video, { onPlaying, isCancelled: () => true });
    expect(result).toEqual({ played: false, blocked: false });
    expect(onPlaying).not.toHaveBeenCalled();
  });

  it('resumeWhenReady makes a single play attempt when canplay fires again while play is pending', async () => {
    const pendingPlay = deferred();
    const playCall = vi.fn(() => pendingPlay.promise);
    const onPlaying = vi.fn();
    const video = videoStub({ readyState: 0, paused: true });

    const result = resumeWhenReady(video, { playCall, onPlaying });
    video.emit('canplay');
    expect(playCall).toHaveBeenCalledTimes(1);
    expect(video.removeEventListener).toHaveBeenCalledWith('canplay', expect.any(Function));

    // Readiness drops and recovers while the first call is still pending.
    video.emit('canplay');
    expect(playCall).toHaveBeenCalledTimes(1);

    pendingPlay.resolve();
    expect(await result).toEqual({ played: true, blocked: false });
    expect(onPlaying).toHaveBeenCalledTimes(1);
  });

  it('resumeWhenReady ignores a play success that arrives after the timeout', async () => {
    vi.useFakeTimers();
    try {
      const pendingPlay = deferred();
      const onPlaying = vi.fn();
      const video = videoStub({ readyState: HTMLMediaElement.HAVE_FUTURE_DATA, paused: true });

      const result = resumeWhenReady(video, { playCall: () => pendingPlay.promise, onPlaying });
      await vi.advanceTimersByTimeAsync(10_001);
      expect(await result).toEqual({ played: false, blocked: false });

      pendingPlay.resolve();
      await vi.advanceTimersByTimeAsync(0);
      expect(onPlaying).not.toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
    }
  });

  it('resumeWhenReady ignores a play success after the owner cancels', async () => {
    let cancelled = false;
    const pendingPlay = deferred();
    const onPlaying = vi.fn();
    const video = videoStub({ readyState: HTMLMediaElement.HAVE_FUTURE_DATA, paused: true });

    const result = resumeWhenReady(video, {
      playCall: () => pendingPlay.promise,
      onPlaying,
      isCancelled: () => cancelled,
    });
    cancelled = true;
    pendingPlay.resolve();

    expect(await result).toEqual({ played: false, blocked: false });
    expect(onPlaying).not.toHaveBeenCalled();
  });

  it('resumeWhenReady still reports a timely success on the active operation', async () => {
    const onPlaying = vi.fn();
    const video = videoStub({ readyState: HTMLMediaElement.HAVE_FUTURE_DATA, paused: true });
    const result = await resumeWhenReady(video, {
      playCall: () => Promise.resolve(),
      onPlaying,
      isCancelled: () => false,
    });
    expect(result).toEqual({ played: true, blocked: false });
    expect(onPlaying).toHaveBeenCalledTimes(1);
  });
});

describe('appendMediaFragment', () => {
  it('adds #t= seek hint to stream URL', () => {
    expect(appendMediaFragment('https://example.com/v.mp4', 90)).toBe(
      'https://example.com/v.mp4#t=90',
    );
    expect(appendMediaFragment('https://example.com/v.mp4#t=1', 90)).toBe(
      'https://example.com/v.mp4#t=90',
    );
    expect(appendMediaFragment('https://example.com/v.mp4', 0)).toBe(
      'https://example.com/v.mp4',
    );
  });
});
