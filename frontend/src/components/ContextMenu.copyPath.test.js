import { beforeEach, describe, expect, it, vi } from "vitest";

const { stateMock, gettersMock, mutationsMock, clipboardMock } = vi.hoisted(() => ({
  stateMock: { req: {}, sources: {}, user: {} },
  gettersMock: { isShare: vi.fn(() => false) },
  mutationsMock: { closeHovers: vi.fn() },
  clipboardMock: vi.fn(),
}));

vi.mock("@/store", () => ({ state: stateMock, getters: gettersMock, mutations: mutationsMock }));
vi.mock("@/api", () => ({ resourcesApi: {}, shareApi: {}, usersApi: {} }));
vi.mock("@/utils", () => ({ url: {} }));
vi.mock("@/utils/clipboard", () => ({ copyToClipboard: clipboardMock }));
vi.mock("@/utils/download", () => ({ default: vi.fn() }));
vi.mock("@/utils/nativeShare", () => ({ canNativeShare: vi.fn(), nativeShareFile: vi.fn() }));
vi.mock("@/components/Action.vue", () => ({ default: {} }));

import ContextMenu from "./ContextMenu.vue";

describe("Copy Path context-menu action", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    gettersMock.isShare.mockReturnValue(false);
    stateMock.req = { source: "Home" };
    stateMock.sources = {
      current: "Home",
      info: {
        Home: { copyPathPrefix: "/home/rt" },
        Data: { copyPathPrefix: "/data" },
        RT: { copyPathWindows: true },
      },
    };
    stateMock.user = { scopes: [{ name: "Home", scope: "/" }, { name: "Data", scope: "/" }] };
  });

  async function copy(item) {
    await ContextMenu.methods.copyPathToClipboard.call({ firstSelected: item });
    expect(mutationsMock.closeHovers).toHaveBeenCalledOnce();
    return clipboardMock.mock.calls[0][0];
  }

  it("uses the selected search result's source instead of the current source", async () => {
    expect(await copy({ source: "Data", path: "/movies/test.mkv" })).toBe("/data/movies/test.mkv");
  });

  it("supports Windows paths through the existing action", async () => {
    expect(await copy({ source: "RT", path: "/D/qwen36-longtest-nomtp.txt" }))
      .toBe("D:\\qwen36-longtest-nomtp.txt");
  });

  it("falls back to the request source for listing items without a source", async () => {
    expect(await copy({ path: "/pact_runs/books/" })).toBe("/home/rt/pact_runs/books/");
  });

  it("uses the matching user scope", async () => {
    stateMock.user.scopes[1].scope = "/movies/";
    expect(await copy({ source: "Data", path: "/test.mkv" })).toBe("/data/movies/test.mkv");
  });

  it("preserves legacy behavior for an unconfigured source", async () => {
    expect(await copy({ source: "Other", path: "/books/" })).toBe("/books/");
  });

  it("does not read inherited properties as source configuration", async () => {
    expect(await copy({ source: "__proto__", path: "/books/" })).toBe("/books/");
  });

  it("does not apply mappings in a public share with a logged-in store", async () => {
    gettersMock.isShare.mockReturnValue(true);
    expect(await copy({ source: "Home", path: "/books/" })).toBe("/books/");
  });

  it("does not copy a prefix when there is no selection", async () => {
    expect(await copy(null)).toBe("");
  });
});
