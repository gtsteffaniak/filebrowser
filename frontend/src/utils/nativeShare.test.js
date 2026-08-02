import { beforeEach, describe, expect, it, vi } from "vitest";

const showErrorMock = vi.fn();

vi.mock("@/api", () => ({
  resourcesApi: {
    getDownloadURL: vi.fn(() => "https://example.com/api/raw/report.xlsx"),
    getDownloadURLPublic: vi.fn(() => "https://example.com/api/public/dl/report.xlsx"),
  },
}));

vi.mock("@/notify", () => ({
  notify: {
    showError: (...args) => showErrorMock(...args),
  },
}));

vi.mock("@/store", () => ({
  getters: {
    isShare: () => false,
  },
  state: {
    req: { source: "default" },
  },
}));

import { canNativeShare, nativeShareFile } from "./nativeShare";

function mockShare(implementation) {
  Object.defineProperty(window, "isSecureContext", {
    configurable: true,
    value: true,
  });
  navigator.share = vi.fn(implementation ?? (() => Promise.resolve()));
}

function stubFetch(blobType) {
  global.fetch = vi.fn(() =>
    Promise.resolve({
      ok: true,
      statusText: "OK",
      status: 200,
      blob: () =>
        Promise.resolve({
          type: blobType,
        }),
    })
  );
}

beforeEach(() => {
  showErrorMock.mockClear();
  delete navigator.canShare;
  delete navigator.share;
  global.fetch = undefined;
});

describe("canNativeShare", () => {
  it("returns false when navigator.share is unavailable", () => {
    expect(canNativeShare()).toBe(false);
  });

  it("returns true when navigator.share exists and context is secure", () => {
    mockShare();
    expect(canNativeShare()).toBe(true);
  });
});

describe("nativeShareFile", () => {
  it("shows an error when sharing isn't supported at all", async () => {
    await nativeShareFile({ name: "report.xlsx", path: "/report.xlsx" });
    expect(showErrorMock).toHaveBeenCalledWith("Sharing is not supported on this device");
  });

  it(
    "falls back to sharing a link (not the file) when the platform rejects the file's real MIME type " +
      "-- regression test for #2659, where a hardcoded text/plain probe file made canShareFiles() always " +
      "return true, so navigator.share() was called with an .xlsx/.docx/.zip file the platform then " +
      "silently rejected, surfacing as 'permission denied'",
    async () => {
      mockShare();
      stubFetch("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet");
      // Platform can share files in general, but rejects this specific
      // (non-safelisted) MIME type -- exactly what real browsers do for
      // office/archive formats.
      navigator.canShare = vi.fn(({ files }) => {
        const file = files?.[0];
        return file ? file.type === "text/plain" : false;
      });

      await nativeShareFile({ name: "report.xlsx", path: "/report.xlsx" });

      expect(navigator.share).toHaveBeenCalledTimes(1);
      const sharedPayload = navigator.share.mock.calls[0][0];
      // Must NOT have attempted to share the actual xlsx file bytes.
      expect(sharedPayload.files).toBeUndefined();
      expect(sharedPayload.url).toBe("https://example.com/api/raw/report.xlsx");
      expect(showErrorMock).not.toHaveBeenCalled();
    }
  );

  it("shares the real file when the platform's canShare accepts its actual type", async () => {
    mockShare();
    stubFetch("application/pdf");
    navigator.canShare = vi.fn(({ files }) => files?.[0]?.type === "application/pdf");

    await nativeShareFile({ name: "doc.pdf", path: "/doc.pdf" });

    expect(navigator.share).toHaveBeenCalledTimes(1);
    const sharedPayload = navigator.share.mock.calls[0][0];
    expect(sharedPayload.files).toHaveLength(1);
    expect(sharedPayload.files[0].name).toBe("doc.pdf");
    expect(sharedPayload.files[0].type).toBe("application/pdf");
  });

  it("shares a link directly when the platform has no canShare at all", async () => {
    mockShare();
    // No navigator.canShare defined -- older/limited platforms.

    await nativeShareFile({ name: "report.xlsx", path: "/report.xlsx" });

    expect(global.fetch).toBeUndefined();
    expect(navigator.share).toHaveBeenCalledWith({
      url: "https://example.com/api/raw/report.xlsx",
      title: "report.xlsx",
    });
  });

  it("surfaces a download failure instead of silently sharing nothing", async () => {
    mockShare();
    navigator.canShare = vi.fn(() => true);
    global.fetch = vi.fn(() =>
      Promise.resolve({ ok: false, status: 403, statusText: "Forbidden" })
    );

    await nativeShareFile({ name: "report.xlsx", path: "/report.xlsx" });

    expect(navigator.share).not.toHaveBeenCalled();
    expect(showErrorMock).toHaveBeenCalledWith("Download failed (403 (Forbidden))");
  });

  it("silently ignores the user cancelling the OS share sheet", async () => {
    mockShare(() => {
      const err = new Error("cancelled");
      err.name = "AbortError";
      return Promise.reject(err);
    });

    await nativeShareFile({ name: "report.xlsx", path: "/report.xlsx" });

    expect(showErrorMock).not.toHaveBeenCalled();
  });
});
