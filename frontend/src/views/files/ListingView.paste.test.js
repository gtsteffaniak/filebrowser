import { beforeEach, describe, expect, it, vi } from "vitest";

const { store, mutations } = vi.hoisted(() => ({
  store: {
    share: false,
    current: "inbox",
    permissions: {
      data: { view: true, download: true, modify: false, create: false, delete: false },
      inbox: { view: true, download: true, modify: true, create: true, delete: true },
    },
  },
  mutations: { showPrompt: vi.fn() },
}));
vi.mock("@/store", () => ({
  state: { get req() { return { path: "/", source: store.current }; } },
  mutations,
  getters: {
    isShare: () => store.share,
    sourcePermissions: (source) => store.permissions[source ?? store.current],
  },
}));
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

const paste = (key, fromSource) =>
  ListingView.methods.handleInternalPaste.call({
    clipboard: { key, items: [{ from: "/a.txt", fromSource, name: "a.txt" }] },
  });

describe("ListingView internal paste permissions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    store.share = false;
    store.current = "inbox";
  });

  it("pastes a copy out of a read-only source into a writable one", async () => {
    await paste("c", "data");
    expect(mutations.showPrompt).toHaveBeenCalledOnce();
  });

  it("refuses a cut out of a source that cannot modify", async () => {
    await paste("x", "data");
    expect(mutations.showPrompt).not.toHaveBeenCalled();
  });

  it("refuses any paste into a source that cannot create", async () => {
    store.current = "data";
    await paste("c", "inbox");
    expect(mutations.showPrompt).not.toHaveBeenCalled();
  });

  it("in a share, requires modify even for a copy", async () => {
    store.share = true;
    store.permissions.share = { download: true, modify: false, create: true };
    store.current = "share";
    await paste("c", "share");
    expect(mutations.showPrompt).not.toHaveBeenCalled();
  });
});
