import { afterEach, describe, expect, it, vi } from "vitest";
import {
  clearTextPreviewCache,
  fetchTextPreview,
  getRenderedTextPreview,
  isTextPreviewableType,
  languageFromPath,
  languageFromPreviewUrl,
  MIN_TEXT_PREVIEW_WIDTH_PX,
  releaseTextPreview,
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

describe("isTextPreviewableType", () => {
  it("is true for text/* mimetypes, with or without parameters", () => {
    for (const mt of ["text/markdown", "text/lyrics", "text/plain; charset=utf-8", "text/x-go"]) {
      expect(isTextPreviewableType(mt), mt).toBe(true);
    }
  });

  it("is true for text-extension mimetypes that are not text/*", () => {
    for (const mt of ["application/json", "application/yaml", "application/sql", "application/x-sh", "application/x-php", "application/x-powershell", "application/x-tex"]) {
      expect(isTextPreviewableType(mt), mt).toBe(true);
    }
  });

  it("is false for non-text mimetypes and missing types", () => {
    for (const mt of ["application/x-sqlite3", "application/postscript", "application/wasm", "application/backup", "application/x-msdownload", "application/octet-stream", "image/svg+xml", "image/png", "directory", "", null, undefined]) {
      expect(isTextPreviewableType(mt), String(mt)).toBe(false);
    }
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

describe("releaseTextPreview", () => {
  afterEach(() => {
    clearTextPreviewCache();
    vi.unstubAllGlobals();
  });

  it("keeps a settled result after the last consumer releases", async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ content: "x", kind: "plain", format: "text", truncated: false, size: 1 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    await fetchTextPreview("/u", "1");
    // The popup/tile unmounts after the fetch resolved.
    releaseTextPreview("/u", "1");

    // Reopening must reuse the cached result, not fetch again.
    await fetchTextPreview("/u", "1");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("aborts a pending request when its last consumer unmounts", async () => {
    let observedSignal: AbortSignal | undefined;
    const fetchMock = vi.fn((_url: string, init?: RequestInit) => {
      observedSignal = init?.signal ?? undefined;
      // Never resolves on its own; only aborts when the consumer goes away.
      return new Promise<Response>((_resolve, reject) => {
        observedSignal?.addEventListener("abort", () => reject(new DOMException("Aborted", "AbortError")));
      });
    });
    vi.stubGlobal("fetch", fetchMock);

    const pending = fetchTextPreview("/u", "1");
    expect(observedSignal?.aborted).toBe(false);
    releaseTextPreview("/u", "1");
    expect(observedSignal?.aborted).toBe(true);
    await expect(pending).rejects.toThrow(/Aborted/);
  });
});
