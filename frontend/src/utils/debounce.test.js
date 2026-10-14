import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { debounce } from "./debounce.js";

describe("debounce", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("runs once after the wait, using the last call's arguments", () => {
    const fn = vi.fn();
    const debounced = debounce(fn, 250);

    debounced("a");
    debounced("b");
    debounced("c");

    expect(fn).not.toHaveBeenCalled();
    vi.advanceTimersByTime(249);
    expect(fn).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(fn).toHaveBeenCalledTimes(1);
    expect(fn).toHaveBeenCalledWith("c");
  });

  it("cancel() drops a pending call", () => {
    const fn = vi.fn();
    const debounced = debounce(fn, 250);

    debounced();
    debounced.cancel();
    vi.advanceTimersByTime(1000);

    expect(fn).not.toHaveBeenCalled();
  });

  it("can fire again after a previous call", () => {
    const fn = vi.fn();
    const debounced = debounce(fn, 250);

    debounced(1);
    vi.advanceTimersByTime(250);
    debounced(2);
    vi.advanceTimersByTime(250);

    expect(fn).toHaveBeenNthCalledWith(1, 1);
    expect(fn).toHaveBeenNthCalledWith(2, 2);
  });
});
