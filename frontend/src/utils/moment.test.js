import { afterEach, describe, expect, it, vi } from "vitest";
import { fromNow, fromNowSeconds } from "./moment.ts";

describe("relative timestamps", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("formats a share expiration in Unix seconds as a future date", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-21T12:00:00Z"));

    // A share with a duration of 240 hours (10 days).
    const expires = Math.floor(Date.parse("2026-10-31T12:00:00Z") / 1000);
    expect(fromNowSeconds(expires, "en-us")).toBe("next week");
  });

  it("preserves the existing millisecond default for other callers", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-21T12:00:00Z"));

    const threeDaysLaterMs = Date.parse("2026-10-24T12:00:00Z");
    expect(fromNow(threeDaysLaterMs, "en-us")).toBe("in 3 days");
    expect(fromNowSeconds(threeDaysLaterMs / 1000, "en-us")).toBe("in 3 days");
  });
});
