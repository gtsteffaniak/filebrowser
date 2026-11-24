import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { VirtualListingHandle } from "../virtualListing.ts";
import { base64Encode } from "@/utils/url.js";
import { registerVirtualListing } from "../virtualListing.ts";
import { selectAndScrollToIndex } from "../selectAndScroll.ts";

const { state, mutations } = vi.hoisted(() => ({
  state: { req: { items: [{ source: "data", path: "/a" }, { source: "data", path: "/b" }] } },
  mutations: { resetSelected: vi.fn(), addSelected: vi.fn() },
}));
vi.mock("@/store/index.ts", () => ({ state, mutations }));

const handle = (scrollToIndex: VirtualListingHandle["scrollToIndex"]): VirtualListingHandle => ({
  isActive: () => true,
  scrollToIndex,
  moveVertical: () => null,
  hitTest: () => null,
});

describe("selectAndScrollToIndex", () => {
  let unregister: (() => void) | null = null;

  beforeEach(() => {
    vi.clearAllMocks();
    Element.prototype.scrollIntoView = vi.fn();
  });
  afterEach(() => {
    unregister?.();
    unregister = null;
    document.body.innerHTML = "";
  });

  it("selects the item and centers its element", () => {
    const el = document.createElement("a");
    el.id = base64Encode(encodeURIComponent("data::/a"));
    document.body.appendChild(el);
    expect(selectAndScrollToIndex(0, true)).toBe(true);
    expect(mutations.resetSelected).toHaveBeenCalled();
    expect(mutations.addSelected).toHaveBeenCalledWith(0);
    expect(el.scrollIntoView).toHaveBeenCalledWith({ behavior: "instant", block: "center" });
  });

  it("reports an item that is not rendered as not found, so the caller tries again later", () => {
    expect(selectAndScrollToIndex(1)).toBe(false);
    expect(mutations.addSelected).toHaveBeenCalledWith(1);
  });

  it("lets a virtual listing bring in an item that is not rendered", () => {
    const scrollToIndex = vi.fn(() => true);
    unregister = registerVirtualListing(handle(scrollToIndex));
    expect(selectAndScrollToIndex(1, true)).toBe(true);
    expect(mutations.addSelected).toHaveBeenCalledWith(1);
    expect(scrollToIndex).toHaveBeenCalledWith(1, { behavior: "instant", block: "center" });
  });

  it("is still not found when the listing does not know the item", () => {
    unregister = registerVirtualListing(handle(() => false));
    expect(selectAndScrollToIndex(1)).toBe(false);
  });

  it("does nothing for an index that is not in the request", () => {
    expect(selectAndScrollToIndex(9)).toBe(false);
    expect(mutations.addSelected).not.toHaveBeenCalled();
  });
});
