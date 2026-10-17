import { describe, expect, it } from 'vitest';
import {
  mediaCoverage,
  seekOutsideCoverage,
  servableCoverage,
  TRANSCODE_SEEK_BACKWARD_EPS_SEC,
  TRANSCODE_SEEK_FORWARD_SLACK_SEC,
} from '@/plyr/transcodeSeek.js';

function fakeSeekable(ranges) {
  return {
    length: ranges.length,
    start: (i) => ranges[i][0],
    end: (i) => ranges[i][1],
  };
}

describe('mediaCoverage', () => {
  it('returns null without an element or seekable ranges', () => {
    expect(mediaCoverage(null)).toBeNull();
    expect(mediaCoverage({ seekable: fakeSeekable([]) })).toBeNull();
  });

  it('returns the first start and last end', () => {
    const el = { seekable: fakeSeekable([[0, 10], [10, 20]]) };
    expect(mediaCoverage(el)).toEqual({ start: 0, end: 20 });
  });

  it('reflects a mid-file generation coverage', () => {
    const el = { seekable: fakeSeekable([[2400, 2420]]) };
    expect(mediaCoverage(el)).toEqual({ start: 2400, end: 2420 });
  });
});

describe('servableCoverage', () => {
  it('covers the whole produced range for a completed full generation', () => {
    const el = { seekable: fakeSeekable([[0, 24]]) };
    const session = { state: 'completed', startSec: 0, endSec: 5112 };
    expect(servableCoverage(el, session)).toEqual({ start: 0, end: 5112 });
  });

  it('bounds a completed tail generation at its produced end', () => {
    const el = { seekable: fakeSeekable([[2400, 2460]]) };
    const session = { state: 'completed', startSec: 2400, endSec: 3400 };
    expect(servableCoverage(el, session)).toEqual({ start: 2400, end: 3400 });
  });

  it('falls back to durationSec when a completed session lacks endSec', () => {
    const el = { seekable: fakeSeekable([[0, 10]]) };
    const session = { state: 'completed', startSec: 0, durationSec: 600 };
    expect(servableCoverage(el, session)).toEqual({ start: 0, end: 600 });
  });

  it('returns null for a completed session with no usable end', () => {
    const el = { seekable: fakeSeekable([[0, 10]]) };
    expect(servableCoverage(el, { state: 'completed', startSec: 0 })).toBeNull();
    expect(servableCoverage(el, { state: 'completed', startSec: 100, endSec: 50 })).toBeNull();
  });

  it('uses the buffered edge for a running generation', () => {
    const el = { seekable: fakeSeekable([[2400, 2440]]) };
    const session = { state: 'running', startSec: 2400, endSec: 0 };
    expect(servableCoverage(el, session)).toEqual({ start: 2400, end: 2440 });
  });

  it('floors the running start at the generation origin', () => {
    const el = { seekable: fakeSeekable([[2415, 2440]]) };
    const session = { state: 'running', startSec: 2400 };
    expect(servableCoverage(el, session)).toEqual({ start: 2400, end: 2440 });
  });

  it('returns null when a running generation has nothing buffered', () => {
    const session = { state: 'running', startSec: 0 };
    expect(servableCoverage({ seekable: fakeSeekable([]) }, session)).toBeNull();
    expect(servableCoverage(null, session)).toBeNull();
    expect(servableCoverage(null, null)).toBeNull();
  });
});

describe('seekOutsideCoverage', () => {
  const cov = { start: 0, end: 100 };

  it('is false for targets inside coverage', () => {
    expect(seekOutsideCoverage(50, cov)).toBe(false);
    expect(seekOutsideCoverage(0, cov)).toBe(false);
    expect(seekOutsideCoverage(100, cov)).toBe(false);
  });

  it('is false within the forward slack past the edge', () => {
    expect(seekOutsideCoverage(100 + TRANSCODE_SEEK_FORWARD_SLACK_SEC - 1, cov)).toBe(false);
  });

  it('is true beyond the forward slack', () => {
    expect(seekOutsideCoverage(100 + TRANSCODE_SEEK_FORWARD_SLACK_SEC + 1, cov)).toBe(true);
    expect(seekOutsideCoverage(3600, cov)).toBe(true);
  });

  it('is true below the generation start', () => {
    const mid = { start: 2400, end: 3000 };
    expect(seekOutsideCoverage(60, mid)).toBe(true);
    expect(seekOutsideCoverage(2400, mid)).toBe(false);
    expect(seekOutsideCoverage(2400 - TRANSCODE_SEEK_BACKWARD_EPS_SEC + 0.5, mid)).toBe(false);
  });

  it('is false for invalid targets or missing coverage', () => {
    expect(seekOutsideCoverage(NaN, cov)).toBe(false);
    expect(seekOutsideCoverage(-5, cov)).toBe(false);
    expect(seekOutsideCoverage(5000, null)).toBe(false);
  });
});
