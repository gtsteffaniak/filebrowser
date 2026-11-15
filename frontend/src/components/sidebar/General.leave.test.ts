import { afterEach, describe, expect, it, vi } from "vitest";
import { collapseEnter, collapseLeave } from "@/utils/transitions";

// The old Options API test called `General.methods.leave` — that helper moved to
// `@/utils/transitions` (`collapseLeave`) in the composition-API pass, and there is
// no `components` tree to mount for a 1-liner assertion.
describe("sidebar File actions leave transition", () => {
  afterEach(() => vi.useRealTimers());

  it("collapseLeave finishes even when no transitionend ever fires", () => {
    vi.useFakeTimers();
    const el = document.createElement("div");
    const done = vi.fn();
    collapseLeave(el, done);
    expect(done).not.toHaveBeenCalled();
    vi.advanceTimersByTime(200);
    expect(done).toHaveBeenCalledOnce();
  });

  it("collapseEnter (the showFileActions mount) also finishes", () => {
    vi.useFakeTimers();
    const el = document.createElement("div");
    const done = vi.fn();
    collapseEnter(el, done);
    expect(done).not.toHaveBeenCalled();
    vi.advanceTimersByTime(250);
    expect(done).toHaveBeenCalledOnce();
  });
});
