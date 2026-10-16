import { beforeEach, describe, expect, it, vi } from "vitest";

const { store } = vi.hoisted(() => ({
  store: { share: false, creatable: [], user: { scopes: [{ name: "data" }, { name: "inbox" }] } },
}));
vi.mock("@/store", () => ({
  state: { get user() { return store.user; } },
  mutations: {},
  getters: {
    isShare: () => store.share,
    canCreateInSource: (source) => store.creatable.includes(source),
  },
}));
vi.mock("@/api", () => ({ resourcesApi: {}, shareApi: {}, usersApi: {} }));
vi.mock("@/components/Action.vue", () => ({ default: { render: () => null } }));
vi.mock("@/notify", () => ({ notify: {} }));
vi.mock("@/utils/tooltipHelp.js", () => ({ hideInteractiveTooltip: vi.fn(), showHoverTooltip: vi.fn() }));
vi.mock("@/utils", () => ({ url: {} }));
vi.mock("@/utils/clipboard", () => ({ copyToClipboard: vi.fn() }));
vi.mock("@/utils/constants.js", () => ({ globalVars: {} }));
vi.mock("@/utils/download", () => ({ default: vi.fn() }));
vi.mock("@/utils/nativeShare", () => ({ canNativeShare: vi.fn(), nativeShareFile: vi.fn() }));
vi.mock("@/utils/mimetype", () => ({ isRichTextPreviewMimeType: vi.fn(), getTypeInfoFromExt: vi.fn() }));
vi.mock("@/utils/mediaFile", () => ({ isMediaRequest: vi.fn() }));
vi.mock("@/utils/expandTransition", () => ({ expandBeforeEnter: vi.fn(), expandEnter: vi.fn(), expandLeave: vi.fn() }));
import ContextMenu from "./ContextMenu.vue";

const showCopy = (permissions) =>
  ContextMenu.computed.showCopy.call({ showLimitedOptions: false, showCreate: false, selectedCount: 1, permissions });

describe("ContextMenu Copy", () => {
  beforeEach(() => {
    store.share = false;
    store.creatable = ["inbox"];
  });

  it("is offered in a read-only source, since copying only reads it", () => {
    expect(showCopy({ download: true, modify: false })).toBe(true);
  });

  it("is hidden when no source accepts the copy", () => {
    store.creatable = [];
    expect(showCopy({ download: true, modify: false })).toBe(false);
  });

  it("is hidden without download", () => {
    expect(showCopy({ download: false, modify: true })).toBe(false);
  });

  it("in a share, still requires modify, which the server checks for every change", () => {
    store.share = true;
    expect(showCopy({ download: true, modify: false })).toBe(false);
    expect(showCopy({ download: true, modify: true })).toBe(true);
  });
});
