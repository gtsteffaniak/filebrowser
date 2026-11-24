import { createApp, h, nextTick, type App } from "vue";
import { afterEach, describe, expect, it, vi } from "vitest";

const { fetchFiles, state } = vi.hoisted(() => ({
  fetchFiles: vi.fn(),
  state: { req: {}, prompts: [], selected: [], sources: { info: { data: {} } } },
}));

// The virtual composable needs measured DOM (jsdom has none), so it just forwards
// groups() as all-items — same seam as the real `useListingVirtualization`.
vi.mock("@/api", () => ({ resourcesApi: { fetchFiles } }));
vi.mock("@/i18n/index.ts", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
vi.mock("@/composables/useListingVirtualization.ts", () => ({
  findScrollParents: () => [],
  findScroller: () => null,
  useListingVirtualization: (options: { groups: () => Array<{ key: string; count: number; at: (index: number) => unknown }> }) => ({
    active: { value: false },
    group: (key: string) => {
      const group = options.groups().find((g) => g.key === key);
      if (!group) return { items: [], position: 0, style: undefined };
      const items: unknown[] = [];
      for (let i = 0; i < group.count; i++) items.push(group.at(i));
      return { items, position: 0, style: undefined };
    },
    scrollToIndex: vi.fn(),
  }),
}));
vi.mock("@/store", () => ({
  state,
  mutations: {
    resetSelected: vi.fn(),
    setSelected: vi.fn(),
    setMultiple: vi.fn(),
    closeTopPrompt: vi.fn(),
    setNavigationTransitioning: vi.fn(),
  },
  getters: {
    isShare: () => false,
    isMobile: () => false,
    shareHash: () => "",
    currentPromptName: () => null,
    pickerSorting: () => ({ by: "name", asc: true }),
    navigationSorting: () => ({ by: "name", asc: true }),
    sorting: () => ({ by: "name", asc: true }),
    searchNavigationItems: () => null,
  },
}));
vi.mock("@/utils", async () => {
  const { removeLastDir } = await import("@/utils/url.js");
  return { url: { removeLastDir } };
});
vi.mock("@/router/index.ts", () => ({ router: {} }));
vi.mock("@/utils/sort.ts", () => ({
  sortedItems: (entries: unknown[]) => entries,
}));
vi.mock("@/components/files/ListingHeader.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/files/ListingItem.vue", () => ({
  default: {
    props: ["name"],
    render(props: { name: string }) {
      return h("button", {}, props.name);
    },
  },
}));
vi.mock("@/components/LoadingSpinner.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/settings/ExpandDropdown.vue", () => ({ default: { render: () => null } }));

import FileList from "./FileList.vue";

let app: App | null = null;
let root: HTMLElement | null = null;
const selection = vi.fn();

function mount(items: unknown[] = [], path = "/child/") {
  state.req = { source: "data", path, items };
  root = document.createElement("div");
  document.body.append(root);
  app = createApp(FileList as never, { hidePathChrome: true, "onUpdate:selected": selection });
  app.mount(root);
}

function row(name: string): HTMLElement | undefined {
  return Array.from(root?.querySelectorAll("button") ?? []).find(
    (button) => button.textContent === name,
  ) as HTMLElement | undefined;
}

/** Assert-and-return: every spec clicks a row that must exist after mount. */
function mustRow(name: string): HTMLElement {
  const found = row(name);
  expect(found).toBeDefined();
  return found as HTMLElement;
}

afterEach(() => {
  app?.unmount();
  root?.remove();
  app = null;
  root = null;
  fetchFiles.mockReset();
  selection.mockReset();
});

describe("file picker parent navigation", () => {
  it("loads the parent and shows sibling folders after one click on '..'", async () => {
    mount();
    fetchFiles.mockResolvedValue({
      source: "data",
      items: [{ name: "sibling", path: "/sibling/", type: "directory" }],
    });
    await nextTick();
    mustRow("..").click();
    expect(selection).toHaveBeenLastCalledWith(expect.objectContaining({ path: "/", source: "data" }));
    expect(fetchFiles).toHaveBeenCalledTimes(1);
    expect(fetchFiles).toHaveBeenCalledWith("data", "/");
    await vi.waitFor(() => expect(row("sibling")).toBeDefined());
  });

  it("keeps single-click selection and double-click navigation for ordinary folders", async () => {
    mount([{ name: "nested", path: "/child/nested/", type: "directory" }]);
    fetchFiles.mockResolvedValue({ source: "data", items: [] });
    await nextTick();
    mustRow("nested").click();
    expect(fetchFiles).not.toHaveBeenCalled();
    mustRow("nested").dispatchEvent(new MouseEvent("dblclick", { bubbles: true }));
    expect(fetchFiles).toHaveBeenCalledTimes(1);
    expect(fetchFiles).toHaveBeenCalledWith("data", "/child/nested/");
  });

  it("does not fetch the parent again when the same row receives a double-click", async () => {
    mount();
    fetchFiles.mockResolvedValue({ source: "data", items: [] });
    await nextTick();
    const parent = mustRow("..");
    parent.click();
    parent.click();
    parent.dispatchEvent(new MouseEvent("dblclick", { bubbles: true }));
    expect(fetchFiles).toHaveBeenCalledTimes(1);
    expect(fetchFiles).toHaveBeenCalledWith("data", "/");
  });

  it("ignores the rest of a double-click on a replacement parent row", async () => {
    mount([], "/parent/child/");
    fetchFiles.mockResolvedValue({ source: "data", items: [] });
    await nextTick();
    const firstParent = mustRow("..");
    firstParent.dispatchEvent(new MouseEvent("click", { bubbles: true, detail: 1 }));
    // `withLoading` keeps `loading=true` for 200ms, so the listing container is
    // unmounted — the `..` row only returns once it clears, then Vue re-renders it.
    await vi.waitFor(() => {
      expect(row("..")).toBeDefined();
      expect(row("..")).not.toBe(firstParent);
    });

    // The second click of the double-click lands on the *new* `..` row whose path
    // is the parent itself, so it must not refetch the parent — the row still
    // carries `path="/parent/"`. The count stays at the single "/parent/" fetch.
    const nextParent = mustRow("..");
    nextParent.dispatchEvent(new MouseEvent("click", { bubbles: true, detail: 2 }));
    nextParent.dispatchEvent(new MouseEvent("dblclick", { bubbles: true, detail: 2 }));
    expect(fetchFiles.mock.calls).toEqual([["data", "/parent/"]]);

    // A fresh single click on the still-mounted `..` row navigates up again,
    // to root.
    nextParent.dispatchEvent(new MouseEvent("click", { bubbles: true, detail: 1 }));
    expect(fetchFiles.mock.calls).toEqual([
      ["data", "/parent/"],
      ["data", "/"],
    ]);
  });
});
