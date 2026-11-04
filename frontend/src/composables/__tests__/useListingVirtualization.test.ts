import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createApp, defineComponent, h, nextTick, ref, shallowRef, type App } from "vue";
import type { fileListItemIndex } from "@/store/types.ts";

const { getters, state } = vi.hoisted(() => ({
  getters: { viewMode: () => "list" },
  state: { user: { gallerySize: 5 } },
}));
vi.mock("@/store/index.ts", () => ({ getters, state }));

import { useListingVirtualization } from "../useListingVirtualization.ts";

const HEADER = 100; // what is above the listing in the scrolled content
const SECTION = 30; // heading of every group
const PADDING = 8; // padding of a group, top and bottom
const GAP = 12; // between rows
const ITEM_HEIGHT = 40;
const ITEM_WIDTH = 100;
const PITCH = ITEM_HEIGHT + GAP;
const VIEWPORT = 600;

let columnsOnScreen = 1;
let scrollTop = 0;

const num = (value: string) => Number.parseFloat(value) || 0;
const rect = (left: number, top: number, width: number, height: number) =>
  ({ left, top, width, height, right: left + width, bottom: top + height, x: left, y: top, toJSON: () => ({}) }) as DOMRect;

function itemsOf(group: HTMLElement) {
  return group.querySelectorAll(":scope > .listing-item").length;
}
function boxHeight(group: HTMLElement) {
  const rows = Math.ceil(itemsOf(group) / columnsOnScreen);
  return PADDING * 2 + (rows > 0 ? rows * PITCH - GAP : 0);
}
/** Where the border box of a group starts, in the scrolled content */
function groupTop(target: HTMLElement) {
  let y = HEADER;
  for (const child of Array.from(target.parentElement?.children ?? []) as HTMLElement[]) {
    if (!child.dataset.virtualGroup) continue;
    y += SECTION + num(child.style.marginTop);
    if (child === target) return y;
    y += boxHeight(child) + num(child.style.marginBottom);
  }
  return y;
}

function installLayout() {
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(function (this: Element) {
    const el = this as HTMLElement;
    if (el.classList.contains("scroll-wrapper")) return rect(0, 0, 900, VIEWPORT);
    if (el.dataset.virtualGroup) return rect(10, groupTop(el) - scrollTop, 800, boxHeight(el));
    if (el.classList.contains("listing-item")) {
      const group = el.parentElement as HTMLElement;
      const at = Array.from(group.children).indexOf(el);
      const row = Math.floor(at / columnsOnScreen);
      const col = at % columnsOnScreen;
      return rect(
        10 + PADDING + col * (ITEM_WIDTH + 10),
        groupTop(group) - scrollTop + PADDING + row * PITCH,
        ITEM_WIDTH,
        ITEM_HEIGHT,
      );
    }
    if (el.classList.contains("listing-items")) return rect(10, HEADER - scrollTop, 800, 10000);
    return rect(0, 0, 0, 0);
  });
  Element.prototype.scrollIntoView = vi.fn();
}

function item(index: number): fileListItemIndex {
  return { index, name: `item-${index}`, path: `/item-${index}`, type: "file", source: "data" } as fileListItemIndex;
}
const range = (from: number, count: number) => Array.from({ length: count }, (_, i) => item(from + i));

const dirs = shallowRef<fileListItemIndex[]>([]);
const files = shallowRef<fileListItemIndex[]>([]);
let api: ReturnType<typeof useListingVirtualization>;
let scroller: HTMLElement;
let scrollerClass = "scroll-wrapper";
let minItems: number | undefined;

const Host = defineComponent({
  setup() {
    const container = ref<HTMLElement | null>(null);
    api = useListingVirtualization({
      container,
      groups: () => [
        { key: "dirs", items: dirs.value },
        { key: "files", items: files.value },
      ],
      columnsHint: () => columnsOnScreen,
      minItems,
      register: true,
    });
    const renderGroup = (key: "dirs" | "files") => {
      const visible = api.group(key);
      return h(
        "div",
        { class: key === "dirs" ? "folder-items" : "file-items", "data-virtual-group": key, style: { paddingTop: `${PADDING}px`, rowGap: `${GAP}px`, ...visible.style } },
        visible.items.map((entry) => h("a", { key: entry.index, class: "listing-item", "data-index": entry.index })),
      );
    };
    return () =>
      h(
        "div",
        {
          class: scrollerClass,
          ref: (el: unknown) => {
            if (!el) return;
            scroller = el as HTMLElement;
            Object.defineProperty(scroller, "scrollTop", {
              configurable: true,
              get: () => scrollTop,
              set: (value: number) => {
                scrollTop = value;
              },
            });
            Object.defineProperty(scroller, "clientHeight", { configurable: true, get: () => VIEWPORT });
          },
        },
        [h("div", { class: "listing-items", ref: container }, [renderGroup("dirs"), renderGroup("files")])],
      );
  },
});

async function flush() {
  for (let i = 0; i < 6; i++) {
    await nextTick();
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}

const rendered = () => Array.from(document.querySelectorAll<HTMLElement>(".listing-item")).map((el) => Number(el.dataset.index));
const present = (index: number) => !!document.querySelector(`.listing-item[data-index="${index}"]`);
const group = (key: string) => document.querySelector<HTMLElement>(`[data-virtual-group="${key}"]`) as HTMLElement;
const renderedIn = (key: string) =>
  Array.from(group(key).querySelectorAll<HTMLElement>(".listing-item")).map((el) => Number(el.dataset.index));

describe("useListingVirtualization", () => {
  let host: HTMLElement;
  let app: App;

  async function mount() {
    app = createApp(Host);
    app.mount(host);
    await flush();
  }

  async function scrollTo(top: number) {
    scrollTop = top;
    scroller.dispatchEvent(new Event("scroll"));
    await flush();
  }

  beforeEach(() => {
    vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => setTimeout(() => cb(0), 0));
    vi.stubGlobal("cancelAnimationFrame", (id: number) => clearTimeout(id));
    columnsOnScreen = 1;
    scrollTop = 0;
    scrollerClass = "scroll-wrapper";
    minItems = undefined;
    installLayout();
    host = document.createElement("div");
    document.body.appendChild(host);
    // 120 folders then 400 files, like req.items: indexes are positions in one list
    dirs.value = range(0, 120);
    files.value = range(120, 400);
  });

  afterEach(() => {
    app.unmount();
    host.remove();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("renders everything while the listing is short", async () => {
    dirs.value = range(0, 20);
    files.value = range(20, 10);
    await mount();
    expect(api.active.value).toBe(false);
    expect(rendered()).toHaveLength(30);
    expect(group("dirs").style.marginTop).toBe("");
  });

  it("starts windowing when a short listing grows past the threshold", async () => {
    dirs.value = range(0, 20);
    files.value = range(20, 10);
    await mount();
    expect(rendered()).toHaveLength(30);
    dirs.value = range(0, 120);
    files.value = range(120, 400);
    await flush();
    expect(api.active.value).toBe(true);
    expect(rendered().length).toBeLessThan(150);
    expect(present(519)).toBe(false);
  });

  it("uses the threshold it is given", async () => {
    minItems = 30;
    dirs.value = range(0, 20);
    files.value = range(20, 20);
    await mount();
    expect(api.active.value).toBe(true);
    expect(rendered().length).toBeLessThan(40);
  });

  it("renders a window of a long listing, with space for the rest", async () => {
    await mount();
    expect(api.active.value).toBe(true);
    expect(rendered().length).toBeLessThan(150);
    expect(rendered().length).toBeGreaterThan(10);
    // on screen at the top: the first folders
    expect(present(0)).toBe(true);
    expect(present(5)).toBe(true);
    expect(present(519)).toBe(false);
  });

  it("keeps the height of the full listing, so the scrollbar and the following groups do not move", async () => {
    await mount();
    // the files group starts where it would if all 120 folders were rendered
    const full = HEADER + SECTION + (PADDING * 2 + 120 * PITCH - GAP) + SECTION;
    expect(groupTop(group("files"))).toBe(full);
    // and its end is where 400 rows end
    const end = groupTop(group("files")) + boxHeight(group("files")) + Number.parseFloat(group("files").style.marginBottom || "0");
    expect(end).toBe(full + PADDING * 2 + 400 * PITCH - GAP);
  });

  it("moves the rendered rows with the scroll position", async () => {
    await mount();
    const filesTop = groupTop(group("files")); // far below the first screen
    await scrollTo(filesTop + 200 * PITCH);
    // row 200 of the files is at the top of the screen
    expect(present(120 + 200)).toBe(true);
    expect(present(120 + 205)).toBe(true);
    expect(present(0)).toBe(false);
    expect(present(120)).toBe(false);
    expect(rendered().length).toBeLessThan(150);
  });

  it("measures the columns of a grid and renders whole rows", async () => {
    columnsOnScreen = 4;
    await mount();
    expect(api.columns.value).toBe(4);
    await scrollTo(groupTop(group("files")) + 40 * PITCH);
    const inFiles = renderedIn("files");
    expect((inFiles[0] as number) - 120).toBe(((inFiles[0] as number) - 120) - (((inFiles[0] as number) - 120) % 4));
    expect(((inFiles[0] as number) - 120) % 4).toBe(0);
    expect(present(120 + 40 * 4)).toBe(true);
  });

  it("scrolls to an item that is not rendered and aligns it once it is", async () => {
    await mount();
    expect(present(450)).toBe(false);
    expect(api.handle.scrollToIndex(450, { block: "center" })).toBe(true);
    await flush();
    expect(present(450)).toBe(true);
    const at = (document.querySelector('.listing-item[data-index="450"]') as HTMLElement).getBoundingClientRect().top;
    expect(at).toBeGreaterThan(0);
    expect(at).toBeLessThan(VIEWPORT);
    // the exact alignment is left to the browser, as for items that were already there
    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith({ behavior: "instant", block: "center", inline: "nearest" });
  });

  it("jumps to an item right after mounting, before anything was measured", async () => {
    // Like going back from a preview: the listing is mounted and the caller scrolls on the very next tick
    app = createApp(Host);
    app.mount(host);
    await nextTick();
    expect(api.handle.scrollToIndex(519, { block: "center" })).toBe(true);
    await flush();
    expect(present(519)).toBe(true);
    expect(rendered().length).toBeLessThan(150);
  });

  it("uses the browser to scroll an item that is already rendered", async () => {
    await mount();
    scrollTop = 0;
    expect(api.handle.scrollToIndex(3, { block: "nearest", behavior: "smooth" })).toBe(true);
    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith({ behavior: "smooth", block: "nearest", inline: "nearest" });
  });

  it("does not know items that are not in the listing", async () => {
    await mount();
    expect(api.handle.scrollToIndex(9999)).toBe(false);
  });

  it("jumps correctly in a grid", async () => {
    columnsOnScreen = 5;
    await mount();
    expect(api.handle.scrollToIndex(120 + 399, { block: "center" })).toBe(true);
    await flush();
    expect(present(519)).toBe(true);
    expect(present(120)).toBe(false);
  });

  it("moves up and down the grid by rows, also where it is not rendered and across groups", async () => {
    columnsOnScreen = 4;
    await mount();
    // folders are 30 full rows: the last row is 116-119. Going down from 119 (col 3) enters the files at col 3
    expect(api.handle.moveVertical(119, 1)).toBe(123);
    expect(api.handle.moveVertical(123, -1)).toBe(119);
    // inside the files
    expect(api.handle.moveVertical(120 + 4, 1)).toBe(120 + 8);
    // a far away item that is not rendered
    expect(present(120 + 300)).toBe(false);
    expect(api.handle.moveVertical(120 + 300, 1)).toBe(120 + 304);
    // edges
    expect(api.handle.moveVertical(0, -1)).toBeNull();
    expect(api.handle.moveVertical(519, 1)).toBeNull();
    // 400 files is a whole number of rows of 4, with 3 columns the last row has a single item
    columnsOnScreen = 3;
    expect(api.handle.moveVertical(9999, 1)).toBeNull();
  });

  it("finds the items under a rectangle without them being rendered", async () => {
    columnsOnScreen = 4;
    await mount();
    const filesBox = group("files").getBoundingClientRect();
    const listing = rect(0, 0, 800, 10000);
    // the box of the third file row: far below the screen, nothing there is rendered
    const rowTop = filesBox.top + PADDING + 2 * PITCH;
    expect(present(120 + 8)).toBe(false);
    const hits = api.handle.hitTest(
      { left: 0, top: rowTop + 5, right: 2000, bottom: rowTop + 20 },
      listing,
    );
    expect(hits).toEqual([128, 129, 130, 131]);
    // not the gap between rows
    expect(
      api.handle.hitTest({ left: 0, top: rowTop + ITEM_HEIGHT + 2, right: 2000, bottom: rowTop + ITEM_HEIGHT + 8 }, listing),
    ).toEqual([]);
  });

  it("goes back to rendering everything when the listing gets short", async () => {
    await mount();
    expect(rendered().length).toBeLessThan(150);
    dirs.value = range(0, 20);
    files.value = range(20, 10);
    await flush();
    expect(api.active.value).toBe(false);
    expect(rendered()).toHaveLength(30);
    expect(group("files").style.marginBottom).toBe("");
  });

  it("is only reachable while the listing is mounted", async () => {
    await mount();
    const { getVirtualListing } = await import("@/utils/listing/virtualListing.ts");
    expect(getVirtualListing()).toBe(api.handle);
    app.unmount();
    expect(getVirtualListing()).toBeNull();
    app = createApp(Host);
    app.mount(host);
  });

  it("renders everything when it cannot follow a scroll container", async () => {
    scrollerClass = "not-the-scroller";
    await mount();
    expect(api.active.value).toBe(false);
    expect(rendered()).toHaveLength(520);
  });

  it("renders everything when nothing can be measured", async () => {
    vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(() => rect(0, 0, 0, 0));
    await mount();
    expect(api.active.value).toBe(false);
    expect(rendered()).toHaveLength(520);
  });

  it("renders the last items after a jump straight to the bottom", async () => {
    await mount();
    const files = group("files");
    const bottom = groupTop(files) + boxHeight(files) + Number.parseFloat(files.style.marginBottom || "0") + PADDING - VIEWPORT;
    await scrollTo(bottom);
    expect(present(519)).toBe(true);
    expect(present(510)).toBe(true); // the screen at the bottom
    expect(present(120)).toBe(false);
  });

  it("keeps a window of about three screens around a jump, and never the whole listing", async () => {
    await mount();
    const files = group("files");
    const bottom = groupTop(files) + boxHeight(files) + Number.parseFloat(files.style.marginBottom || "0") + PADDING - VIEWPORT;
    await scrollTo(bottom);
    expect(present(519)).toBe(true);
    expect(rendered().length).toBeLessThan(150);
  });
});
