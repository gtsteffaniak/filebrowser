import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { type App, createApp, defineComponent, h } from "vue";
import { usePreviewNavigation } from "../usePreviewNavigation.ts";

const { stateMock, gettersMock, mutationsMock, routerMock, navigatePlaybackQueue } = vi.hoisted(() => ({
  stateMock: {
    req: {} as Record<string, unknown>,
    route: { path: "/files/srv/music/song.mp3" },
    navigation: { previousLink: "", nextLink: "" },
    previousHistoryItem: null as null | Record<string, unknown>,
    shareInfo: {},
  },
  gettersMock: {
    isPreviewPlaybackQueueNavMode: vi.fn(() => false),
    playbackQueueCanGoPrevious: vi.fn(() => true),
    playbackQueueCanGoNext: vi.fn(() => true),
    currentPromptName: vi.fn(() => ""),
    isShare: vi.fn(() => false),
    fileViewingDisabled: vi.fn(() => false),
  },
  mutationsMock: {
    setNavigationTransitioning: vi.fn(),
    clearNavigation: vi.fn(),
    closeHovers: vi.fn(),
    setPreviousHistoryItem: vi.fn(),
    replaceRequest: vi.fn(),
    showPrompt: vi.fn(),
  },
  routerMock: { push: vi.fn(), replace: vi.fn() },
  navigatePlaybackQueue: vi.fn(),
}));

vi.mock("@/store/index.ts", () => ({
  state: stateMock,
  getters: gettersMock,
  mutations: mutationsMock,
}));
vi.mock("@/router/index.ts", () => ({ router: routerMock }));
vi.mock("@/utils/playbackQueue.js", () => ({ navigatePlaybackQueue }));
vi.mock("@/api/index.js", () => ({ resourcesApi: { getPreviewURL: vi.fn(() => "preview-url") } }));
vi.mock("@/api/viewToken.js", () => ({
  ensureViewToken: vi.fn(),
  requestViewIdentity: vi.fn(),
}));
vi.mock("@/utils/url.js", () => ({
  goToItem: vi.fn(),
  removeLastDir: (path: string) => path.slice(0, path.lastIndexOf("/")),
}));

let app: App | null = null;

function mountComposable(options?: Parameters<typeof usePreviewNavigation>[0]) {
  let api!: ReturnType<typeof usePreviewNavigation>;
  app = createApp(
    defineComponent({
      setup() {
        api = usePreviewNavigation(options);
        return () => h("div");
      },
    }),
  );
  app.mount(document.createElement("div"));
  return api;
}

function pressKey(init: KeyboardEventInit) {
  window.dispatchEvent(new KeyboardEvent("keydown", init));
}

describe("usePreviewNavigation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    stateMock.navigation = { previousLink: "/prev", nextLink: "/next" };
    stateMock.previousHistoryItem = null;
    stateMock.req = { source: "srv", path: "/music/song.mp3", name: "song.mp3" };
    gettersMock.isPreviewPlaybackQueueNavMode.mockReturnValue(false);
    gettersMock.playbackQueueCanGoPrevious.mockReturnValue(true);
    gettersMock.playbackQueueCanGoNext.mockReturnValue(true);
    gettersMock.currentPromptName.mockReturnValue("");
  });

  afterEach(() => {
    app?.unmount();
    app = null;
  });

  it("follows the folder neighbours outside queue mode", () => {
    const api = mountComposable();
    api.navigatePrevious();
    expect(mutationsMock.setNavigationTransitioning).toHaveBeenCalledWith(true);
    expect(routerMock.replace).toHaveBeenCalledWith({ path: "/prev" });
    api.navigateNext();
    expect(routerMock.replace).toHaveBeenLastCalledWith({ path: "/next" });
    expect(navigatePlaybackQueue).not.toHaveBeenCalled();
  });

  it("walks the playback queue in queue mode", () => {
    gettersMock.isPreviewPlaybackQueueNavMode.mockReturnValue(true);
    const api = mountComposable();
    api.navigatePrevious();
    expect(navigatePlaybackQueue).toHaveBeenLastCalledWith(-1);
    api.navigateNext();
    expect(navigatePlaybackQueue).toHaveBeenLastCalledWith(1);
    expect(routerMock.replace).not.toHaveBeenCalled();
  });

  it("does nothing at the ends of the playback queue", () => {
    gettersMock.isPreviewPlaybackQueueNavMode.mockReturnValue(true);
    gettersMock.playbackQueueCanGoPrevious.mockReturnValue(false);
    gettersMock.playbackQueueCanGoNext.mockReturnValue(false);
    const api = mountComposable();
    api.navigatePrevious();
    api.navigateNext();
    expect(navigatePlaybackQueue).not.toHaveBeenCalled();
  });

  it("closes to the parent folder on Escape and Alt+ArrowUp", () => {
    mountComposable();
    pressKey({ key: "Escape" });
    expect(mutationsMock.replaceRequest).toHaveBeenCalledWith({});
    expect(routerMock.push).toHaveBeenLastCalledWith({ path: "/files/srv/music/" });
    pressKey({ key: "ArrowUp", altKey: true });
    expect(routerMock.push).toHaveBeenCalledTimes(2);
  });

  it("opens the delete prompt on Delete and ignores keys while a prompt is open", () => {
    mountComposable();
    pressKey({ key: "Delete" });
    expect(mutationsMock.showPrompt).toHaveBeenCalledTimes(1);
    gettersMock.currentPromptName.mockReturnValue("delete");
    pressKey({ key: "Delete" });
    pressKey({ key: "Escape" });
    expect(mutationsMock.showPrompt).toHaveBeenCalledTimes(1);
    expect(routerMock.push).not.toHaveBeenCalled();
  });

  it("tears down the view first, then clears navigation, and stops listening", () => {
    const order: string[] = [];
    mutationsMock.clearNavigation.mockImplementation(() => {
      order.push("clearNavigation");
    });
    mountComposable({
      onBeforeTeardown: () => {
        order.push("teardown");
      },
    });
    app?.unmount();
    app = null;
    expect(order).toEqual(["teardown", "clearNavigation"]);
    pressKey({ key: "Escape" });
    expect(routerMock.push).not.toHaveBeenCalled();
  });
});
