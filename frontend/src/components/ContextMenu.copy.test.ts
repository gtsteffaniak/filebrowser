import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createApp, h, type App } from "vue";

const { store } = vi.hoisted(() => ({
  store: {
    share: false,
    creatable: [] as string[],
    scopes: [{ name: "data" }, { name: "inbox" }],
    permissions: { view: true, download: true, modify: false, create: false, delete: false },
  },
}));

vi.mock("@/store", () => ({
  state: {
    selected: [],
    multiple: false,
    isSearchActive: false,
    shareInfo: {},
    navigation: {},
    sources: { current: "data" },
    req: { items: [], source: "data", path: "/" },
    user: {
      get scopes() {
        return store.scopes;
      },
    },
  },
  mutations: {
    setContextMenuHasItems: vi.fn(),
    closeHovers: vi.fn(),
    closeTopPrompt: vi.fn(),
    showPrompt: vi.fn(),
  },
  getters: {
    isShare: () => store.share,
    canCreateInSource: (source: string) => store.creatable.includes(source),
    globalPermissions: () => ({}),
    sourcePermissions: () => store.permissions,
    currentPromptName: () => "ContextMenu",
    currentPrompt: () => ({ props: {} }),
    currentTool: () => null,
    currentView: () => "listingView",
    isMobile: () => false,
    isAdmin: () => false,
    isLoggedIn: () => true,
    isPreviewView: () => false,
    isSplitViewActive: () => false,
  },
}));
vi.mock("@/i18n/index.ts", () => ({
  default: { global: { t: (key: string) => key } },
  useI18n: () => ({ t: (key: string) => key }),
}));
vi.mock("vue-router", () => ({ useRouter: () => ({ push: vi.fn(), replace: vi.fn() }) }));
vi.mock("@/api", () => ({ resourcesApi: {}, shareApi: {}, usersApi: {} }));
vi.mock("@/notify", () => ({ notify: {} }));
vi.mock("@/utils/tooltipHelp.js", () => ({ hideInteractiveTooltip: vi.fn(), showHoverTooltip: vi.fn() }));
vi.mock("@/utils", () => ({ url: {} }));
vi.mock("@/utils/url.js", () => ({ goToItem: vi.fn(), getParentDir: vi.fn(), removeLastDir: vi.fn() }));
vi.mock("@/utils/clipboard", () => ({ copyToClipboard: vi.fn() }));
vi.mock("@/utils/download", () => ({ default: vi.fn() }));
vi.mock("@/utils/nativeShare", () => ({ canNativeShare: () => false, nativeShareFile: vi.fn() }));
vi.mock("@/utils/mimetype", () => ({
  isRichTextPreviewMimeType: () => false,
  getTypeInfoFromExt: () => ({ materialSymbol: "", classes: "" }),
}));
vi.mock("@/utils/mediaFile", () => ({ isMediaRequest: () => false }));
import ContextMenu from "./ContextMenu.vue";

let app: App | null = null;
let host: HTMLElement | null = null;

/** Mounts the menu on one selected item and returns the icons of the actions it offers. */
function offeredIcons(permissions: Partial<typeof store.permissions>): string[] {
  store.permissions = { view: true, download: false, modify: false, create: false, delete: false, ...permissions };
  host = document.createElement("div");
  document.body.append(host);
  app = createApp({
    render: () => h(ContextMenu, { items: [{ name: "a.txt", path: "/a.txt", source: "data" }] }),
  });
  app.mount(host);
  return [...host.querySelectorAll("#context-menu .action > i")].map((el) => el.textContent ?? "");
}

describe("ContextMenu Copy", () => {
  beforeEach(() => {
    store.share = false;
    store.creatable = ["inbox"];
  });

  afterEach(() => {
    app?.unmount();
    host?.remove();
    app = null;
    host = null;
  });

  it("is offered in a read-only source, since copying only reads it", () => {
    expect(offeredIcons({ download: true, modify: false })).toContain("file_copy");
  });

  it("is hidden when no source accepts the copy", () => {
    store.creatable = [];
    expect(offeredIcons({ download: true, modify: false })).not.toContain("file_copy");
  });

  it("is hidden without download", () => {
    expect(offeredIcons({ download: false, modify: true })).not.toContain("file_copy");
  });

  it("in a share, still requires modify, which the server checks for every change", () => {
    store.share = true;
    expect(offeredIcons({ download: true, modify: false })).not.toContain("file_copy");
    app?.unmount();
    host?.remove();
    expect(offeredIcons({ download: true, modify: true })).toContain("file_copy");
  });
});
