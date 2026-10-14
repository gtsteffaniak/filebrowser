import { describe, expect, it, vi } from "vitest";

const { store } = vi.hoisted(() => ({ store: { create: false } }));
vi.mock("@/store", () => ({
  state: { selected: [], isSafari: false },
  mutations: { setSearch: vi.fn() },
  getters: { isShare: () => false, sourcePermissions: () => ({ create: store.create }) },
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

const dropEvents = ["dragenter", "dragleave", "drop"];
const types = (spy) => spy.mock.calls.map(([type]) => type);

describe("ListingView drop listeners", () => {
  // The listing is not remounted when navigating between sources, so a user who lands on a source
  // they cannot write to must still get a drop target once they move to one they can.
  it("are attached and removed even when the user cannot write at mount", () => {
    const el = { addEventListener: vi.fn(), removeEventListener: vi.fn() };
    const ctx = { $el: el, permissions: { modify: false, create: false } };

    ListingView.mounted.call(ctx);
    expect(types(el.addEventListener)).toEqual(expect.arrayContaining(dropEvents));

    ListingView.beforeUnmount.call(ctx);
    expect(types(el.removeEventListener)).toEqual(expect.arrayContaining(dropEvents));
  });

  // dragEnter shows the drop indicator; if the source is found read-only before the drop, the
  // refused drop must still take the indicator down (browsers send no dragleave on drop).
  it("clear the drop indicator when the drop is refused", async () => {
    const el = {};
    const ctx = { $el: el, dragTargets: new Set([el]) };
    const event = { preventDefault: vi.fn(), dataTransfer: { types: ["Files"] } };
    store.create = false;

    await ListingView.methods.drop.call(ctx, event);

    expect(ctx.dragTargets.size).toBe(0);
  });
});
