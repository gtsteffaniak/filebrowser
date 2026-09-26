import { afterEach, describe, expect, it, vi } from "vitest";

import {
  initViewportLayoutListener,
  isMobileLayout,
  isMobileLayoutWidth,
  resetViewportLayoutForTests,
} from "@/utils/viewport.js";

describe("isMobileLayoutWidth", () => {
  it("is true at the breakpoint", () => {
    expect(isMobileLayoutWidth(768)).toBe(true);
  });

  it("is false above the breakpoint", () => {
    expect(isMobileLayoutWidth(769)).toBe(false);
  });
});

describe("isMobileLayout", () => {
  afterEach(() => {
    resetViewportLayoutForTests();
    isMobileLayout.value = false;
  });

  it("initializes from matchMedia.matches", () => {
    window.matchMedia = vi.fn(() => ({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }));

    initViewportLayoutListener();
    expect(isMobileLayout.value).toBe(true);
    expect(window.matchMedia).toHaveBeenCalledWith("(max-width: 768px)");
  });

  it("updates only on matchMedia change (not window resize)", () => {
    const listeners = new Map();
    window.matchMedia = vi.fn(() => ({
      matches: false,
      addEventListener: (type, fn) => listeners.set(type, fn),
      removeEventListener: (type, fn) => {
        if (listeners.get(type) === fn) {
          listeners.delete(type);
        }
      },
    }));

    initViewportLayoutListener();
    expect(isMobileLayout.value).toBe(false);

    window.dispatchEvent(new Event("resize"));
    expect(isMobileLayout.value).toBe(false);

    listeners.get("change")?.({ matches: true });
    expect(isMobileLayout.value).toBe(true);
  });
});
