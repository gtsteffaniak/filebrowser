import { describe, it, expect, vi } from 'vitest';

vi.mock('@/api/tools.js', () => ({ search: vi.fn() }));
vi.mock('@/api', () => ({ resourcesApi: { getViewURL: vi.fn() } }));
vi.mock('@/store', () => ({
  state: { req: {}, route: {}, shareInfo: null },
  getters: { fileViewingDisabled: () => false, isShare: () => false },
  mutations: {},
}));

import { effectiveMaxItemsCap, SLIDESHOW_MAX_ITEMS_HARD_CAP, saveSlideshowConfig, resetSlideshowConfig } from '@/utils/slideshowConfig.js';
import { slidesFromListing, shuffled, mediaInfo, countMediaInListing } from '@/utils/slideshow.js';

describe('effectiveMaxItemsCap', () => {
  it('uses the server limit when it is below the hard cap', () => {
    expect(effectiveMaxItemsCap(1000)).toBe(1000);
  });

  it('never exceeds the hard cap of 5000', () => {
    expect(effectiveMaxItemsCap(20000)).toBe(SLIDESHOW_MAX_ITEMS_HARD_CAP);
    expect(SLIDESHOW_MAX_ITEMS_HARD_CAP).toBe(5000);
  });

  it('falls back to the hard cap when the server limit is missing', () => {
    expect(effectiveMaxItemsCap(undefined)).toBe(SLIDESHOW_MAX_ITEMS_HARD_CAP);
    expect(effectiveMaxItemsCap(0)).toBe(SLIDESHOW_MAX_ITEMS_HARD_CAP);
  });
});

describe('mediaInfo', () => {
  it('classifies images and videos by extension when no mime type is given', () => {
    expect(mediaInfo('photo.jpg', '')).toEqual({ kind: 'image', preview: null });
    expect(mediaInfo('clip.mp4', '')).toEqual({ kind: 'video', preview: null });
  });

  it('shows RAW files through the server preview', () => {
    expect(mediaInfo('shot.dng', '')).toEqual({ kind: 'image', preview: 'raw' });
    expect(mediaInfo('shot.CR3', '')).toEqual({ kind: 'image', preview: 'raw' });
  });

  it('skips HEIC when neither Safari nor ffmpeg conversion is available', () => {
    expect(mediaInfo('IMG_0001.HEIC', '')).toBeNull();
  });

  it('skips documents and other non-media files', () => {
    expect(mediaInfo('notes.txt', '')).toBeNull();
    expect(mediaInfo('archive.zip', '')).toBeNull();
  });
});

describe('slidesFromListing', () => {
  it('keeps images and videos, drops folders and other files, in listing order', () => {
    const listing = [
      { name: 'a.jpg', path: '/pics/a.jpg', type: 'image/jpeg' },
      { name: 'sub', path: '/pics/sub', type: 'directory' },
      { name: 'notes.txt', path: '/pics/notes.txt', type: 'text/plain' },
      { name: 'b.mp4', path: '/pics/b.mp4', type: 'video/mp4' },
    ];
    const slides = slidesFromListing(listing, 'files');
    expect(slides.map((s: { name: string }) => s.name)).toEqual(['a.jpg']);
    expect(slides[0].source).toBe('files');
  });

  it('includes videos only when include videos is on', () => {
    saveSlideshowConfig({ includeVideos: true });
    const listing = [
      { name: 'a.jpg', path: '/pics/a.jpg', type: 'image/jpeg' },
      { name: 'b.mp4', path: '/pics/b.mp4', type: 'video/mp4' },
    ];
    const slides = slidesFromListing(listing, 'files');
    expect(slides.map((s: { kind: string }) => s.kind)).toEqual(['image', 'video']);
    resetSlideshowConfig();
  });

  it('keeps the preview flag for RAW items so their URL comes from the preview endpoint', () => {
    const slides = slidesFromListing([{ name: 'shot.nef', path: '/x/shot.nef', type: '', modified: 'm' }], 'files');
    expect(slides[0]).toMatchObject({ kind: 'image', preview: 'raw', modified: 'm' });
  });

  it('counts only viewable media for the entry button threshold', () => {
    const listing = [
      { name: 'only.jpg', path: '/x/only.jpg', type: 'image/jpeg' },
      { name: 'folder', path: '/x/folder', type: 'directory' },
    ];
    expect(countMediaInListing(listing)).toBe(1);
    expect(countMediaInListing(null)).toBe(0);
  });
});

describe('shuffled', () => {
  it('returns a permutation without mutating the input', () => {
    const input = [1, 2, 3, 4, 5];
    const out = shuffled(input);
    expect(input).toEqual([1, 2, 3, 4, 5]);
    expect([...out].sort()).toEqual([1, 2, 3, 4, 5]);
  });
});
