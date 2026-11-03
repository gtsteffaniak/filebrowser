import { describe, expect, it } from "vitest";
import { parseDelimited } from "./delimited.ts";

describe("parseDelimited", () => {
  it("parses simple CSV rows", () => {
    expect(parseDelimited("name,age\nann,30\nbob,41\n", ",")).toEqual([
      ["name", "age"],
      ["ann", "30"],
      ["bob", "41"],
    ]);
  });

  it("handles quoted delimiters, doubled quotes, and embedded newlines", () => {
    expect(parseDelimited('a,"b,c","say ""hi"""\n"line1\nline2",x', ",")).toEqual([
      ["a", "b,c", 'say "hi"'],
      ["line1\nline2", "x"],
    ]);
  });

  it("accepts CRLF line endings and drops blank lines", () => {
    expect(parseDelimited("a,b\r\n\r\n1,2\r\n", ",")).toEqual([
      ["a", "b"],
      ["1", "2"],
    ]);
  });

  it("parses TSV with a tab delimiter", () => {
    expect(parseDelimited("a\tb\n1\t2", "\t")).toEqual([
      ["a", "b"],
      ["1", "2"],
    ]);
  });

  it("keeps empty cells", () => {
    expect(parseDelimited("a,,c\n,,", ",")).toEqual([
      ["a", "", "c"],
      ["", "", ""],
    ]);
  });
});
