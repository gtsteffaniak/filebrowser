import { createApp } from "vue";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { store } = vi.hoisted(() => ({
  store: {
    prompt: { props: {} },
    // "data" is the source being browsed (read-only), "inbox" a writable destination
    permissions: { data: { modify: false }, inbox: { modify: true } },
  },
}));
vi.mock("@/store", () => ({
  state: { shareInfo: null },
  getters: {
    currentPrompt: () => store.prompt,
    isShare: () => false,
    sourcePermissions: (source) => store.permissions[source ?? "data"],
  },
}));
import ReplaceRename from "./ReplaceRename.vue";

let app, root;
function mount(props) {
  store.prompt = { props, confirm: vi.fn() };
  root = document.createElement("div");
  document.body.append(root);
  app = createApp(ReplaceRename);
  app.config.globalProperties.$t = (key) => key;
  app.mount(root);
  return root.querySelector('button[aria-label="general.replace"]');
}

describe("ReplaceRename replace permission", () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => { app?.unmount(); root?.remove(); });

  it("offers Replace when the destination allows modify, even if the browsed source does not", () => {
    expect(mount({ operation: "copy", destSource: "inbox" })).not.toBeNull();
  });

  it("checks the browsed source when no destination is given", () => {
    expect(mount({ operation: "copy" })).toBeNull();
  });
});
