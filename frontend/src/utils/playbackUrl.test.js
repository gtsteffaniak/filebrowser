import { describe, expect, it } from 'vitest';
import {
  buildPlaybackQueryPatch,
  formatDurationForQuery,
  parseDurationString,
  parsePlaybackTimeFromQuery,
  parseTranscodeModeFromQuery,
  playbackQueryChanged,
} from './playbackUrl.js';

describe('parseDurationString', () => {
  it('parses compact duration tokens', () => {
    expect(parseDurationString('2h3m4s')).toBe(7384);
    expect(parseDurationString('2h')).toBe(7200);
    expect(parseDurationString('3m')).toBe(180);
    expect(parseDurationString('30s')).toBe(30);
    expect(parseDurationString('2m30s')).toBe(150);
    expect(parseDurationString('2h30s')).toBe(7200 + 30);
  });

  it('accepts plain numeric seconds', () => {
    expect(parseDurationString('125')).toBe(125);
    expect(parseDurationString('12.5')).toBe(12.5);
  });

  it('rejects invalid values', () => {
    expect(parseDurationString('')).toBeNull();
    expect(parseDurationString('4x')).toBeNull();
    expect(parseDurationString('-30s')).toBeNull();
  });
});

describe('formatDurationForQuery', () => {
  it('formats seconds into compact tokens', () => {
    expect(formatDurationForQuery(7384)).toBe('2h3m4s');
    expect(formatDurationForQuery(7200)).toBe('2h');
    expect(formatDurationForQuery(180)).toBe('3m');
    expect(formatDurationForQuery(30)).toBe('30s');
    expect(formatDurationForQuery(150)).toBe('2m30s');
  });
});

describe('playbackUrl', () => {
  it('parses playback time from query', () => {
    expect(parsePlaybackTimeFromQuery({ time: '2m30s' })).toBe(150);
    expect(parsePlaybackTimeFromQuery({ time: '2h' })).toBe(7200);
    expect(parsePlaybackTimeFromQuery({ t: '125' })).toBe(125);
    expect(parsePlaybackTimeFromQuery({})).toBeNull();
  });

  it('parses transcode mode from query', () => {
    expect(parseTranscodeModeFromQuery({ transcode: 'quality' })).toBe('quality');
    expect(parseTranscodeModeFromQuery({ transcode: 'datasaver' })).toBe('datasaver');
    expect(parseTranscodeModeFromQuery({ transcode: 'native' })).toBeNull();
  });

  it('builds query patch for shareable links', () => {
    expect(buildPlaybackQueryPatch({}, { time: 90, transcodeMode: 'quality' })).toEqual({
      time: '1m30s',
      transcode: 'quality',
    });
    expect(buildPlaybackQueryPatch({ t: '120', transcode: 'quality' }, { time: 0, transcodeMode: 'native' })).toEqual({});
    expect(buildPlaybackQueryPatch({ foo: 'bar' }, { time: 7200, transcodeMode: 'datasaver' })).toEqual({
      foo: 'bar',
      time: '2h',
      transcode: 'datasaver',
    });
  });

  it('preserves time and transcode when patch omits them', () => {
    expect(buildPlaybackQueryPatch({ time: '2m', transcode: 'quality' }, { time: 90 })).toEqual({
      time: '1m30s',
      transcode: 'quality',
    });
  });

  it('detects playback query changes by position, not string form', () => {
    expect(playbackQueryChanged({ time: '1m' }, { time: '2m' })).toBe(true);
    expect(playbackQueryChanged({ time: '90s' }, { time: '1m30s' })).toBe(false);
    expect(playbackQueryChanged({ time: '1m' }, { time: '1m', transcode: 'quality' })).toBe(true);
  });
});
