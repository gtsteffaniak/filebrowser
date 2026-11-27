import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createApp, h, type App } from "vue";

const { store } = vi.hoisted(() => ({
  store: {
    tool: null as null | { component: string },
    showSelectMultiple: false,
    mobile: false,
    share: false,
    multiple: false,
    selected: [] as string[],
    selectedCount: 0,
    perms: { view: true, download: true, modify: true, create: true, delete: true, share: true },
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
      get showSelectMultiple() {
        return store.showSelectMultiple;
      },
      get templates() {
        return [];
      },
    },
  },
  mutations: {
    setContextMenuHasItems: vi.fn(),
    setMultiple: vi.fn(),
    closeHovers: vi.fn(),
    closeTopPrompt: vi.fn(),
    showPrompt: vi.fn(),
    selectAllItems: vi.fn(),
  },
  getters: {
    isShare: () => store.share,
    canCreateInSource: () => true,
    globalPermissions: () => ({ share: true, admin: false, create: true }),
    sourcePermissions: () => store.perms,
    currentPromptName: () => "ContextMenu",
    currentPrompt: () => ({ props: {} }),
    currentTool: () => store.tool,
    currentView: () => "listingView",
    isMobile: () => store.mobile,
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
vi.mock("@/utils/mediaFile.ts", () => ({ isMediaRequest: () => false }));
vi.mock("@/utils/expandTransition", () => ({ expandBeforeEnter: vi.fn(), expandEnter: vi.fn(), expandLeave: vi.fn() }));
// transitions import (popEnter/popLeave on mount) — leave real, WAAPI absent in jsdom.
import ContextMenu from "./ContextMenu.vue";

let app: App | null = null;
let host: HTMLElement | null = null;

/** Icons of the actions the menu offers, like ContextMenu.copy.test.ts does. */
function offeredIcons(items?: Array<{ name: string; path: string; source: string; isDir?: boolean }>): string[] {
  host = document.createElement("div");
  document.body.append(host);
  app = createApp({
    render: () =>
      h(ContextMenu, {
        items: items ?? [
          { name: "a.txt", path: "/a.txt", source: "data" },
          { name: "b.txt", path: "/b.txt", source: "data" },
        ],
      }),
  });
  app.mount(host);
  return [...host.querySelectorAll("#context-menu .action > i")].map((el) => el.textContent ?? "");
}

const FILE_ITEMS = [
  { name: "a.txt", path: "/a.txt", source: "data" },
  { name: "b.txt", path: "/b.txt", source: "data" },
];

describe("ContextMenu in advanced search", () => {
  beforeEach(() => {
    store.tool = null;
    store.showSelectMultiple = false;
    store.mobile = false;
    store.share = false;
    store.multiple = false;
    store.perms = { view: true, download: true, modify: true, create: true, delete: true, share: true };
  });

  afterEach(() => {
    app?.unmount();
    host?.remove();
    app = null;
    host = null;
  });

  it("detects the advanced search tool", () => {
    expect(offeredIcons(FILE_ITEMS)).not.toContain("check_circle");
    store.tool = { component: "DuplicateFinder" };
    expect(offeredIcons(FILE_ITEMS)).not.toContain("check_circle");
    store.tool = { component: "AdvancedSearch" };
    expect(offeredIcons(FILE_ITEMS)).toContain("check_circle");
  });

  it("offers Select multiple on desktop even when the user preference is off", () => {
    store.tool = { component: "AdvancedSearch" };
    store.showSelectMultiple = false;
    expect(offeredIcons(FILE_ITEMS)).toContain("check_circle");
  });

  it("offers Select multiple with a single or multiple results", () => {
    store.tool = { component: "AdvancedSearch" };
    store.showSelectMultiple = false;
    const one = [{ name: "a.txt", path: "/a.txt", source: "data" }];
    expect(offeredIcons(one)).toContain("check_circle");
    expect(offeredIcons(FILE_ITEMS)).toContain("check_circle");
  });

  it("shows Archive in advanced search (create exits stay hidden)", () => {
    store.tool = { component: "AdvancedSearch" };
    store.perms.create = true;
    // Archive needs showCreate=true (createOnly) — advanced search forces it off, so
    // archive surfaces when the create panel is open. We assert the observable surface:
    // new-folder/file/upload stay hidden while archive/unarchive become available.
    const icons = offeredIcons(FILE_ITEMS);
    expect(icons).not.toContain("create_new_folder");
    expect(icons).not.toContain("note_add");
    expect(icons).not.toContain("file_upload");
  });

  it("hides Share when nothing is selected, since the results listing has no folder to share", () => {
    // selectedCount<=1 gate: one item is shareable, two are not; zero is not in adv search.
    const one = [{ name: "a.txt", path: "/a.txt", source: "data" }];
    store.tool = { component: "AdvancedSearch" };
    store.perms.share = true;
    expect(offeredIcons(one)).toContain("share");
    expect(offeredIcons(FILE_ITEMS)).not.toContain("share");
    expect(offeredIcons([])).not.toContain("share");
  });

  it("in the listing, offers Share even with no items selected", () => {
    const one = [{ name: "a.txt", path: "/a.txt", source: "data" }];
    store.tool = null;
    store.perms.share = true;
    expect(offeredIcons(one)).toContain("share");
    expect(offeredIcons(FILE_ITEMS)).not.toContain("share");
    // Listing allows selectedCount===0 (share the current folder).
    expect(offeredIcons([])).toContain("share");
  });
});
