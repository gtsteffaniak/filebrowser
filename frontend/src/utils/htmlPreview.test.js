import { describe, expect, it, vi } from "vitest";

vi.mock("@/utils/constants", () => ({
  globalVars: {
    baseURL: "/",
    externalUrl: "",
  },
}));

vi.mock("@/store", () => ({
  getters: { isShare: () => false },
  state: {
    shareInfo: { subPath: "", hash: "", token: "" },
    req: {
      parentDirItems: [
        { name: "app.js", path: "/files/app.js", viewToken: "view-token" },
        { name: "theme.css", path: "/files/theme.css", viewToken: "view-token" },
        { name: "app.js", path: "/pages/app.js", viewToken: "view-token" },
        { name: "theme.css", path: "/pages/theme.css", viewToken: "view-token" },
        { name: "a.png", path: "/pages/a.png", viewToken: "view-token" },
        { name: "small.png", path: "/docs/small.png", viewToken: "view-token" },
        { name: "large.png", path: "/docs/large.png", viewToken: "view-token" },
        { name: "roboto.woff2", path: "/site/fonts/roboto.woff2", viewToken: "view-token" },
        { name: "bg.png", path: "/images/bg.png", viewToken: "view-token" },
        { name: "photo.jpg", path: "/pages/photo.jpg", viewToken: "view-token" },
        { name: "photo-2x.jpg", path: "/pages/photo-2x.jpg", viewToken: "view-token" },
        { name: "other.html", path: "/pages/other.html", viewToken: "view-token" },
      ],
    },
  },
}));

vi.mock("@/api/resources", () => ({
  getDownloadURL: (_source, path) =>
    `http://localhost/api/resources/download?file=${encodeURIComponent(path)}&inline=true`,
  getDownloadURLPublic: () => "http://localhost/public/download",
  getViewURL: (_source, path, viewToken, _share, allowDownloadFallback) => {
    if (viewToken) {
      return `http://localhost/api/resources/view?file=${encodeURIComponent(path)}&viewToken=${viewToken}`;
    }
    if (allowDownloadFallback) {
      return `http://localhost/api/resources/download?file=${encodeURIComponent(path)}&inline=true`;
    }
    return null;
  },
}));

vi.mock("@/api/media", () => ({}));

import {
  HTML_PREVIEW_CSP,
  HTML_PREVIEW_SANDBOX,
  buildHtmlPreview,
  buildPreviewResourceUrl,
  isAllowedTopNavigationUrl,
  isLocalResourceReference,
  rewriteCssContent,
  rewriteSrcset,
} from "./htmlPreview.ts";
import { state } from "@/store/index.ts";

describe("HTML_PREVIEW_SANDBOX", () => {
  it("uses same-origin without scripts so the session cookie reaches subresources", () => {
    expect(HTML_PREVIEW_SANDBOX).toContain("allow-same-origin");
    expect(HTML_PREVIEW_SANDBOX).not.toContain("allow-scripts");
  });

  it("never combines allow-scripts with allow-same-origin (XSS breakout risk)", () => {
    const tokens = HTML_PREVIEW_SANDBOX.split(/\s+/);
    expect(tokens.includes("allow-scripts") && tokens.includes("allow-same-origin")).toBe(false);
  });

  it("allows https passive resources but never scripts", () => {
    expect(HTML_PREVIEW_CSP).toContain("img-src");
    expect(HTML_PREVIEW_CSP).toContain("https:");
    expect(HTML_PREVIEW_CSP).toContain("script-src 'none'");
    expect(HTML_PREVIEW_CSP).not.toContain("script-src https:");
    expect(HTML_PREVIEW_CSP).toContain("connect-src 'none'");
  });
});

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
  it("rewrites relative paths for same-directory siblings", () => {
    const base = "/files/index.html";
    expect(buildPreviewResourceUrl("./app.js", base, "src")).toContain(
      encodeURIComponent("/files/app.js"),
    );
    expect(buildPreviewResourceUrl("theme.css", base, "src")).toContain(
      encodeURIComponent("/files/theme.css"),
    );
  });

  it("leaves paths outside the sibling directory unchanged when no token exists", () => {
    expect(buildPreviewResourceUrl("../img/a.png", "/files/index.html", "src")).toBe(
      "../img/a.png",
    );
  });

  it("does not match sibling tokens from a different directory", () => {
    const original = state.req.parentDirItems;
    state.req.parentDirItems = [
      { name: "app.js", path: "/other/app.js", viewToken: "wrong-token" },
    ];
    try {
      const url = buildPreviewResourceUrl("app.js", "/files/index.html", "src");
      expect(url).toBe("app.js");
      expect(url).not.toContain("wrong-token");
    } finally {
      state.req.parentDirItems = original;
    }
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

describe("isAllowedTopNavigationUrl", () => {
  it("accepts in-app HTML viewer routes for the active source", () => {
    const href = `${window.location.origin}/files/src/pages/other.html`;
    expect(isAllowedTopNavigationUrl(href, "src")).toBe(true);
  });

  it("rejects external and non-html viewer paths", () => {
    expect(isAllowedTopNavigationUrl("https://evil.test/x.html", "src")).toBe(false);
    expect(isAllowedTopNavigationUrl(`${window.location.origin}/files/src/pages/other.txt`, "src")).toBe(false);
  });
});

describe("buildHtmlPreview", () => {
  it("routes same-source HTML links to the viewer with target=_top", () => {
    const { srcdoc } = buildHtmlPreview('<a href="other.html">Next</a>', "/pages/index.html", "src");
    const doc = new DOMParser().parseFromString(srcdoc, "text/html");
    const anchor = doc.querySelector("a");
    expect(anchor.getAttribute("target")).toBe("_top");
    expect(anchor.getAttribute("href")).toBe(`${window.location.origin}/files/src/pages/other.html`);
    expect(anchor.getAttribute("href")).not.toContain("viewToken");
    expect(doc.querySelector("script")).toBeNull();
  });

  it("opens external http(s) links in a new tab", () => {
    const { srcdoc } = buildHtmlPreview('<a href="https://example.com/page">External</a>', "/index.html", "src");
    const anchor = new DOMParser().parseFromString(srcdoc, "text/html").querySelector("a");
    expect(anchor.getAttribute("target")).toBe("_blank");
    expect(anchor.getAttribute("rel")).toBe("noopener noreferrer");
  });

  it("rewrites linked assets and strips user scripts from the preview document", () => {
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
    expect(srcdoc).toContain(encodeURIComponent("/pages/photo.jpg"));
    const rendered = new DOMParser().parseFromString(srcdoc, "text/html");
    expect(rendered.querySelector("a").getAttribute("href")).toBe(
      `${window.location.origin}/files/src/pages/other.html`,
    );
    expect(srcdoc).not.toContain("app.js");
    expect(srcdoc).not.toContain("<script src=");
    expect(srcdoc).not.toContain("javascript:");
  });

  it("strips dangerous protocols from attributes", () => {
    const html = `<html><body><a href="javascript:alert(1)">bad</a></body></html>`;
    const { srcdoc } = buildHtmlPreview(html, "/index.html", "src");
    expect(srcdoc).not.toContain("javascript:");
  });

  it("declares UTF-8, CSP, and preserves multilingual text", () => {
    const text = "中文 — café € 🎥";
    const { srcdoc } = buildHtmlPreview(`<p>${text}</p><meta http-equiv="refresh" content="0;url=https://evil.test">`, "/index.html", "src");
    const doc = new DOMParser().parseFromString(srcdoc, "text/html");
    expect(doc.head.firstElementChild?.getAttribute("charset")).toBe("utf-8");
    expect(doc.querySelector('meta[http-equiv="Content-Security-Policy"]')?.getAttribute("content")).toBe(HTML_PREVIEW_CSP);
    expect(doc.querySelector("p").textContent).toBe(text);
    expect(doc.querySelector('meta[http-equiv="refresh"]')).toBeNull();
  });

  it("does not honor forged navigation markers from uploaded HTML", () => {
    const { srcdoc } = buildHtmlPreview('<a href="https://evil.test/page.html" data-filebrowser-html-link="true">External</a>', "/index.html", "src");
    const doc = new DOMParser().parseFromString(srcdoc, "text/html");
    expect(doc.querySelector("a").hasAttribute("data-filebrowser-html-link")).toBe(false);
    expect(doc.querySelector("a").getAttribute("target")).toBe("_blank");
  });

  it("preserves inline SVG", () => {
    const html = `<html><body><svg viewBox="0 0 10 10"><circle cx="5" cy="5" r="4"/></svg></body></html>`;
    const { srcdoc } = buildHtmlPreview(html, "/index.html", "src");
    expect(srcdoc).toContain("<svg");
    expect(srcdoc).toContain("<circle");
  });
});
