import { beforeEach, describe, expect, it, vi } from "vitest";

const { storeMock } = vi.hoisted(() => ({
  storeMock: {
    state: {
      user: null,
      toolAccessDefaultsPolicy: { items: [] },
    },
    getters: {
      isAdmin: vi.fn(() => false),
    },
  },
}));

vi.mock("@/store", () => storeMock);
vi.mock("@/utils/constants", () => ({
  tools: () => [
    { id: "sizeViewer", path: "/tools/size-viewer", name: "Size viewer" },
    { id: "fileWatcher", path: "/tools/file-watcher", name: "File watcher" },
    { id: "advancedSearch", path: "/tools/search", name: "Search" },
  ],
}));

import {
  hasToolAccess,
  normalizeUserToolAccess,
  resolveHasToolAccess,
} from "./toolAccess.js";

describe("toolAccess", () => {
  beforeEach(() => {
    storeMock.state.user = {
      toolAccess: { fileWatcher: true },
      effectiveToolAccess: { fileWatcher: false },
    };
    storeMock.state.toolAccessDefaultsPolicy = {
      items: [
        { toolId: "fileWatcher", enabled: false, enforced: false },
        { toolId: "sizeViewer", enabled: false, enforced: true },
      ],
    };
    storeMock.getters.isAdmin.mockReturnValue(false);
  });

  it("ignores stale effectiveToolAccess in favor of policy and overrides", () => {
    expect(hasToolAccess("fileWatcher")).toBe(true);
    expect(hasToolAccess("sizeViewer")).toBe(false);
  });

  it("allows admins regardless of policy", () => {
    storeMock.getters.isAdmin.mockReturnValue(true);
    expect(hasToolAccess("sizeViewer")).toBe(true);
  });

  it("normalizes sparse user maps across the catalog", () => {
    const normalized = normalizeUserToolAccess(
      { fileWatcher: true },
      storeMock.state.toolAccessDefaultsPolicy.items
    );
    expect(normalized).toEqual({
      sizeViewer: false,
      fileWatcher: true,
      advancedSearch: true,
    });
  });

  it("resolveHasToolAccess accepts explicit policy items", () => {
    expect(
      resolveHasToolAccess("sizeViewer", {
        user: { toolAccess: { sizeViewer: true } },
        policyItems: [{ toolId: "sizeViewer", enabled: false, enforced: true }],
        isAdmin: false,
      })
    ).toBe(false);
  });
});
