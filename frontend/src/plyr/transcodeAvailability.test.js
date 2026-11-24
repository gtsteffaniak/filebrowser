import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/store', () => ({ state: { mediaSettings: null } }));
vi.mock('@/utils/constants', () => ({
  globalVars: { ffmpegAvailable: true },
}));

import { state } from '@/store/index.ts';
import { globalVars } from '@/utils/constants.ts';
import { isTranscodeAvailable } from '@/plyr/transcodeMode.js';

describe('isTranscodeAvailable', () => {
  beforeEach(() => {
    state.mediaSettings = null;
    globalVars.ffmpegAvailable = true;
  });

  it('stays off until live media settings load', () => {
    expect(isTranscodeAvailable()).toBe(false);
  });

  it('follows the live media settings', () => {
    state.mediaSettings = { enabled: false };
    expect(isTranscodeAvailable()).toBe(false);

    state.mediaSettings = { enabled: true };
    expect(isTranscodeAvailable()).toBe(true);
  });

  it('stays off when the server has no ffmpeg', () => {
    globalVars.ffmpegAvailable = false;
    state.mediaSettings = { enabled: true };
    expect(isTranscodeAvailable()).toBe(false);
  });

  it('stays off when ffmpeg is missing even if settings claim enabled', () => {
    globalVars.ffmpegAvailable = false;
    state.mediaSettings = { enabled: true };
    expect(isTranscodeAvailable()).toBe(false);
  });
});
