import { describe, expect, it } from "vitest";
import { sanitizeDocxHtml } from "./docxPreview";

describe("sanitizeDocxHtml", () => {
  it("strips javascript: hyperlinks", () => {
    const input =
      '<p><a href="javascript:alert(document.cookie)">CLICK HERE</a></p>';
    const output = sanitizeDocxHtml(input);
    expect(output).not.toContain("javascript:");
    expect(output).toContain("CLICK HERE");
  });

  it("preserves normal formatting", () => {
    const input = "<p>Hello <strong>world</strong></p>";
    expect(sanitizeDocxHtml(input)).toBe(input);
  });

  it("preserves embedded data-uri images from mammoth", () => {
    const input = '<p><img src="data:image/png;base64,abc" alt="chart"></p>';
    const output = sanitizeDocxHtml(input);
    expect(output).toContain('src="data:image/png;base64,abc"');
    expect(output).toContain('alt="chart"');
  });

  it("strips inline event handlers", () => {
    const input = '<p onclick="alert(1)">text</p>';
    const output = sanitizeDocxHtml(input);
    expect(output).not.toContain("onclick");
    expect(output).toContain("text");
  });

  it("preserves https hyperlinks", () => {
    const input = '<p><a href="https://example.com/doc">link</a></p>';
    expect(sanitizeDocxHtml(input)).toBe(input);
  });
});
