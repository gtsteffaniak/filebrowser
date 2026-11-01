import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createApp, defineComponent, h, ref, type App } from "vue";

const { state, mutations, getters } = vi.hoisted(() => ({
  state: { selected: [] as number[], multiple: false },
  mutations: { resetSelected: vi.fn(), setSelected: vi.fn(), selectAllItems: vi.fn() },
  getters: { currentPromptName: () => "", isMobile: () => false },
}));
vi.mock("@/store", () => ({ state, mutations, getters }));
import { useMouseSelection } from "../useRectangleSelection.ts";

let active = true;
let hitTest: ((rect: { left: number; top: number; right: number; bottom: number }, listing: DOMRect) => number[] | null) | undefined;
const Host = defineComponent({
  setup() {
    const container = ref<HTMLElement | null>(null);
    const bounds = ref<HTMLElement | null>(null);
    useMouseSelection({ container, bounds, items: () => [], isActive: () => active, hitTest: (...args) => hitTest?.(...args) ?? null });
    return () =>
      h("div", { ref: container, class: "container" }, [
        h("div", { ref: bounds, class: "bounds" }, [
          h("div", { class: "listing-item", "data-index": "0" }),
          h("div", { class: "listing-item", "data-index": "1" }),
        ]),
      ]);
  },
});

const box = (left: number, top: number, size: number) =>
  ({ left, top, right: left + size, bottom: top + size, width: size, height: size, x: left, y: top }) as DOMRect;
const mouse = (target: EventTarget, type: string, x: number, y: number, init: MouseEventInit = {}) =>
  target.dispatchEvent(new MouseEvent(type, { bubbles: true, cancelable: true, button: 0, clientX: x, clientY: y, ...init }));

describe("useMouseSelection", () => {
  let host: HTMLElement;
  let app: App;
  let container: HTMLElement;
  let item0: HTMLElement;
  let item1: HTMLElement;

  beforeEach(() => {
    vi.clearAllMocks();
    state.selected = [];
    active = true;
    hitTest = undefined;
    host = document.createElement("div");
    document.body.appendChild(host);
    app = createApp(Host);
    app.mount(host);
    container = host.querySelector(".container") as HTMLElement;
    const items = host.querySelectorAll<HTMLElement>(".listing-item");
    item0 = items[0] as HTMLElement;
    item1 = items[1] as HTMLElement;
    vi.spyOn(host.querySelector(".bounds") as HTMLElement, "getBoundingClientRect").mockReturnValue(
      box(0, 0, 200),
    );
    vi.spyOn(item0, "getBoundingClientRect").mockReturnValue(box(10, 10, 50));
    vi.spyOn(item1, "getBoundingClientRect").mockReturnValue(box(100, 100, 50));
  });

  afterEach(() => {
    app.unmount();
    host.remove();
    vi.restoreAllMocks();
  });

  it("selects the items the dragged rectangle touches", () => {
    mouse(container, "mousedown", 0, 0);
    mouse(document, "mouseup", 70, 70);
    expect(mutations.resetSelected).toHaveBeenCalled();
    expect(mutations.setSelected).toHaveBeenLastCalledWith([0]);
  });

  it("adds to the selection when Ctrl is held", () => {
    state.selected = [1];
    mouse(container, "mousedown", 0, 0, { ctrlKey: true });
    mouse(document, "mouseup", 70, 70, { ctrlKey: true });
    expect(mutations.resetSelected).not.toHaveBeenCalled();
    expect(mutations.setSelected).toHaveBeenLastCalledWith([1, 0]);
  });

  it("does not start on an item", () => {
    mouse(item0, "mousedown", 20, 20);
    mouse(document, "mouseup", 70, 70);
    expect(mutations.resetSelected).not.toHaveBeenCalled();
    expect(mutations.setSelected).not.toHaveBeenCalled();
  });

  it("selects everything on a double click in empty space", () => {
    mouse(container, "dblclick", 5, 5);
    expect(mutations.selectAllItems).toHaveBeenCalledWith({ multiple: false });

    mutations.selectAllItems.mockClear();
    mouse(item0, "dblclick", 20, 20);
    mouse(container, "dblclick", 5, 5, { ctrlKey: true });
    expect(mutations.selectAllItems).not.toHaveBeenCalled();
  });

  it("clears the selection on a click outside the items", () => {
    mouse(document.body, "click", 5, 5);
    expect(mutations.resetSelected).toHaveBeenCalledOnce();

    mutations.resetSelected.mockClear();
    mouse(item0, "click", 20, 20);
    expect(mutations.resetSelected).not.toHaveBeenCalled();
  });

  it("does nothing while its view is not the active one", () => {
    active = false;
    mouse(container, "dblclick", 5, 5);
    mouse(document.body, "click", 5, 5);
    expect(mutations.selectAllItems).not.toHaveBeenCalled();
    expect(mutations.resetSelected).not.toHaveBeenCalled();
  });

  // Virtual scrolling: most items have no element, so the listing works out which ones are under the rectangle
  it("selects what the listing says is under the rectangle, even without elements", () => {
    hitTest = vi.fn(() => [40, 41, 900]);
    mouse(container, "mousedown", 0, 0);
    mouse(document, "mouseup", 70, 70);
    expect(hitTest).toHaveBeenLastCalledWith({ left: 0, top: 0, right: 70, bottom: 70 }, expect.anything());
    expect(mutations.setSelected).toHaveBeenLastCalledWith([40, 41, 900]);
  });

  it("adds those to the selection with Ctrl", () => {
    state.selected = [900, 5];
    hitTest = () => [40, 900];
    mouse(container, "mousedown", 0, 0, { ctrlKey: true });
    mouse(document, "mouseup", 70, 70, { ctrlKey: true });
    expect(mutations.setSelected).toHaveBeenLastCalledWith([900, 5, 40]);
  });

  it("measures the elements when the listing has no answer", () => {
    hitTest = () => null;
    mouse(container, "mousedown", 0, 0);
    mouse(document, "mouseup", 70, 70);
    expect(mutations.setSelected).toHaveBeenLastCalledWith([0]);
  });
});
