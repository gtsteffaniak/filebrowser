import { beforeEach, describe, expect, it, vi } from "vitest";

const { store } = vi.hoisted(() => ({ store: { tool: null, showSelectMultiple: false, mobile: false } }));
vi.mock("@/store", () => ({
  state: {
    get user() {
      return { showSelectMultiple: store.showSelectMultiple };
    },
  },
  mutations: {},
  getters: {
    currentTool: () => store.tool,
    isMobile: () => store.mobile,
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

const { computed } = ContextMenu;
const inAdvancedSearch = () => computed.isAdvancedSearch.call({});

describe("ContextMenu in advanced search", () => {
  beforeEach(() => {
    store.tool = null;
    store.showSelectMultiple = false;
    store.mobile = false;
  });

  it("detects the advanced search tool", () => {
    expect(inAdvancedSearch()).toBe(false);
    store.tool = { component: "DuplicateFinder" };
    expect(inAdvancedSearch()).toBe(false);
    store.tool = { component: "AdvancedSearch" };
    expect(inAdvancedSearch()).toBe(true);
  });

  it("offers Select multiple on desktop even when the user preference is off", () => {
    const menu = (isAdvancedSearch) => ({ showLimitedOptions: false, isMultiple: false, isSearchActive: false, isAdvancedSearch });
    expect(computed.showSelectMultiple.call(menu(false))).toBe(false);
    expect(computed.showSelectMultiple.call(menu(true))).toBe(true);
  });

  it("doesn't offer Select multiple once multiple mode is on", () => {
    expect(
      computed.showSelectMultiple.call({ showLimitedOptions: false, isMultiple: true, isSearchActive: false, isAdvancedSearch: true }),
    ).toBe(false);
  });

  it("hides the create (+) toggle, since results have no single folder to create in", () => {
    const toggle = (isAdvancedSearch) =>
      computed.showCreateToggle.call({ showLimitedOptions: false, createOnly: false, isSearchActive: false, isAdvancedSearch });
    expect(toggle(false)).toBe(true);
    expect(toggle(true)).toBe(false);
  });

  it("hides Share when nothing is selected, since the results listing has no folder to share", () => {
    const share = (isAdvancedSearch, selectedCount) =>
      computed.showShareAction.call({ showLimitedOptions: false, isAdvancedSearch, selectedCount, showShare: true });
    expect(share(true, 0)).toBe(false);
    expect(share(true, 1)).toBe(true);
    expect(share(true, 2)).toBe(false);
    expect(share(false, 0)).toBe(true);
  });
});
