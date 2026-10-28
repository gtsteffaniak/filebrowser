import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createApp } from "vue";

const { store, mutations } = vi.hoisted(() => ({
  store: {
    share: false,
    current: "inbox",
    permissions: {
      data: { view: true, download: true, modify: false, create: false, delete: false },
      inbox: { view: true, download: true, modify: true, create: true, delete: true },
    },
  },
  mutations: { showPrompt: vi.fn(), setSearch: vi.fn() },
}));
vi.mock("@/store", () => ({
  state: {
    selected: [0],
    isSafari: false,
    user: { gallerySize: 0 },
    listing: { listingScrollTop: 0 },
    shareInfo: { shareType: "" },
    route: { path: "/files" },
    get req() {
      return {
        path: "/",
        source: store.current,
        items: [{ name: "a.txt", path: "/a.txt", type: "file" }],
      };
    },
  },
  mutations,
  getters: {
    isShare: () => store.share,
    isLoading: () => false,
    reqItems: () => ({ pinned: [], dirs: [], files: [] }),
    viewMode: () => "normal",
    isCardView: () => true,
    isMobile: () => false,
    isStickySidebar: () => false,
    currentView: () => "listingView",
    currentPromptName: () => "",
    globalPermissions: () => ({}),
    sourcePermissions: (source) => store.permissions[source ?? store.current],
  },
}));
vi.mock("vue-i18n", () => ({ useI18n: () => ({ t: (key) => key }) }));
vi.mock("@/utils/download", () => ({ default: vi.fn() }));
vi.mock("@/api", () => ({ resourcesApi: {} }));
vi.mock("@/router", () => ({ router: {} }));
vi.mock("@/utils/upload", () => ({ readAllDirectoryEntries: vi.fn(), checkConflict: vi.fn() }));
vi.mock("@/utils/throttle", () => ({ default: (fn) => fn }));
vi.mock("@/utils", () => ({ url: {} }));
vi.mock("@/components/files/ListingItem.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/prompts/Upload.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/LoadingSpinner.vue", () => ({ default: { render: () => null } }));
vi.mock("@/utils/listingTypeAhead.js", () => ({
  isTypeAheadSessionActive: vi.fn(),
  processTypeAheadKey: vi.fn(),
  resetTypeAheadSession: vi.fn(),
}));
vi.mock("@/utils/appNotifications", () => ({ notifyMoveCopyFailure: vi.fn() }));
import ListingView from "./ListingView.vue";

// Ctrl+C / Ctrl+X in the source the item is copied from, then Ctrl+V in the current one.
const copyOrCut = (key, fromSource) => {
  store.current = fromSource;
  const which = key === "x" ? 88 : 67;
  const event = new KeyboardEvent("keydown", { key, ctrlKey: true, bubbles: true });
  Object.defineProperty(event, "which", { value: which });
  document.body.dispatchEvent(event);
};
const paste = (key, fromSource, toSource = store.current) => {
  copyOrCut(key, fromSource);
  store.current = toSource;
  window.dispatchEvent(new Event("paste", { cancelable: true }));
};

describe("ListingView internal paste permissions", () => {
  let host;
  let app;

  beforeEach(() => {
    vi.clearAllMocks();
    store.share = false;
    store.current = "inbox";
    vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
    host = document.createElement("div");
    document.body.appendChild(host);
    app = createApp(ListingView);
    app.config.globalProperties.$t = (key) => key;
    app.mount(host);
  });

  afterEach(() => {
    app.unmount();
    host.remove();
    vi.unstubAllGlobals();
  });

  it("pastes a copy out of a read-only source into a writable one", () => {
    paste("c", "data", "inbox");
    expect(mutations.showPrompt).toHaveBeenCalledOnce();
  });

  it("refuses a cut out of a source that cannot modify", () => {
    paste("x", "data", "inbox");
    expect(mutations.showPrompt).not.toHaveBeenCalled();
  });

  it("refuses any paste into a source that cannot create", () => {
    paste("c", "inbox", "data");
    expect(mutations.showPrompt).not.toHaveBeenCalled();
  });

  it("in a share, requires modify even for a copy", () => {
    store.share = true;
    store.permissions.share = { download: true, modify: false, create: true };
    paste("c", "share", "share");
    expect(mutations.showPrompt).not.toHaveBeenCalled();
  });
});
