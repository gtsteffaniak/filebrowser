import { describe, expect, it } from 'vitest';
import { getHumanReadableFilesize } from './filesizes.js';

describe('testSort', () => {

  it('validate human readable sizes', () => {
    const tests = [
      {input: 1, expected:"1.0 bytes"},
      {input: 1150, expected:"1.1 KB"},
      {input: 5105650, expected:"4.9 MB"},
      {input: Number('156518899684'), expected:"145.8 GB"},
      {input: Number('1020993183744'), expected:"950.9 GB"},
      {input: Number('4891498498488'), expected:"4.4 TB"},
      {input: Number('11991498498488488'), expected:"10.7 PB"},
    ]
    for (const test of tests) {
      expect(getHumanReadableFilesize(test.input)).toEqual(test.expected);
    }
  });
});
