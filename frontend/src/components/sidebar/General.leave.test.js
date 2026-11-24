import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/utils/auth", () => ({ logout: vi.fn() }));
vi.mock("@/utils/constants", () => ({ globalVars: {} }));
vi.mock("@/store", () => ({ state: {}, getters: {}, mutations: {} }));
vi.mock("@/utils/tooltipHelp.js", () => ({ hideInteractiveTooltip: vi.fn(), showHoverTooltip: vi.fn() }));
vi.mock("./Links.vue", () => ({ default: { render: () => null } }));
vi.mock("@/utils", () => ({ url: {} }));
import General from "./General.vue";

describe("sidebar File actions leave transition", () => {
  afterEach(() => vi.useRealTimers());

  it("finishes even when no transitionend ever fires", () => {
    vi.useFakeTimers();
    const el = document.createElement("div");
    const done = vi.fn();
    General.methods.leave(el, done);
    expect(done).not.toHaveBeenCalled();
    vi.advanceTimersByTime(200);
    expect(done).toHaveBeenCalledOnce();
  });
});
