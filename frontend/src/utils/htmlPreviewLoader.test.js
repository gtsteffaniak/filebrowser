import { describe, expect, it, vi } from "vitest";
import { createHtmlPreviewLoader } from "./htmlPreviewLoader.ts";

function setup() {
  let serial = 0;
  const createURL = vi.fn(() => `blob:preview-${++serial}`);
  const revokeURL = vi.fn();
  const frame = { location: { replace: vi.fn() } };
  return { frame, createURL, revokeURL, loader: createHtmlPreviewLoader(createURL, revokeURL) };
}

describe("HTML preview history", () => {
  it("marks the default blob as UTF-8", () => {
    let blob;
    const createObjectURL = vi.fn((value) => { blob = value; return "blob:utf8"; });
    vi.stubGlobal("URL", { createObjectURL, revokeObjectURL: vi.fn() });
    try {
      const loader = createHtmlPreviewLoader();
      loader.load({ location: { replace: vi.fn() } }, "中文 — café € 🎥");
      expect(blob.type).toBe("text/html;charset=utf-8");
      loader.dispose();
    } finally {
      vi.unstubAllGlobals();
    }
  });
  it("replaces the frame document for each page and avoids duplicate renders", () => {
    const { frame, createURL, revokeURL, loader } = setup();
    loader.load(frame, "index");
    loader.load(frame, "post");
    loader.load(frame, "post");
    expect(frame.location.replace.mock.calls).toEqual([["blob:preview-1"], ["blob:preview-2"]]);
    expect(createURL).toHaveBeenCalledTimes(2);
    expect(revokeURL).toHaveBeenCalledWith("blob:preview-1");
  });
  it("reloads the document when browser Back selects the earlier app route", () => {
    const { frame, loader } = setup();
    loader.load(frame, "index");
    loader.load(frame, "post");
    loader.load(frame, "index");
    expect(frame.location.replace).toHaveBeenCalledTimes(3);
  });
  it("renders into a recreated iframe and releases URLs on unmount", () => {
    const { frame, loader, revokeURL } = setup();
    loader.load(frame, "index");
    const replacement = { location: { replace: vi.fn() } };
    loader.load(replacement, "index");
    expect(replacement.location.replace).toHaveBeenCalledWith("blob:preview-2");
    loader.dispose();
    loader.dispose();
    expect(revokeURL.mock.calls).toEqual([["blob:preview-1"], ["blob:preview-2"]]);
  });
  it("releases a failed navigation URL without discarding the current document", () => {
    const { frame, loader, revokeURL } = setup();
    loader.load(frame, "index");
    frame.location.replace.mockImplementationOnce(() => { throw new Error("blocked"); });
    expect(() => loader.load(frame, "post")).toThrow("blocked");
    expect(revokeURL).toHaveBeenCalledWith("blob:preview-2");
    expect(revokeURL).not.toHaveBeenCalledWith("blob:preview-1");
    loader.dispose();
    expect(revokeURL).toHaveBeenCalledWith("blob:preview-1");
  });
});
