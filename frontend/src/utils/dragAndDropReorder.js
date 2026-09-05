import { getObjectProperty } from "@/utils/object.js";

const AUTO_SCROLL_EDGE = 50;
const AUTO_SCROLL_MAX_SPEED = 16;

function getScrollParent(node) {
  let el = node?.parentElement;
  while (el) {
    const style = getComputedStyle(el);
    const scrollable =
      (style.overflowY === "auto" || style.overflowY === "scroll") &&
      el.scrollHeight > el.clientHeight;
    if (scrollable) return el;
    el = el.parentElement;
  }
  return document.scrollingElement || document.documentElement;
}

/**
 * @param {Object} hooks
 * @param {() => any[]} hooks.getOrder - returns copy of the current order
 * @param {(snapshot: any[]) => void} hooks.setOrder - applies a reordered array back to state
 */
export function createDragReorder({ getOrder, setOrder }) {
  return {
    draggingIndex: null,
    itemRefs: {},
    originalSnapshot: null,
    pointerId: null,
    dragClone: null,
    dragOffsetY: 0,
    dragCorrectionX: 0,
    dragCorrectionY: 0,
    scrollParent: null,
    autoScrollRAF: null,
    autoScrollDelta: 0,
    lastClientY: 0,

    onPointerDown(index, event) {
      if (event.pointerType === "mouse" && event.button !== 0) return;
      event.preventDefault();
      this.draggingIndex = index;
      this.originalSnapshot = getOrder();
      this.pointerId = event.pointerId;
      this.lastClientY = event.clientY;
      // make all pointermove/up events for this pointer go to this element
      event.currentTarget.setPointerCapture(event.pointerId);

      const row = getObjectProperty(this.itemRefs, index);
      if (row) {
        const rect = row.getBoundingClientRect();
        this.dragOffsetY = event.clientY - rect.top;
        const clone = row.cloneNode(true);
        clone.style.position = "fixed";
        clone.style.left = `${rect.left}px`;
        clone.style.top = `${rect.top}px`;
        clone.style.width = `${rect.width}px`;
        clone.style.margin = "0";
        clone.style.pointerEvents = "none";
        clone.style.zIndex = "9999";
        clone.style.boxShadow = "0 4px 12px rgba(0, 0, 0, 0.25)";
        row.parentNode.insertBefore(clone, row.nextSibling);
        const actualRect = clone.getBoundingClientRect();
        this.dragCorrectionX = rect.left - actualRect.left;
        this.dragCorrectionY = rect.top - actualRect.top;
        if (this.dragCorrectionX || this.dragCorrectionY) {
          clone.style.left = `${rect.left + this.dragCorrectionX}px`;
          clone.style.top = `${rect.top + this.dragCorrectionY}px`;
        }
        this.dragClone = clone;
        this.scrollParent = getScrollParent(row);
      }
    },

    onPointerMove(event) {
      if (this.draggingIndex === null || event.pointerId !== this.pointerId) return;
      event.preventDefault();
      this.lastClientY = event.clientY;
      if (this.dragClone) {
        const top = event.clientY - this.dragOffsetY + (this.dragCorrectionY || 0);
        this.dragClone.style.top = `${top}px`;
      }
      this.updateHoveredIndex(event.clientY);
      this.updateAutoScroll(event.clientY);
    },

    onPointerUp(event) {
      if (event.pointerId !== this.pointerId) return;
      this.finishDrag(true);
    },

    onPointerCancel(event) {
      if (event.pointerId !== this.pointerId) return;
      this.finishDrag(false);
    },

    // Called in beforeUnmount to clean up if the component is destroyed in the middle of a drag
    cancel() {
      if (this.draggingIndex !== null || this.autoScrollRAF !== null) {
        this.finishDrag(false);
      }
    },

    finishDrag(commit) {
      // If the drag was cancelled restore original order
      if (!commit && this.originalSnapshot !== null) {
        setOrder(this.originalSnapshot);
      }
      this.originalSnapshot = null;
      this.draggingIndex = null;
      this.pointerId = null;

      if (this.dragClone) {
        this.dragClone.remove();
        this.dragClone = null;
      }
      if (this.autoScrollRAF) {
        cancelAnimationFrame(this.autoScrollRAF);
        this.autoScrollRAF = null;
      }
      this.autoScrollDelta = 0;
      this.scrollParent = null;
    },

    updateHoveredIndex(clientY) {
      if (this.draggingIndex === null) return;
      const entries = Object.entries(this.itemRefs)
        .map(([i, el]) => [Number(i), el])
        .filter(([, el]) => !!el)
        .sort((a, b) => a[0] - b[0]);

      let targetIndex = entries.length;
      for (const [i, el] of entries) {
        if (i === this.draggingIndex) continue;
        const rect = el.getBoundingClientRect();
        if (clientY < rect.top + rect.height / 2) {
          targetIndex = i;
          break;
        }
      }
      if (targetIndex !== this.draggingIndex) {
        const snapshot = [...getOrder()];
        const [moved] = snapshot.splice(this.draggingIndex, 1);
        if (targetIndex > this.draggingIndex) targetIndex -= 1;
        snapshot.splice(targetIndex, 0, moved);
        setOrder(snapshot);
        this.draggingIndex = targetIndex;
      }
    },

    updateAutoScroll(clientY) {
      const parent = this.scrollParent;
      if (!parent) return;
      const isWindowScroll = parent === document.scrollingElement || parent === document.documentElement;
      const rect = isWindowScroll ? null : parent.getBoundingClientRect();
      const top = isWindowScroll ? 0 : rect.top;
      const bottom = isWindowScroll ? window.innerHeight : rect.bottom;

      let delta = 0;
      if (clientY < top + AUTO_SCROLL_EDGE) {
        delta = -AUTO_SCROLL_MAX_SPEED * (1 - Math.max(0, clientY - top) / AUTO_SCROLL_EDGE);
      } else if (clientY > bottom - AUTO_SCROLL_EDGE) {
        delta = AUTO_SCROLL_MAX_SPEED * (1 - Math.max(0, bottom - clientY) / AUTO_SCROLL_EDGE);
      }
      this.autoScrollDelta = delta;
      if (delta !== 0 && this.autoScrollRAF === null) {
        this.autoScroll();
      }
    },

    autoScroll() {
      const isWindowScroll =
        this.scrollParent === document.scrollingElement || this.scrollParent === document.documentElement;
      const step = () => {
        if (this.autoScrollDelta !== 0 && this.scrollParent) {
          if (isWindowScroll) {
            window.scrollBy(0, this.autoScrollDelta);
          } else {
            this.scrollParent.scrollTop += this.autoScrollDelta;
          }
          this.updateHoveredIndex(this.lastClientY);
          this.autoScrollRAF = requestAnimationFrame(step);
        } else {
          this.autoScrollRAF = null;
        }
      };
      this.autoScrollRAF = requestAnimationFrame(step);
    },
  };
}
