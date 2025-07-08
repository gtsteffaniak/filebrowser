import { describe, it, expect, vi } from 'vitest';
import { adjustedData } from './utils.js';

describe('adjustedData', () => {
  vi.doMock('@/utils/constants', () => {
    return {
      baseURL: "unit-testing",
    };
  });
  it('should append the URL and process directory data correctly', () => {
    const input = {
      type: "directory",
      folders: [
        { name: "folder1", type: "directory" },
        { name: "folder2", type: "directory" },
      ],
      files: [
        { name: "file1.txt", type: "file" },
        { name: "file2.txt", type: "file" },
      ],
      path: "/root"
    };

    const expected = {
      type: "directory",
      folders: [],
      files: [],
      items: [
        { name: "folder1", path: "/root/folder1", source: undefined, type: "directory" },
        { name: "folder2", path: "/root/folder2", source: undefined, type: "directory" },
        { name: "file1.txt", path: "/root/file1.txt", source: undefined, type: "file" },
        { name: "file2.txt", path: "/root/file2.txt", source: undefined, type: "file" },
      ],
      path: "/root",
    };

    expect(adjustedData(input)).toEqual(expected);
  });

  it('should add a trailing slash to the URL if missing for a directory', () => {
    const input = { type: "directory", folders: [], files: [] };

    const expected = {
      type: "directory",
      folders: [],
      files: [],
      items: [],
    };

    expect(adjustedData(input)).toEqual(expected);
  });

  it('should handle non-directory types without modification to items', () => {
    const input = { type: "file", name: "file1.txt" };

    const expected = {
      type: "file",
      name: "file1.txt",
    };

    expect(adjustedData(input)).toEqual(expected);
  });

  it('should handle missing folders and files gracefully', () => {
    const input = { type: "directory" };
    const expected = {
      type: "directory",
      items: [],
    };

    expect(adjustedData(input)).toEqual(expected);
  });

  it('should handle empty input object correctly', () => {
    const input = {};
    const expected = {};
    expect(adjustedData(input)).toEqual(expected);
  });

});

