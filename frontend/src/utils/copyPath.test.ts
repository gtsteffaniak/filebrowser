import { describe, expect, it } from "vitest";
import { formatCopyPath } from "./copyPath.ts";

describe("formatCopyPath", () => {
  it("preserves the original path without an explicit mapping", () => {
    for (const path of ["", "/", "/D/file.txt", "/pact_runs/books/", "relative.txt"]) {
      expect(formatCopyPath(path)).toBe(path);
      expect(formatCopyPath(path, {}, "/users/alice/")).toBe(path);
      expect(formatCopyPath(path, { copyPathPrefix: "", copyPathWindows: false })).toBe(path);
    }
  });

  it.each([
    ["/pact_runs/books/", "/home/rt", "/home/rt/pact_runs/books/"],
    ["/movies/test.mkv", "/data/", "/data/movies/test.mkv"],
    ["/", "/data/", "/data/"],
    ["/books/", "/", "/books/"],
    ["/Книги/a b%20.txt", "/home/rt", "/home/rt/Книги/a b%20.txt"],
  ])("maps POSIX path %s with prefix %s", (path, prefix, expected) => {
    expect(formatCopyPath(path, { copyPathPrefix: prefix })).toBe(expected);
  });

  it.each([
    ["/D/qwen36-longtest-nomtp.txt", "D:\\qwen36-longtest-nomtp.txt"],
    ["/d/Книги/a b.txt", "d:\\Книги\\a b.txt"],
    ["/D/books/", "D:\\books\\"],
    ["/D/", "D:\\"],
    ["/D", "D:\\"],
    ["/", "/"],
    ["/Data/file.txt", "/Data/file.txt"],
    ["/1/file.txt", "/1/file.txt"],
    ["relative.txt", "relative.txt"],
  ])("converts only a single-letter drive component in %s", (path, expected) => {
    expect(formatCopyPath(path, { copyPathWindows: true })).toBe(expected);
  });

  it("restores the user's source-relative scope only when mapping is enabled", () => {
    expect(formatCopyPath("/books/", { copyPathPrefix: "/home" }, "/rt/"))
      .toBe("/home/rt/books/");
    expect(formatCopyPath("/test.txt", { copyPathWindows: true }, "/D/work/"))
      .toBe("D:\\work\\test.txt");
  });

  it("applies the prefix before Windows conversion", () => {
    expect(formatCopyPath("/file.txt", { copyPathPrefix: "/D/work/", copyPathWindows: true }))
      .toBe("D:\\work\\file.txt");
    expect(formatCopyPath("/D/file.txt", { copyPathPrefix: "/data", copyPathWindows: true }))
      .toBe("/data/D/file.txt");
  });

  it("keeps an empty selection empty even when configured", () => {
    expect(formatCopyPath("", { copyPathPrefix: "/home/rt", copyPathWindows: true })).toBe("");
  });
});
