import { describe, it, expect } from "vitest";
import {
  baseSidebarCategory,
  isRootOnlySidebarCategory,
  isSourceSidebarCategory,
  withRootOnlySuffix,
} from "./sidebarCategory.ts";

describe("sidebarCategory", () => {
  it("strips the -root suffix", () => {
    expect(baseSidebarCategory("source-root")).toBe("source");
    expect(baseSidebarCategory("source-hybrid-2-root")).toBe("source-hybrid-2");
    expect(baseSidebarCategory("source-hybrid")).toBe("source-hybrid");
    expect(baseSidebarCategory("tool")).toBe("tool");
    expect(baseSidebarCategory(undefined)).toBe("");
  });

  it("detects root-only categories", () => {
    expect(isRootOnlySidebarCategory("source-alt-root")).toBe(true);
    expect(isRootOnlySidebarCategory("source-alt")).toBe(false);
    expect(isRootOnlySidebarCategory(undefined)).toBe(false);
  });

  it("recognizes all source variants as source categories", () => {
    for (const c of [
      "source",
      "source-minimal",
      "source-alt",
      "source-hybrid",
      "source-hybrid-2",
      "source-root",
      "source-alt-root",
      "source-hybrid-root",
      "source-hybrid-2-root",
    ]) {
      expect(isSourceSidebarCategory(c)).toBe(true);
    }
    expect(isSourceSidebarCategory("tool")).toBe(false);
    expect(isSourceSidebarCategory("custom")).toBe(false);
    expect(isSourceSidebarCategory("divider")).toBe(false);
  });

  it("applies and removes the root suffix", () => {
    expect(withRootOnlySuffix("source", true)).toBe("source-root");
    expect(withRootOnlySuffix("source-hybrid-2", true)).toBe("source-hybrid-2-root");
    expect(withRootOnlySuffix("source-root", false)).toBe("source");
    expect(withRootOnlySuffix("source-minimal", true)).toBe("source-minimal");
    expect(withRootOnlySuffix("tool", true)).toBe("tool");
  });
});
