import { describe, it, expect } from 'vitest';
import { removePrefix } from './url.js';

describe('testurl', () => {

  it('url prefix', () => {
    let tests = [
      {input: "test",trimArg: "",expected:"/test"},
      {input: "/test", trimArg: "test",expected:"/"},
      {input: "/my/test/file", trimArg: "",expected:"/my/test/file"},
      {input: "/my/test/file", trimArg: "my",expected:"/test/file"},
      {input: "/files/my/test/file", trimArg: "files",expected:"/my/test/file"},
    ]
    for (let test of tests) {
      expect(removePrefix(test.input, test.trimArg)).toEqual(test.expected);
    }
  });

});
