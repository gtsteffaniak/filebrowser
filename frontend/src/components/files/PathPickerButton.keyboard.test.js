import { afterEach, describe, expect, it, vi } from "vitest";
import { createApp, h } from "vue";

vi.mock("@/store", () => ({ state: { sources: {} }, mutations: { showPrompt: vi.fn() } }));
vi.mock("@/store/eventBus", () => ({ eventBus: { on: vi.fn(), off: vi.fn() } }));
import PathPickerButton from "./PathPickerButton.vue";

describe("PathPickerButton keyboard access", () => {
  let app;
  afterEach(() => app?.unmount());

  it("can be focused and is exposed as a button, so its Enter and Space handlers can run", () => {
    const el = document.createElement("div");
    app = createApp({ render: () => h(PathPickerButton, { path: "/", source: "" }) });
    app.config.globalProperties.$t = (key) => key;
    app.mount(el);
    const picker = el.querySelector(".unified-path-picker");
    expect(picker.getAttribute("tabindex")).toBe("0");
    expect(picker.getAttribute("role")).toBe("button");
  });
});
