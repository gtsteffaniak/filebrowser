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
  DIRECTORY_README_MAX_BYTES,
  findDirectoryReadme,
  readTextResponseUpTo,
  renderMarkdown,
} from "./markdown";

describe("findDirectoryReadme", () => {
  it("finds a markdown README without case sensitivity", () => {
    const readme = findDirectoryReadme({
      type: "directory",
      items: [
        { name: "notes.md", type: "text/markdown" },
        { name: "ReadMe.md", type: "text/markdown" },
      ],
    });

    expect(readme?.name).toBe("ReadMe.md");
  });

  it("ignores directories named README.md", () => {
    expect(findDirectoryReadme({
      type: "directory",
      items: [{ name: "README.md", type: "directory" }],
    })).toBeNull();
  });
});

describe("renderMarkdown", () => {
  it("sanitizes unsafe HTML", () => {
    const rendered = renderMarkdown(
      "# Welcome\n<script>alert('bad')</script><a href=\"javascript:alert(1)\">bad</a>",
      "/README.md",
      "shared",
    );

    expect(rendered).toContain("<h1>Welcome</h1>");
    expect(rendered).not.toContain("<script");
    expect(rendered).not.toContain("javascript:");
  });

  it("rewrites relative image references through the preview endpoint", () => {
    const rendered = renderMarkdown("![Logo](images/logo.png)", "/docs/README.md", "shared");

    expect(rendered).toContain(encodeURIComponent("/docs/images/logo.png"));
  });

  it("hardens automatically rendered Markdown", () => {
    const rendered = renderMarkdown(
      '[external](https://example.com)' +
        '<p style="position:fixed;inset:0">overlay</p>' +
        '![Tracker](https://tracker.example/pixel.png)' +
        '<form><input value="phishing"></form>',
      "/README.md",
      "shared",
      true,
    );

    expect(rendered).toContain('href="https://example.com"');
    expect(rendered).toContain('rel="noopener noreferrer"');
    expect(rendered).not.toContain("style=");
    expect(rendered).not.toContain("tracker.example");
    expect(rendered).not.toContain("<form");
    expect(rendered).not.toContain("<input");
  });

  it("blocks external images", () => {
    const rendered = renderMarkdown(
      "![Tracker](http://tracker.example/pixel.png)",
      "/README.md",
      "shared",
      true,
    );

    expect(rendered).not.toContain("tracker.example");
  });

  it("rewrites relative links to explicit file routes for inline READMEs", () => {
    const rendered = renderMarkdown("[Guide](guide.md#intro)", "/docs/README.md", "shared", true);

    expect(rendered).toContain('href="/files/shared/docs/guide.md#intro"');
  });

  it("defines a conservative automatic rendering limit", () => {
    expect(DIRECTORY_README_MAX_BYTES).toBe(262144);
  });
});

describe("readTextResponseUpTo", () => {
  it("reads a response within the byte limit", async () => {
    const response = new Response("# Welcome");

    await expect(readTextResponseUpTo(response, 64)).resolves.toBe("# Welcome");
  });

  it("cancels a response exceeding the byte limit", async () => {
    const response = new Response("x".repeat(65));

    await expect(readTextResponseUpTo(response, 64)).resolves.toBeNull();
  });
});
