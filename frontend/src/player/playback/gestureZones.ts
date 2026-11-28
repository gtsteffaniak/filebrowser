export type GestureZone = "left" | "center" | "right";

export type EdgeKind = "horizontal" | "vertical-dismiss" | "vertical-fullscreen";

export function zoneFromClientX(clientX: number, rect: Pick<DOMRect, "left" | "width">): GestureZone {
  const x = clientX - rect.left;
  const w = rect.width;
  if (w <= 0) {
    return "center";
  }
  if (x < w / 3) {
    return "left";
  }
  if (x > (2 * w) / 3) {
    return "right";
  }
  return "center";
}

/** True when swipe handler should preserve edge-tap state */
export function isStationaryTap(dx: number, dy: number, edgeKind: EdgeKind | null = null): boolean {
  const ax = Math.abs(dx);
  const ay = Math.abs(dy);
  return !edgeKind && ax < 5 && ay < 5;
}
