/**
 * Orthogonal projection of p onto the plane through a with normal n.
 * n does not need to be normalized: the projection subtracts
 * dot(p-a, n) / dot(n, n) * n, so the endpoint lies on the plane.
 */
export function projectPointOnPlane(p, a, n) {
  const nn = n[0] * n[0] + n[1] * n[1] + n[2] * n[2] || 1;
  const t = (n[0] * (p[0] - a[0]) + n[1] * (p[1] - a[1]) + n[2] * (p[2] - a[2])) / nn;
  return [p[0] - t * n[0], p[1] - t * n[1], p[2] - t * n[2]];
}
