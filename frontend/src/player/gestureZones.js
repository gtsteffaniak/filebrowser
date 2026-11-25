/** @param {number} clientX @param {{ left: number, width: number }} rect */
export function zoneFromClientX(clientX, rect) {
  const x = clientX - rect.left;
  const w = rect.width;
  if (w <= 0) {
    return 'center';
  }
  if (x < w / 3) {
    return 'left';
  }
  if (x > (2 * w) / 3) {
    return 'right';
  }
  return 'center';
}

/** True when swipe handler should preserve edge-tap state (stationary tap). */
export function isStationaryTap(dx, dy, edgeKind = null) {
  const ax = Math.abs(dx);
  const ay = Math.abs(dy);
  return !edgeKind && ax < 5 && ay < 5;
}
