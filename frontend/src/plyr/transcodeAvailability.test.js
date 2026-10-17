import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/constants', () => ({
  globalVars: { mediaAvailable: true, transcodeEnabled: true },
}));

import { globalVars } from '@/utils/constants.js';
import { isTranscodeAvailable } from '@/plyr/transcodeMode.js';

describe('isTranscodeAvailable', () => {
  beforeEach(() => {
    globalVars.mediaAvailable = true;
    globalVars.transcodeEnabled = true;
  });

  it('reflects page constants', () => {
    expect(isTranscodeAvailable()).toBe(true);

    globalVars.transcodeEnabled = false;
    expect(isTranscodeAvailable()).toBe(false);
  });

  it('stays off when the server has no ffmpeg', () => {
    globalVars.mediaAvailable = false;
    globalVars.transcodeEnabled = true;
    expect(isTranscodeAvailable()).toBe(false);
  });
});
