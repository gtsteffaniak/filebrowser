import { describe, expect, it } from 'vitest';
import {
  TRANSCODE_HLS_MODES,
  TRANSCODE_MODES,
  isTranscodeHlsMode,
  normalizeTranscodeMode,
  normalizeTranscodeSessionStatus,
} from '@/plyr/transcodeMode.js';

describe('transcodeMode', () => {
  it('normalizes supported modes', () => {
    expect(normalizeTranscodeMode('quality')).toBe('quality');
    expect(normalizeTranscodeMode('unknown')).toBe('native');
  });

  it('identifies HLS modes', () => {
    expect(TRANSCODE_MODES).toEqual(['native', 'quality', 'datasaver']);
    expect(TRANSCODE_HLS_MODES).toEqual(['quality', 'datasaver']);
    expect(isTranscodeHlsMode('quality')).toBe(true);
    expect(isTranscodeHlsMode('native')).toBe(false);
  });

  it('treats same-file active sessions as capacity-neutral', () => {
    const status = {
      canStart: false,
      blockReason: 'user_limit',
      sessions: [{ source: 's', path: '/clip.mp4' }],
    };
    const normalized = normalizeTranscodeSessionStatus(status, {
      source: 's',
      path: '/clip.mp4',
    });
    expect(normalized.canStart).toBe(true);
    expect(normalized.blockReason).toBe('');
  });
});
