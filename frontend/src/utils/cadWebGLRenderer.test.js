import { describe, expect, it } from "vitest";
import { cameraBasisFromOffset, faceColor } from "./cadWebGLRenderer.js";

const dot = (a, b) => a.reduce((sum, value, i) => sum + value * b[i], 0);

describe("CAD camera basis", () => {
  it("keeps the top view aligned to the SolidWorks up direction", () => {
    const basis = cameraBasisFromOffset([0, 1, 0]);
    expect(basis.right[0]).toBeCloseTo(1);
    expect(basis.right[1]).toBeCloseTo(0);
    expect(basis.right[2]).toBeCloseTo(0);
    expect(basis.up).toEqual([0, 0, -1]);
    expect(basis.back).toEqual([0, 1, 0]);
  });

  it("creates an orthonormal basis for diagonal views", () => {
    const basis = cameraBasisFromOffset([1, 1, 1]);
    expect(basis).not.toBeNull();
    expect(Math.hypot(...basis.right)).toBeCloseTo(1);
    expect(Math.hypot(...basis.up)).toBeCloseTo(1);
    expect(Math.hypot(...basis.back)).toBeCloseTo(1);
    expect(dot(basis.right, basis.up)).toBeCloseTo(0);
    expect(dot(basis.up, basis.back)).toBeCloseTo(0);
    expect(dot(basis.right, basis.back)).toBeCloseTo(0);
  });
});

describe("CAD face colours", () => {
  it("falls back to neutral gray for dark or missing face colours", () => {
    expect(faceColor({})).toEqual([0.894, 0.894, 0.894]);
    expect(faceColor({ couleur: [0, 0, 0] })).toEqual([0.894, 0.894, 0.894]);
    expect(faceColor({ couleur: [0.02, 0.03, 0.01] })).toEqual([0.894, 0.894, 0.894]);
    expect(faceColor({ couleur: [0.2, 0.3, 0.4] })).toEqual([0.2, 0.3, 0.4]);
  });
});
