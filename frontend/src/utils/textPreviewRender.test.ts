import DOMPurify from "dompurify";
import { describe, expect, it } from "vitest";
import { plainTextFromHtml, renderMarkdownTile, sanitizeTileHtml } from "./textPreviewRender.ts";

describe("plainTextFromHtml", () => {
  it("returns visible text without tags", () => {
    expect(plainTextFromHtml(sanitizeTileHtml("<h1>Title</h1><p>Body <strong>text</strong></p>"))).toBe("TitleBody text");
  });
});

describe("sanitizeTileHtml", () => {
  it("removes scripts and inline event handlers", () => {
    const out = sanitizeTileHtml('<p onclick="x()">hi</p><script>alert(1)</script><img src=x onerror="alert(1)">');
    expect(out).toContain("<p>hi</p>");
    expect(out).not.toContain("<script");
    expect(out).not.toContain("onclick");
    expect(out).not.toContain("onerror");
  });

  it("removes javascript: and remote URLs so tiles make no requests", () => {
    const out = sanitizeTileHtml(
      '<a href="javascript:alert(1)">a</a><img src="https://example.com/t.png" srcset="https://example.com/t2.png 2x"><video poster="https://example.com/p.jpg"></video>',
    );
    expect(out).not.toContain("javascript:");
    expect(out).not.toContain("example.com");
    expect(out).not.toContain("poster");
  });

  it("drops document styles and style attributes", () => {
    const out = sanitizeTileHtml('<style>body{display:none}</style><p style="color:red">x</p>');
    expect(out).not.toContain("<style");
    expect(out).not.toContain("style=");
    expect(out).toContain("<p>x</p>");
  });

  it("removes forms and embedded frames, and turns inputs into disabled checkboxes", () => {
    const out = sanitizeTileHtml('<form action="https://evil"><input type="text" name="a"></form><iframe src="https://evil"></iframe>');
    expect(out).not.toContain("<form");
    expect(out).not.toContain("<iframe");
    expect(out).not.toContain('type="text"');
    expect(out).toContain('type="checkbox"');
    expect(out).toContain("disabled");
  });

  it("keeps safe formatting", () => {
    const out = sanitizeTileHtml("<h1>Title</h1><ul><li><strong>bold</strong></li></ul><pre><code>x</code></pre>");
    expect(out).toContain("<h1>Title</h1>");
    expect(out).toContain("<strong>bold</strong>");
    expect(out).toContain("<pre><code>x</code></pre>");
  });

  it("does not change DOMPurify behaviour for other callers", () => {
    sanitizeTileHtml("<p>warm up</p>");
    // MarkdownViewer uses the default DOMPurify instance and must still keep image URLs.
    expect(DOMPurify.sanitize('<img src="https://example.com/a.png">')).toContain('src="https://example.com/a.png"');
  });
});

describe("renderMarkdownTile", () => {
  it("renders GFM markdown and sanitizes raw HTML in it", () => {
    const out = renderMarkdownTile("# Hello\n\n- [x] done\n\n<script>alert(1)</script>");
    expect(out).toContain("<h1");
    expect(out).toContain("Hello");
    expect(out).toContain('type="checkbox"');
    expect(out).not.toContain("<script");
  });

  it("removes images so markdown tiles do not fetch remote content", () => {
    const out = renderMarkdownTile("![logo](https://example.com/logo.png)");
    expect(out).toContain("<img");
    expect(out).not.toContain("example.com");
  });
});
