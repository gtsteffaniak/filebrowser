import { describe, it, expect } from "vitest";
import { cappedUsagePercent } from "./progressDisplay.js";

describe("cappedUsagePercent", () => {
  it("returns normal percentages under capacity", () => {
    expect(cappedUsagePercent(50, 100)).toBe(50);
    expect(cappedUsagePercent(0, 100)).toBe(0);
    expect(cappedUsagePercent(100, 100)).toBe(100);
  });

  it("caps above 100% when used exceeds total (nested mounts)", () => {
    // 1.3 TB / 106.5 GB ≈ 1294% without capping
    expect(cappedUsagePercent(1.3e12, 106.5e9)).toBe(100);
    expect(cappedUsagePercent(200, 100)).toBe(100);
  });

  it("returns 0 for invalid max", () => {
    expect(cappedUsagePercent(50, 0)).toBe(0);
    expect(cappedUsagePercent(50, -1)).toBe(0);
  });
});
