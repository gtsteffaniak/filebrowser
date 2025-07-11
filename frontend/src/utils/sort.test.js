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

});
