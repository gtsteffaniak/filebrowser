import { describe, expect, it } from "vitest";
import { createLightningBolt, createLightningStrike } from "./lightning-bolt.js";

function pseudoRandom(seed) {
  let s = seed;
  return () => {
    s = (s * 1664525 + 1013904223) % 4294967296;
    return s / 4294967296;
  };
}

describe("lightning-bolt", () => {
  it("createLightningBolt returns a main path and viewBox", () => {
    const random = pseudoRandom(42);
    const bolt = createLightningBolt({ random, branchChance: 0 });

    expect(bolt.viewBox).toMatch(/^0 0 \d+ \d+$/);
    expect(bolt.main).toMatch(/^M /);
    expect(bolt.main).toContain(" L ");
    expect(bolt.branches).toEqual([]);
  });

  it("createLightningBolt can emit branches with a deterministic random", () => {
    const random = pseudoRandom(7);
    const bolt = createLightningBolt({ random, branchChance: 1 });

    expect(bolt.branches.length).toBeGreaterThan(0);
    for (const branch of bolt.branches) {
      expect(branch).toMatch(/^M /);
    }
  });

  it("createLightningStrike returns viewport positioning", () => {
    const random = pseudoRandom(99);
    const strike = createLightningStrike(random);

    expect(strike.style.left).toMatch(/%$/);
    expect(strike.style.height).toMatch(/vh$/);
    expect(strike.main).toBeTruthy();
  });
});
