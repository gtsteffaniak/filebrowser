import { describe, expect, it, beforeEach } from 'vitest';
import {
  DEFAULT_TRANSCODE_MODE,
  isTranscodeModeRequestedInUrl,
  loadPreferredTranscodeMode,
  resolveTranscodeModeForPlayback,
  savePreferredTranscodeMode,
  TRANSCODE_PREFERENCE_STORAGE_KEY,
} from './transcodePreference.js';

describe('transcodePreference', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('defaults to quality', () => {
    expect(loadPreferredTranscodeMode()).toBe(DEFAULT_TRANSCODE_MODE);
  });

  it('persists quality and datasaver', () => {
    savePreferredTranscodeMode('datasaver');
    expect(localStorage.getItem(TRANSCODE_PREFERENCE_STORAGE_KEY)).toBe('datasaver');
    expect(loadPreferredTranscodeMode()).toBe('datasaver');
  });

  it('ignores invalid stored values', () => {
    localStorage.setItem(TRANSCODE_PREFERENCE_STORAGE_KEY, 'native');
    expect(loadPreferredTranscodeMode()).toBe('quality');
  });

  it('does not save native or unknown modes', () => {
    savePreferredTranscodeMode('native');
    expect(localStorage.getItem(TRANSCODE_PREFERENCE_STORAGE_KEY)).toBeNull();
  });

  it('prefers URL transcode over localStorage', () => {
    savePreferredTranscodeMode('datasaver');
    expect(resolveTranscodeModeForPlayback({ transcode: 'quality' })).toBe('quality');
    expect(resolveTranscodeModeForPlayback({})).toBe('datasaver');
  });

  it('detects transcode requested in URL', () => {
    expect(isTranscodeModeRequestedInUrl({ transcode: 'quality' })).toBe(true);
    expect(isTranscodeModeRequestedInUrl({})).toBe(false);
  });
});
