import { beforeEach, describe, expect, it, vi } from "vitest";

const { vars, searchMock, pushMock } = vi.hoisted(() => ({
  vars: { minSearchLength: 1 },
  searchMock: vi.fn(),
  pushMock: vi.fn(() => ({ catch: () => {} })),
}));

vi.mock("@/api", () => ({ resourcesApi: {}, toolsApi: { search: searchMock } }));
vi.mock("@/components/HelpTooltipIcon.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/files/Icon.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/LoadingSpinner.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/settings/SettingsItem.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/settings/ExpandDropdown.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/settings/ToggleSwitch.vue", () => ({ default: { render: () => null } }));
vi.mock("@/router", () => ({ router: { push: pushMock } }));
vi.mock("@/store", () => ({
  getters: {},
  mutations: { setSearch: vi.fn() },
  state: { sources: { current: "disk", info: {} }, isSearchActive: false },
}));
vi.mock("@/utils/", () => ({
  url: {
    removeTrailingSlash: (s) => String(s).replace(/\/+$/, ""),
    removeLeadingSlash: (s) => String(s).replace(/^\/+/, ""),
    extractSourceFromPath: () => ({ source: "", path: "/" }),
  },
}));
vi.mock("@/utils/constants", () => ({ globalVars: vars }));
vi.mock("@/utils/filesizes", () => ({ getHumanReadableFilesize: vi.fn() }));
vi.mock("@/utils/listingTypeAhead.js", () => ({ isTypeAheadSessionActive: () => false }));
vi.mock("@/utils/moment", () => ({ utcStartOfDaySecondsFromDateInput: () => null }));
vi.mock("@/utils/tooltipHelp.js", () => ({ shouldIgnoreOutsideTap: () => false }));

import Search from "./Search.vue";

const availability = () => Search.computed.contentSearchAvailable.call({});
const initialData = () => Search.data.call({ $t: (key) => key });

function submitContext(overrides = {}) {
  return {
    results: [],
    value: "budget",
    searchTypes: "",
    largerThan: "",
    smallerThan: "",
    modifiedOlderThan: "",
    modifiedNewerThan: "",
    useWildcardSearch: false,
    caseExactSearch: false,
    searchContent: true,
    contentSearchAvailable: true,
    ongoing: 0,
    selectedSource: "",
    sourceInfo: { disk: {} },
    getContext: "/",
    noneMessage: "",
    $t: (key) => key,
    ...overrides,
  };
}

function advancedJumpContext(overrides = {}) {
  return {
    selectedSource: "",
    sourceInfo: { disk: {} },
    value: "budget",
    searchTypes: "",
    largerThan: "",
    smallerThan: "",
    modifiedOlderThan: "",
    modifiedNewerThan: "",
    useWildcardSearch: false,
    caseExactSearch: false,
    searchContent: true,
    foldersOnly: false,
    getContext: "/",
    ...overrides,
  };
}

describe("quick search content-search availability", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchMock.mockResolvedValue([]);
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
});

describe("quick search defaults to content search", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchMock.mockResolvedValue([]);
    delete vars.contentSearchAvailable;
  });

  it("starts enabled when the server supports it", () => {
    vars.contentSearchAvailable = true;
    expect(initialData().searchContent).toBe(true);
  });

  it("starts enabled when an older backend omits the flag", () => {
    expect(initialData().searchContent).toBe(true);
  });

  it("starts disabled when the server has it off", () => {
    vars.contentSearchAvailable = false;
    expect(initialData().searchContent).toBe(false);
  });

  it("sends searchContent=true on a normal quick search", async () => {
    const ctx = submitContext();
    await Search.methods.submit.call(ctx);
    expect(searchMock).toHaveBeenCalledTimes(1);
    expect(searchMock.mock.calls[0][4]).toMatchObject({ searchContent: true });
  });

  it("omits searchContent when the user turns the toggle off", async () => {
    const ctx = submitContext({ searchContent: false });
    await Search.methods.submit.call(ctx);
    expect(searchMock.mock.calls[0][4]).not.toHaveProperty("searchContent");
  });

  it("never sends searchContent when the server lacks the capability", async () => {
    vars.contentSearchAvailable = false;
    const ctx = submitContext({ searchContent: true, contentSearchAvailable: false });
    await Search.methods.submit.call(ctx);
    expect(searchMock.mock.calls[0][4]).not.toHaveProperty("searchContent");
  });
});

describe("quick search wildcard/content mutual exclusion", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchMock.mockResolvedValue([]);
  });

  it("turns content off when wildcard turns on, letting its watcher re-search", () => {
    const ctx = { useWildcardSearch: true, searchContent: true, submit: vi.fn() };
    Search.watch.useWildcardSearch.call(ctx);
    expect(ctx.searchContent).toBe(false);
    expect(ctx.submit).not.toHaveBeenCalled();
  });

  it("submits directly when wildcard turns on with content already off", () => {
    const ctx = { useWildcardSearch: true, searchContent: false, submit: vi.fn() };
    Search.watch.useWildcardSearch.call(ctx);
    expect(ctx.submit).toHaveBeenCalledTimes(1);
  });

  it("keeps content on when wildcard turns back off", () => {
    const ctx = { useWildcardSearch: false, searchContent: true, submit: vi.fn() };
    Search.watch.useWildcardSearch.call(ctx);
    expect(ctx.searchContent).toBe(true);
    expect(ctx.submit).toHaveBeenCalledTimes(1);
  });

  it("re-runs the search when the content toggle changes", () => {
    const ctx = { submit: vi.fn() };
    Search.watch.searchContent.call(ctx);
    expect(ctx.submit).toHaveBeenCalledTimes(1);
  });
});

describe("quick search hands content state to advanced search", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("carries content=1 when content search is on", () => {
    Search.methods.openInAdvancedSearch.call(advancedJumpContext());
    expect(pushMock).toHaveBeenCalledTimes(1);
    expect(pushMock.mock.calls[0][0].query.content).toBe("1");
  });

  it("omits content when the toggle is off", () => {
    Search.methods.openInAdvancedSearch.call(advancedJumpContext({ searchContent: false }));
    expect(pushMock.mock.calls[0][0].query).not.toHaveProperty("content");
  });
});
