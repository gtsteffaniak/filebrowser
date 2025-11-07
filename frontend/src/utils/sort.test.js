import { describe, it, expect } from 'vitest';
import { sortedItems } from './sort.js';

describe('testSort', () => {

  it('sort items by name correctly', () => {
    const input = [
      { name: "zebra" },
      { name: "10 something.txt" },
      { name: "1 something.txt" },
      { name: "2 something.txt" },
      { name: "Apple" },
      { name: "bee 20" },
      { name: "cave 10.txt" },
      { name: "cave 1.txt" },
      { name: "cave 2.txt" },
      { name: "2" },
      { name: "bee 2" },
    ]
    const expected = [
      { name: "1 something.txt" },
      { name: "2" },
      { name: "2 something.txt" },
      { name: "10 something.txt" },
      { name: "Apple" },
      { name: "bee 2" },
      { name: "bee 20" },
      { name: "cave 1.txt" },
      { name: "cave 2.txt" },
      { name: "cave 10.txt" },
      { name: "zebra" }
    ]
    expect(sortedItems(input, "name")).toEqual(expected);
  });

  it('sort items with extensions by name correctly', () => {
    const input = [
      { name: "zebra.txt" },
      { name: "1.txt" },
      { name: "10.txt" },
      { name: "Apple.txt" },
      { name: "2.txt" },
      { name: "0" }
    ]
    const expected = [
      { name: "0" },
      { name: "1.txt" },
      { name: "2.txt" },
      { name: "10.txt" },
      { name: "Apple.txt" },
      { name: "zebra.txt" }
    ]
    expect(sortedItems(input, "name")).toEqual(expected);
  });

  it('sort items by size correctly', () => {
    const input = [
      { size: "10" },
      { size: "0" },
      { size: "5000" },
    ]
    const expected = [
      { size: "0" },
      { size: "10" },
      { size: "5000" }
    ]
    expect(sortedItems(input, "size")).toEqual(expected);
  });

  it('sort items by date correctly', () => {
    const now = new Date();
    const tenMinutesAgo = new Date(now.getTime() - 10 * 60 * 1000);
    const tenMinutesFromNow = new Date(now.getTime() + 10 * 60 * 1000);

    const input = [
      { date: now },
      { date: tenMinutesAgo },
      { date: tenMinutesFromNow },
    ]
    const expected = [
      { date: tenMinutesAgo },
      { date: now },
      { date: tenMinutesFromNow }
    ]
    expect(sortedItems(input, "date")).toEqual(expected);
  });

  it('sort items by duration correctly (ascending)', () => {
    const input = [
      { name: "video3.mp4", metadata: { duration: 300.5 } },
      { name: "video1.mp4", metadata: { duration: 120.0 } },
      { name: "video2.mp4", metadata: { duration: 240.75 } },
      { name: "video4.mp4", metadata: { duration: 60.25 } },
      { name: "audio.mp3", metadata: { duration: 180.5 } },
    ]
    const expected = [
      { name: "video4.mp4", metadata: { duration: 60.25 } },
      { name: "video1.mp4", metadata: { duration: 120.0 } },
      { name: "audio.mp3", metadata: { duration: 180.5 } },
      { name: "video2.mp4", metadata: { duration: 240.75 } },
      { name: "video3.mp4", metadata: { duration: 300.5 } },
    ]
    expect(sortedItems(input, "duration", true)).toEqual(expected);
  });

  it('sort items by duration correctly (descending)', () => {
    const input = [
      { name: "video3.mp4", metadata: { duration: 300.5 } },
      { name: "video1.mp4", metadata: { duration: 120.0 } },
      { name: "video2.mp4", metadata: { duration: 240.75 } },
      { name: "video4.mp4", metadata: { duration: 60.25 } },
      { name: "audio.mp3", metadata: { duration: 180.5 } },
    ]
    const expected = [
      { name: "video3.mp4", metadata: { duration: 300.5 } },
      { name: "video2.mp4", metadata: { duration: 240.75 } },
      { name: "audio.mp3", metadata: { duration: 180.5 } },
      { name: "video1.mp4", metadata: { duration: 120.0 } },
      { name: "video4.mp4", metadata: { duration: 60.25 } },
    ]
    expect(sortedItems(input, "duration", false)).toEqual(expected);
  });

  it('sort items by duration with missing metadata', () => {
    const input = [
      { name: "video2.mp4", metadata: { duration: 240.75 } },
      { name: "image.jpg" }, // no metadata
      { name: "video1.mp4", metadata: { duration: 120.0 } },
      { name: "document.pdf", metadata: {} }, // metadata but no duration
    ]
    const result = sortedItems(input, "duration", true);
    
    // Items with no duration (treated as 0) should come first
    expect(result[0].metadata?.duration ?? 0).toBe(0);
    expect(result[1].metadata?.duration ?? 0).toBe(0);
    // Then items with actual duration values in ascending order
    expect(result[2]).toEqual({ name: "video1.mp4", metadata: { duration: 120.0 } });
    expect(result[3]).toEqual({ name: "video2.mp4", metadata: { duration: 240.75 } });
  });

});
