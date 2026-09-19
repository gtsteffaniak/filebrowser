import { afterEach, describe, expect, it, vi } from "vitest";

const { gettersMock } = vi.hoisted(() => ({
  gettersMock: { isMobile: vi.fn(() => false) },
}));

vi.mock("@/store", () => ({ getters: gettersMock }));

import {
  isChromiumBasedBrowser,
  shouldUsePdfPreviewFallback,
} from "@/utils/pdfPreview.js";

function setUserAgent(ua) {
  Object.defineProperty(navigator, "userAgent", {
    value: ua,
    configurable: true,
  });
}

describe("isChromiumBasedBrowser", () => {
  afterEach(() => {
    setUserAgent("");
  });

  it("detects Edge on Android", () => {
    setUserAgent(
      "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/120.0.0.0 Mobile Safari/537.36 EdgA/120.0.0.0",
    );
    expect(isChromiumBasedBrowser()).toBe(true);
  });

  it("excludes Firefox on Android", () => {
    setUserAgent(
      "Mozilla/5.0 (Android 14; Mobile; rv:128.0) Gecko/128.0 Firefox/128.0",
    );
    expect(isChromiumBasedBrowser()).toBe(false);
  });

  it("excludes iOS Safari", () => {
    setUserAgent(
      "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1",
    );
    expect(isChromiumBasedBrowser()).toBe(false);
  });
});

describe("shouldUsePdfPreviewFallback", () => {
  afterEach(() => {
    gettersMock.isMobile.mockReturnValue(false);
    setUserAgent("");
  });

  it("is true for mobile Chromium", () => {
    gettersMock.isMobile.mockReturnValue(true);
    setUserAgent(
      "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/120.0.0.0 Mobile Safari/537.36",
    );
    expect(shouldUsePdfPreviewFallback()).toBe(true);
  });

  it("is false for mobile Firefox", () => {
    gettersMock.isMobile.mockReturnValue(true);
    setUserAgent(
      "Mozilla/5.0 (Android 14; Mobile; rv:128.0) Gecko/128.0 Firefox/128.0",
    );
    expect(shouldUsePdfPreviewFallback()).toBe(false);
  });

  it("is false for desktop Chrome", () => {
    gettersMock.isMobile.mockReturnValue(false);
    setUserAgent(
      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
    );
    expect(shouldUsePdfPreviewFallback()).toBe(false);
  });
});
