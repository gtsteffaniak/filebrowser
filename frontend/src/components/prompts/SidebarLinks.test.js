import { createApp, nextTick } from "vue";
import { afterEach, describe, expect, it, vi } from "vitest";

const { shareApiCreate, eventBusEmit, closeTopPrompt, showSuccessToast, showError, syncSidebarLinkDefaultsPolicy } =
  vi.hoisted(() => ({
    shareApiCreate: vi.fn(),
    eventBusEmit: vi.fn(),
    closeTopPrompt: vi.fn(),
    showSuccessToast: vi.fn(),
    showError: vi.fn(),
    syncSidebarLinkDefaultsPolicy: vi.fn(),
  }));

vi.mock("@/api", () => ({
  shareApi: {
    create: shareApiCreate,
  },
}));

vi.mock("@/store", () => ({
  mutations: {
    closeTopPrompt,
    syncSidebarLinkDefaultsPolicy,
    updateCurrentUser: vi.fn(),
  },
}));

vi.mock("@/store/eventBus", () => ({
  eventBus: {
    emit: eventBusEmit,
  },
}));

vi.mock("@/notify", () => ({
  notify: {
    showSuccessToast,
    showError,
  },
}));

vi.mock("@/components/sidebar/SidebarLinksEditor.vue", () => ({
  default: {
    name: "SidebarLinksEditor",
    props: ["embedded", "mode", "shareData", "showPromptActions"],
    emits: ["save", "cancel"],
    template: "<div />",
  },
}));

import SidebarLinks from "./SidebarLinks.vue";

let app;
let root;
let vm;

function mountSidebarLinks(props) {
  root = document.createElement("div");
  document.body.append(root);
  app = createApp(SidebarLinks, props);
  app.config.globalProperties.$t = (key) => key;
  vm = app.mount(root);
  return vm;
}

afterEach(() => {
  app?.unmount();
  app = undefined;
  vm = undefined;
  root?.remove();
  root = undefined;
  vi.clearAllMocks();
});

describe("SidebarLinks prompt", () => {
  it("updates Share Defaults via eventBus without calling shareApi.create", async () => {
    const links = [{ name: "Docs", category: "custom", target: "https://example.com" }];
    const instance = mountSidebarLinks({
      context: "share",
      shareData: {
        hash: "share-defaults",
        sidebarLinks: [],
      },
    });
    await nextTick();

    await instance.saveLinks({ links, showToolsInSidebar: false });

    expect(shareApiCreate).not.toHaveBeenCalled();
    expect(eventBusEmit).toHaveBeenCalledWith("shareSidebarLinksUpdated", {
      hash: "share-defaults",
      sidebarLinks: links,
    });
    expect(closeTopPrompt).toHaveBeenCalledTimes(1);
    expect(showError).not.toHaveBeenCalled();
  });

  it("updates unsaved new share via eventBus without calling shareApi.create", async () => {
    const links = [{ name: "Docs", category: "custom", target: "https://example.com" }];
    const instance = mountSidebarLinks({
      context: "share",
      shareData: {
        hash: "new",
        sidebarLinks: [],
      },
    });
    await nextTick();

    await instance.saveLinks({ links, showToolsInSidebar: false });

    expect(shareApiCreate).not.toHaveBeenCalled();
    expect(eventBusEmit).toHaveBeenCalledWith("shareSidebarLinksUpdated", {
      hash: "new",
      sidebarLinks: links,
    });
    expect(closeTopPrompt).toHaveBeenCalledTimes(1);
    expect(showError).not.toHaveBeenCalled();
  });

  it("persists sidebar links via shareApi.create for an existing share hash", async () => {
    shareApiCreate.mockResolvedValue({});
    const links = [{ name: "Docs", category: "custom", target: "https://example.com" }];
    const instance = mountSidebarLinks({
      context: "share",
      shareData: {
        hash: "abc123",
        sidebarLinks: [],
      },
    });
    await nextTick();

    await instance.saveLinks({ links, showToolsInSidebar: false });

    expect(shareApiCreate).toHaveBeenCalledWith({
      hash: "abc123",
      sidebarLinks: links,
    });
    expect(eventBusEmit).toHaveBeenCalledWith("shareSidebarLinksUpdated", {
      hash: "abc123",
      sidebarLinks: links,
    });
    expect(closeTopPrompt).toHaveBeenCalledTimes(1);
    expect(showError).not.toHaveBeenCalled();
  });
});
