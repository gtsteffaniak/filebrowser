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

  it("removes legacy background attributes so table tiles make no requests", () => {
    const out = sanitizeTileHtml('<table background="https://example.com/x.png"><tr><td background="https://example.com/x.png">a</td><th background="https://example.com/x.png">b</th></tr></table>');
    expect(out).toContain("<td>a</td>");
    expect(out).not.toContain("example.com");
    expect(out).not.toContain("background");
  });

  it("never renders images, whether relative, absolute, or CSS backgrounds", () => {
    const out = sanitizeTileHtml(
      '<img src="rel.png"><picture><source srcset="https://example.com/a.webp"><img src="b.jpg"></picture>' +
        '<svg><image href="https://example.com/c.png"></image><use href="https://example.com/s.svg#i"></use></svg>' +
        '<div style="background:url(https://example.com/d.png)">x</div><input type="image" src="https://example.com/e.png">',
    );
    expect(out).not.toContain("<img");
    expect(out).not.toContain("<picture");
    expect(out).not.toContain("<image");
    expect(out).not.toContain("example.com");
    expect(out).not.toContain("rel.png");
    expect(out).not.toContain("b.jpg");
  });

  it("omits video from thumbnails, including its fallback text", () => {
    const out = sanitizeTileHtml(
      '<video controls src="https://example.com/v.mp4" poster="https://example.com/p.jpg">fallback</video><source src="https://example.com/s.mp4"><p>after</p>',
    );
    expect(out).not.toContain("<video");
    expect(out).not.toContain("fallback");
    expect(out).not.toContain("example.com");
    expect(out).toContain("<p>after</p>");
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
    const out = renderMarkdownTile("![logo](https://example.com/logo.png)\n\n![rel](images/a.png)");
    expect(out).not.toContain("<img");
    expect(out).not.toContain("example.com");
    expect(out).not.toContain("images/a.png");
  });
});
