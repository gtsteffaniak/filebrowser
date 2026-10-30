import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createApp, defineComponent, h, nextTick, ref, type App } from "vue";

const { store, mutations } = vi.hoisted(() => ({
  store: { isSafari: true, promptName: "" },
  mutations: { showPrompt: vi.fn() },
}));
vi.mock("@/store", () => ({
  state: {
    get isSafari() {
      return store.isSafari;
    },
  },
  getters: { currentPromptName: () => store.promptName, isMobile: () => false },
  mutations,
}));
import { useListingContextMenu } from "../useListingContextMenu.ts";

const show = ref(true);
const Host = defineComponent({
  setup() {
    const el = ref<HTMLElement | null>(null);
    useListingContextMenu(el, { getProps: () => ({ createOnly: true }) });
    return () => (show.value ? h("div", { ref: el, class: "box" }) : null);
  },
});

const touch = (el: Element, type: string, x: number, y: number) => {
  const event = new Event(type, { bubbles: true, cancelable: true });
  Object.assign(event, { touches: [{ clientX: x, clientY: y }] });
  el.dispatchEvent(event);
};
const rightClick = (el: Element) => {
  const event = new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: 5, clientY: 6 });
  el.dispatchEvent(event);
  return event;
};
const menuProps = (extra: Record<string, unknown>) => ({
  name: "ContextMenu",
  props: { showCentered: false, createOnly: true, ...extra },
});

describe("useListingContextMenu", () => {
  let host: HTMLElement;
  let app: App;
  const mount = () => {
    app = createApp(Host);
    app.mount(host);
    return host.querySelector(".box") as HTMLElement;
  };

  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    store.isSafari = true;
    store.promptName = "";
    show.value = true;
    host = document.createElement("div");
    document.body.appendChild(host);
  });

  afterEach(() => {
    app.unmount();
    host.remove();
    vi.useRealTimers();
  });

  it("opens the menu on right click at the pointer, with the view's props", () => {
    const box = mount();
    const event = rightClick(box);
    expect(event.defaultPrevented).toBe(true);
    expect(mutations.showPrompt).toHaveBeenCalledWith(menuProps({ posX: 5, posY: 6 }));
  });

  it("does not open a second menu while one is open", () => {
    const box = mount();
    store.promptName = "ContextMenu";
    rightClick(box);
    expect(mutations.showPrompt).not.toHaveBeenCalled();
  });

  // iOS Safari never fires contextmenu on a long press, so it is emulated with a timer.
  it("opens the menu after a long press on Safari, at the touch position", () => {
    const box = mount();
    touch(box, "touchstart", 10, 20);
    vi.advanceTimersByTime(499);
    expect(mutations.showPrompt).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(mutations.showPrompt).toHaveBeenCalledWith(menuProps({ posX: 10, posY: 20 }));
  });

  it("cancels the long press on a swipe", () => {
    const box = mount();
    touch(box, "touchstart", 10, 20);
    touch(box, "touchmove", 40, 20);
    vi.advanceTimersByTime(600);
    expect(mutations.showPrompt).not.toHaveBeenCalled();
  });

  it("cancels the long press when the finger is lifted", () => {
    const box = mount();
    touch(box, "touchstart", 10, 20);
    touch(box, "touchend", 10, 20);
    vi.advanceTimersByTime(600);
    expect(mutations.showPrompt).not.toHaveBeenCalled();
  });

  it("does not emulate a long press outside Safari", () => {
    store.isSafari = false;
    const box = mount();
    touch(box, "touchstart", 10, 20);
    vi.advanceTimersByTime(600);
    expect(mutations.showPrompt).not.toHaveBeenCalled();
    rightClick(box);
    expect(mutations.showPrompt).toHaveBeenCalledOnce();
  });

  // The results of a search only exist after searching, so the element comes and goes.
  it("follows the element when it appears later and when it is removed", async () => {
    show.value = false;
    mount();
    expect(host.querySelector(".box")).toBeNull();

    show.value = true;
    await nextTick();
    const box = host.querySelector(".box") as HTMLElement;
    rightClick(box);
    expect(mutations.showPrompt).toHaveBeenCalledTimes(1);

    show.value = false;
    await nextTick();
    rightClick(box);
    expect(mutations.showPrompt).toHaveBeenCalledTimes(1);
  });
});
