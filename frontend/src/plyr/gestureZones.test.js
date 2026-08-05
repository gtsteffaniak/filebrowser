import { describe, expect, it } from 'vitest';
import { isStationaryTap, zoneFromClientX } from '@/plyr/gestureZones.js';

describe('gesture zone helpers', () => {
  const rect = { left: 0, width: 300, top: 0, height: 200 };

  it('classifies left, center, and right zones', () => {
    expect(zoneFromClientX(50, rect)).toBe('left');
    expect(zoneFromClientX(150, rect)).toBe('center');
    expect(zoneFromClientX(250, rect)).toBe('right');
  });

  it('stationary tap preserves edge state', () => {
    expect(isStationaryTap(0, 0)).toBe(true);
    expect(isStationaryTap(3, 2)).toBe(true);
    expect(isStationaryTap(6, 0)).toBe(false);
    expect(isStationaryTap(0, 0, 'horizontal')).toBe(false);
  });
});
