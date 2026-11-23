import { type ShallowRef, watch } from "vue";
// The only value import of `plyr`: the whole player is created and destroyed here so the
// Video.js 10 migration has a single place to replace. Its stylesheet is in `MediaPlayer.vue`.
import Plyr from "plyr";
import { useI18n } from "@/i18n/index.ts";
import { blockSeekOnInput } from "@/player/seekOnRelease.js";
import { router } from "@/router/index.ts";
import { mutations, state } from "@/store/index.ts";
import { globalVars } from "@/utils/constants.ts";
import { buildPlaybackQueryPatch, playbackQueryChanged } from "@/utils/playbackQuery.js";
import { getEndOfMediaAction, navigatePlaybackQueue } from "@/utils/playbackQueue.js";
import { buildItemUrl, removeTrailingSlash } from "@/utils/url.js";

/** Vue Router and buildItemUrl do not always encode the same characters, so route-leave checks compare both forms. */
function decodePathSafely(path: string): string {
  try {
    return decodeURI(path);
  } catch (_) {
    return path;
  }
}

type NativePlay = () => Promise<void> | void;
type PlyrRuntimeOptions = Plyr.Options & { playsinline?: boolean; preload?: string };

/** Plyr keeps `media` and a no-arg `off()` at runtime but its typings omit them. */
interface PlyrRuntime {
  media?: HTMLMediaElement;
  off: () => void;
  captions?: { currentTrack: number; currentTrackNode?: TextTrack };
}

export interface MountedFileKey {
  source: string;
  path: string;
}

/** The props both players take, besides what only one of them needs. */
export interface PlayerFile {
  name?: string;
  type?: string;
  path?: string;
  source?: string;
}

export type PlayerEmits = {
  "playback-start": [];
  "navigate-previous": [];
  "navigate-next": [];
  "close-preview": [];
};

/** Drops the `time` query param so a reload does not seek to a stale position. */
export async function stripPlaybackTimeFromUrl(): Promise<void> {
  const route = router.currentRoute.value;
  const nextQuery = buildPlaybackQueryPatch(route.query as Record<string, string | undefined>, { time: null });
  if (!playbackQueryChanged(route.query, nextQuery)) return;
  await router.replace({ path: route.path, query: nextQuery, hash: route.hash });
}

/** The source/path a player instance mounts for; used to detect a route change away from it. */
export function resolveMountedFileKey(source?: string, path?: string): MountedFileKey {
  return {
    source: source || (state.sources?.current as string | undefined) || "",
    path: path ? (path.startsWith("/") ? path : `/${path}`) : "",
  };
}

const PLYR_CONTROLS = [
  "play-large",
  "rewind",
  "play",
  "fast-forward",
  "progress",
  "current-time",
  "duration",
  "mute",
  "volume",
  "captions",
  "settings",
  "pip",
  "fullscreen",
];
const AUDIO_EXCLUDED_CONTROLS = ["play-large", "captions", "pip", "fullscreen"];

export type MediaKind = "audio" | "video";

interface MediaPlayerOptions {
  /** Decides the control set and whether fullscreen is available. */
  kind: MediaKind;
  player: ShallowRef<Plyr | null>;
  /** The <audio>/<video> element rendered by the component. */
  getMediaElement: () => HTMLMediaElement | null | undefined;
  /** Source/path of the file currently requested (may be cleared during a close). */
  getRequestedFile: () => { source?: string; path?: string } | null | undefined;
  /** Called on play, pause, seek and the other events that change what the media session reports. */
  onPlaybackState: () => void;
  /** Full teardown of the owning component state (called when the route left the file). */
  destroy: () => void;
  /** Runs before an element is stopped, e.g. to cancel a pending stream attach. */
  beforeStopMedia?: () => void;
  /** Runs after the element is stopped by `releaseFile`, e.g. to end a transcode session. */
  onReleaseFile?: () => void;
  restoreSource?: () => void | Promise<void>;
}

/** Creates, wires, stops and destroys Plyr , and ends/advances playback. */
export function useMediaPlayer(options: MediaPlayerOptions) {
  const { player } = options;
  const { t } = useI18n();
  let nativePlay: NativePlay | null = null;
  let releasedFor: MountedFileKey | null = null;

  const lifecycle: {
    mountedFile: MountedFileKey | null;
    tornDown: boolean;
    unmounted: boolean;
    mountInProgress: boolean;
  } = { mountedFile: null, tornDown: false, unmounted: false, mountInProgress: false };

  /** The Plyr options both players share; `createPlayer` merges the per-player extras over them. */
  function buildPlyrOptions(extra: Plyr.Options): PlyrRuntimeOptions {
    const isVideo = options.kind === "video";
    return {
      controls: isVideo ? [...PLYR_CONTROLS] : PLYR_CONTROLS.filter((id) => !AUDIO_EXCLUDED_CONTROLS.includes(id)),
      settings: ["captions", "quality", "speed", "playback", "loop"],
      speed: {
        selected: 1,
        options: [0.25, 0.5, 0.75, 1, 1.25, 1.5, 2],
      },
      disableContextMenu: true,
      seekTime: 10,
      hideControls: true,
      keyboard: { focused: true, global: true },
      loop: { active: false },
      blankVideo: "",
      muted: false,
      autoplay: false,
      playsinline: true,
      clickToPlay: false, // we manage this ourselves with the gestures, plyr has a issue where this doesn't work in mobile.
      resetOnEnd: false,
      preload: "none",
      fullscreen: {
        enabled: isVideo,
        fallback: true,
        iosNative: true,
        container: ".media-player",
      },
      iconUrl: `${globalVars.baseURL}public/static/img/plyr.svg`,
      listeners: {
        seek: blockSeekOnInput,
      },
      ...extra,
      i18n: {
        playback: t("player.playbackMode"),
        loop: t("player.loop"),
        ...extra.i18n,
      },
      tooltips: { controls: true, seek: true, ...extra.tooltips },
    };
  }

  /** `extra` holds what only one player needs */
  function createPlayer(extra: Plyr.Options = {}): Plyr | null {
    const el = options.getMediaElement();
    if (!el) return null;
    player.value = new Plyr(el, buildPlyrOptions(extra));
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

  /** The events both players handle the same way; `extra` runs after the shared handling of play, pause and timeupdate. */
  function setupSharedEvents(extra: { play?: () => void; pause?: () => void; timeupdate?: () => void } = {}): void {
    const current = player.value;
    if (!current) return;
    mutations.setPlaybackState(current.playing);
    const events: [Plyr.StandardEvent | Plyr.Html5Event, () => void][] = [
      ["ended", handleMediaEnd],
      [
        "play",
        () => {
          mutations.setPlaybackState(true);
          options.onPlaybackState();
          extra.play?.();
        },
      ],
      [
        "pause",
        () => {
          mutations.setPlaybackState(false);
          options.onPlaybackState();
          extra.pause?.();
        },
      ],
      [
        "timeupdate",
        () => {
          options.onPlaybackState();
          extra.timeupdate?.();
        },
      ],
      ["seeked", options.onPlaybackState],
      ["loadedmetadata", options.onPlaybackState],
      ["ratechange", options.onPlaybackState],
      ["canplay", options.onPlaybackState],
    ];
    for (const [event, handler] of events) {
      onPlayerEvent(event, handler);
    }
  }

  function getActiveMediaElement(): HTMLMediaElement | null {
    return (player.value as (Plyr & PlyrRuntime) | null)?.media ?? options.getMediaElement() ?? null;
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
    mutations.setPlaybackState(false);
    el.removeAttribute("src");
    try {
      el.load();
    } catch (_) {
      // ignore
    }
  }

  /** Plyr keeps the selected caption by index and track node. When the next file has a track at the same index, Plyr sees "no change" */
  function resetCaptionTrack(): void {
    const captions = (player.value as (Plyr & PlyrRuntime) | null)?.captions;
    if (!captions) return;
    captions.currentTrack = -1;
    captions.currentTrackNode = undefined;
  }

  /** Stops the current file and keeps the instance alive; the playback rate survives the `load()`. */
  function releaseFile(): void {
    const media = getActiveMediaElement();
    if (media) media.defaultPlaybackRate = media.playbackRate;
    stopMediaElement(media);
    resetCaptionTrack();
    options.onReleaseFile?.();
    releasedFor = lifecycle.mountedFile;
  }
  function resumeFile(): void {
    releasedFor = null;
  }
  function getReleasedFile(): MountedFileKey | null {
    return releasedFor;
  }

  /** Marks the file the instance now plays, for the route-leave check. */
  function setMountedFile(source?: string, path?: string): void {
    lifecycle.mountedFile = resolveMountedFileKey(source, path);
  }

  /** Re-applies the rate kept by `releaseFile`; call after attaching the next source. */
  function reapplyPlaybackRate(): void {
    const media = getActiveMediaElement();
    if (!media) return;
    const rate = media.defaultPlaybackRate || 1;
    if (player.value) player.value.speed = rate;
    else media.playbackRate = rate;
  }

  /** Removes listeners, stops the element and destroys the instance. No-op without an instance. */
  function destroyPlayer(media: HTMLMediaElement | null): void {
    const current = player.value;
    if (!current) {
      stopMediaElement(media);
      return;
    }
    (current as Plyr & PlyrRuntime).off();
    stopMediaElement(media);
    current.destroy();
    player.value = null;
    // hls.js can re-attach its MediaSource while Plyr tears down; stop the
    // element once more so no audio survives the unmount.
    stopMediaElement(media);
  }

  function restoreSourceIfReleased(): void {
    if (!releasedFor || releasedFor !== lifecycle.mountedFile) {
      return;
    }
    releasedFor = null;
    void options.restoreSource?.();
  }

  function stopIfRouteLeftThisFile(): void {
    if (lifecycle.tornDown) {
      return;
    }
    const requested = options.getRequestedFile();
    const mounted = lifecycle.mountedFile;
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
      restoreSourceIfReleased();
      return;
    }
    if (
      decodePathSafely(normalizedCurrent) === decodePathSafely(normalizedExpected) ||
      decodePathSafely(current).startsWith(`${decodePathSafely(expected)}#`)
    ) {
      restoreSourceIfReleased();
      return;
    }
    if (releasedFor && releasedFor === mounted) {
      return;
    }
    releaseFile();
  }

  function togglePlayPause(): void {
    if (lifecycle.tornDown) {
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

  watch(
    () => state.playbackQueue.shouldTogglePlayPause || false,
    (newVal, oldVal) => {
      if (newVal !== oldVal) togglePlayPause();
    },
  );
  watch(() => state.route.path, stopIfRouteLeftThisFile);

  return {
    lifecycle,
    setupSharedEvents,
    createPlayer,
    overridePlay,
    getNativePlay,
    restoreNativePlay,
    onPlayerEvent,
    getActiveMediaElement,
    stopMediaElement,
    releaseFile,
    resumeFile,
    getReleasedFile,
    setMountedFile,
    reapplyPlaybackRate,
    destroyPlayer,
    stopIfRouteLeftThisFile,
  };
}
