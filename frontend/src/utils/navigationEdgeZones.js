export function getRootEmSize() {
  if (typeof document === 'undefined') {
    return 16;
  }
  return parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
}

/** Tap zone (3em) — matches nextPrevious handleDocumentClick. */
export function isInLeftNavTapZone(clientX, { moveWithSidebar = false, sidebarWidthEm = 20 } = {}) {
  const emSize = getRootEmSize();
  const zoneWidth = 3 * emSize;
  const sidebarOffset = moveWithSidebar ? sidebarWidthEm * emSize : 0;
  return clientX >= sidebarOffset && clientX <= sidebarOffset + zoneWidth;
}

/** Tap zone (3em) — matches nextPrevious handleDocumentClick. */
export function isInRightNavTapZone(clientX) {
  if (typeof window === 'undefined') {
    return false;
  }
  const emSize = getRootEmSize();
  const zoneWidth = 3 * emSize;
  return clientX >= window.innerWidth - zoneWidth && clientX <= window.innerWidth;
}

/** Hover zone (5em) — matches nextPrevious handleGlobalMouseMove. */
export function isInLeftNavHoverZone(clientX, { moveWithSidebar = false, sidebarWidthEm = 20 } = {}) {
  const emSize = getRootEmSize();
  const zoneWidth = 5 * emSize;
  const sidebarOffset = moveWithSidebar ? sidebarWidthEm * emSize : 0;
  return clientX >= sidebarOffset && clientX <= sidebarOffset + zoneWidth;
}

export function isInRightNavHoverZone(clientX) {
  if (typeof window === 'undefined') {
    return false;
  }
  const emSize = getRootEmSize();
  const zoneWidth = 5 * emSize;
  return clientX >= window.innerWidth - zoneWidth && clientX <= window.innerWidth;
}
