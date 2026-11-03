import { afterEach, describe, expect, it, vi } from "vitest";
import { clearTextPreviewCache, fetchTextPreview, languageFromPreviewUrl, textPreviewUrlFor } from "./textPreview.ts";

describe("languageFromPreviewUrl", () => {
  it("reads the lowercase extension from the path parameter", () => {
    expect(languageFromPreviewUrl("http://localhost/api/resources/preview/text?path=%2Fsrc%2FMain.GO&source=a")).toBe("go");
  });

  it("returns null for files without an extension", () => {
    expect(languageFromPreviewUrl("http://localhost/api/resources/preview/text?path=%2FMakefile")).toBeNull();
    expect(languageFromPreviewUrl("http://localhost/api/resources/preview/text?path=%2F.env")).toBeNull();
  });
});

describe("textPreviewUrlFor", () => {
  it("maps the authenticated preview URL to the text endpoint and keeps params", () => {
    const thumb = "http://localhost/api/resources/preview?path=%2Fa.md&key=123&source=main&inline=true";
    expect(textPreviewUrlFor(thumb)).toBe(
      "http://localhost/api/resources/preview/text?path=%2Fa.md&key=123&source=main&inline=true",
    );
  });

  it("maps the public share preview URL and keeps the hash", () => {
    const thumb = "http://localhost/public/api/resources/preview?path=%2Fa.txt&hash=abc";
    expect(textPreviewUrlFor(thumb)).toBe("http://localhost/public/api/resources/preview/text?path=%2Fa.txt&hash=abc");
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
