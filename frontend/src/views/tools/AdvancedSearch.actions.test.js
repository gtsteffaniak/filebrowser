import { beforeEach, describe, expect, it, vi } from "vitest";

const { router, mutations, store } = vi.hoisted(() => ({
  router: { go: vi.fn(), push: vi.fn(() => Promise.resolve()) },
  mutations: {
    resetSelected: vi.fn(),
    selectAllItems: vi.fn(),
    setMultiple: vi.fn(),
  },
  store: {
    multiple: false,
    selected: [],
  },
}));
vi.mock("@/router", () => ({ router }));
vi.mock("@/store", () => ({
  state: {
    get multiple() {
      return store.multiple;
    },
  },
  getters: { selectedCount: () => store.selected.length },
  mutations,
}));
vi.mock("@/store/eventBus", () => ({ eventBus: { on: vi.fn(), off: vi.fn() } }));
vi.mock("@/api", () => ({ toolsApi: {} }));
vi.mock("@/utils/constants", () => ({ globalVars: {} }));
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
