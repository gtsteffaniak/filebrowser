import { describe, it, expect } from 'vitest';
import { getHumanReadableFilesize } from './filesizes.js';

describe('testSort', () => {

  it('sort items by name correctly', () => {
    const tests = [
      {input: 1, expected:"1.0 bytes"},
      {input: 1150, expected:"1.1 KB"},
      {input: 5105650, expected:"4.9 MB"},
      {input: 156518899684, expected:"145.8 GB"},
      {input: 4891498498488, expected:"4.4 TB"},
      {input: 11991498498488488, expected:"10.7 PB"},
    ]
    for (let i in tests) {
      expect(getHumanReadableFilesize(tests[i].input)).toEqual(tests[i].expected);
    }
  });

});
