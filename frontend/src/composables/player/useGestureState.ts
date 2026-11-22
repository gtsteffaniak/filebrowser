import { onBeforeUnmount, ref } from "vue";

/** Tap/long-press shared by the double-tap seek and the swipe/long-press gestures, both staying in sync */
export function useGestureState() {
  const skipNextTap = ref(false);
  let skipNextTapTimer: ReturnType<typeof setTimeout> | null = null;

  const edgeTapLastTime = ref(0);
  const edgeTapLastZone = ref<string | null>(null);

  // Long-press for 2x speed (touch only)
  const longPressTimer = ref<number | null>(null);
  const longPressPending = ref(false);
  const longPressTriggered = ref(false);
  const longPressPreviousSpeed = ref(1);

  function clearEdgeTapGestureState(): void {
    edgeTapLastTime.value = 0;
    edgeTapLastZone.value = null;
  }

  function clearLongPressTimer(): void {
    if (longPressTimer.value) {
      clearTimeout(longPressTimer.value);
      longPressTimer.value = null;
    }
  }

  function clearSkipNextTap(): void {
    skipNextTap.value = false;
    if (skipNextTapTimer) {
      clearTimeout(skipNextTapTimer);
      skipNextTapTimer = null;
    }
  }

  function setSkipNextTap(delay: number): void {
    clearEdgeTapGestureState();
    if (skipNextTapTimer) {
      clearTimeout(skipNextTapTimer);
      skipNextTapTimer = null;
    }
    skipNextTap.value = true;
    skipNextTapTimer = setTimeout(() => {
      skipNextTap.value = false;
      skipNextTapTimer = null;
    }, delay);
  }

  onBeforeUnmount(clearSkipNextTap);

  return {
    skipNextTap,
    edgeTapLastTime,
    edgeTapLastZone,
    longPressTimer,
    longPressPending,
    longPressTriggered,
    longPressPreviousSpeed,
    clearEdgeTapGestureState,
    clearLongPressTimer,
    clearSkipNextTap,
    setSkipNextTap,
  };
}

export type GestureState = ReturnType<typeof useGestureState>;
