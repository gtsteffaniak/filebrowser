import { describe, it, expect } from 'vitest';
import { sortedItems } from './sort.js';

describe('testSort', () => {

  it('sort items by name correctly', () => {
    const input = [
      { name: "zebra" },
      { name: "1" },
      { name: "10" },
      { name: "Apple" },
      { name: "2" },
    ]
    const expected = [
      { name: "1" },
      { name: "2" },
      { name: "10" },
      { name: "Apple" },
      { name: "zebra" }
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


});
