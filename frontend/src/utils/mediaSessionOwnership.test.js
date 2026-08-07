import { describe, expect, it } from "vitest";
import { ownsMediaSession } from "./mediaSessionOwnership";

describe("ownsMediaSession", () => {
  it("rejects uninitialized viewers", () => {
    expect(ownsMediaSession(0, 0)).toBe(false);
    expect(ownsMediaSession(0, 2)).toBe(false);
  });

  it("accepts the active owner", () => {
    expect(ownsMediaSession(2, 2)).toBe(true);
  });

  it("rejects stale owners after another viewer takes over", () => {
    expect(ownsMediaSession(1, 2)).toBe(false);
  });
});
