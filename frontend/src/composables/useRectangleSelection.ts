import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from "vue";
import { state, mutations, getters } from "@/store/index.ts";

interface Point {
  x: number;
  y: number;
}

interface ItemRect {
  index: number;
  left: number;
  top: number;
  right: number;
  bottom: number;
}

interface PointerInfo extends Point {
  additive: boolean;
}

interface MouseSelectionOptions {
  // Where mouse events are listened to and the `.listing-item` elements are looked up
  container: Ref<HTMLElement | null>;
  // Positioned ancestor of the selection overlay; the rectangle is measured against it
  bounds: Ref<HTMLElement | null>;
  // Changes whenever the items do, so the cached item boxes are measured again
  items: () => unknown;
  isActive: () => boolean;
}

export function useMouseSelection(options: MouseSelectionOptions) {
  const isRectangleSelecting = ref(false);
  const rectangleStart = ref<Point>({ x: 0, y: 0 });
  const rectangleEnd = ref<Point>({ x: 0, y: 0 });

  // Non reactive on purpose: big DOM-derived caches and transient interaction state.
  let rectangleItemRects: ItemRect[] | null = null;
  let lastSelected: typeof state.selected | null = null;
  let rafId: number | null = null;
  let lastPointer: PointerInfo | null = null;
  let selectionUpdatePending = false;
  let initialSelectionState: typeof state.selected = [];

  function invalidateRects() {
    rectangleItemRects = null;
  }

  const rectangleStyle = computed(() => {
    if (!isRectangleSelecting.value) return { display: "none" };
    const left = Math.min(rectangleStart.value.x, rectangleEnd.value.x);
    const top = Math.min(rectangleStart.value.y, rectangleEnd.value.y);
    const width = Math.abs(rectangleStart.value.x - rectangleEnd.value.x);
    const height = Math.abs(rectangleStart.value.y - rectangleEnd.value.y);
    return {
      left: `${left}px`,
      top: `${top}px`,
      width: `${width}px`,
      height: `${height}px`,
    };
  });

  function clickClear(event: MouseEvent) {
    if (!options.isActive()) {
      return;
    }
    const targetClasses: unknown = (event.target as Element).className;

    if (typeof targetClasses === "string" && targetClasses.includes("listing-item")) {
      return;
    }
    // if control, meta, or shift is pressed, do not clear the selection
    if (event.ctrlKey || event.metaKey || event.shiftKey) {
      return;
    }
    const sameAsBefore = state.selected === lastSelected;
    if (sameAsBefore && !state.multiple && getters.currentPromptName() === "") {
      mutations.resetSelected();
    }
    lastSelected = state.selected;
  }

  // Item boxes relative to the listing element. Relative vals don't change while scrolling
  // so they can be measured once and reused.
  function measureItemRects(listingRect: DOMRect) {
    const rects: ItemRect[] = [];
    for (const element of options.container.value?.querySelectorAll(".listing-item[data-index]") ?? []) {
      const index = parseInt(element.getAttribute("data-index") as string, 10);
      if (Number.isNaN(index)) continue;
      const r = element.getBoundingClientRect();
      rects.push({
        index,
        left: r.left - listingRect.left,
        top: r.top - listingRect.top,
        right: r.right - listingRect.left,
        bottom: r.bottom - listingRect.top,
      });
    }
    return rects;
  }

  function measureSelectionBounds() {
    const listingRect = (options.bounds.value as HTMLElement).getBoundingClientRect();
    const statusBar = document.getElementById("status-bar");
    const statusBarVisible = statusBar && getComputedStyle(statusBar).display !== "none";
    const maxY = statusBarVisible
      ? Math.max(0, statusBar.getBoundingClientRect().top - listingRect.top)
      : listingRect.height;
    return { left: listingRect.left, top: listingRect.top, maxX: listingRect.width, maxY };
  }

  function startRectangleSelection(event: MouseEvent) {
    // Start rectangle selection when clicking on empty space
    const target = event.target as Element;
    if (target.closest(".listing-item") || target.closest(".header") || target.closest("#status-bar")) {
      return;
    }
    // Don't start if it's a right click
    if (event.button !== 0) return;
    if (!options.bounds.value) return;

    isRectangleSelecting.value = true;
    rectangleItemRects = null;

    // Get the position to the view container
    const { left, top, maxX, maxY } = measureSelectionBounds();
    let startX = event.clientX - left;
    let startY = event.clientY - top;

    // Clamp to container bounds
    startX = Math.max(0, Math.min(startX, maxX));
    startY = Math.max(0, Math.min(startY, maxY));

    rectangleStart.value = {
      x: startX,
      y: startY
    };
    rectangleEnd.value = {
      x: startX,
      y: startY
    };
    lastPointer = {
      x: event.clientX,
      y: event.clientY,
      additive: event.ctrlKey || event.metaKey,
    };
    document.addEventListener("scroll", handleRectangleScroll, { capture: true, passive: true });

    // Store the current selection state when starting rectangle
    initialSelectionState = [...state.selected];

    // Only clear selection when CTRL is not holded
    const hasModifier = event.ctrlKey || event.metaKey;
    if (!hasModifier) {
      mutations.resetSelected();
    }

    event.preventDefault();
  }

  function updateRectangleSelection(event: MouseEvent) {
    if (!isRectangleSelecting.value) return;
    lastPointer = {
      x: event.clientX,
      y: event.clientY,
      additive: event.ctrlKey || event.metaKey,
    };
    scheduleRectangleUpdate();
  }

  function handleRectangleScroll() {
    rectangleItemRects = null;
    scheduleRectangleUpdate();
  }

  // mousemove + scroll into one update
  function scheduleRectangleUpdate() {
    if (!isRectangleSelecting.value || selectionUpdatePending) return;
    selectionUpdatePending = true;
    rafId = requestAnimationFrame(() => {
      selectionUpdatePending = false;
      rafId = null;
      if (!isRectangleSelecting.value || !lastPointer) return;

      const { left, top, maxX, maxY } = measureSelectionBounds();
      const endX = Math.max(0, Math.min(lastPointer.x - left, maxX));
      const endY = Math.max(0, Math.min(lastPointer.y - top, maxY));
      rectangleEnd.value = { x: endX, y: endY };
      updateSelectedItemsInRectangle(lastPointer.additive);
    });
  }

  function stopRectangleScrollListener() {
    document.removeEventListener("scroll", handleRectangleScroll, { capture: true });
  }

  function endRectangleSelection(event: MouseEvent) {
    if (!isRectangleSelecting.value) return;

    // Cancel pendings animation frame
    if (rafId) {
      cancelAnimationFrame(rafId);
      rafId = null;
    }
    stopRectangleScrollListener();

    const { left, top, maxX, maxY } = measureSelectionBounds();
    rectangleEnd.value = {
      x: Math.max(0, Math.min(event.clientX - left, maxX)),
      y: Math.max(0, Math.min(event.clientY - top, maxY)),
    };
    updateSelectedItemsInRectangle(event.ctrlKey || event.metaKey);

    isRectangleSelecting.value = false;
    rectangleItemRects = null;
    selectionUpdatePending = false;
    lastPointer = null;

    // Clear rectangle after a short delay
    setTimeout(() => {
      rectangleStart.value = { x: 0, y: 0 };
      rectangleEnd.value = { x: 0, y: 0 };
      initialSelectionState = [];
    }, 100);
  }

  function updateSelectedItemsInRectangle(isAdditive: boolean) {
    if (!isRectangleSelecting.value) return;

    const listingRect = (options.bounds.value as HTMLElement).getBoundingClientRect();
    const rect = {
      left: Math.min(rectangleStart.value.x, rectangleEnd.value.x),
      top: Math.min(rectangleStart.value.y, rectangleEnd.value.y),
      right: Math.max(rectangleStart.value.x, rectangleEnd.value.x),
      bottom: Math.max(rectangleStart.value.y, rectangleEnd.value.y)
    };

    const rectangleSelectedIndexes: number[] = [];

    // Measure the items once per layout
    if (!rectangleItemRects) {
      rectangleItemRects = measureItemRects(listingRect);
    }
    for (const box of rectangleItemRects) {
      // Check if the item intersects with the rectangle
      if (
        box.left < rect.right &&
        box.right > rect.left &&
        box.top < rect.bottom &&
        box.bottom > rect.top
      ) {
        rectangleSelectedIndexes.push(box.index);
      }
    }
    // Compute the selection and only touch the store when it actually changed
    let nextSelection: typeof state.selected;
    if (isAdditive) {
      const base = initialSelectionState || [];
      const known = new Set(base);
      nextSelection = [...base];
      for (const index of rectangleSelectedIndexes) {
        if (!known.has(index)) {
          known.add(index);
          nextSelection.push(index);
        }
      }
    } else {
      // Select only the items in the rectangle and reset initial selection
      // PS: If you don't want that just hold ctrl, the selection will not be reset, allowing multi select.
      nextSelection = rectangleSelectedIndexes;
    }
    const current = state.selected;
    const unchanged =
      current.length === nextSelection.length &&
      nextSelection.every((value, i) => current[i] === value);
    if (unchanged) return;
    mutations.setSelected(nextSelection);
  }

  function handleDoubleClick(event: MouseEvent) {
    if ((event.target as Element).closest(".listing-item")) return;
    if (!options.isActive() || getters.currentPromptName()) return;
    if (event.ctrlKey || event.metaKey || event.shiftKey) return; // Don't interfere when pressing any mod key
    mutations.selectAllItems({ multiple: getters.isMobile() });
  }

  watch(options.items, invalidateRects);

  // The container can appear and disappear so listeners follow the element
  watch(
    options.container,
    (el, _previous, onCleanup) => {
      if (!el) return;
      el.addEventListener("mousedown", startRectangleSelection);
      el.addEventListener("dblclick", handleDoubleClick);
      onCleanup(() => {
        el.removeEventListener("mousedown", startRectangleSelection);
        el.removeEventListener("dblclick", handleDoubleClick);
      });
    },
    { immediate: true, flush: "sync" },
  );

  onMounted(() => {
    lastSelected = state.selected;
    window.addEventListener("resize", invalidateRects);
    window.addEventListener("click", clickClear);
    document.addEventListener("mousemove", updateRectangleSelection, { passive: true });
    document.addEventListener("mouseup", endRectangleSelection);
  });

  onBeforeUnmount(() => {
    window.removeEventListener("resize", invalidateRects);
    window.removeEventListener("click", clickClear);
    document.removeEventListener("mousemove", updateRectangleSelection);
    stopRectangleScrollListener();
    if (rafId) {
      cancelAnimationFrame(rafId);
      rafId = null;
    }
    document.removeEventListener("mouseup", endRectangleSelection);
  });

  return { isRectangleSelecting, rectangleStyle };
}
