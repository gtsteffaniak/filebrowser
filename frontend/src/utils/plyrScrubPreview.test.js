import { describe, expect, it } from 'vitest';
import {
  formatScrubPreviewTime,
  positionScrubPreviewPopup,
  quantizeScrubPercent,
  scrubPercentChanged,
  scrubPreviewDelayMs,
  scrubPreviewDimensions,
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

describe('scrubPreviewDimensions', () => {
  it('derives height from aspect ratio', () => {
    expect(scrubPreviewDimensions(16 / 9, 160, { progressTop: 900, viewportWidth: 1200, viewportHeight: 900 }))
      .toEqual({ width: 160, height: 90 });
    expect(scrubPreviewDimensions(2, 200, { progressTop: 900, viewportWidth: 1200, viewportHeight: 900 }))
      .toEqual({ width: 200, height: 100 });
  });

  it('scales down to fit viewport width', () => {
    expect(scrubPreviewDimensions(16 / 9, 500, { progressTop: 900, viewportWidth: 300, viewportHeight: 900 }))
      .toEqual({ width: 268, height: 151 });
  });

  it('scales down to fit space above the progress bar', () => {
    expect(scrubPreviewDimensions(16 / 9, 500, { progressTop: 120, viewportWidth: 1200, viewportHeight: 800 }))
      .toEqual({ width: 110, height: 62 });
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
