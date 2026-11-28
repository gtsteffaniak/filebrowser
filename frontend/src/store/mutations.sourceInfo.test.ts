import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SourceInfo } from "@/types/sources.ts";

const { stateMock, makeSource, SOURCE_TOTAL } = vi.hoisted(() => {
  const total = 50 * 1024 ** 3;
  const build = (overrides: Partial<SourceInfo> = {}): SourceInfo => ({
    used: 0,
    total,
    usedAlt: 0,
    totalRoot: 0,
    usedAltRoot: 0,
    usedPercentage: 0,
    status: "ready",
    name: "data",
    files: 0,
    folders: 0,
    lastIndex: 0,
    quickScanDurationSeconds: 0,
    fullScanDurationSeconds: 0,
    complexity: 0,
    scanners: [],
    readOnly: false,
    private: false,
    ...overrides,
  });
  return {
    SOURCE_TOTAL: total,
    makeSource: build,
    stateMock: {
      sources: {
        hasSourceInfo: false,
        info: { data: build() } as Record<string, SourceInfo>,
      },
      realtimeActive: undefined,
      realtimeDownCount: 0,
      user: { username: "alice" },
    },
  };
});

vi.mock("./state.ts", () => ({ state: stateMock }));
vi.mock("./eventBus.ts", () => ({ emitStateChanged: vi.fn() }));
vi.mock("@/api/viewToken.js", () => ({ setActiveViewGrantScope: vi.fn() }));
vi.mock("@/api/index.js", () => ({ resourcesApi: {}, usersApi: {} }));
vi.mock("@/api/settings.ts", () => ({
  getEnforcedUserDefaults: vi.fn(),
  getShareDefaultsPolicy: vi.fn(),
  getSidebarLinkDefaultsPolicy: vi.fn(),
  getToolAccessDefaultsPolicy: vi.fn(),
}));
vi.mock("@/i18n/index.ts", () => ({ detectLocale: () => "en", setLocale: vi.fn() }));
vi.mock("@/notify/index.ts", () => ({ notify: { show: vi.fn() } }));
vi.mock("@/utils/index.ts", () => ({ url: {} }));
vi.mock("@/utils/mimetype.js", () => ({ getTypeInfo: vi.fn() }));
vi.mock("@/utils/sort.ts", () => ({ sortedItems: vi.fn() }));
vi.mock("@/utils/sidebarCategory.ts", () => ({ isSourceSidebarCategory: vi.fn() }));
vi.mock("@/utils/pwaManifest.js", () => ({ updateManifestLink: vi.fn() }));
vi.mock("@/utils/theme.js", () => ({ syncEventTheme: vi.fn() }));
vi.mock("@/utils/uploadNavigationGuard.ts", () => ({
  cancelCloseUploadPrompt: vi.fn(),
  confirmCloseUploadPrompt: vi.fn(),
  hasCloseWithActiveUploadsPrompt: vi.fn(),
}));
vi.mock("./getters.ts", () => ({ getters: {} }));

import { mutations } from "./mutations.ts";

const SCOPE_LIMIT = 10 * 1024 * 1024;
const SCOPE_USED = 2 * 1024 * 1024;

const SCOPE_QUOTA = {
  quotaKind: "scope",
  limitBytes: SCOPE_LIMIT,
  usedBytes: SCOPE_USED,
  reservedBytes: 0,
  meter: "index_scope",
};

describe("mutations.updateSourceInfo scope quota", () => {
  beforeEach(() => {
    stateMock.sources.hasSourceInfo = false;
    stateMock.sources.info = { data: makeSource() };
  });

  it("merges scopeQuota from /api/settings/sources into store source info", () => {
    mutations.updateSourceInfo({
      data: { used: 100, total: SOURCE_TOTAL, status: "ready", name: "data", scopeQuota: SCOPE_QUOTA },
    });

    const info = stateMock.sources.info.data;
    expect(info?.scopeQuota?.limitBytes).toBe(SCOPE_LIMIT);
    expect(info?.scopeQuota?.usedBytes).toBe(SCOPE_USED);
    expect(info?.total).toBe(SOURCE_TOTAL);
    expect(stateMock.sources.hasSourceInfo).toBe(true);
  });

  it("keeps hasSourceInfo true when a partial update only touches an empty source", () => {
    mutations.updateSourceInfo({
      data: { used: 100, total: SOURCE_TOTAL, status: "ready", name: "data", scopeQuota: SCOPE_QUOTA },
    });
    stateMock.sources.info.other = makeSource({ name: "other", total: 0 });

    // SSE events carry one source at a time. An empty source must not clear the flag while
    // "data" still holds quota information.
    mutations.updateSourceInfo({
      other: { used: 0, total: 0, status: "ready", name: "other" },
    });

    expect(stateMock.sources.info.data?.scopeQuota?.limitBytes).toBe(SCOPE_LIMIT);
    expect(stateMock.sources.hasSourceInfo).toBe(true);
  });
});
