import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useGestureState } from "../useGestureState.ts";
import { withSetup } from "./withSetup.ts";

describe("useGestureState", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("setSkipNextTap clears edge-tap memory and expires after the delay", () => {
    const { result: g } = withSetup(() => useGestureState());
    g.edgeTapLastTime.value = 123;
    g.edgeTapLastZone.value = "left";
    g.setSkipNextTap(300);
    expect(g.skipNextTap.value).toBe(true);
    expect(g.edgeTapLastTime.value).toBe(0);
    expect(g.edgeTapLastZone.value).toBeNull();
    vi.advanceTimersByTime(299);
    expect(g.skipNextTap.value).toBe(true);
    vi.advanceTimersByTime(1);
    expect(g.skipNextTap.value).toBe(false);
  });

  it("clearLongPressTimer cancels a pending long press", () => {
    const { result: g } = withSetup(() => useGestureState());
    const fired = vi.fn();
    g.longPressTimer.value = setTimeout(fired, 500);
    g.clearLongPressTimer();
    vi.advanceTimersByTime(600);
    expect(fired).not.toHaveBeenCalled();
    expect(g.longPressTimer.value).toBeNull();
  });

  it("clears the skip-tap timer on unmount", () => {
    const { result: g, app } = withSetup(() => useGestureState());
    g.setSkipNextTap(300);
    app.unmount();
    expect(g.skipNextTap.value).toBe(false);
  });
});
