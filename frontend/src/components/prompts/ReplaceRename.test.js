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
  mutations: { closeTopPrompt: vi.fn() },
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

describe("ReplaceRename action order", () => {
  let app, root;
  const mountPrompt = (props) => {
    store.prompt = { props, confirm: vi.fn(), cancel: vi.fn() };
    root = document.createElement("div");
    document.body.append(root);
    app = createApp(ReplaceRename);
    app.config.globalProperties.$t = (key) => key;
    app.mount(root);
    return root;
  };

  afterEach(() => {
    app?.unmount();
    root?.remove();
  });

  it("keeps Replace as the last action, because Enter presses the last button", () => {
    const el = mountPrompt({ allowSkip: true, allowRename: true, destSource: "inbox" });
    const buttons = [...el.querySelectorAll(".card-actions button")];
    expect(buttons[0].getAttribute("aria-label")).toBe("general.cancel");
    expect(buttons.at(-1).getAttribute("aria-label")).toBe("general.replace");
  });

  it("cancels without handing an option to the confirm handler of the caller", () => {
    const el = mountPrompt({ allowSkip: true, allowRename: true });
    el.querySelector('button[aria-label="general.cancel"]').click();
    expect(store.prompt.confirm).not.toHaveBeenCalled();
    expect(store.prompt.cancel).toHaveBeenCalledTimes(1);
  });
});
