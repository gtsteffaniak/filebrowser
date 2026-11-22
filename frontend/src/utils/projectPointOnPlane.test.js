import { describe, expect, it } from "vitest";
import { projectPointOnPlane } from "./projectPointOnPlane.js";

describe("projectPointOnPlane", () => {
  it("uses a non-unit normal and puts the endpoint on the triangle plane", () => {
    const a = [0, 0, 0];
    const b = [4, 0, 0];
    const c = [0, 3, 0];
    const ab = b.map((v, i) => v - a[i]);
    const ac = c.map((v, i) => v - a[i]);
    // cross(ab, ac) = [0, 0, 12] — deliberately not normalized.
    const n = [
      ab[1] * ac[2] - ab[2] * ac[1],
      ab[2] * ac[0] - ab[0] * ac[2],
      ab[0] * ac[1] - ab[1] * ac[0],
    ];
    const p = [1, 2, 5];

    const q = projectPointOnPlane(p, a, n);

    expect(q).toEqual([1, 2, 0]);
    expect(n.reduce((sum, v, i) => sum + v * (q[i] - a[i]), 0)).toBe(0);
  });
});
