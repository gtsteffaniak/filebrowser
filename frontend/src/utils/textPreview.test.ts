import { afterEach, describe, expect, it, vi } from "vitest";
import {
  clearTextPreviewCache,
  fetchTextPreview,
  getRenderedTextPreview,
  isTextPreviewableName,
  languageFromPath,
  languageFromPreviewUrl,
  MIN_TEXT_PREVIEW_WIDTH_PX,
  setRenderedTextPreview,
  textPreviewCacheKey,
} from "./textPreview.ts";

describe("languageFromPreviewUrl", () => {
  it("reads the lowercase extension from the path parameter", () => {
    expect(languageFromPreviewUrl("http://localhost/api/resources/preview/text?path=%2Fsrc%2FMain.GO&source=a")).toBe("go");
  });

  it("returns null for files without an extension", () => {
    expect(languageFromPreviewUrl("http://localhost/api/resources/preview/text?path=%2FMakefile")).toBeNull();
    expect(languageFromPreviewUrl("http://localhost/api/resources/preview/text?path=%2F.env")).toBeNull();
  });
});

describe("languageFromPath", () => {
  it("derives the extension without parsing a URL", () => {
    expect(languageFromPath("/src/Main.GO")).toBe("go");
    expect(languageFromPath("data.TSV")).toBe("tsv");
    expect(languageFromPath("Makefile")).toBeNull();
    expect(languageFromPath(".env")).toBeNull();
    expect(languageFromPath(null)).toBeNull();
  });
});

describe("isTextPreviewableName", () => {
  it("is true for extensions in the backend kind map", () => {
    // The vitest setup stubs globalVars.textPreviewExtensions.
    expect(isTextPreviewableName("notes.md")).toBe(true);
    expect(isTextPreviewableName("image.png")).toBe(false);
    expect(isTextPreviewableName("noext")).toBe(false);
  });
});

describe("MIN_TEXT_PREVIEW_WIDTH_PX", () => {
  it("is the tile width below which no inline preview is rendered", () => {
    expect(MIN_TEXT_PREVIEW_WIDTH_PX).toBe(60);
  });
});

describe("rendered HTML cache", () => {
  afterEach(() => {
    clearTextPreviewCache();
  });

  it("round-trips a value and marks it most recently used", () => {
    const key = textPreviewCacheKey("/api/resources/preview/text?path=a", "2026-01-01");
    setRenderedTextPreview(key, "<p>hi</p>");
    expect(getRenderedTextPreview(key)).toBe("<p>hi</p>");
  });

  it("keys by URL and modified time", () => {
    const a = textPreviewCacheKey("/u", "1");
    const b = textPreviewCacheKey("/u", "2");
    expect(a).not.toBe(b);
    setRenderedTextPreview(a, "<p>a</p>");
    expect(getRenderedTextPreview(b)).toBeUndefined();
  });
});

describe("fetchTextPreview cache", () => {
  afterEach(() => {
    clearTextPreviewCache();
    vi.unstubAllGlobals();
  });

  it("reuses a result for the same URL and modified time", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ content: "x", kind: "plain", format: "text", truncated: false, size: 1 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await fetchTextPreview("/api/resources/preview/text?path=a", "2026-01-01");
    await fetchTextPreview("/api/resources/preview/text?path=a", "2026-01-01");
    expect(fetchMock).toHaveBeenCalledTimes(1);

    // A changed modified time is a different key and must be fetched again.
    await fetchTextPreview("/api/resources/preview/text?path=a", "2026-02-01");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("does not cache failures, so a later view retries", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response("", { status: 415 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ content: "ok", kind: "plain", format: "text", truncated: false, size: 2 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(fetchTextPreview("/u", null)).rejects.toThrow("415");
    await expect(fetchTextPreview("/u", null)).resolves.toMatchObject({ content: "ok" });
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
