import { beforeEach, describe, expect, it, vi } from "vitest";

const { store, mutations } = vi.hoisted(() => ({
  store: { tool: null, share: false, user: {} },
  mutations: { resetSelected: vi.fn(), showPrompt: vi.fn() },
}));
vi.mock("@/utils/auth", () => ({ logout: vi.fn() }));
vi.mock("@/utils/constants", () => ({ globalVars: {} }));
vi.mock("@/store", () => ({
  state: {
    get user() {
      return store.user;
    },
  },
  getters: {
    currentTool: () => store.tool,
    isShare: () => store.share,
  },
  mutations,
}));
vi.mock("@/utils/tooltipHelp.js", () => ({ hideInteractiveTooltip: vi.fn(), showHoverTooltip: vi.fn() }));
vi.mock("./Links.vue", () => ({ default: { render: () => null } }));
vi.mock("@/utils", () => ({ url: {} }));
import General from "./General.vue";

const { computed, methods } = General;

describe("sidebar File actions in advanced search", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    store.tool = null;
    store.share = false;
    store.user = {};
  });

  it("detects the advanced search tool", () => {
    expect(computed.isAdvancedSearch()).toBe(false);
    store.tool = { component: "AdvancedSearch" };
    expect(computed.isAdvancedSearch()).toBe(true);
  });

  it("is shown in advanced search even without create permissions", () => {
    const ctx = { isAdvancedSearch: true, hideSidebarFileActions: true, isListingView: false };
    expect(computed.showFileActions.call(ctx)).toBe(true);
  });

  it("respects the user's choice to hide the sidebar file actions", () => {
    store.user = { hideSidebarFileActions: true };
    expect(computed.showFileActions.call({ isAdvancedSearch: true })).toBe(false);
  });

  it("is not shown in a share's advanced search", () => {
    store.share = true;
    expect(computed.showFileActions.call({ isAdvancedSearch: true })).toBe(false);
  });

  it("keeps the old rule in the listing", () => {
    expect(computed.showFileActions.call({ isAdvancedSearch: false, hideSidebarFileActions: false, isListingView: true })).toBe(true);
    expect(computed.showFileActions.call({ isAdvancedSearch: false, hideSidebarFileActions: true, isListingView: true })).toBe(false);
    expect(computed.showFileActions.call({ isAdvancedSearch: false, hideSidebarFileActions: false, isListingView: false })).toBe(false);
  });

  it("opens the menu on the current selection in advanced search, without the create-only mode", () => {
    methods.openContextMenu.call({ isAdvancedSearch: true });
    expect(mutations.resetSelected).not.toHaveBeenCalled();
    expect(mutations.showPrompt).toHaveBeenCalledWith({ name: "ContextMenu", props: { showCentered: true } });
  });

  it("opens the create-only menu in the listing, as before", () => {
    methods.openContextMenu.call({ isAdvancedSearch: false });
    expect(mutations.resetSelected).toHaveBeenCalled();
    expect(mutations.showPrompt).toHaveBeenCalledWith({
      name: "ContextMenu",
      props: { showCentered: true, createOnly: true },
    });
  });
});
