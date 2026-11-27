import { afterEach, describe, expect, it, vi } from "vitest";
import { getHumanReadableTime } from "./IndexInfo.vue";

describe("getHumanReadableTime", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("treats lastIndex as Unix seconds, not milliseconds", () => {
    vi.useFakeTimers();
    // 1795593600 is 2026-11-25 08:00:00 UTC (the lastIndexedUnixTime from #3455).
    vi.setSystemTime(new Date("2026-11-25T10:00:00Z"));
    expect(getHumanReadableTime(1795593600, "en-us")).toBe("2 hours ago");
  });

  it("returns now for a zero timestamp", () => {
    expect(getHumanReadableTime(0, "en-us")).toBe("now");
  });
});
