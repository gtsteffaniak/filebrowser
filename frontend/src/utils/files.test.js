import { describe, it, expect } from 'vitest';
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
    for (let i in tests) {
      expect(getFileExtension(tests[i].input)).toEqual(tests[i].expected);
    }
  });

});
