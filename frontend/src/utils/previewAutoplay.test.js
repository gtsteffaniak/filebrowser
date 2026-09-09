import { describe, expect, it } from 'vitest';

import { shouldAutoPlayPreview } from './previewAutoplay.js';

describe('shouldAutoPlayPreview', () => {
  it('autoplays when autoplayMedia preference is enabled', () => {
    expect(shouldAutoPlayPreview(true, false, false)).toBe(true);
    expect(shouldAutoPlayPreview(true, true, false)).toBe(true);
  });

  it('does not autoplay after play when leaving queue navigation', () => {
    expect(shouldAutoPlayPreview(false, true, false)).toBe(false);
  });

  it('autoplays queue navigation after user pressed play', () => {
    expect(shouldAutoPlayPreview(false, true, true)).toBe(true);
  });

  it('does not autoplay queue navigation before user pressed play', () => {
    expect(shouldAutoPlayPreview(false, false, true)).toBe(false);
  });
});
