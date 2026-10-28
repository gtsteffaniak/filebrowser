import { describe, expect, it } from 'vitest';
import { servableCoverage } from '@/plyr/transcodePlaybackController.js';

describe('servableCoverage', () => {
  it('returns the full media range for any VOD session', () => {
    const session = { vodPlaylist: true, durationSec: 5112 };
    expect(servableCoverage(session)).toEqual({ start: 0, end: 5112 });
  });

  it('covers the whole file even while the producer is still encoding', () => {
    const session = { state: 'running', durationSec: 5112 };
    expect(servableCoverage(session)).toEqual({ start: 0, end: 5112 });
  });

  it('covers the whole file for a completed session', () => {
    const session = { state: 'completed', durationSec: 600 };
    expect(servableCoverage(session)).toEqual({ start: 0, end: 600 });
  });

  it('returns null without a usable duration', () => {
    expect(servableCoverage({ durationSec: 0 })).toBeNull();
    expect(servableCoverage({ durationSec: NaN })).toBeNull();
    expect(servableCoverage(null)).toBeNull();
  });
});
