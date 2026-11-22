import type { ShallowRef } from "vue";
// The only value import of `plyr`: the whole player is created and destroyed here so the
// Video.js 10 migration has a single place to replace. Its stylesheet and our shared
// overrides are imported next to it, in this order.
import Plyr from "plyr";
import "plyr/dist/plyr.css";
import "@/css/player.css";
import { mutations, state } from "@/store/index.ts";
import { getEndOfMediaAction, navigatePlaybackQueue } from "@/utils/playbackQueue.js";
import { buildItemUrl, removeTrailingSlash } from "@/utils/url.js";

/** Fullscreen state carried across a navigation between two media files (module-level on purpose). */
let pendingFullscreenResume = false;
let pendingResumeTimestamp = 0;

/**
 * Percent-decode a route path, tolerating malformed escapes. Vue Router and
 * buildItemUrl do not always encode the same characters (spaces, quotes,
 * parentheses), so route-leave checks compare both forms.
 */
function decodePathSafely(path: string): string {
  try {
    return decodeURI(path);
  } catch (_) {
    return path;
  }
}

type NativePlay = () => Promise<void> | void;

/** Plyr keeps `media` and a no-arg `off()` at runtime but its typings omit them. */
interface PlyrRuntime {
  media?: HTMLMediaElement;
  off: () => void;
}

export interface MountedFileKey {
  source: string;
  path: string;
}

/** The source/path a player instance mounts for; used to detect a route change away from it. */
export function resolveMountedFileKey(source?: string, path?: string): MountedFileKey {
  return {
    source: source || (state.sources?.current as string | undefined) || "",
    path: path ? (path.startsWith("/") ? path : `/${path}`) : "",
  };
}

interface MediaPlayerOptions {
  player: ShallowRef<Plyr | null>;
  /** The <audio>/<video> element rendered by the component. */
  getMediaElement: () => HTMLMediaElement | null | undefined;
  /** Source/path of the file currently requested (may be cleared during a close). */
  getRequestedFile: () => { source?: string; path?: string } | null | undefined;
  /** Source/path this instance mounted for. */
  getMountedFile: () => MountedFileKey | null | undefined;
  isTornDown: () => boolean;
  /** Full teardown of the owning component state (called when the route left the file). */
  destroy: () => void;
  /** Runs before an element is stopped, e.g. to cancel a pending stream attach. */
  beforeStopMedia?: () => void;
}

/** Creates, wires, stops and destroys the Plyr instance, and ends/advances playback. */
export function useMediaPlayer(options: MediaPlayerOptions) {
  const { player } = options;
  let nativePlay: NativePlay | null = null;

  function createPlayer(plyrOptions: Plyr.Options): Plyr | null {
    const el = options.getMediaElement();
    if (!el) return null;
    player.value = new Plyr(el, plyrOptions);
    return player.value;
  }

  /**
   * Replaces `play()` so the handler can attach a deferred source first; the
   * original stays reachable through `getNativePlay()`.
   */
  function overridePlay(handler: (nativePlay: NativePlay) => Promise<void> | void): void {
    const current = player.value;
    if (!current) return;
    nativePlay = current.play.bind(current);
    current.play = () => handler(() => (nativePlay as NativePlay)());
  }

  function getNativePlay(): NativePlay | null {
    return nativePlay;
  }

  function restoreNativePlay(): void {
    if (player.value && nativePlay) {
      player.value.play = nativePlay;
    }
    nativePlay = null;
  }

  /** Subscribes to player events; `player.off()` in `destroyPlayer` removes them all. */
  function onPlayerEvent(event: Plyr.StandardEvent | Plyr.Html5Event, fn: () => void): void {
    player.value?.on(event, fn);
  }

  function getActiveMediaElement(): HTMLMediaElement | null {
    return (player.value as unknown as PlyrRuntime | null)?.media ?? options.getMediaElement() ?? null;
  }

  function stopMediaElement(media: HTMLMediaElement | null = null): void {
    const el = media ?? getActiveMediaElement();
    if (!el) {
      return;
    }
    options.beforeStopMedia?.();
    try {
      el.pause();
    } catch (_) {
      // ignore
    }
    el.removeAttribute("src");
    try {
      el.load();
    } catch (_) {
      // ignore
    }
  }

  /** Remembers fullscreen so the next file opened right after this one can re-enter it. */
  function rememberFullscreenForTransition(): void {
    pendingFullscreenResume = player.value?.fullscreen?.active || false;
    pendingResumeTimestamp = Date.now();
  }

  /** Re-enters fullscreen if the previous file left it pending (stale after 3s). */
  function resumeFullscreenAfterNavigation(): void {
    const isStale = Date.now() - pendingResumeTimestamp > 3000;
    if (pendingFullscreenResume) {
      pendingFullscreenResume = false;
      if (!isStale) player.value?.fullscreen?.enter();
    }
  }

  /** Removes listeners, stops the element and destroys the instance. No-op without an instance. */
  function destroyPlayer(media: HTMLMediaElement | null): void {
    const current = player.value;
    if (!current) {
      stopMediaElement(media);
      return;
    }
    (current as unknown as PlyrRuntime).off();
    stopMediaElement(media);
    current.destroy();
    player.value = null;
    // hls.js can re-attach its MediaSource while Plyr tears down; stop the
    // element once more so no audio survives the unmount.
    stopMediaElement(media);
  }

  /** Stops playback when the route no longer points at the file this instance mounted for. */
  function stopIfRouteLeftThisFile(): void {
    if (options.isTornDown()) {
      return;
    }
    const requested = options.getRequestedFile();
    const mounted = options.getMountedFile();
    const source = mounted?.source ?? requested?.source;
    const path = mounted?.path ?? requested?.path;
    if (!path) {
      options.destroy();
      return;
    }
    const expected = buildItemUrl(source as string, path);
    const current = state.route.path as string;
    const normalizedCurrent = removeTrailingSlash(current);
    const normalizedExpected = removeTrailingSlash(expected);
    if (
      normalizedCurrent === normalizedExpected ||
      current === expected ||
      current.startsWith(`${expected}#`)
    ) {
      return;
    }
    // Encoding differences must not look like a route change while the file
    // is still open, nor hide a real departure.
    if (
      decodePathSafely(normalizedCurrent) === decodePathSafely(normalizedExpected) ||
      decodePathSafely(current).startsWith(`${decodePathSafely(expected)}#`)
    ) {
      return;
    }
    options.destroy();
  }

  function togglePlayPause(): void {
    if (options.isTornDown()) {
      return;
    }
    if (player.value) {
      player.value.togglePlay();
      return;
    }
    const media = options.getMediaElement();
    if (!media) return;
    if (media.paused) media.play();
    else media.pause();
  }

  function restartCurrentFile(): void {
    if (player.value) {
      player.value.currentTime = 0;
      player.value.play();
    }
  }

  function handleMediaEnd(): void {
    mutations.setPlaybackState(false);
    const { queue, currentIndex, loop } = state.playbackQueue;
    const action = getEndOfMediaAction(queue, currentIndex, loop);
    if (action === "next") {
      navigatePlaybackQueue(1);
    } else if (action === "restart") {
      restartCurrentFile();
    }
  }

  return {
    createPlayer,
    overridePlay,
    getNativePlay,
    restoreNativePlay,
    onPlayerEvent,
    getActiveMediaElement,
    stopMediaElement,
    rememberFullscreenForTransition,
    resumeFullscreenAfterNavigation,
    destroyPlayer,
    stopIfRouteLeftThisFile,
    togglePlayPause,
    restartCurrentFile,
    handleMediaEnd,
  };
}
