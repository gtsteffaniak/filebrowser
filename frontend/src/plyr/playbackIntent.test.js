import { describe, expect, it, vi } from 'vitest';
import {
  continuePlay,
  isPlayBlockedError,
  isPlayTeardownError,
  primePlay,
  resumeWhenReady,
} from '@/plyr/playbackIntent.js';

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

  it('resumeWhenReady calls playCall when canplay already available', () => {
    const playCall = vi.fn(() => Promise.resolve());
    const onPlaying = vi.fn();
    const video = {
      readyState: HTMLMediaElement.HAVE_FUTURE_DATA,
      paused: true,
      addEventListener: vi.fn(),
    };
    resumeWhenReady(video, { playCall, onPlaying });
    expect(playCall).toHaveBeenCalledOnce();
  });

  it('continuePlay invokes onPlaying when primed promise resolves', async () => {
    const onPlaying = vi.fn();
    await continuePlay(null, Promise.resolve(), { onPlaying });
    expect(onPlaying).toHaveBeenCalledOnce();
  });

  it('continuePlay falls back when primed promise fails during handoff', async () => {
    const playCall = vi.fn(() => Promise.resolve());
    const onPlaying = vi.fn();
    const video = {
      readyState: HTMLMediaElement.HAVE_FUTURE_DATA,
      paused: true,
      addEventListener: vi.fn(),
    };
    await continuePlay(
      video,
      Promise.reject(Object.assign(new Error('no source'), { name: 'NotSupportedError' })),
      { playCall, onPlaying },
    );
    expect(playCall).toHaveBeenCalledOnce();
  });

  it('continuePlay does not retry when autoplay is blocked', async () => {
    const playCall = vi.fn(() => Promise.resolve());
    const video = {
      readyState: HTMLMediaElement.HAVE_FUTURE_DATA,
      paused: true,
      addEventListener: vi.fn(),
    };
    await continuePlay(
      video,
      Promise.reject(Object.assign(new Error('blocked'), { name: 'NotAllowedError' })),
      { playCall },
    );
    expect(playCall).not.toHaveBeenCalled();
  });
});
