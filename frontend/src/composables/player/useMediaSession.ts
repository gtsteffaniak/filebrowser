import type { ShallowRef } from "vue";
import type Plyr from "plyr";
import { state } from "@/store/index.ts";
import { globalVars } from "@/utils/constants.ts";
import { ownsMediaSession } from "@/utils/mediaSessionOwnership.js";
import { getNextItem, navigatePlaybackQueue } from "@/utils/playbackQueue.js";

/** Latest owner id handed out; only the instance holding it may touch the global Media Session. */
let mediaSessionOwner = 0;

export interface MediaSessionInfo {
  title: string;
  artist: string;
  album: string;
  /** Embedded artwork or thumbnail; falls back to the login icon when absent. */
  artworkSrc?: string;
}

/** Browser Media Session metadata, transport handlers and playback state for one player. */
export function useMediaSession(player: Readonly<ShallowRef<Plyr | null>>) {
  let ownerId = 0;

  function owns(): boolean {
    return ownsMediaSession(ownerId, mediaSessionOwner);
  }

  function updateMediaSessionNavHandlers(): void {
    if (!("mediaSession" in navigator) || !owns()) return;
    const { queue, currentIndex, loop } = state.playbackQueue;
    const hasPrevious = queue.length > 1 && !!getNextItem(queue, currentIndex, loop, -1);
    const hasNext = queue.length > 1 && !!getNextItem(queue, currentIndex, loop, 1);
    try {
      navigator.mediaSession.setActionHandler(
        "previoustrack",
        hasPrevious ? () => navigatePlaybackQueue(-1) : null,
      );
    } catch (_e) {
      /* ignore */
    }
    try {
      navigator.mediaSession.setActionHandler(
        "nexttrack",
        hasNext ? () => navigatePlaybackQueue(1) : null,
      );
    } catch (_e) {
      /* ignore */
    }
  }

  function updateMediaSessionPlaybackState(): void {
    if (!("mediaSession" in navigator)) return;
    if (!owns()) return;
    const current = player.value;
    if (current) {
      navigator.mediaSession.playbackState = current.playing ? "playing" : "paused";
      if (navigator.mediaSession.setPositionState) {
        navigator.mediaSession.setPositionState({
          duration: current.duration,
          playbackRate: current.speed || 1,
          position: current.currentTime,
        });
      }
    }
  }

  function setupMediaSession(info: MediaSessionInfo): void {
    if (!("mediaSession" in navigator) || !player.value) return;
    ownerId = ++mediaSessionOwner;
    // Fresh fallback URL with a timestamp to prevent caching issues
    const fallbackIcon: string = globalVars.loginIcon;
    const timestamp = Date.now();
    const fallbackUrl = fallbackIcon.includes("?")
      ? `${fallbackIcon}&t=${timestamp}`
      : `${fallbackIcon}?t=${timestamp}`;
    // In current versions of Firefox the artwork will not work, seems that doesn't like blob URLs.
    // But testing in 149.0a1 (nightly builds), it seems to work, so this something that will solve over time :)
    navigator.mediaSession.metadata = new MediaMetadata({
      title: info.title,
      artist: info.artist,
      album: info.album,
      artwork: [{ src: info.artworkSrc || fallbackUrl }],
    });
    const actionHandlers: [MediaSessionAction, MediaSessionActionHandler][] = [
      ["play", () => player.value?.play()],
      ["pause", () => player.value?.pause()],
      ["seekbackward", (details) => player.value?.rewind(details.seekOffset || 10)],
      ["seekforward", (details) => player.value?.forward(details.seekOffset || 10)],
      [
        "seekto",
        (details) => {
          if (details.fastSeek && details.fastSeek === ("optional" as unknown)) return;
          if (player.value && details.seekTime !== undefined) {
            player.value.currentTime = details.seekTime;
          }
        },
      ],
    ];
    for (const [action, handler] of actionHandlers) {
      try {
        navigator.mediaSession.setActionHandler(action, handler);
      } catch (e) {
        console.warn(`The media session action "${String(action)}" is not supported`, e);
      }
    }
    updateMediaSessionNavHandlers();
    updateMediaSessionPlaybackState();
  }

  function clearMediaSession(): void {
    if (!("mediaSession" in navigator)) return;
    if (!owns()) return;
    navigator.mediaSession.metadata = null;
    const actions: MediaSessionAction[] = [
      "play",
      "pause",
      "previoustrack",
      "nexttrack",
      "seekbackward",
      "seekforward",
      "seekto",
      "stop",
    ];
    for (const action of actions) {
      navigator.mediaSession.setActionHandler(action, null);
    }
    if (navigator.mediaSession.setPositionState) {
      navigator.mediaSession.setPositionState(undefined);
    }
    navigator.mediaSession.playbackState = "none";
    ownerId = 0;
  }

  return {
    setupMediaSession,
    updateMediaSessionNavHandlers,
    updateMediaSessionPlaybackState,
    clearMediaSession,
  };
}
