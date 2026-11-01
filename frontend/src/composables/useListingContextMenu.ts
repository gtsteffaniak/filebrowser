import { watch, type Ref } from "vue";
import { state, getters, mutations } from "@/store/index.ts";

interface ContextMenuTriggerOptions {
  getProps?: () => Record<string, unknown>;
  beforeOpen?: (target: EventTarget | null) => boolean | undefined;
}

export function useListingContextMenu(
  container: Ref<HTMLElement | null>,
  options: ContextMenuTriggerOptions = {},
) {
  let contextTimeout: ReturnType<typeof setTimeout> | null = null;
  let isSwipe = false;
  let touchStartX = 0;
  let touchStartY = 0;
  let touchTarget: EventTarget | null = null;

  function openContext(position: { posX: number; posY: number }) {
    // Prevent opening if already open
    if (getters.currentPromptName() === "ContextMenu") {
      return;
    }
    mutations.showPrompt({
      name: "ContextMenu",
      props: {
        showCentered: getters.isMobile(),
        ...position,
        ...options.getProps?.(),
      },
    });
  }

  function canOpen(target: EventTarget | null) {
    return getters.currentPromptName() === "ContextMenu" || options.beforeOpen?.(target) !== false;
  }

  function onContextMenu(event: MouseEvent) {
    if (!canOpen(event.target)) return;
    event.preventDefault();
    event.stopPropagation();
    openContext({ posX: event.clientX, posY: event.clientY });
  }

  function cancelContext() {
    if (contextTimeout) {
      clearTimeout(contextTimeout);
      contextTimeout = null;
    }
  }

  function openContextForSafari(event: TouchEvent) {
    cancelContext(); // Clear any previous timeouts
    isSwipe = false; // Reset swipe detection

    const touch = event.touches[0];
    if (!touch) return;
    touchStartX = touch.clientX;
    touchStartY = touch.clientY;
    touchTarget = event.target;
    contextTimeout = setTimeout(() => {
      if (!isSwipe && canOpen(touchTarget)) {
        openContext({ posX: touchStartX, posY: touchStartY });
      }
    }, 500);
  }

  function handleTouchMove(event: TouchEvent) {
    const touch = event.touches[0];
    if (!touch) return;
    const deltaX = Math.abs(touch.clientX - touchStartX);
    const deltaY = Math.abs(touch.clientY - touchStartY);
    // Set a threshold for movement to detect a swipe
    const movementThreshold = 10;
    if (deltaX > movementThreshold || deltaY > movementThreshold) {
      isSwipe = true;
      cancelContext(); // Cancel long press if swipe is detected
    }
  }

  // The container can appear and disappear, so listeners follow the element
  watch(
    container,
    (el, _previous, onCleanup) => {
      if (!el) return;
      el.addEventListener("contextmenu", onContextMenu);
      if (state.isSafari) {
        // add touchstart to open the context menu
        el.addEventListener("touchstart", openContextForSafari, { passive: true });
        el.addEventListener("touchmove", handleTouchMove, { passive: true });
        // clear the timeout if the user taps quickly
        el.addEventListener("touchend", cancelContext);
      }
      onCleanup(() => {
        el.removeEventListener("contextmenu", onContextMenu);
        el.removeEventListener("touchstart", openContextForSafari);
        el.removeEventListener("touchmove", handleTouchMove);
        el.removeEventListener("touchend", cancelContext);
        cancelContext();
      });
    },
    { immediate: true, flush: "sync" },
  );
}
