import { beforeEach, describe, expect, it, vi } from "vitest";

const { store } = vi.hoisted(() => ({ store: { share: false, reqSource: "" } }));
vi.mock("@/store", () => ({
  state: {
    get req() {
      return { source: store.reqSource };
    },
  },
  mutations: {},
  getters: { isShare: () => store.share },
}));
vi.mock("@/store/eventBus", () => ({ eventBus: { on: vi.fn(), off: vi.fn() } }));
vi.mock("@/api", () => ({ resourcesApi: {} }));
vi.mock("@/utils/buttons", () => ({ default: {} }));
vi.mock("@/utils/upload", () => ({ checkConflict: vi.fn() }));
vi.mock("@/utils", () => ({ url: {} }));
vi.mock("@/notify", () => ({ notify: {} }));
vi.mock("@/utils/appNotifications", () => ({
  notifyMoveCopyComplete: vi.fn(),
  notifyOperationError: vi.fn(),
  notifyMoveCopyFailure: vi.fn(),
  extractMoveCopyErrorMessage: vi.fn(),
}));
vi.mock("@/utils/notificationActions", () => ({ goToItemNotificationButton: vi.fn() }));
vi.mock("@/components/LoadingSpinner.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/files/PathPickerButton.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/files/ListingFilter.vue", () => ({ default: { render: () => null } }));
vi.mock("../files/FileList.vue", () => ({ default: { render: () => null } }));
import MoveCopy from "./MoveCopy.vue";

const needsDestination = (destSource) =>
  MoveCopy.computed.needsDestination.call({
    destSource,
    isShareContext: MoveCopy.computed.isShareContext.call({}),
  });

describe("MoveCopy needs a destination", () => {
  beforeEach(() => {
    store.share = false;
    store.reqSource = "";
  });

  it("requires choosing a destination when the open request has no source (advanced search)", () => {
    expect(needsDestination(null)).toBe(true);
  });

  it("is satisfied once a destination source is chosen", () => {
    expect(needsDestination("data")).toBe(false);
  });

  it("does not require it in a normal listing, which has its own source", () => {
    store.reqSource = "data";
    expect(needsDestination(null)).toBe(false);
  });

  it("does not require it in a share", () => {
    store.share = true;
    expect(needsDestination(null)).toBe(false);
  });
});
