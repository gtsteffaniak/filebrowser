import { describe, expect, it, beforeEach, afterEach } from 'vitest';
import {
  isInLeftNavTapZone,
  isInRightNavTapZone,
} from '@/utils/navigationEdgeZones.js';

describe('navigationEdgeZones', () => {
  beforeEach(() => {
    document.documentElement.style.fontSize = '16px';
  });

  afterEach(() => {
    document.documentElement.style.fontSize = '';
  });

  it('detects left tap zone with sidebar offset', () => {
    expect(isInLeftNavTapZone(20, { moveWithSidebar: false })).toBe(true);
    expect(isInLeftNavTapZone(60, { moveWithSidebar: false })).toBe(false);
    expect(isInLeftNavTapZone(340, { moveWithSidebar: true, sidebarWidthEm: 20 })).toBe(true);
    expect(isInLeftNavTapZone(300, { moveWithSidebar: true, sidebarWidthEm: 20 })).toBe(false);
  });

  it('detects right tap zone', () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1000 });
    expect(isInRightNavTapZone(980)).toBe(true);
    expect(isInRightNavTapZone(900)).toBe(false);
  });
});
