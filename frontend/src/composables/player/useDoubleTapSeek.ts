import { onBeforeUnmount, ref } from "vue";
import type { ShallowRef } from "vue";
import type Plyr from "plyr";
import { getters, mutations, state } from "@/store/index.ts";
import { globalVars } from "@/utils/constants.ts";
import { zoneFromClientX } from "@/player/gestureZones.js";
import { enableOverlaidHintController } from "@/player/overlaidHintController.js";
import { isInLeftNavTapZone, isInRightNavTapZone } from "@/utils/navigationEdgeZones.js";
import { getPlayerSeekTime } from "./settingsElements.ts";
import type { GestureState } from "./useGestureState.ts";

interface DoubleTapSeekOptions {
  player: Readonly<ShallowRef<Plyr | null>>;
  gestures: GestureState;
  getPreviewType: () => "audio" | "video";
  /** Gesture surface of the audio player (the whole player, not only the Plyr wrapper). */
  getAudioGestureRoot: () => HTMLElement | null | undefined;
  /** Starts playback from the surface; the owner decides about transcode offers. */
  requestPlay: () => void;
}

/**
 * Double-tap / double-click seek on the left and right edges, the rewind and
 * fast-forward feedback flash, and the "overlaid" play-button hint.
 */
export function useDoubleTapSeek({
  player,
  gestures,
  getPreviewType,
  getAudioGestureRoot,
  requestPlay,
}: DoubleTapSeekOptions) {
  const skipFeedbackVisible = ref(false);
  const skipFeedbackSide = ref<"left" | "right">("left");
  const skipFeedbackIcon = ref("replay_10");
  const skipFeedbackKey = ref(0);
  const hasStartedPlayback = ref(false);

  let skipFeedbackTimer: ReturnType<typeof setTimeout> | null = null;
  let doubleTapSeekCleanup: (() => void) | null = null;
  let rewindForwardFeedback: (() => void) | null = null;
  let overlaidHintCleanup: (() => void) | null = null;
  let overlaidHintApi: ReturnType<typeof enableOverlaidHintController> | null = null;
  let ignoreClickUntil = 0;

  function setupDoubleTapSeek() {
    if (!player.value) {
      return;
    }
    teardownDoubleTapSeek();
    const surface = getGestureSurface();
    if (!surface || !player.value) return;

    const EDGE_CLICK_MS = 200;

    const peekNavChromeForEdgeTap = (clientX: number, zone: string) => {
      if (!state.navigation.enabled) {
        return;
      }
      const moveWithSidebar = getters.isSidebarVisible() && getters.isStickySidebar();
      const sidebarWidthEm = state.sidebar?.width || 20;
      const opts = { moveWithSidebar, sidebarWidthEm };
      if (zone === 'left' && isInLeftNavTapZone(clientX, opts)) {
        mutations.peekNavigationChrome('left');
      } else if (zone === 'right' && isInRightNavTapZone(clientX)) {
        mutations.peekNavigationChrome('right');
      }
    };

    const zoneFromSurfaceX = (clientX: number) => {
      const rect = surface.getBoundingClientRect();
      return zoneFromClientX(clientX, rect);
    };
    const OVERLAID_BUTTON_TOUCH_PADDING = 8;
    const isOverlaidButtonHit = (clientX: number, clientY: number) => {
      const btn = player.value?.elements?.container?.querySelector('.plyr__control--overlaid');
      if (!btn) return false;
      const r = btn.getBoundingClientRect();
      if (!r.width || !r.height) return false;
      const cx = r.left + (r.width / 2);
      const cy = r.top + (r.height / 2);
      const radius = (Math.min(r.width, r.height) / 2) + OVERLAID_BUTTON_TOUCH_PADDING;
      const dx = clientX - cx;
      const dy = clientY - cy;
      return (dx * dx) + (dy * dy) <= radius * radius;
    };

    const applySeek = (rewind: boolean) => {
      gestures.clearEdgeTapGestureState();
      gestures.clearLongPressTimer();
      gestures.longPressPending.value = false;

      const current = player.value;
      if (!current) return;
      const step = getPlayerSeekTime(current) || 10;
      const cur = current.currentTime;
      const dur = current.duration;
      const next = rewind ? cur - step : cur + step;
      const max = Number.isFinite(dur) && dur > 0 ? dur : next;
      current.currentTime = Math.max(0, Math.min(next, max));
      flashSkipFeedback(rewind);
    };

    const togglePlayPause = () => {
      if (player.value?.playing) {
        player.value.pause();
        return;
      }
      requestPlay();
    };
    const handleEdgeZoneTap = (zone: string, event?: Event) => {
      const now = Date.now();
      if (zone === gestures.edgeTapLastZone.value && now - gestures.edgeTapLastTime.value < 300) {
        applySeek(zone === 'left');
        if (event) {
          event.preventDefault();
          event.stopPropagation();
        }
        return;
      }
      gestures.edgeTapLastTime.value = now;
      gestures.edgeTapLastZone.value = zone;
      if (event) {
        event.preventDefault();
        event.stopPropagation();
      }
    };

    const handleCenterTap = (event?: Event) => {
      gestures.clearEdgeTapGestureState();
      if (getPreviewType() === 'video') {
        togglePlayPause();
      }
      if (event) {
        event.preventDefault();
        event.stopPropagation();
      }
    };

    const onTouchEnd = (event: TouchEvent) => {
      if (gestures.longPressTriggered.value) {
        return;
      }
      if (gestures.skipNextTap.value) return;

      if (event.changedTouches.length !== 1) return;
      const t = event.changedTouches[0];
      if (!t) return;
      const topEl = typeof document.elementFromPoint === 'function'
        ? document.elementFromPoint(t.clientX, t.clientY)
        : null;
      if (isControlOrMenuTarget(topEl)) {
        gestures.clearEdgeTapGestureState();
        return;
      }
      if (isOverlaidButtonHit(t.clientX, t.clientY)) {
        handleCenterTap(event);
        ignoreClickUntil = Date.now() + 500;
        return;
      }
      const zone = zoneFromSurfaceX(t.clientX);
      if (zone === 'left' || zone === 'right') {
        handleEdgeZoneTap(zone, event);
        peekNavChromeForEdgeTap(t.clientX, zone);
        ignoreClickUntil = Date.now() + 500;
        return;
      }
      gestures.clearEdgeTapGestureState();
      ignoreClickUntil = Date.now() + 500;
    };
    let edgeClickToggleTimer: ReturnType<typeof setTimeout> | null = null;
    const clearEdgeClickToggleTimer = () => {
      if (edgeClickToggleTimer) {
        clearTimeout(edgeClickToggleTimer);
        edgeClickToggleTimer = null;
      }
    };
    const onClick = (event: MouseEvent) => {
      if (isControlOrMenuTarget(event.target)) return;
      if (Date.now() < ignoreClickUntil) {
        event.preventDefault();
        event.stopPropagation();
        return;
      }
      if (gestures.skipNextTap.value) return;
      gestures.clearEdgeTapGestureState();
      const zone = zoneFromSurfaceX(event.clientX);
      if (zone === 'left' || zone === 'right') {
        if (event.detail >= 2) {
          clearEdgeClickToggleTimer();
        } else {
          clearEdgeClickToggleTimer();
          edgeClickToggleTimer = setTimeout(() => {
            edgeClickToggleTimer = null;
            if (getPreviewType() === 'video') {
              togglePlayPause();
            }
          }, EDGE_CLICK_MS);
        }
        peekNavChromeForEdgeTap(event.clientX, zone);
      } else if (getPreviewType() === 'video') {
        togglePlayPause();
      }

      event.preventDefault();
      event.stopPropagation();
    };

    const onDblClick = (event: MouseEvent) => {
      if (Date.now() < ignoreClickUntil) {
        return;
      }
      if (isControlOrMenuTarget(event.target)) {
        return;
      }
      const zone = zoneFromSurfaceX(event.clientX);
      if (zone === 'left' || zone === 'right') {
        clearEdgeClickToggleTimer();
        applySeek(zone === 'left');
        event.preventDefault();
        event.stopPropagation();
      }
    };

    surface.addEventListener('touchend', onTouchEnd, { passive: false });
    surface.addEventListener('click', onClick);
    surface.addEventListener('dblclick', onDblClick);

    doubleTapSeekCleanup = () => {
      surface.removeEventListener('touchend', onTouchEnd);
      surface.removeEventListener('click', onClick);
      surface.removeEventListener('dblclick', onDblClick);
      clearEdgeClickToggleTimer();
      gestures.clearEdgeTapGestureState();
    };
  }

  function teardownDoubleTapSeek() {
    if (typeof doubleTapSeekCleanup === 'function') {
      doubleTapSeekCleanup();
      doubleTapSeekCleanup = null;
    }
  }

  function flashSkipFeedback(rewind: boolean) {
    if (skipFeedbackTimer) {
      clearTimeout(skipFeedbackTimer);
    }
    skipFeedbackSide.value = rewind ? 'left' : 'right';
    skipFeedbackIcon.value = rewind ? 'replay_10' : 'forward_10';
    skipFeedbackKey.value += 1;
    skipFeedbackVisible.value = true;
    skipFeedbackTimer = setTimeout(() => {
      skipFeedbackVisible.value = false;
      skipFeedbackTimer = null;
    }, 700);
  }

  function setupRewindForwardFeedback() {
    clearRewindForwardFeedback();
    const container = player.value?.elements?.container;
    if (!container) {
      return;
    }
    const rewind = container.querySelector('[data-plyr="rewind"]');
    const forward = container.querySelector('[data-plyr="fast-forward"]');
    if (!rewind && !forward) {
      return;
    }
    const onRewind = () => flashSkipFeedback(true);
    const onForward = () => flashSkipFeedback(false);
    rewind?.addEventListener('click', onRewind);
    forward?.addEventListener('click', onForward);
    rewindForwardFeedback = () => {
      rewind?.removeEventListener('click', onRewind);
      forward?.removeEventListener('click', onForward);
    };
  }

  function clearRewindForwardFeedback() {
    if (typeof rewindForwardFeedback === 'function') {
      rewindForwardFeedback();
    }
    rewindForwardFeedback = null;
  }

  function setupOverlaidHintController() {
    teardownOverlaidHintController();
    if (!player.value) {
      return;
    }
    const api = enableOverlaidHintController({
      player: player.value,
      hasStartedPlayback: () => hasStartedPlayback.value,
      baseUrl: globalVars.baseURL,
      isPlaying: (state.playbackQueue.isPlaying || false),
    });
    overlaidHintApi = api;
    overlaidHintCleanup = api.cleanup;
  }

  function teardownOverlaidHintController() {
    if (typeof overlaidHintCleanup === 'function') {
      overlaidHintCleanup();
    }
    overlaidHintCleanup = null;
    overlaidHintApi = null;
  }


  function getGestureSurface() {
    if (
      getPreviewType() === 'audio' &&
      player.value &&
      getAudioGestureRoot()
    ) {
      return getAudioGestureRoot();
    }
    if (!player.value?.elements) {
      return null;
    }
    if (player.value.elements.wrapper) {
      return player.value.elements.wrapper;
    }
    return player.value.elements.container ?? null;
  }

  function isControlOrMenuTarget(target: EventTarget | null | undefined): boolean {
    const el = target as Element | null | undefined;
    if (!el || typeof el.closest !== 'function') return false;
    if (el.closest('.plyr__control--overlaid')) {
      return true;
    }
    return !!el.closest(
      '.plyr__controls, .plyr__control, .plyr__menu__container, .plyr__menu, ' +
      '[data-plyr="seek"], .plyr__progress, [data-plyr="volume"], .plyr__volume, ' +
      '.audio-side-panel .sliding-tab, ' +
      '.fab-button, ' +
      '.audio-side-panel .lyric-line, ' +
      '.audio-side-panel .repeat-one-btn, ' +
      '.audio-side-panel .queue-item, ' +
      '.audio-side-panel .clear-queue-btn, ' +
      '.lyrics-mobile .lyric-line'
    );
  }

  function resetOverlaidHints(): void {
    overlaidHintApi?.reset();
  }

  function syncOverlaidHintPlayback(isPlaying: boolean): void {
    overlaidHintApi?.syncPlayback(isPlaying);
  }

  onBeforeUnmount(() => {
    if (skipFeedbackTimer) {
      clearTimeout(skipFeedbackTimer);
    }
  });

  return {
    skipFeedbackVisible,
    skipFeedbackSide,
    skipFeedbackIcon,
    skipFeedbackKey,
    hasStartedPlayback,
    flashSkipFeedback,
    setupDoubleTapSeek,
    teardownDoubleTapSeek,
    setupRewindForwardFeedback,
    clearRewindForwardFeedback,
    setupOverlaidHintController,
    teardownOverlaidHintController,
    resetOverlaidHints,
    syncOverlaidHintPlayback,
    getGestureSurface,
    isControlOrMenuTarget,
  };
}
