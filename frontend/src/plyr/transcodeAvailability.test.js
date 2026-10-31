import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/store', () => ({ state: { mediaPlayerSettings: null } }));
vi.mock('@/utils/constants', () => ({
  globalVars: { ffmpegAvailable: true, transcodeEnabled: true },
}));

import { state } from '@/store/index.ts';
import { globalVars } from '@/utils/constants.js';
import { isTranscodeAvailable } from '@/plyr/transcodeMode.js';

describe('isTranscodeAvailable', () => {
  beforeEach(() => {
    state.mediaPlayerSettings = null;
    globalVars.ffmpegAvailable = true;
    globalVars.transcodeEnabled = true;
  });

  it('falls back to the page constant before settings load', () => {
    expect(isTranscodeAvailable()).toBe(true);

    globalVars.transcodeEnabled = false;
    expect(isTranscodeAvailable()).toBe(false);
  });

  it('prefers the live media settings over the page constant', () => {
    state.mediaPlayerSettings = { enabled: false };
    expect(isTranscodeAvailable()).toBe(false);

    state.mediaPlayerSettings = { enabled: true };
    expect(isTranscodeAvailable()).toBe(true);
  });

  it('stays off when the server has no ffmpeg', () => {
    globalVars.ffmpegAvailable = false;
    globalVars.transcodeEnabled = false;
    expect(isTranscodeAvailable()).toBe(false);
  });

  it('stays off when ffmpeg is missing even if settings claim enabled', () => {
    globalVars.ffmpegAvailable = false;
    state.mediaPlayerSettings = { enabled: true };
    expect(isTranscodeAvailable()).toBe(false);
  });
});
