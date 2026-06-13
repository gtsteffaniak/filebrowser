import { describe, expect, it } from 'vitest';
import { getFileExtension } from './files.js';

describe('testSort', () => {

  it('get extension from file', () => {
    const tests = [
      {input: "hi.txt", expected:".txt"},
      {input: "hello world.exe", expected:".exe"},
      {input: "Amazon.com - Order.pdf", expected:".pdf"},
      {input: "file", expected:""},
      {input: "file.", expected:""},
      {input: "file.tar.gz", expected:".tar.gz"},
    ]
    for (const test of tests) {
      expect(getFileExtension(test.input)).toEqual(test.expected);
    }
  });
});
