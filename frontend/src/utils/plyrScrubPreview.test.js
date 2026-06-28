import { describe, expect, it } from 'vitest';
import {
  fitScrubPreviewImageSize,
  formatScrubPreviewTime,
  getScrubPreviewMount,
  positionScrubPreviewPopup,
  quantizeScrubPercent,
  scrubPercentChanged,
  scrubPreviewDelayMs,
} from './plyrScrubPreview';

describe('quantizeScrubPercent', () => {
  it('clamps and rounds', () => {
    expect(quantizeScrubPercent(-5)).toBe(0);
    expect(quantizeScrubPercent(42.4)).toBe(42);
    expect(quantizeScrubPercent(42.6)).toBe(43);
    expect(quantizeScrubPercent(120)).toBe(100);
  });
});

describe('scrubPercentChanged', () => {
  it('detects integer bucket changes', () => {
    expect(scrubPercentChanged(10, 10)).toBe(false);
    expect(scrubPercentChanged(10, 11)).toBe(true);
    expect(scrubPercentChanged(null, 0)).toBe(true);
  });
});

describe('scrubPreviewDelayMs', () => {
  it('waits until min interval elapsed', () => {
    expect(scrubPreviewDelayMs(1000, 1100, 250)).toBe(150);
    expect(scrubPreviewDelayMs(1000, 1300, 250)).toBe(0);
  });
});

describe('formatScrubPreviewTime', () => {
  it('formats mm:ss and hh:mm:ss', () => {
    expect(formatScrubPreviewTime(65)).toBe('1:05');
    expect(formatScrubPreviewTime(3661)).toBe('1:01:01');
  });
});

describe('fitScrubPreviewImageSize', () => {
  it('preserves preview image size when it already fits', () => {
    expect(fitScrubPreviewImageSize(256, 256, { progressTop: 900, viewportWidth: 1200, viewportHeight: 900 }))
      .toEqual({ width: 256, height: 256 });
    expect(fitScrubPreviewImageSize(400, 225, { progressTop: 900, viewportWidth: 1200, viewportHeight: 900 }))
      .toEqual({ width: 400, height: 225 });
  });

  it('scales down when the preview image exceeds max width', () => {
    expect(fitScrubPreviewImageSize(640, 360, { progressTop: 900, viewportWidth: 1200, viewportHeight: 900 }))
      .toEqual({ width: 600, height: 338 });
  });

  it('scales down to fit viewport width', () => {
    expect(fitScrubPreviewImageSize(1200, 675, { progressTop: 900, viewportWidth: 300, viewportHeight: 900 }))
      .toEqual({ width: 268, height: 151 });
  });

  it('scales down to fit space above the progress bar', () => {
    expect(fitScrubPreviewImageSize(640, 360, { progressTop: 120, viewportWidth: 1200, viewportHeight: 800 }))
      .toEqual({ width: 110, height: 62 });
  });

  it('caps height at the max preview height', () => {
    expect(fitScrubPreviewImageSize(800, 2000, { progressTop: 1200, viewportWidth: 1200, viewportHeight: 1200 }))
      .toEqual({ width: 240, height: 600 });
  });

  it('uses 16:9 placeholder dimensions while waiting for preview image', () => {
    expect(fitScrubPreviewImageSize(600, Math.round(600 / (16 / 9)), { progressTop: 900, viewportWidth: 1200, viewportHeight: 900 }))
      .toEqual({ width: 600, height: 338 });
  });

  it('returns null for invalid image dimensions', () => {
    expect(fitScrubPreviewImageSize(0, 256)).toBeNull();
    expect(fitScrubPreviewImageSize(Number.NaN, 256)).toBeNull();
  });
});

describe('getScrubPreviewMount', () => {
  it('prefers the plyr container when not in native fullscreen', () => {
    const container = document.createElement('div');
    const player = { elements: { container }, fullscreen: { active: false } };
    expect(getScrubPreviewMount(/** @type {any} */ (player))).toBe(container);
  });

  it('uses the plyr container during fallback fullscreen', () => {
    const container = document.createElement('div');
    const player = { elements: { container }, fullscreen: { active: true } };
    expect(getScrubPreviewMount(/** @type {any} */ (player))).toBe(container);
  });
});

describe('positionScrubPreviewPopup', () => {
  it('clamps horizontal position within the viewport', () => {
    const popup = document.createElement('div');
    document.body.appendChild(popup);
    Object.defineProperty(popup, 'offsetWidth', { value: 200, configurable: true });

    positionScrubPreviewPopup(popup, { top: 400 }, 10, 1000);
    expect(popup.style.left).toBe('108px');

    positionScrubPreviewPopup(popup, { top: 400 }, 990, 1000);
    expect(popup.style.left).toBe('892px');
    expect(popup.style.top).toBe('400px');

    popup.remove();
  });
});
