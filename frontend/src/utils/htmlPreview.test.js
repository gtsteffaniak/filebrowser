import { describe, expect, it, vi } from "vitest";

vi.mock("@/utils/constants", () => ({
  globalVars: {
    baseURL: "/",
    externalUrl: "",
  },
}));

vi.mock("@/store", () => ({
  getters: { isShare: () => false },
  state: { shareInfo: { subPath: "", hash: "", token: "" } },
}));

vi.mock("@/api/resources", () => ({
  getDownloadURL: (_source, path) =>
    `http://localhost/api/resources/download?file=${encodeURIComponent(path)}&inline=true`,
  getDownloadURLPublic: () => "http://localhost/public/download",
}));

import {
  buildHtmlPreview,
  buildPreviewResourceUrl,
  isLocalResourceReference,
  rewriteCssContent,
  rewriteSrcset,
} from "./htmlPreview";

describe("isLocalResourceReference", () => {
  it("treats relative paths as local", () => {
    expect(isLocalResourceReference("./style.css")).toBe(true);
    expect(isLocalResourceReference("../img/a.png")).toBe(true);
    expect(isLocalResourceReference("script.js")).toBe(true);
  });

  it("rejects absolute and dangerous protocols", () => {
    expect(isLocalResourceReference("https://example.com/a.css")).toBe(false);
    expect(isLocalResourceReference("//cdn.example.com/a.css")).toBe(false);
    expect(isLocalResourceReference("file:///etc/passwd")).toBe(false);
    expect(isLocalResourceReference("javascript:alert(1)")).toBe(false);
    expect(isLocalResourceReference("data:text/html,<script>")).toBe(false);
    expect(isLocalResourceReference("vbscript:alert(1)")).toBe(false);
  });

  it("rejects fragment-only references", () => {
    expect(isLocalResourceReference("#section")).toBe(false);
  });
});

describe("buildPreviewResourceUrl", () => {
  it("rewrites relative paths for any file type", () => {
    const base = "/files/index.html";
    expect(buildPreviewResourceUrl("./app.js", base, "src")).toContain(
      encodeURIComponent("/files/app.js"),
    );
    expect(buildPreviewResourceUrl("theme.css", base, "src")).toContain(
      encodeURIComponent("/files/theme.css"),
    );
    expect(buildPreviewResourceUrl("../img/a.png", base, "src")).toContain(
      encodeURIComponent("/img/a.png"),
    );
  });

  it("leaves external URLs unchanged", () => {
    const url = "https://cdn.example.com/lib.js";
    expect(buildPreviewResourceUrl(url, "/files/index.html", "src")).toBe(url);
  });
});

describe("rewriteSrcset", () => {
  it("rewrites each candidate URL", () => {
    const result = rewriteSrcset(
      "small.png 1x, large.png 2x",
      "/docs/index.html",
      "src",
    );
    expect(result).toContain(encodeURIComponent("/docs/small.png"));
    expect(result).toContain(encodeURIComponent("/docs/large.png"));
    expect(result).toContain("1x");
    expect(result).toContain("2x");
  });
});

describe("rewriteCssContent", () => {
  it("rewrites url() and @import references", () => {
    const css = `
      @import "fonts/roboto.woff2";
      body { background: url('../images/bg.png'); }
    `;
    const rewritten = rewriteCssContent(css, "/site/page.html", "src");
    expect(rewritten).toContain(encodeURIComponent("/site/fonts/roboto.woff2"));
    expect(rewritten).toContain(encodeURIComponent("/images/bg.png"));
  });
});

describe("buildHtmlPreview", () => {
  it("rewrites linked assets and preserves scripts in sandboxed srcdoc", () => {
    const html = `<!DOCTYPE html>
<html>
<head>
  <link rel="stylesheet" href="theme.css">
  <script src="app.js"></script>
</head>
<body>
  <img src="photo.jpg" srcset="photo.jpg 1x, photo-2x.jpg 2x">
  <a href="other.html">Next</a>
</body>
</html>`;

    const { srcdoc } = buildHtmlPreview(html, "/pages/index.html", "src");
    expect(srcdoc).toContain(encodeURIComponent("/pages/theme.css"));
    expect(srcdoc).toContain(encodeURIComponent("/pages/app.js"));
    expect(srcdoc).toContain(encodeURIComponent("/pages/photo.jpg"));
    expect(srcdoc).toContain(encodeURIComponent("/pages/other.html"));
    expect(srcdoc).toContain("<script");
    expect(srcdoc).not.toContain("javascript:");
  });

  it("strips dangerous protocols from attributes", () => {
    const html = `<html><body><a href="javascript:alert(1)">bad</a></body></html>`;
    const { srcdoc } = buildHtmlPreview(html, "/index.html", "src");
    expect(srcdoc).not.toContain("javascript:");
  });

  it("preserves inline SVG", () => {
    const html = `<html><body><svg viewBox="0 0 10 10"><circle cx="5" cy="5" r="4"/></svg></body></html>`;
    const { srcdoc } = buildHtmlPreview(html, "/index.html", "src");
    expect(srcdoc).toContain("<svg");
    expect(srcdoc).toContain("<circle");
  });
});
