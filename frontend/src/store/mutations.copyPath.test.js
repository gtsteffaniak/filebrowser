import { beforeEach, describe, expect, it, vi } from "vitest";

const { stateMock } = vi.hoisted(() => ({ stateMock: { sources: {}, user: {} } }));
vi.mock("./state.ts", () => ({ state: stateMock }));
vi.mock("./getters.ts", () => ({ getters: {} }));
vi.mock("./eventBus.ts", () => ({ emitStateChanged: vi.fn() }));
vi.mock("@/api/index.js", () => ({ resourcesApi: {}, usersApi: {} }));
vi.mock("@/api/settings.js", () => ({}));
vi.mock("@/api/viewToken.js", () => ({ setActiveViewGrantScope: vi.fn() }));
vi.mock("@/i18n/index.ts", () => ({ detectLocale: vi.fn(), setLocale: vi.fn() }));
vi.mock("@/utils/index.ts", () => ({ url: {} }));
vi.mock("@/utils/pwaManifest.js", () => ({ updateManifestLink: vi.fn() }));
vi.mock("@/utils/theme.js", () => ({ syncEventTheme: vi.fn() }));

import { mutations } from "./mutations.ts";

describe("source Copy Path configuration in the store", () => {
  const user = { username: "alice", scopes: [{ name: "Home", scope: "/" }, { name: "RT", scope: "/" }] };

  beforeEach(() => {
    stateMock.user = user;
    stateMock.sources = { current: "Home", info: {}, count: 0, hasSourceInfo: false };
    mutations.setSources(user);
  });

  it("merges API/SSE configuration and preserves it across same-user refreshes", () => {
    mutations.updateSourceInfo({ Home: { copyPathPrefix: "/home/rt" }, RT: { copyPathWindows: true } });
    mutations.setSources(user);
    expect(stateMock.sources.info.Home.copyPathPrefix).toBe("/home/rt");
    expect(stateMock.sources.info.RT.copyPathWindows).toBe(true);
  });

  it("removes an old mapping when a new source payload omits the options", () => {
    mutations.updateSourceInfo({ Home: { copyPathPrefix: "/home/rt" } });
    mutations.updateSourceInfo({ Home: { name: "Home" } });
    expect(stateMock.sources.info.Home.copyPathPrefix).toBeUndefined();
    expect(stateMock.sources.info.Home.copyPathWindows).toBeUndefined();
  });

  it("does not keep configuration for a different user or an inaccessible source", () => {
    mutations.updateSourceInfo({ Home: { copyPathPrefix: "/home/rt" }, Secret: { copyPathPrefix: "/secret" } });
    expect(stateMock.sources.info.Secret).toBeUndefined();
    mutations.setSources({ username: "bob", scopes: user.scopes });
    expect(stateMock.sources.info.Home.copyPathPrefix).toBeUndefined();
  });
});
