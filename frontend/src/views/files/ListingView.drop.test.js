import { afterEach, describe, expect, it, vi } from "vitest";
import { createApp, nextTick } from "vue";

const { store } = vi.hoisted(() => ({ store: { create: false } }));
vi.mock("@/store", () => ({
  state: {
    selected: [],
    isSafari: false,
    user: { gallerySize: 0 },
    listing: { listingScrollTop: 0 },
    shareInfo: { shareType: "" },
    req: { path: "/", source: "data", items: [] },
  },
  mutations: { setSearch: vi.fn() },
  getters: {
    isShare: () => false,
    isLoading: () => false,
    reqItems: () => ({ pinned: [], dirs: [], files: [] }),
    viewMode: () => "normal",
    isCardView: () => true,
    isMobile: () => false,
    isStickySidebar: () => false,
    globalPermissions: () => ({}),
    sourcePermissions: () => ({ create: store.create }),
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

const dropEvents = ["dragenter", "dragleave", "drop"];
const typesOn = (spy, el) =>
  spy.mock.calls.filter((_call, i) => spy.mock.contexts[i] === el).map(([type]) => type);

const fire = (el, type) => {
  const event = new Event(type, { bubbles: true, cancelable: true });
  Object.assign(event, { dataTransfer: { types: ["Files"] }, relatedTarget: null });
  el.dispatchEvent(event);
};

let mounted = null;
const mountListing = () => {
  const host = document.createElement("div");
  document.body.appendChild(host);
  const app = createApp(ListingView);
  app.config.globalProperties.$t = (key) => key;
  app.mount(host);
  mounted = { app, host };
  return { app, root: host.firstElementChild };
};

describe("ListingView drop listeners", () => {
  afterEach(() => {
    mounted?.app.unmount();
    mounted?.host.remove();
    mounted = null;
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  // The listing is not remounted when navigating between sources, so a user who lands on a source
  // they cannot write to must still get a drop target once they move to one they can.
  it("are attached and removed even when the user cannot write at mount", async () => {
    vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
    store.create = false;
    const add = vi.spyOn(EventTarget.prototype, "addEventListener");
    const remove = vi.spyOn(EventTarget.prototype, "removeEventListener");
    const { app, root } = mountListing();
    expect(typesOn(add, root)).toEqual(expect.arrayContaining(dropEvents));

    store.create = true;
    fire(root, "dragenter");
    await nextTick();
    expect(root.querySelector(".dropping")).not.toBeNull();

    app.unmount();
    expect(typesOn(remove, root)).toEqual(expect.arrayContaining(dropEvents));
  });

  // dragEnter shows the drop indicator; if the source is found read-only before the drop, the
  // refused drop must still take the indicator down (browsers send no dragleave on drop).
  it("clear the drop indicator when the drop is refused", async () => {
    vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
    store.create = true;
    const { root } = mountListing();
    fire(root, "dragenter");
    await nextTick();
    expect(root.querySelector(".dropping")).not.toBeNull();

    store.create = false;
    fire(root, "drop");
    await nextTick();

    expect(root.querySelector(".dropping")).toBeNull();
  });
});
