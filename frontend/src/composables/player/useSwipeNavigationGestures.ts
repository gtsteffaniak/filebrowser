import { computed, ref } from "vue";
import type { ShallowRef } from "vue";
import type Plyr from "plyr";
import { getters, mutations, state } from "@/store/index.ts";
import type { GestureState } from "./useGestureState.ts";

type EdgeKind = "horizontal" | "vertical-dismiss" | "vertical-fullscreen";
type NavHintDir = "prev" | "next";

interface SwipeNavigationOptions {
  player: Readonly<ShallowRef<Plyr | null>>;
  gestures: GestureState;
  getPreviewType: () => "audio" | "video";
  /** The swipe is paused while the mobile lyrics overlay is scrolling */
  isMobileLyricsActive: () => boolean;
  /** The element the touch/mouse listeners are attached to. */
  getGestureSurface: () => HTMLElement | null | undefined;
  isControlOrMenuTarget: (target: EventTarget | null) => boolean;
  onNavigatePrevious: () => void;
  onNavigateNext: () => void;
  onClosePreview: () => void;
}

/**
 * Edge/swipe gestures for audio and video: horizontal swipe to the
 * previous/next file, vertical swipe down to close (or leave fullscreen), swipe
 * up to toggle fullscreen on video, and the touch long-press 2x speed.
 */
export function useSwipeNavigationGestures({
  player,
  gestures,
  getPreviewType,
  isMobileLyricsActive,
  getGestureSurface,
  isControlOrMenuTarget,
  onNavigatePrevious,
  onNavigateNext,
  onClosePreview,
}: SwipeNavigationOptions) {
  let edgeKind: EdgeKind | null = null;
  let edgeStartX = 0;
  let edgeStartY = 0;
  let edgeDx = 0;
  let edgeDy = 0;
  let dragOffsetX = 0;
  let dragOffsetY = 0;
  let gestureSnapBack = false;
  let edgeMouseActive = false;
  let showNavHint = false;
  let navHintDir: NavHintDir = "next";
  let showDismissHint = false;
  let dismissFlashActive = false;
  const edgeHintPx = 44;
  const edgeCommitX = 130;
  const edgeCommitY = 110;
  const edgeRubberMax = 100;
  let swipeCleanup: (() => void) | null = null;
  let swipeSuppressedTouchId: number | null = null;
  let swipeActiveTouchId: number | null = null;
  let snapBackTimer: ReturnType<typeof setTimeout> | null = null;
  let dismissCloseTimer: ReturnType<typeof setTimeout> | null = null;
  let dismissHintTimer: ReturnType<typeof setTimeout> | null = null;

  /** Toast shown while the long-press 2x speed is held. */
  const speedToastVisible = ref(false);
  const speedToastMessage = ref("");

  const swipeGesturesActive = computed(
    () =>
      !!player.value &&
      !isMobileLyricsActive(),
  );
  const navigationGestureAllowed = computed(
    () => state.navigation.enabled && getters.currentPrompt() === null,
  );
  const hasPreviousNav = computed(() => {
    if (getters.isPreviewPlaybackQueueNavMode()) {
      return navigationGestureAllowed.value && getters.playbackQueueCanGoPrevious();
    }
    return navigationGestureAllowed.value && state.navigation.previousLink !== "";
  });
  const hasNextNav = computed(() => {
    if (getters.isPreviewPlaybackQueueNavMode()) {
      return navigationGestureAllowed.value && getters.playbackQueueCanGoNext();
    }
    return navigationGestureAllowed.value && state.navigation.nextLink !== "";
  });

  function applySwipeTransform() {
    const el = getGestureSurface();
    if (!el) {
      return;
    }
    const transition = gestureSnapBack
      ? 'transform 0.22s cubic-bezier(0.32, 0.72, 0, 1)'
      : 'none';
    el.style.transition = transition;
    const x = dragOffsetX;
    const y = dragOffsetY;
    el.style.transform = x || y ? `translate(${x}px, ${y}px)` : '';
  }
  function edgeCommitState() {
    const ax = Math.abs(edgeDx);
    const ay = Math.abs(edgeDy);
    return {
      previous: hasPreviousNav.value && edgeDx >= edgeCommitX && ax >= ay,
      next: hasNextNav.value && edgeDx <= -edgeCommitX && ax >= ay,
      dismiss: edgeDy >= edgeCommitY && ay >= ax,
    };
  }
  function syncNavigationGestureHintToStore() {
    const commit = edgeCommitState();
    const navPrevReady = commit.previous;
    const navNextReady = commit.next;
    const dismissReady = commit.dismiss;
    let kind: "close" | "previous" | "next" | null = null;
    let commitReady = false;
    let flashClose = false;
    if (dismissFlashActive) {
      kind = 'close';
      commitReady = dismissReady;
      flashClose = true;
    } else if (showDismissHint) {
      kind = 'close';
      commitReady = dismissReady;
    } else if (showNavHint && navHintDir === 'prev' && hasPreviousNav.value) {
      kind = 'previous';
      commitReady = navPrevReady;
    } else if (showNavHint && navHintDir === 'next' && hasNextNav.value) {
      kind = 'next';
      commitReady = navNextReady;
    }
    mutations.setNavigationGestureHint({ kind, commitReady, flashClose });
  }
  function rubberband(value: number, max: number): number {
    const sign = value < 0 ? -1 : 1;
    const a = Math.abs(value);
    if (a <= max) {
      return value;
    }
    return sign * (max + (a - max) * 0.32);
  }
  function decideEdgeKind() {
    if (edgeKind) return;
    const ax = Math.abs(edgeDx);
    const ay = Math.abs(edgeDy);
    if (ax < 12 && ay < 12) return;
    if (getPreviewType() === 'audio') {
      if (ay > ax * 1.12 && ay > 14 && edgeDy > 0) {
        edgeKind = 'vertical-dismiss';
      } else if (ax > ay * 1.12 && ax > 14) {
        edgeKind = 'horizontal';
      }
      return;
    }
    if (ay > ax * 1.12 && ay > 14) {
      edgeKind = edgeDy > 0 ? 'vertical-dismiss' : 'vertical-fullscreen';
    } else if (ax > ay * 1.12 && ax > 14) {
      edgeKind = 'horizontal';
    }
  }
  function applyEdgeVisuals() {
    if (getPreviewType() === 'audio' && edgeDy < 0 && Math.abs(edgeDy) > Math.abs(edgeDx)) {
      dragOffsetX = 0;
      dragOffsetY = 0;
      showNavHint = false;
      showDismissHint = false;
      applySwipeTransform();
      syncNavigationGestureHintToStore();
      return;
    }

    const kind = edgeKind;
    if (kind === 'horizontal') {
      dragOffsetX = rubberband(edgeDx, edgeRubberMax);
      dragOffsetY = 0;
      const adx = Math.abs(edgeDx);
      showNavHint = adx >= edgeHintPx;
      navHintDir = edgeDx > 0 ? 'prev' : 'next';
      if (navHintDir === 'prev' && !hasPreviousNav.value) showNavHint = false;
      if (navHintDir === 'next' && !hasNextNav.value) showNavHint = false;
      showDismissHint = false;
    } else if (kind === 'vertical-dismiss') {
      dragOffsetX = 0;
      dragOffsetY = edgeDy;
      showDismissHint = edgeDy >= edgeHintPx;
      showNavHint = false;
    } else if (kind === 'vertical-fullscreen') {
      dragOffsetX = 0;
      dragOffsetY = edgeDy;
      showDismissHint = false;
      showNavHint = false;
    } else {
      dragOffsetX = 0;
      dragOffsetY = 0;
      showNavHint = false;
      showDismissHint = false;
    }
    applySwipeTransform();
    syncNavigationGestureHintToStore();
  }
  function clearSnapBackTimer() {
    if (snapBackTimer) {
      clearTimeout(snapBackTimer);
      snapBackTimer = null;
    }
  }
  function findTouchById(list: TouchList, id: number | null): Touch | null {
    if (id === null) return null;
    for (let i = 0; i < list.length; i++) {
      const touch = list.item(i);
      if (touch && touch.identifier === id) return touch;
    }
    return null;
  }
  function snapBackEdgeGesture() {
    clearSnapBackTimer();
    gestureSnapBack = true;
    dragOffsetX = 0;
    dragOffsetY = 0;
    showNavHint = false;
    showDismissHint = false;
    edgeKind = null;
    edgeDx = 0;
    edgeDy = 0;
    gestures.setSkipNextTap(200);
    applySwipeTransform();
    mutations.setNavigationGestureHint({});
    snapBackTimer = setTimeout(() => {
      snapBackTimer = null;
      gestureSnapBack = false;
      applySwipeTransform();
    }, 240);
  }
  function resetSwipeVisualState() {
    clearDismissAnimTimers();
    clearSnapBackTimer();
    swipeSuppressedTouchId = null;
    swipeActiveTouchId = null;
    edgeKind = null;
    edgeDx = 0;
    edgeDy = 0;
    dragOffsetX = 0;
    dragOffsetY = 0;
    showNavHint = false;
    showDismissHint = false;
    gestureSnapBack = false;
    dismissFlashActive = false;
    applySwipeTransform();
    mutations.setNavigationGestureHint({});
  }
  function resetEdgeGestureImmediate() {
    if (gestures.longPressTriggered.value) {
      releaseLongPressHold();
    }
    resetSwipeVisualState();
    gestures.clearEdgeTapGestureState();
    gestures.clearSkipNextTap();
  }
  function clearDismissAnimTimers() {
    if (dismissCloseTimer) {
      clearTimeout(dismissCloseTimer);
      dismissCloseTimer = null;
    }
    if (dismissHintTimer) {
      clearTimeout(dismissHintTimer);
      dismissHintTimer = null;
    }
  }
  function finishEdgeGesture() {
    if (!swipeGesturesActive.value) {
      resetEdgeGestureImmediate();
      return;
    }
    const ax0 = Math.abs(edgeDx);
    const ay0 = Math.abs(edgeDy);
    if (!edgeKind && ax0 < 5 && ay0 < 5) {
      resetSwipeVisualState();
      return;
    }
    let kind = edgeKind;
    if (!kind) {
      const ax = Math.abs(edgeDx);
      const ay = Math.abs(edgeDy);
      if (ax < edgeHintPx && ay < edgeHintPx) {
        snapBackEdgeGesture();
        return;
      }
      kind = ax >= ay ? 'horizontal' : 'vertical-dismiss';
    }
    const commit = edgeCommitState();
    if (kind === 'horizontal') {
      if (commit.previous) {
        onNavigatePrevious();
        resetEdgeGestureImmediate();
        return;
      }
      if (commit.next) {
        onNavigateNext();
        resetEdgeGestureImmediate();
        return;
      }
    } else if (kind === 'vertical-dismiss') {
      if (commit.dismiss) {
        clearDismissAnimTimers();
        dismissFlashActive = true;
        showDismissHint = true;
        dragOffsetX = 0;
        dragOffsetY = 0;
        edgeKind = null;
        applySwipeTransform();
        syncNavigationGestureHintToStore();
        gestures.setSkipNextTap(200);
        // If we're in fullscreen, exit fullscreen instead of closing preview
        if (player.value?.fullscreen?.active) {
          player.value.fullscreen.exit();
          dismissHintTimer = setTimeout(() => {
            dismissHintTimer = null;
            dismissFlashActive = false;
            showDismissHint = false;
            mutations.setNavigationGestureHint({});
          }, 420);
          return;
        }
        // Normal close preview
        dismissCloseTimer = setTimeout(() => {
          dismissCloseTimer = null;
          onClosePreview();
        }, 120);
        dismissHintTimer = setTimeout(() => {
          dismissHintTimer = null;
          dismissFlashActive = false;
          showDismissHint = false;
          mutations.setNavigationGestureHint({});
        }, 420);
        return;
      }
    } else if (kind === 'vertical-fullscreen') {
      if (!player.value) {
        resetEdgeGestureImmediate();
        return;
      }
      if (edgeDy <= -edgeCommitY) {
        player.value.fullscreen.toggle();
        resetEdgeGestureImmediate();
        // Set skipNextTap to prevent play/pause toggle
        gestures.setSkipNextTap(300);
        return;
      }
    }
    snapBackEdgeGesture();
  }
  function teardownSwipeMouseDocListeners() {
    document.removeEventListener('mousemove', onSwipeMouseDocMove, true);
    document.removeEventListener('mouseup', onSwipeMouseDocUp, true);
    edgeMouseActive = false;
  }
  function onSwipeMouseDocMove(event: MouseEvent) {
    if (!edgeMouseActive || !swipeGesturesActive.value) {
      return;
    }
    edgeDx = event.clientX - edgeStartX;
    edgeDy = event.clientY - edgeStartY;
    decideEdgeKind();
    applyEdgeVisuals();
    if (Math.abs(edgeDx) > 3 || Math.abs(edgeDy) > 3) {
      event.preventDefault();
    }
  }
  function onSwipeMouseDocUp(event: MouseEvent) {
    if (!edgeMouseActive) {
      teardownSwipeMouseDocListeners();
      return;
    }
    edgeDx = event.clientX - edgeStartX;
    edgeDy = event.clientY - edgeStartY;
    finishEdgeGesture();
    teardownSwipeMouseDocListeners();
    if (Math.abs(edgeDx) > 3 || Math.abs(edgeDy) > 3) {
      event.preventDefault();
    }
  }
  function onSwipeMouseDown(event: MouseEvent) {
    if (event.button !== 0 || !swipeGesturesActive.value) return;
    // Don't start a gesture if we are selecting some text
    if ((window.getSelection()?.toString().length ?? 0) > 0) return;
    if (isControlOrMenuTarget(event.target)) {
      return;
    }
    clearDismissAnimTimers();
    teardownSwipeMouseDocListeners();
    edgeMouseActive = true;
    edgeStartX = event.clientX;
    edgeStartY = event.clientY;
    edgeDx = 0;
    edgeDy = 0;
    edgeKind = null;
    document.addEventListener('mousemove', onSwipeMouseDocMove, true);
    document.addEventListener('mouseup', onSwipeMouseDocUp, true);
  }
  function onSwipeTouchStart(event: TouchEvent) {
    if (event.touches.length > 1) {
      gestures.clearLongPressTimer();
      gestures.longPressPending.value = false;
      if (!gestures.longPressTriggered.value) {
        resetEdgeGestureImmediate();
      }
      return;
    }
    if (!swipeGesturesActive.value || event.touches.length !== 1) return;
    swipeActiveTouchId = null;
    // Don't start a gesture if we are selecting some text
    if ((window.getSelection()?.toString().length ?? 0) > 0) return;
    if (isControlOrMenuTarget(event.target)) {
      swipeSuppressedTouchId = event.targetTouches[0]?.identifier ?? null;
      return;
    }
    swipeSuppressedTouchId = null;
    clearDismissAnimTimers();

    // long-press timer
    gestures.clearLongPressTimer();
    gestures.longPressPending.value = true;
    gestures.longPressTriggered.value = false;
    gestures.longPressPreviousSpeed.value = player.value?.speed || 1;

    // only start timer if not already at 2x
    if (player.value && player.value.speed !== 2) {
      gestures.longPressTimer.value = window.setTimeout(() => {
        gestures.longPressTimer.value = null;
        if (player.value && gestures.longPressPending.value) {
          gestures.longPressPreviousSpeed.value = player.value.speed || 1;
          player.value.speed = 2;
          gestures.longPressTriggered.value = true;
          gestures.longPressPending.value = false;
          // Show toast
          speedToastVisible.value = true;
          speedToastMessage.value = '2x';
        }
      }, 500);
    } else {
      // If already at 2x don't change speed again
      if (player.value?.speed === 2) {
        gestures.longPressPending.value = false;
      }
    }

    const touch = event.targetTouches[0];
    if (!touch) return;
    swipeActiveTouchId = touch.identifier;
    edgeStartX = touch.pageX;
    edgeStartY = touch.pageY;
    edgeDx = 0;
    edgeDy = 0;
    edgeKind = null;
    dragOffsetX = 0;
    dragOffsetY = 0;
  }
  function onSwipeTouchMove(event: TouchEvent) {
    if (gestures.longPressTriggered.value) {
      event.preventDefault();
      return;
    }

    // If there's significant movement cancel any pending long press so if when using swipe gestures they don't change speed
    if (gestures.longPressPending.value) {
      const touch = event.targetTouches[0];
      if (!touch) return;
      const dx = Math.abs(touch.pageX - edgeStartX);
      const dy = Math.abs(touch.pageY - edgeStartY);
      if (dx > 20 || dy > 20) {
        gestures.clearLongPressTimer();
        gestures.longPressPending.value = false;
        gestures.clearEdgeTapGestureState();
      }
    }

    if (!swipeGesturesActive.value || event.targetTouches.length !== 1) {
      if (edgeKind || edgeDx || edgeDy) {
        resetEdgeGestureImmediate();
      }
      return;
    }
    const touch = event.targetTouches[0];
    if (!touch) return;
    if (
      swipeSuppressedTouchId !== null &&
      touch.identifier === swipeSuppressedTouchId
    ) {
      return;
    }
    if (touch.identifier !== swipeActiveTouchId) return;
    edgeDx = touch.pageX - edgeStartX;
    edgeDy = touch.pageY - edgeStartY;
    decideEdgeKind();
    applyEdgeVisuals();
    const ax = Math.abs(edgeDx);
    const ay = Math.abs(edgeDy);
    if (edgeKind || ax > 14 || ay > 14) {
      event.preventDefault();
    }
  }
  function releaseLongPressHold() {
    gestures.clearLongPressTimer();
    speedToastVisible.value = false;
    if (player.value && gestures.longPressPreviousSpeed.value !== 2) {
      player.value.speed = gestures.longPressPreviousSpeed.value;
    }
    gestures.longPressTriggered.value = false;
    gestures.clearEdgeTapGestureState();
  }
  function onSwipeTouchEnd(event: TouchEvent) {
    if (!swipeGesturesActive.value) {
      if (
        gestures.longPressPending.value &&
        findTouchById(event.changedTouches, swipeActiveTouchId)
      ) {
        gestures.clearLongPressTimer();
        gestures.longPressPending.value = false;
      }
      if (
        gestures.longPressTriggered.value &&
        findTouchById(event.changedTouches, swipeActiveTouchId)
      ) {
        swipeActiveTouchId = null;
        releaseLongPressHold();
        resetEdgeGestureImmediate();
        event.preventDefault();
      }
      return;
    }
    if (event.changedTouches.length === 0) {
      return;
    }
    const first = event.changedTouches[0];
    if (!first) return;
    if (
      swipeSuppressedTouchId !== null &&
      first.identifier === swipeSuppressedTouchId
    ) {
      swipeSuppressedTouchId = null;
      return;
    }
    const t = findTouchById(event.changedTouches, swipeActiveTouchId);
    if (!t) return;
    swipeActiveTouchId = null;
    if (gestures.longPressPending.value) {
      gestures.clearLongPressTimer();
      gestures.longPressPending.value = false;
    }
    // Handle long-press release
    if (gestures.longPressTriggered.value) {
      releaseLongPressHold();
      resetEdgeGestureImmediate();
      event.preventDefault();
      return;
    }
    // Normal swipe gesture
    edgeDx = t.pageX - edgeStartX;
    edgeDy = t.pageY - edgeStartY;
    const ax = Math.abs(edgeDx);
    const ay = Math.abs(edgeDy);
    const hadLockedKind = edgeKind !== null;
    finishEdgeGesture();
    if (hadLockedKind || ax > 14 || ay > 14) {
      gestures.clearEdgeTapGestureState();
      event.preventDefault();
    }
  }
  function onSwipeTouchCancel(event?: TouchEvent) {
    swipeActiveTouchId = null;
    // If long-press was triggered, handle release
    if (gestures.longPressTriggered.value) {
      gestures.clearLongPressTimer();
      speedToastVisible.value = false;
      if (player.value && gestures.longPressPreviousSpeed.value !== 2) {
        player.value.speed = gestures.longPressPreviousSpeed.value;
      }
      gestures.longPressTriggered.value = false;
      gestures.longPressPending.value = false;
      resetEdgeGestureImmediate();
      if (event) event.preventDefault();
      return;
    }
    // If long-press was pending clear it
    if (gestures.longPressPending.value) {
      gestures.clearLongPressTimer();
      gestures.longPressPending.value = false;
    }

    if (event?.changedTouches?.length) {
      const t = event.changedTouches[0];
      if (!t) return;
      if (
        swipeSuppressedTouchId !== null &&
        t.identifier === swipeSuppressedTouchId
      ) {
        swipeSuppressedTouchId = null;
        return;
      }
    }
    if (edgeKind || edgeDx || edgeDy) {
      snapBackEdgeGesture();
    }
  }
  function setupSwipeGestures() {
    teardownSwipeGestures();
    if (!player.value) {
      return;
    }
    const surface = getGestureSurface();
    if (!surface) {
      return;
    }
    const touchOpts: AddEventListenerOptions = { passive: false };
    surface.addEventListener('touchstart', onSwipeTouchStart, touchOpts);
    surface.addEventListener('touchmove', onSwipeTouchMove, touchOpts);
    surface.addEventListener('touchend', onSwipeTouchEnd, touchOpts);
    surface.addEventListener('touchcancel', onSwipeTouchCancel, touchOpts);
    surface.addEventListener('mousedown', onSwipeMouseDown);

    swipeCleanup = () => {
      surface.removeEventListener('touchstart', onSwipeTouchStart, touchOpts);
      surface.removeEventListener('touchmove', onSwipeTouchMove, touchOpts);
      surface.removeEventListener('touchend', onSwipeTouchEnd, touchOpts);
      surface.removeEventListener('touchcancel', onSwipeTouchCancel, touchOpts);
      surface.removeEventListener('mousedown', onSwipeMouseDown);
      teardownSwipeMouseDocListeners();
    };
  }
  function teardownSwipeGestures() {
    if (typeof swipeCleanup === 'function') {
      swipeCleanup();
      swipeCleanup = null;
    }
    clearDismissAnimTimers();
    resetEdgeGestureImmediate();
    gestures.clearLongPressTimer();
    gestures.clearLongPressTimer();
    gestures.longPressPending.value = false;
    speedToastVisible.value = false;
    if (gestures.longPressTriggered.value && player.value && gestures.longPressPreviousSpeed.value !== 2) {
      player.value.speed = gestures.longPressPreviousSpeed.value;
    }
    gestures.longPressTriggered.value = false;
  }

  return {
    speedToastVisible,
    speedToastMessage,
    setupSwipeGestures,
    teardownSwipeGestures,
    clearDismissAnimTimers,
    resetEdgeGestureImmediate,
  };
}
