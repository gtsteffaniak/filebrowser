import { beforeEach, describe, expect, it, vi } from "vitest";

const { vars } = vi.hoisted(() => ({ vars: {} }));

vi.mock("@/api", () => ({ toolsApi: {} }));
vi.mock("@/notify/index.ts", () => ({ notify: {} }));
vi.mock("@/router", () => ({ router: {} }));
vi.mock("@/store", () => ({ state: {}, getters: {}, mutations: {} }));
vi.mock("@/store/eventBus", () => ({ eventBus: {} }));
vi.mock("@/utils/constants", () => ({ globalVars: vars }));
vi.mock("@/components/settings/ToggleSwitch.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/settings/SettingsItem.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/settings/ExpandDropdown.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/files/PathPickerButton.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/LoadingSpinner.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/files/ListingItem.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/files/ListingHeader.vue", () => ({ default: { render: () => null } }));
vi.mock("@/utils/url.js", () => ({
  removeTrailingSlash: (s) => String(s).replace(/\/+$/, ""),
  removeLeadingSlash: (s) => String(s).replace(/^\/+/, ""),
}));
vi.mock("@/utils/moment", () => ({ utcStartOfDaySecondsFromDateInput: () => null }));
vi.mock("@/utils/object.js", () => ({
  getObjectProperty: (obj, key) => (obj == null ? undefined : obj[key]),
  setObjectProperty: (obj, key, value) => ({ ...obj, [key]: value }),
}));

import AdvancedSearch from "./AdvancedSearch.vue";

const availability = () => AdvancedSearch.computed.contentSearchAvailable.call({});

// Minimal fake `this` for applyQueryFromRoute: empty source catalogue keeps the
// source/scope branches inert so the test isolates the content flag restore.
function routeRestoreContext(available) {
  return {
    $route: { query: { content: "1" } },
    sourceNameList: [],
    sourceEnabledFlags: {},
    sourceScopedPaths: {},
    termInputs: [""],
    termsJoinAnd: false,
    entryTypeFilter: "",
    selectedMediaTypes: [],
    largerThan: "",
    smallerThan: "",
    modifiedOlderThan: "",
    modifiedNewerThan: "",
    resultLimit: 500,
    maxSearchResults: 1000,
    useWildcardSearch: false,
    caseExactSearch: false,
    searchContent: false,
    contentSearchAvailable: available,
    parseTypeFiltersFromQuery: () => ({ entryTypeFilter: "", selectedMediaTypes: [] }),
    applyDefaultCurrentSourceIfNone: vi.fn(),
  };
}

describe("advanced search content-search availability", () => {
  beforeEach(() => {
    delete vars.contentSearchAvailable;
  });

  it("is available when the server enables full-text search", () => {
    vars.contentSearchAvailable = true;
    expect(availability()).toBe(true);
  });

  it("is unavailable when the server disables full-text search", () => {
    vars.contentSearchAvailable = false;
    expect(availability()).toBe(false);
  });

  it("stays available when an older backend omits the flag", () => {
    expect(availability()).toBe(true);
  });

  it("ignores content=1 from a shared URL when the server disables it", () => {
    const ctx = routeRestoreContext(false);
    AdvancedSearch.methods.applyQueryFromRoute.call(ctx);
    expect(ctx.searchContent).toBe(false);
  });

  it("restores content=1 from a shared URL when the server enables it", () => {
    const ctx = routeRestoreContext(true);
    AdvancedSearch.methods.applyQueryFromRoute.call(ctx);
    expect(ctx.searchContent).toBe(true);
  });
});
