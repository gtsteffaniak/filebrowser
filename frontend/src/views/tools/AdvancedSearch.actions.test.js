import { beforeEach, describe, expect, it, vi } from "vitest";

const { router, mutations, store, downloadFiles } = vi.hoisted(() => ({
  router: { go: vi.fn(), push: vi.fn(() => Promise.resolve()) },
  mutations: {
    resetSelected: vi.fn(),
    selectAllItems: vi.fn(),
    setMultiple: vi.fn(),
    closeHovers: vi.fn(),
    showPrompt: vi.fn(),
  },
  downloadFiles: vi.fn(),
  store: {
    multiple: false,
    selected: [],
    items: [],
    perms: {},
    creatable: [],
    scopes: [{ name: "data" }, { name: "inbox" }],
  },
}));
vi.mock("@/router", () => ({ router }));
vi.mock("@/store", () => ({
  state: {
    get multiple() {
      return store.multiple;
    },
    get selected() {
      return store.selected;
    },
    get req() {
      return { items: store.items };
    },
    get user() {
      return { scopes: store.scopes };
    },
    sources: { current: "data" },
  },
  getters: {
    selectedCount: () => store.selected.length,
    sourcePermissions: (source) => store.perms[source] ?? { download: true, modify: true },
    canCreateInSource: (source) => store.creatable.includes(source),
  },
  mutations,
}));
vi.mock("@/store/eventBus", () => ({ eventBus: { on: vi.fn(), off: vi.fn() } }));
vi.mock("@/api", () => ({ toolsApi: {} }));
vi.mock("@/utils/constants", () => ({ globalVars: {} }));
vi.mock("@/utils/download", () => ({ default: downloadFiles }));
vi.mock("@/utils/moment", () => ({ utcStartOfDaySecondsFromDateInput: vi.fn() }));
vi.mock("@/components/settings/ToggleSwitch.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/settings/SettingsItem.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/settings/ExpandDropdown.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/files/PathPickerButton.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/LoadingSpinner.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/files/ListingItem.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/files/ListingHeader.vue", () => ({ default: { render: () => null } }));
import AdvancedSearch from "./AdvancedSearch.vue";

const { methods, computed } = AdvancedSearch;

describe("AdvancedSearch close", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.history.replaceState({}, "");
  });

  it("goes back to the page the search was opened from", () => {
    window.history.replaceState({ back: "/files/data/photos/" }, "");
    methods.closeSearch.call({});
    expect(router.go).toHaveBeenCalledWith(-1);
    expect(router.push).not.toHaveBeenCalled();
  });

  it("opens the files when there is no previous page, e.g. the search URL was opened directly", () => {
    methods.closeSearch.call({});
    expect(router.go).not.toHaveBeenCalled();
    expect(router.push).toHaveBeenCalledWith({ path: "/files" });
  });

  it("drops the selection so it doesn't leak into the page it returns to", () => {
    methods.closeSearch.call({});
    expect(mutations.resetSelected).toHaveBeenCalled();
  });
});

describe("AdvancedSearch selection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    store.multiple = false;
    store.selected = [];
    store.items = [];
    store.perms = {};
    store.creatable = [];
  });

  it("selects every result in multiple mode", () => {
    methods.selectAllResults.call({});
    expect(mutations.selectAllItems).toHaveBeenCalledWith({ multiple: true });
  });

  it("turns on multiple selection so clicks select instead of opening, even with single click enabled", () => {
    methods.enableMultipleSelection.call({});
    expect(mutations.setMultiple).toHaveBeenCalledWith(true);
  });

  it("clears the selection and leaves multiple mode", () => {
    methods.clearSelectedResults.call({});
    expect(mutations.resetSelected).toHaveBeenCalled();
  });

  it("reflects the shared selection state for the toolbar buttons", () => {
    store.multiple = true;
    store.selected = [0, 1, 2];
    expect(computed.isMultiple.call({})).toBe(true);
    expect(computed.selectedCount.call({})).toBe(3);
  });
});

const results = [
  { name: "a.jpg", path: "/Boda/a.jpg", source: "data" },
  { name: "b.jpg", path: "/Otros/b.jpg", source: "inbox" },
];

describe("AdvancedSearch selected results", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    store.items = results;
    store.selected = [0, 1];
    store.perms = {};
    store.creatable = ["inbox"];
  });

  it("maps the selected indexes to the search results", () => {
    expect(computed.selectedResults.call({})).toEqual(results);
  });

  it("accepts a selection made of items instead of indexes", () => {
    store.selected = [results[1]];
    expect(computed.selectedResults.call({})).toEqual([results[1]]);
  });

  it("is empty without a selection", () => {
    store.selected = [];
    expect(computed.selectedResults.call({})).toEqual([]);
  });

  it("offers download, copy and move when every source allows them", () => {
    const self = { selectedResults: results, selectionPermissions: computed.selectionPermissions.call({ selectedResults: results }) };
    expect(computed.canDownloadSelection.call(self)).toBe(true);
    expect(computed.canMoveSelection.call(self)).toBe(true);
    expect(computed.canCopySelection.call(self)).toBe(true);
  });

  it("needs permission on every source of the selection, not just one", () => {
    store.perms = { inbox: { download: true, modify: false } };
    const selectionPermissions = computed.selectionPermissions.call({ selectedResults: results });
    expect(selectionPermissions).toEqual({ download: true, modify: false });
    const self = { selectedResults: results, selectionPermissions };
    expect(computed.canMoveSelection.call(self)).toBe(false);
    expect(computed.canDownloadSelection.call(self)).toBe(true);
  });

  it("hides download and copy when a source doesn't allow downloading", () => {
    store.perms = { data: { download: false, modify: true } };
    const selectionPermissions = computed.selectionPermissions.call({ selectedResults: results });
    const self = { selectedResults: results, selectionPermissions };
    expect(computed.canDownloadSelection.call(self)).toBe(false);
    expect(computed.canCopySelection.call(self)).toBe(false);
  });

  it("hides copy when no source accepts the copy", () => {
    store.creatable = [];
    const self = { selectedResults: results, selectionPermissions: { download: true, modify: true } };
    expect(computed.canCopySelection.call(self)).toBe(false);
  });

  it("offers nothing when nothing is selected", () => {
    const self = { selectedResults: [], selectionPermissions: { download: true, modify: true } };
    expect(computed.canDownloadSelection.call(self)).toBe(false);
    expect(computed.canMoveSelection.call(self)).toBe(false);
    expect(computed.canCopySelection.call(self)).toBe(false);
  });

  it("downloads the selected results", () => {
    methods.downloadSelected.call({ selectedResults: results });
    expect(downloadFiles).toHaveBeenCalledWith(results);
  });

  it("opens the copy and move prompts with the selected results", () => {
    methods.copySelected.call({ selectedResults: results });
    expect(mutations.showPrompt).toHaveBeenLastCalledWith({ name: "copy", props: { items: results, operation: "copy" } });
    methods.moveSelected.call({ selectedResults: results });
    expect(mutations.showPrompt).toHaveBeenLastCalledWith({ name: "move", props: { items: results, operation: "move" } });
  });
});
