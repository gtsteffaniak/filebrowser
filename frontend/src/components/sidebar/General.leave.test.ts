import { afterEach, describe, expect, it, vi } from "vitest";
import { collapseLeave } from "@/utils/transitions.ts";

describe("sidebar File actions leave transition", () => {
  afterEach(() => vi.useRealTimers());

  it("finishes even when Web Animations API is unavailable", () => {
    vi.useFakeTimers();
    const el = document.createElement("div");
    const done = vi.fn();
    // jsdom: no native animate — collapseLeave falls back to setTimeout(durationMs).
    el.animate = undefined as unknown as typeof el.animate;
    collapseLeave(el, done);
    expect(done).not.toHaveBeenCalled();
    vi.advanceTimersByTime(160);
    expect(done).toHaveBeenCalledOnce();
  });
});