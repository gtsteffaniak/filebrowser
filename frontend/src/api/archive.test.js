import { beforeEach, describe, expect, it, vi } from "vitest";
const { fetchMock } = vi.hoisted(() => ({ fetchMock: vi.fn() }));
vi.mock("./utils", () => ({ fetchURL: fetchMock }));
vi.mock("@/utils/url.js", () => ({ getApiPath: (path) => `/api/${path}` }));
vi.mock("@/notify", () => ({ notify: { showError: vi.fn() } }));
import { unarchive } from "./archive.js";

describe("ZIP encoding requests", () => {
  beforeEach(() => fetchMock.mockReset());
  it("inspects without forwarding deletion even when requested", async () => {
    const preview = { suggested: "cp932", candidates: [{ encoding: "cp932", names: ["テスト.txt"] }] };
    fetchMock.mockResolvedValue({ json: async () => preview });
    expect(await unarchive({ fromSource: "media", path: "/test.zip", destination: "/", preview: true, deleteAfter: true })).toEqual(preview);
    const body = JSON.parse(fetchMock.mock.calls[0][1].body);
    expect(body).toEqual({ fromSource: "media", path: "/test.zip", destination: "/", preview: true });
  });
  it("sends the encoding selected in the preview when extracting", async () => {
    fetchMock.mockResolvedValue({ json: async () => ({ path: "/out" }) });
    await unarchive({ fromSource: "media", toSource: "other", path: "/test.zip", destination: "/out", filenameEncoding: "cp932", deleteAfter: true });
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ fromSource: "media", toSource: "other", path: "/test.zip", destination: "/out", filenameEncoding: "cp932", deleteAfter: true });
  });
});
