import { describe, expect, it } from 'vitest';

import {
  buildPlaybackQueryPatch,
  formatTimeForQuery,
  parseClockTimeString,
  parsePlaybackTimeFromQuery,
  playbackQueryChanged,
} from './playbackQuery.js';

describe('parseClockTimeString', () => {
  it('parses HH:MM:SS', () => {
    expect(parseClockTimeString('00:03:00')).toBe(180);
    expect(parseClockTimeString('01:23:45')).toBe(5025);
    expect(parseClockTimeString('00:00:30')).toBe(30);
  });

  it('parses MM:SS', () => {
    expect(parseClockTimeString('03:00')).toBe(180);
    expect(parseClockTimeString('01:30')).toBe(90);
  });

  it('rejects invalid clock values', () => {
    expect(parseClockTimeString('00:60:00')).toBeNull();
    expect(parseClockTimeString('abc')).toBeNull();
    expect(parseClockTimeString('1')).toBeNull();
    expect(parseClockTimeString('1:2:3:4')).toBeNull();
  });
});

describe('formatTimeForQuery', () => {
  it('formats seconds as HH:MM:SS', () => {
    expect(formatTimeForQuery(0)).toBe('00:00:00');
    expect(formatTimeForQuery(180)).toBe('00:03:00');
    expect(formatTimeForQuery(5025)).toBe('01:23:45');
  });
});

describe('parsePlaybackTimeFromQuery', () => {
  it('reads time as HH:MM:SS only', () => {
    expect(parsePlaybackTimeFromQuery({ time: '00:03:00' })).toBe(180);
    expect(parsePlaybackTimeFromQuery({ time: '03:00' })).toBeNull();
  });

  it('reads legacy t as plain seconds or clock format', () => {
    expect(parsePlaybackTimeFromQuery({ t: '180' })).toBe(180);
    expect(parsePlaybackTimeFromQuery({ t: '00:03:00' })).toBe(180);
  });

  it('prefers time over legacy t', () => {
    expect(parsePlaybackTimeFromQuery({ time: '00:01:00', t: '180' })).toBe(60);
  });
});

describe('buildPlaybackQueryPatch', () => {
  it('writes time as HH:MM:SS', () => {
    expect(buildPlaybackQueryPatch({}, { time: 180 })).toEqual({ time: '00:03:00' });
  });

  it('removes time when patched to zero', () => {
    expect(buildPlaybackQueryPatch({ time: '00:03:00' }, { time: 0 })).toEqual({});
  });
});

describe('playbackQueryChanged', () => {
  it('detects time changes', () => {
    expect(playbackQueryChanged({ time: '00:01:00' }, { time: '00:02:00' })).toBe(true);
    expect(playbackQueryChanged({ time: '00:01:00' }, { time: '00:01:00' })).toBe(false);
  });
});
