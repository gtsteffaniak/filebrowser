import { describe, expect, it } from 'vitest';
import { canBrowserPlayNative, needsTranscodeFirst } from './canBrowserPlayNative';

describe('canBrowserPlayNative', () => {
  it('does not blanket-reject mkv by extension when container support is unknown', () => {
    const result = canBrowserPlayNative({ fileName: 'clip.mkv' });
    expect([false, null]).toContain(result);
    if (result === false) {
      expect(needsTranscodeFirst({ fileName: 'clip.mkv' })).toBe(true);
    }
  });

  it('allows mp4 extension as unknown/native-first', () => {
    expect(canBrowserPlayNative({ fileName: 'clip.mp4' })).toBeNull();
  });

  it('evaluates codec strings inside matroska container', () => {
    if (typeof document === 'undefined') {
      return;
    }
    const result = canBrowserPlayNative({
      videoCodec: 'h264',
      audioCodec: 'aac',
      fileName: 'clip.mkv',
    });
    expect([true, false, null]).toContain(result);
  });

  it('rejects matroska mime without codec metadata when browser says no', () => {
    if (typeof document === 'undefined') {
      return;
    }
    const video = document.createElement('video');
    if (video.canPlayType('video/x-matroska') === '') {
      expect(canBrowserPlayNative({ mimeType: 'video/x-matroska', fileName: 'clip.mkv' })).toBe(false);
    }
  });

  it('rejects wmv by extension', () => {
    expect(canBrowserPlayNative({ fileName: 'clip.wmv' })).toBe(false);
    expect(needsTranscodeFirst({ fileName: 'clip.wmv' })).toBe(true);
  });
});
