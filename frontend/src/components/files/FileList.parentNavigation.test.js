import { createApp, h, nextTick } from "vue";
import { afterEach, describe, expect, it, vi } from "vitest";

const { fetchFiles, state } = vi.hoisted(() => ({
  fetchFiles: vi.fn(),
  state: { req: {}, prompts: [], selected: [], sources: { info: { data: {} } } },
}));
vi.mock("@/api", () => ({ resourcesApi: { fetchFiles } }));
vi.mock("@/store", () => ({
  state,
  mutations: {},
  getters: {
    isShare: () => false,
    isMobile: () => false,
    currentPromptName: () => null,
    pickerSorting: () => ({ by: "name", asc: true }),
  },
}));
vi.mock("@/utils", async () => {
  const { removeLastDir } = await import("@/utils/url.js");
  return { url: { removeLastDir } };
});
vi.mock("@/router/index.ts", () => ({ router: {} }));
vi.mock("@/components/files/ListingHeader.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/files/ListingItem.vue", () => ({
  default: {
    props: ["name"],
    render() {
      return h("button", {}, this.name);
    },
  },
}));

import FileList from "./FileList.vue";

let app, root;
const selection = vi.fn();
function mount(items = []) {
  state.req = { source: "data", path: "/child/", items };
  root = document.createElement("div");
  document.body.append(root);
  app = createApp(FileList, { hidePathChrome: true, "onUpdate:selected": selection });
  app.config.globalProperties.$t = (key) => key;
  app.mount(root);
}

function row(name) {
  return Array.from(root.querySelectorAll("button")).find((button) => button.textContent === name);
}

afterEach(() => {
  app?.unmount();
  root?.remove();
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
    row("..").click();
    expect(selection).toHaveBeenLastCalledWith(expect.objectContaining({ path: "/", source: "data" }));
    expect(fetchFiles).toHaveBeenCalledExactlyOnceWith("data", "/");
    await vi.waitFor(() => expect(row("sibling")).toBeDefined());
  });

  it("keeps single-click selection and double-click navigation for ordinary folders", async () => {
    mount([{ name: "nested", path: "/child/nested/", type: "directory" }]);
    fetchFiles.mockResolvedValue({ source: "data", items: [] });
    await nextTick();
    row("nested").click();
    expect(fetchFiles).not.toHaveBeenCalled();
    row("nested").dispatchEvent(new MouseEvent("dblclick", { bubbles: true }));
    expect(fetchFiles).toHaveBeenCalledExactlyOnceWith("data", "/child/nested/");
  });

  it("does not fetch the parent again when the same row receives a double-click", async () => {
    mount();
    fetchFiles.mockResolvedValue({ source: "data", items: [] });
    await nextTick();
    const parent = row("..");
    parent.click();
    parent.click();
    parent.dispatchEvent(new MouseEvent("dblclick", { bubbles: true }));
    expect(fetchFiles).toHaveBeenCalledExactlyOnceWith("data", "/");
  });
});
