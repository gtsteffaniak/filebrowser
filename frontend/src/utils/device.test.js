import { afterEach, describe, expect, it, vi } from "vitest";

import { computeIsMobileLayout } from "@/utils/device.js";

function setViewport(width, height) {
  Object.defineProperty(window, "innerWidth", {
    value: width,
    configurable: true,
  });
  Object.defineProperty(window, "innerHeight", {
    value: height,
    configurable: true,
  });
}

describe("computeIsMobileLayout", () => {
  afterEach(() => {
    vi.stubGlobal("navigator", { maxTouchPoints: 0 });
    vi.stubGlobal("matchMedia", undefined);
    setViewport(1920, 1080);
  });

  it("treats portrait phone as mobile", () => {
    setViewport(390, 844);
    expect(computeIsMobileLayout()).toBe(true);
  });

  it("treats landscape phone as mobile (short side is width)", () => {
    setViewport(844, 390);
    expect(computeIsMobileLayout()).toBe(true);
  });

  it("treats desktop as non-mobile", () => {
    setViewport(1920, 1080);
    expect(computeIsMobileLayout()).toBe(false);
  });

  it("treats Playwright Desktop Firefox viewport as non-mobile", () => {
    setViewport(1280, 720);
    expect(computeIsMobileLayout()).toBe(false);
  });

  it("treats Playwright Desktop Firefox viewport as non-mobile with coarse pointer", () => {
    setViewport(1280, 720);
    vi.stubGlobal("navigator", { maxTouchPoints: 5 });
    vi.stubGlobal("matchMedia", (query) => ({
      matches: query.includes("coarse"),
      media: query,
    }));
    expect(computeIsMobileLayout()).toBe(false);
  });

  it("uses touch coarse pointer for landscape between 769 and 1024", () => {
    setViewport(1100, 900);
    vi.stubGlobal("navigator", { maxTouchPoints: 5 });
    vi.stubGlobal("matchMedia", (query) => ({
      matches: query.includes("coarse"),
      media: query,
    }));
    expect(computeIsMobileLayout()).toBe(true);
  });
});
