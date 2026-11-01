import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createApp, defineComponent, h, ref, type App } from "vue";
import type { fileListItemIndex } from "@/store/types.ts";

const { state, mutations, getters, router } = vi.hoisted(() => ({
  state: {
    selected: [] as number[],
    isSearchActive: false,
    route: { path: "/x" },
    shareInfo: { hash: "" },
    req: { path: "/a/b", source: "data", items: [] as unknown[] },
  },
  mutations: {
    resetSelected: vi.fn(),
    addSelected: vi.fn(),
    selectAllItems: vi.fn(),
    showPrompt: vi.fn(),
  },
  getters: {
    currentPromptName: () => "",
    isCardView: () => false,
    isShare: () => false,
    getFirstSelected: vi.fn(),
  },
  router: { push: vi.fn() },
}));
vi.mock("@/store", () => ({ state, mutations, getters }));
vi.mock("@/router", () => ({ router }));
vi.mock("@/utils", () => ({
  url: { removeLastDir: () => "/a", buildItemUrl: (source: string, path: string) => `/files/${source}${path}` },
}));
vi.mock("@/utils/download", () => ({ default: vi.fn() }));
vi.mock("@/api", () => ({ resourcesApi: { getPreviewURL: vi.fn() } }));
vi.mock("@/utils/listingTypeAhead.js", () => ({
  isTypeAheadSessionActive: () => false,
  processTypeAheadKey: () => ({ matches: [], nextPos: 0 }),
  resetTypeAheadSession: vi.fn(),
}));
vi.mock("@/composables/useListingClipboard", () => ({
  useListingClipboard: () => ({ copyCut: vi.fn() }),
}));
import { useListingKeyboard } from "../useListingKeyboard.ts";

const items = [
  { index: 0, name: "a", path: "/a", type: "file", source: "data" },
  { index: 1, name: "b", path: "/b", type: "file", source: "data" },
] as fileListItemIndex[];

let options: Partial<Parameters<typeof useListingKeyboard>[0]> = {};
const Host = defineComponent({
  setup() {
    const container = ref<HTMLElement | null>(null);
    useListingKeyboard({
      container,
      items: () => items,
      isActive: () => true,
      canModify: () => true,
      ...options,
    });
    return () => h("div", { ref: container });
  },
});

const press = (key: string, target: EventTarget = document.body, init: KeyboardEventInit = {}) => {
  const event = new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true, ...init });
  target.dispatchEvent(event);
  return event;
};
const ctrl = (key: string, target: EventTarget) => {
  const event = new KeyboardEvent("keydown", { key, ctrlKey: true, bubbles: true, cancelable: true });
  Object.defineProperty(event, "which", { value: key.toUpperCase().charCodeAt(0) });
  target.dispatchEvent(event);
};

describe("useListingKeyboard", () => {
  let host: HTMLElement;
  let app: App;
  const mount = (opts: typeof options = {}) => {
    options = opts;
    app = createApp(Host);
    app.mount(host);
  };

  beforeEach(() => {
    vi.clearAllMocks();
    state.selected = [];
    state.req.items = [];
    host = document.createElement("div");
    document.body.appendChild(host);
  });

  afterEach(() => {
    app.unmount();
    host.remove();
  });

  it("selects the first item on an arrow key when nothing is selected", () => {
    mount();
    press("ArrowDown");
    expect(mutations.addSelected).toHaveBeenCalledWith(0);
  });

  it("moves the selection to the next item", () => {
    state.selected = [0];
    mount();
    press("ArrowDown");
    expect(mutations.addSelected).toHaveBeenCalledWith(1);
  });

  it("does nothing while its view is not the active one", () => {
    mount({ isActive: () => false });
    press("ArrowDown");
    expect(mutations.addSelected).not.toHaveBeenCalled();
  });

  // The advanced search page has text fields: keys typed in them must stay theirs.
  it("can ignore keys typed in text fields", () => {
    state.selected = [0];
    mount({ ignoreEditableTargets: true });
    const input = document.createElement("input");
    host.appendChild(input);
    press("ArrowDown", input);
    ctrl("a", input);
    expect(mutations.addSelected).not.toHaveBeenCalled();
    expect(mutations.selectAllItems).not.toHaveBeenCalled();

    press("ArrowDown");
    ctrl("a", document.body);
    expect(mutations.addSelected).toHaveBeenCalledWith(1);
    expect(mutations.selectAllItems).toHaveBeenCalledOnce();
  });

  it("handles keys typed in text fields by default", () => {
    state.selected = [0];
    mount();
    const input = document.createElement("input");
    host.appendChild(input);
    press("ArrowDown", input);
    expect(mutations.addSelected).toHaveBeenCalledWith(1);
  });

  it("goes to the parent folder on Backspace unless parent navigation is off", () => {
    mount();
    press("Backspace");
    expect(router.push).toHaveBeenCalledWith({ path: "/files/data/a" });

    app.unmount();
    vi.clearAllMocks();
    mount({ parentNavigation: false });
    const event = press("Backspace");
    expect(router.push).not.toHaveBeenCalled();
    expect(event.defaultPrevented).toBe(false);
  });

  it("opens the selected item on Enter", () => {
    state.selected = [0];
    getters.getFirstSelected.mockReturnValue({ source: "data", path: "/a/x" });
    mount();
    press("Enter");
    expect(router.push).toHaveBeenCalledWith({ path: "/files/data/a/x" });
  });

  it("lets the view consume Escape before clearing the selection", () => {
    mount({ onEscape: () => true });
    press("Escape");
    expect(mutations.resetSelected).not.toHaveBeenCalled();

    app.unmount();
    mount({ onEscape: () => false });
    press("Escape");
    expect(mutations.resetSelected).toHaveBeenCalledOnce();
  });

  it("asks for confirmation on Delete only when the selection can be modified", () => {
    state.selected = [0];
    state.req.items = [{ path: "/a", source: "data", type: "file", size: 1, modified: "", created: "", hasPreview: false }];
    mount({ canModify: () => false });
    press("Delete");
    expect(mutations.showPrompt).not.toHaveBeenCalled();

    app.unmount();
    mount({ canModify: () => true });
    press("Delete");
    expect(mutations.showPrompt).toHaveBeenCalledWith(expect.objectContaining({ name: "delete" }));
  });
});
