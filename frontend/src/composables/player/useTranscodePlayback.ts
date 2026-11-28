import type Plyr from "plyr";
import { computed, nextTick, ref, type Ref, type ShallowRef, watch } from "vue";
import type { RouteLocationNormalizedLoaded } from "vue-router";
import { getTranscodeSessions } from "@/api/transcode.ts";
import type { TranscodeStatus } from "@/types/transcode.ts";
import type { TranscodeSession } from "@/types/transcode.ts";
import { getCachedViewToken } from "@/api/viewToken.js";
import { useI18n } from "@/i18n/index.ts";
import { notify } from "@/notify/index.ts";
import { appendMediaFragment, isPlayBlockedError, type PlayPrime } from "@/player/playback/playbackIntent.ts";
import { errorMessage, type PlayerError } from "@/player/playerErrors.ts";
import { transcodeLog, transcodeWarn } from "@/player/transcoding/transcodeDebug.ts";
import {
  isTranscodeAvailable,
  isTranscodeHlsMode,
  normalizeTranscodeMode,
  normalizeTranscodeSessionStatus,
  type TranscodeMode,
} from "@/player/transcoding/transcodeMode.ts";
import {
  TranscodePlaybackController,
  type TranscodeRecoverContext,
  type TranscodeStartFailureReason,
} from "@/player/transcoding/transcodePlaybackController.ts";
import { createTranscodeClientId } from "@/player/transcoding/transcodeSession.ts";
import { router } from "@/router/index.ts";
import { getters, mutations } from "@/store/index.ts";
import { needsTranscodeFirst } from "@/player/transcoding/canBrowserPlayNative.ts";
import {
  buildPlaybackQueryPatch,
  parsePlaybackTimeFromQuery,
  parseTranscodeModeFromQuery,
  playbackQueryChanged,
} from "@/player/playback/playbackQuery.ts";
import { resolveTranscodeModeForPlayback, savePreferredTranscodeMode } from "@/composables/player/playerStorage.ts";

/** The parts of the requested file the transcode logic reads. */
export interface TranscodeRequest {
  name?: string;
  type?: string;
  path?: string;
  source?: string;
  viewToken?: string;
  metadata?: { videoCodec?: string; audioCodec?: string };
}

/** What the video area shows instead of the player when native playback is not possible. */
export interface TranscodeOffer {
  mode: "offer" | "unsupported" | "user_limit" | "system_limit";
  sessions: TranscodeSession[];
  systemLimit?: number;
  userLimit?: number;
}

/** Playhead restore (transcode -> native); applied when the video stream attaches. */
interface ResumeSnapshot {
  currentTime: number;
  wasPlaying?: boolean;
}

/** Outcome of startTranscodePlayback; `ok` only when the HLS attachment succeeded. */
export interface StartTranscodeResult {
  ok: boolean;
  reason?: TranscodeStartFailureReason;
}

/** Reasons that end a start without it being a playback failure */
const NON_FAILURE_START_REASONS: readonly TranscodeStartFailureReason[] = ["cancelled", "disposed", "stale", "recovered"];

interface StartTranscodeOptions {
  replaceSessionId?: string;
  startSec?: number;
  mode?: TranscodeMode;
  autoplay?: boolean;
  playPrime?: PlayPrime;
  manageSwitchState?: boolean;
}

interface TeardownTranscodeOptions {
  stopSession?: boolean;
  keepAlive?: boolean;
  keepalive?: boolean;
}

export interface UseTranscodePlaybackOptions {
  player: ShallowRef<Plyr | null>;
  shouldAutoplay: Ref<boolean>;
  getReq: () => TranscodeRequest;
  getRaw: () => string;
  getRoute: () => RouteLocationNormalizedLoaded;
  getVideoElement: () => HTMLVideoElement | null | undefined;
  /** The element Plyr plays on; falls back to the template ref before the player exists. */
  getPlaybackMedia: () => HTMLMediaElement | null | undefined;
  /** The browser's own play(), captured by useMediaPlayer before it is overridden. */
  getNativePlay: () => (() => void | Promise<void>) | null | undefined;
  isTornDown: () => boolean;
  isPlaybackLoading: () => boolean;
  /** Called when the native stream is bound, so the loading indicator expects playback. */
  onStreamAttach: () => void;
  onPlaybackStart: () => void;
  /** Re-applies the custom Plyr settings menus after the transcode state changed. */
  ensurePlaybackModeApplied: () => void;
  /** Syncs the (Plyr DOM) transcode menu with `transcodeMode`. */
  onModeMenuChange: () => void;
  /** The player must be released: the offer replaces it in the template. */
  onOfferShown: () => void;
}

/**
 * Transcode and native-stream lifecycle of the video player: HLS start/recovery/teardown through
 * TranscodePlaybackController, profile switching, session reuse, resume after a transcode->native switch,
 * deferred native stream attach, and the "transcode this file" offer
 */
export function useTranscodePlayback(options: UseTranscodePlaybackOptions) {
  const {
    player,
    shouldAutoplay,
    getReq,
    getRaw,
    getRoute,
    getVideoElement,
    getPlaybackMedia,
    getNativePlay,
    isTornDown,
    isPlaybackLoading,
    onStreamAttach,
    onPlaybackStart,
    ensurePlaybackModeApplied,
    onModeMenuChange,
    onOfferShown,
  } = options;
  const { t } = useI18n();

  // --- State ---
  /** Native video: defer stream URL until play so opening preview does not range-fetch the file. */
  const videoStreamAttached = ref(false);
  const resumeSnapshot = ref<ResumeSnapshot | null>(null);
  /** Seek target passed via Media Fragments URI (#t=) when re-attaching the native stream. */
  const resumeSeekSeconds = ref<number | null>(null);
  /** Keeps :src stable after a resume attach so Vue does not rebind without #t=. */
  const lockedNativeVideoSrc = ref<string | null>(null);
  const transcodePlayback = ref<TranscodePlaybackController | null>(null);
  const transcodeSwitchInProgress = ref(false);
  let transcodeSwitchOwner: symbol | null = null;
  let modeSwitchToken: symbol | null = null;
  const transcodeMode = ref<TranscodeMode>("native");
  const transcodeOffer = ref<TranscodeOffer | null>(null);
  /** The user accepted the offer: play through transcoding from the start. */
  const startRequested = ref(false);
  /** The user paused before a profile change: keep it paused through native attachment */
  const profileChangePaused = ref(false);
  /** The offer whose acceptance is awaiting its status lookup */
  let pendingAcceptance: TranscodeOffer | null = null;
  const transcodeClientId = createTranscodeClientId();

  let attachVideoStreamResume: (() => void) | null = null;
  /** Cancels the native resume in flight */
  let cancelNativeResume: (() => void) | null = null;
  let fileGeneration = 0;
  let nativeListenerCleanups: Array<() => void> = [];
  let offerRequestId = 0;
  let offerLookupPending = false;
  let startPreparationId = 0;
  let transcodeFallbackAttempted = false;
  let transcodeOfferEmitted = false;
  interface PendingModeRequest {
    mode: TranscodeMode;
    playPrime?: PlayPrime;
  }
  let pendingModeRequest: PendingModeRequest | null = null;
  function takePendingModeRequest(): PendingModeRequest | null {
    const request = pendingModeRequest;
    pendingModeRequest = null;
    return request;
  }

  function applyPendingModeRequest(generation: number): boolean {
    if (generation !== fileGeneration) {
      return false;
    }
    const next = takePendingModeRequest();
    if (!next || next.mode === transcodeMode.value) {
      return false;
    }
    void switchTranscodeMode(next.mode, { playPrime: next.playPrime });
    return true;
  }
  let resumeOnGestureArmed = false;
  let resumeOnGestureHandler: (() => void) | null = null;

  // --- Derived state ---
  const transcodeEnabled = computed(() => isTranscodeAvailable());
  const transcodeActive = computed(() => transcodePlayback.value?.active === true);
  const transcodeStartInProgress = computed(() => transcodePlayback.value?.startInProgress === true);
  const isTranscodePlaybackMode = computed(
    () => transcodeEnabled.value && isTranscodeHlsMode(transcodeMode.value),
  );
  const wantsTranscodePlayback = computed(() => isTranscodePlaybackMode.value);
  const formatNeedsTranscode = computed(() => needsTranscodeFirst(transcodeProbeParams()));
  const shouldProactivelyOfferTranscode = computed(() => {
    if (startRequested.value) {
      return false;
    }
    if (isTranscodePlaybackMode.value || getters.isShare()) {
      return false;
    }
    return formatNeedsTranscode.value;
  });

  const shouldAttachVideoStream = computed(() => {
    if (formatNeedsTranscode.value) {
      return false;
    }
    return videoStreamAttached.value || shouldAutoplay.value;
  });

  const nativeVideoSrc = computed(() => {
    if (transcodeActive.value || transcodeStartInProgress.value) {
      return null;
    }
    if (isTranscodeHlsMode(transcodeMode.value) || formatNeedsTranscode.value) {
      return null;
    }
    if (lockedNativeVideoSrc.value) {
      return lockedNativeVideoSrc.value;
    }
    if (shouldAttachVideoStream.value) {
      const resumeTime = resumeSeekSeconds.value ?? resumeSnapshot.value?.currentTime;
      return appendMediaFragment(getRaw(), resumeTime ?? 0);
    }
    return null;
  });

  const resumeShouldAutoplay = computed(() => {
    const snap = resumeSnapshot.value;
    if (!snap && !resumeSeekSeconds.value) {
      return false;
    }
    return snap?.wasPlaying !== false;
  });

  const videoElementAutoplay = computed(() => {
    if (isTranscodeHlsMode(transcodeMode.value) || transcodeStartInProgress.value) {
      return false;
    }
    if (formatNeedsTranscode.value || profileChangePaused.value) {
      return false;
    }
    if (resumeShouldAutoplay.value) {
      return true;
    }
    if (resumeSnapshot.value || resumeSeekSeconds.value) {
      return false;
    }
    return shouldAutoplay.value;
  });

  function transcodeProbeParams() {
    const meta = getReq()?.metadata || {};
    return {
      videoCodec: meta.videoCodec,
      audioCodec: meta.audioCodec,
      mimeType: getReq()?.type,
      fileName: getReq()?.name,
    };
  }

  function syncPlaybackUrl({ transcodeMode: explicit }: { transcodeMode?: TranscodeMode } = {}): void {
    if (transcodeSwitchInProgress.value) {
      return;
    }
    const explicitMode = explicit !== undefined;
    const resolvedMode = normalizeTranscodeMode(explicitMode ? explicit : transcodeMode.value);
    let modeForQuery: TranscodeMode | null = null;
    if (isTranscodeHlsMode(resolvedMode)) {
      if (explicitMode || transcodeActive.value) {
        modeForQuery = resolvedMode;
      }
    }
    const nextQuery = buildPlaybackQueryPatch(getRoute().query, {
      time: null,
      transcodeMode: modeForQuery,
    });
    if (!playbackQueryChanged(getRoute().query, nextQuery)) {
      return;
    }
    void router.replace({
      path: getRoute().path,
      query: nextQuery,
      hash: getRoute().hash,
    });
  }

  // --- Resume and native stream attach ---
  function applyResumeSeek(el: HTMLVideoElement | null | undefined, snapshot: ResumeSnapshot | null, onDone?: () => void): void {
    cancelNativeResume?.();
    if (!el || !snapshot) {
      onDone?.();
      return;
    }
    const targetTime = Number.isFinite(snapshot.currentTime) ? snapshot.currentTime : 0;
    let cancelled = false;
    let finished = false;
    let resumeCompleted = false;
    const cleanups: Array<() => void> = [];
    const cancel = () => {
      if (cancelled) {
        return;
      }
      cancelled = true;
      for (const cleanup of cleanups.splice(0)) {
        cleanup();
      }
      if (cancelNativeResume === cancel) {
        cancelNativeResume = null;
      }
    };
    cancelNativeResume = cancel;
    const once = (type: string, handler: () => void) => {
      const listener = () => {
        if (!cancelled) {
          handler();
        }
      };
      el.addEventListener(type, listener, { once: true });
      cleanups.push(() => el.removeEventListener(type, listener));
    };
    const after = (ms: number, handler: () => void) => {
      const id = window.setTimeout(() => {
        if (!cancelled) {
          handler();
        }
      }, ms);
      cleanups.push(() => window.clearTimeout(id));
    };
    const completeResume = () => {
      if (cancelled || resumeCompleted) {
        return;
      }
      resumeCompleted = true;
      if (snapshot.wasPlaying !== false) {
        attemptResumePlay(el, snapshot);
      }
      const boundSrc = el.currentSrc || el.getAttribute("src");
      if (boundSrc) {
        lockedNativeVideoSrc.value = boundSrc;
      }
      resumeSnapshot.value = null;
      resumeSeekSeconds.value = null;
      cancel();
      onDone?.();
    };
    const finish = () => {
      if (finished) {
        return;
      }
      finished = true;
      if (snapshot.wasPlaying !== false && el.readyState < HTMLMediaElement.HAVE_FUTURE_DATA) {
        once("canplay", completeResume);
        after(8000, completeResume);
      } else {
        completeResume();
      }
    };
    const readTime = () => player.value?.currentTime ?? el.currentTime;
    const writeTime = (time: number) => {
      if (player.value) {
        player.value.currentTime = time;
      } else {
        el.currentTime = time;
      }
    };
    const nearTarget = () => targetTime <= 0 || Math.abs(readTime() - targetTime) <= 1.5;
    const seek = () => {
      if (targetTime <= 0) {
        finish();
        return;
      }
      if (nearTarget()) {
        finish();
        return;
      }
      try {
        writeTime(targetTime);
      } catch (_) {
        finish();
        return;
      }
      let retried = false;
      const giveUp = () => {
        if (finished) {
          return;
        }
        console.warn("native resume: could not restore the playback position", {
          targetTime,
          currentTime: readTime(),
        });
        finished = true;
        completeResume();
      };
      const retrySeek = () => {
        if (finished) {
          return;
        }
        if (retried) {
          giveUp();
          return;
        }
        retried = true;
        try {
          writeTime(targetTime);
        } catch (_) {
          giveUp();
          return;
        }
        once("seeked", () => {
          if (nearTarget()) {
            finish();
          } else {
            giveUp();
          }
        });
        after(3000, giveUp);
      };
      const onSeeked = () => {
        if (nearTarget()) {
          finish();
          return;
        }
        if (el.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) {
          retrySeek();
          return;
        }
        once("canplay", retrySeek);
      };
      once("seeked", onSeeked);
      after(5000, () => {
        if (finished) {
          return;
        }
        if (nearTarget()) {
          finish();
        } else if (el.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA && !retried) {
          retrySeek();
        } else {
          giveUp();
        }
      });
    };
    const whenSeekable = () => {
      if (nearTarget()) {
        finish();
        return;
      }
      if (el.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) {
        seek();
        return;
      }
      once("canplay", seek);
    };
    if (el.readyState >= HTMLMediaElement.HAVE_METADATA) {
      whenSeekable();
    } else {
      once("loadedmetadata", whenSeekable);
    }
  }

  function listenOnceForFile(el: HTMLMediaElement, type: string, handler: () => void, generation: number): void {
    const cleanup = () => {
      el.removeEventListener(type, listener);
      nativeListenerCleanups = nativeListenerCleanups.filter((fn) => fn !== cleanup);
    };
    const listener = () => {
      cleanup();
      if (generation === fileGeneration) {
        handler();
      }
    };
    el.addEventListener(type, listener);
    nativeListenerCleanups.push(cleanup);
  }

  function clearNativeListeners(): void {
    for (const cleanup of nativeListenerCleanups.splice(0)) {
      cleanup();
    }
  }

  function cancelNativeAttach(): void {
    fileGeneration++;
    clearNativeListeners();
  }

  async function attachVideoStreamAndPlay(): Promise<void> {
    if (isTornDown() || videoStreamAttached.value || !getRaw()) {
      return;
    }
    const generation = fileGeneration;
    videoStreamAttached.value = true;
    onStreamAttach();
    await nextTick();
    await nextTick();
    const el = getVideoElement();
    if (generation !== fileGeneration || isTornDown() || !el) {
      return;
    }
    const snap = resumeSnapshot.value;
    const resumeTime = snap?.currentTime ?? resumeSeekSeconds.value;
    const streamUrl = appendMediaFragment(getRaw(), resumeTime ?? 0);
    if (streamUrl && !el.getAttribute("src")) {
      el.setAttribute("src", streamUrl);
      lockedNativeVideoSrc.value = streamUrl;
      if (snap?.wasPlaying !== false) {
        el.autoplay = true;
      }
    }
    if (snap?.wasPlaying !== false) {
      const onCanPlay = () => attemptResumePlay(el, snap);
      if (el.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) {
        onCanPlay();
      } else {
        listenOnceForFile(el, "canplay", onCanPlay, generation);
      }
    }

    const startDefault = () => {
      clearAttachVideoStreamWait();
      if (generation !== fileGeneration || isTornDown()) {
        return;
      }
      void el.play().catch(() => {});
    };

    const startWithResume = () => {
      clearAttachVideoStreamWait();
      const onMeta = () => {
        applyResumeSeek(el, snap);
      };
      if (el.readyState >= HTMLMediaElement.HAVE_METADATA) {
        onMeta();
      } else {
        listenOnceForFile(el, "loadedmetadata", onMeta, generation);
      }
    };

    const start = snap ? startWithResume : startDefault;
    attachVideoStreamResume = start;
    if (snap) {
      if (el.readyState >= HTMLMediaElement.HAVE_METADATA) {
        start();
      } else {
        listenOnceForFile(el, "loadedmetadata", start, generation);
      }
    } else {
      start();
    }
  }

  function clearAttachVideoStreamWait(): void {
    const el = getVideoElement();
    if (el && attachVideoStreamResume) {
      el.removeEventListener("canplay", attachVideoStreamResume);
      el.removeEventListener("loadedmetadata", attachVideoStreamResume);
    }
    attachVideoStreamResume = null;
  }

  function attemptResumePlay(el: HTMLVideoElement | null | undefined, snapshot: ResumeSnapshot | null): void {
    if (isTornDown() || !el || snapshot?.wasPlaying === false) {
      return;
    }
    const generation = fileGeneration;
    const tryPlay = () => {
      if (generation !== fileGeneration || isTornDown()) {
        return;
      }
      const playCall = getNativePlay() ?? (() => player.value?.play?.() ?? el.play());
      void Promise.resolve(playCall())
        .then(() => {
          if (generation !== fileGeneration) {
            return;
          }
          mutations.setPlaybackState(true);
          onPlaybackStart();
        })
        .catch(() => {});
    };
    if (el.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA && !el.paused) {
      return;
    }
    if (el.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) {
      tryPlay();
      return;
    }
    listenOnceForFile(el, "canplay", tryPlay, generation);
  }

  function resolveViewTokenForTranscode(): string {
    return getReq()?.viewToken || getCachedViewToken(getReq()?.source) || "";
  }

  function invalidateOfferRequest(): void {
    if (!offerLookupPending) {
      return;
    }
    offerRequestId++;
    offerLookupPending = false;
    transcodeOfferEmitted = false;
  }

  async function offerTranscodePlayback({ confirmedFailure = false }: { confirmedFailure?: boolean } = {}): Promise<void> {
    if (isTornDown() || transcodeSwitchInProgress.value || isTranscodePlaybackMode.value) {
      return;
    }
    if (transcodeOfferEmitted) {
      return;
    }
    if (getters.isShare()) {
      // Shares cannot transcode
      if (confirmedFailure || formatNeedsTranscode.value) {
        notify.showError(t("player.transcodeUnsupportedFormat"));
      }
      return;
    }
    if (!confirmedFailure && !formatNeedsTranscode.value) {
      return;
    }
    transcodeOfferEmitted = true;
    if (!transcodeEnabled.value) {
      // Transcoding is off on this server: show the unsupported notice without
      // asking for sessions, which would answer 503.
      showTranscodeOffer({ enabled: false, canStart: false, sessions: [] });
      return;
    }
    const requestId = ++offerRequestId;
    offerLookupPending = true;
    try {
      const status = await getTranscodeSessions();
      if (requestId !== offerRequestId || isTornDown()) {
        return;
      }
      offerLookupPending = false;
      showTranscodeOffer(normalizeTranscodeSessionStatus(status, {
          source: getReq()?.source,
          path: getReq()?.path,
        }),
      );
    } catch (err) {
      if (requestId !== offerRequestId || isTornDown()) {
        return;
      }
      offerLookupPending = false;
      console.error("Failed to check transcode sessions:", err);
      showTranscodeOffer({ enabled: true, canStart: true, sessions: [] });
    }
  }
  /** Looks up the session status and shows the offer for it */
  async function loadAndShowTranscodeOffer(): Promise<void> {
    const requestId = ++offerRequestId;
    offerLookupPending = true;
    try {
      const status = await getTranscodeSessions();
      if (requestId !== offerRequestId || isTornDown()) {
        return;
      }
      offerLookupPending = false;
      showTranscodeOffer(
        normalizeTranscodeSessionStatus(status, {
          source: getReq()?.source,
          path: getReq()?.path,
        }),
      );
    } catch (err) {
      if (requestId !== offerRequestId || isTornDown()) {
        return;
      }
      offerLookupPending = false;
      console.error("Failed to check transcode sessions:", err);
      showTranscodeOffer({ enabled: true, canStart: true, sessions: [] });
    }
  }

  function onVideoLoadedMetadata(): void {
    if (transcodeOfferEmitted || getters.isShare()) {
      return;
    }
    const video = getVideoElement();
    if (!video || video.videoWidth > 0 || video.videoHeight > 0) {
      return;
    }
    if (!Number.isFinite(video.duration) || video.duration <= 0) {
      return;
    }
    void offerTranscodePlayback({ confirmedFailure: true });
  }

  function getPlaybackVideo(): HTMLVideoElement | null {
    const media = getPlaybackMedia();
    return media instanceof HTMLVideoElement ? media : null;
  }

  function syncPlayerAfterTranscodeReady(): void {
    if (!transcodeActive.value || !player.value) {
      return;
    }
    const video = getPlaybackVideo();
    if (!video) {
      return;
    }
    const container = player.value.elements?.container;
    if (container && video.videoWidth > 0 && video.videoHeight > 0) {
      container.style.aspectRatio = `${video.videoWidth} / ${video.videoHeight}`;
    }
    ensurePlaybackModeApplied();
  }

  function resumeTranscodePlayback(): Promise<void> {
    const media = getPlaybackMedia();
    if (isTornDown()) {
      return Promise.resolve();
    }
    transcodeLog("resumeTranscodePlayback", {
      playerPaused: player.value?.paused,
      mediaPaused: media?.paused,
      readyState: media?.readyState,
    });
    const playResult = player.value?.play ? player.value.play() : media?.play();
    return Promise.resolve(playResult)
      .then(() => {
        transcodeLog("resumeTranscodePlayback play resolved");
      })
      .catch((playErr: Error) => {
        transcodeWarn("player.play failed during transcode attach", {
          name: playErr?.name,
          message: playErr?.message,
        });
        if (isPlayBlockedError(playErr)) {
          armResumeOnNextGesture();
        }
        throw playErr;
      });
  }

  /**
   * Autoplay was blocked. Arm a one-shot resume so a single tap or keypress
   * anywhere restarts playback instead of the user hunting for the play button.
   */
  function armResumeOnNextGesture(): void {
    if (resumeOnGestureArmed) {
      return;
    }
    resumeOnGestureArmed = true;
    const handler = () => {
      disarmResumeOnNextGesture();
      const media = getPlaybackMedia();
      if (!transcodeActive.value || !media?.paused) {
        return;
      }
      transcodeLog("resuming transcode playback after user gesture");
      void resumeTranscodePlayback().catch(() => {});
    };
    resumeOnGestureHandler = handler;
    window.addEventListener("pointerdown", handler, { once: true, capture: true });
    window.addEventListener("keydown", handler, { once: true, capture: true });
  }

  function disarmResumeOnNextGesture(): void {
    const handler = resumeOnGestureHandler;
    resumeOnGestureArmed = false;
    resumeOnGestureHandler = null;
    if (!handler) {
      return;
    }
    window.removeEventListener("pointerdown", handler, { capture: true });
    window.removeEventListener("keydown", handler, { capture: true });
  }

  function syncTranscodePlaybackState(): void {
    if (isTornDown()) {
      return;
    }
    mutations.setPlaybackState(true);
    onPlaybackStart();
  }

  function initTranscodeModeFromRoute(): void {
    if (!transcodeEnabled.value) {
      transcodeMode.value = "native";
      return;
    }
    if (startRequested.value) {
      transcodeMode.value = normalizeTranscodeMode(resolveTranscodeModeForPlayback(getRoute().query));
      return;
    }
    if (needsTranscodeFirst(transcodeProbeParams())) {
      transcodeMode.value = "native";
      return;
    }
    const urlTranscode = parseTranscodeModeFromQuery(getRoute().query);
    if (urlTranscode) {
      transcodeMode.value = normalizeTranscodeMode(urlTranscode);
      return;
    }
    transcodeMode.value = "native";
  }

  function resolveActiveTranscodeMode(): TranscodeMode {
    const urlMode = parseTranscodeModeFromQuery(getRoute().query);
    if (urlMode) {
      return normalizeTranscodeMode(urlMode);
    }
    if (isTranscodeHlsMode(transcodeMode.value)) {
      return transcodeMode.value;
    }
    return normalizeTranscodeMode(resolveTranscodeModeForPlayback(getRoute().query));
  }

  async function resolveReuseSessionId(): Promise<string | undefined> {
    if (!getReq()?.source || !getReq()?.path) {
      return undefined;
    }
    try {
      const status = await getTranscodeSessions();
      const sameFile = (status.sessions || []).filter(
        (session: { source?: string; path?: string }) =>
          session.source === getReq()?.source && session.path === getReq()?.path,
      );
      // Prefer a live session so a completed cache entry for another profile
      // is not passed as replaceSessionId while an older profile is still active.
      const active = sameFile.find(
        (session: { state?: string }) => session.state === "starting" || session.state === "running",
      );
      return active?.id ?? sameFile[0]?.id;
    } catch {
      return undefined;
    }
  }

  async function startTranscodePlayback({
    replaceSessionId,
    startSec,
    mode,
    autoplay,
    playPrime,
    manageSwitchState = true,
  }: StartTranscodeOptions = {}): Promise<StartTranscodeResult> {
    if (isTornDown()) {
      return { ok: false, reason: "cancelled" };
    }
    transcodeLog("startTranscodePlayback called", {
      source: getReq()?.source,
      path: getReq()?.path,
      transcodeEnabled: transcodeEnabled.value,
      transcodeMode: transcodeMode.value,
      requestedMode: mode,
      hasPlayer: Boolean(player.value),
      replaceSessionId,
      startSec,
    });
    if (!getReq()?.source || !getReq()?.path || !transcodeEnabled.value) {
      transcodeWarn("startTranscodePlayback skipped — preconditions failed", {
        source: getReq()?.source,
        path: getReq()?.path,
        transcodeEnabled: transcodeEnabled.value,
      });
      return { ok: false, reason: "preconditions" };
    }
    const activeMode = isTranscodeHlsMode(mode) ? normalizeTranscodeMode(mode) : resolveActiveTranscodeMode();
    if (!isTranscodeHlsMode(activeMode)) {
      transcodeWarn("startTranscodePlayback skipped — not an HLS mode", { activeMode });
      return { ok: false, reason: "preconditions" };
    }
    const viewToken = resolveViewTokenForTranscode();
    if (!viewToken) {
      transcodeWarn("startTranscodePlayback skipped — missing view token");
      return { ok: false, reason: "preconditions" };
    }
    // switching the source to HLS: a native resume still waiting must not touch the new source
    cancelNativeResume?.();
    clearNativeListeners();
    invalidateOfferRequest();
    const startSource = getReq()?.source;
    const startPath = getReq()?.path;
    const preparationId = ++startPreparationId;
    const startGeneration = fileGeneration;
    const switchToken = Symbol("transcodeSwitch");
    const manageSwitch =
      manageSwitchState && (!transcodeSwitchInProgress.value || transcodeSwitchOwner !== null);
    if (manageSwitch) {
      transcodeSwitchInProgress.value = true;
      transcodeSwitchOwner = switchToken;
    }
    const releaseSwitch = () => {
      if (transcodeSwitchOwner === switchToken) {
        transcodeSwitchOwner = null;
        transcodeSwitchInProgress.value = false;
      }
    };
    let effectiveReplaceSessionId: string | undefined =
      replaceSessionId ?? transcodePlayback.value?.sessionController?.session?.id;
    if (effectiveReplaceSessionId === undefined || effectiveReplaceSessionId === null) {
      effectiveReplaceSessionId = await resolveReuseSessionId();
      if (
        preparationId !== startPreparationId ||
        isTornDown() ||
        getReq()?.source !== startSource ||
        getReq()?.path !== startPath
      ) {
        transcodeLog("startTranscodePlayback dropped — request changed while preparing");
        if (manageSwitch) {
          releaseSwitch();
        }
        return { ok: false, reason: "cancelled" };
      }
    }
    const seekSec = startSec ?? player.value?.currentTime ?? 0;
    const shouldPlay = autoplay ?? shouldAutoplay.value;
    transcodeLog("startTranscodePlayback params", {
      activeMode,
      manageSwitch,
      shouldPlay,
      seekSec,
      hasPlayPrime: Boolean(playPrime),
      transcodeSwitchInProgress: transcodeSwitchInProgress.value,
    });

    let pendingUrlSync: { transcodeMode?: TranscodeMode } | null = null;
    const requestUrlSync = (opts: { transcodeMode?: TranscodeMode } = {}) => {
      if (manageSwitch) {
        pendingUrlSync = opts;
      } else {
        syncPlaybackUrl(opts);
      }
    };

    try {
      transcodeLog("preparing video element for HLS", {
        replaceSessionId: effectiveReplaceSessionId,
        seekSec,
      });
      videoStreamAttached.value = true;
      transcodeMode.value = activeMode;
      lockedNativeVideoSrc.value = null;

      const startParams = {
        source: startSource,
        path: startPath,
        viewToken,
        mode: activeMode,
        clientId: transcodeClientId,
        replaceSessionId: effectiveReplaceSessionId,
        startSec: seekSec,
        playPrime,
        // The element Plyr plays on is the <video> of this player.
        getVideoElement: getPlaybackVideo,
        nextTick: () => nextTick(),
        shouldAutoplay: shouldPlay,
        onFirstBuffered: () => {
          transcodeLog("first HLS fragment buffered — starting playback");
          syncPlayerAfterTranscodeReady();
        },
        onResumePlayback: () => resumeTranscodePlayback(),
        onPlaybackStarted: () => syncTranscodePlaybackState(),
        onFatal: () => {
          notify.showError(t("player.transcodeFailed"));
        },
        onRecoverPlayback: async (ctx: TranscodeRecoverContext) => {
          transcodeLog("recovering transcode playback after HLS failure", {
            startSec: ctx.startSec,
            sessionId: ctx.sessionId,
          });
          const recovered = await startTranscodePlayback({
            mode: ctx.mode ?? resolveActiveTranscodeMode(),
            startSec: ctx.startSec,
            replaceSessionId: ctx.sessionId,
            autoplay: ctx.shouldAutoplay ?? shouldAutoplay.value,
            manageSwitchState: true,
            playPrime: ctx.playPrime,
          });
          return recovered.ok || (recovered.reason !== undefined && NON_FAILURE_START_REASONS.includes(recovered.reason));
        },
        onCapacityStatus: (status: TranscodeStatus) => {
          showTranscodeOffer(normalizeTranscodeSessionStatus(status, {
              source: getReq()?.source,
              path: getReq()?.path,
            }),
          );
        },
        onShowError: (err: PlayerError) => {
          notify.showError(errorMessage(err) || t("player.transcodeFailed"));
        },
        onAttachNative: () => {
          if (!transcodeFallbackAttempted && !wantsTranscodePlayback.value) {
            void attachVideoStreamAndPlay();
          }
        },
      };

      const result = await ensureTranscodePlaybackController().start(startParams);

      if (!result.ok && result.reason !== "cancelled" && result.reason !== "disposed" && result.reason !== "recovered") {
        transcodeWarn("transcode playback did not start", { reason: result.reason });
        if (result.reason === "error") {
          notify.showError(t("player.transcodeFailed"));
        }
      }

      if (result.reason === "capacity" && isTranscodeHlsMode(transcodeMode.value)) {
        // Fall back fully: leaving transcodeMode on an HLS profile would make
        // the play event fired by attachVideoStreamAndPlay re-enter
        // startTranscodePlayback and spin on the 409 limit.
        transcodeMode.value = "native";
        onModeMenuChange();
        requestUrlSync();
        await switchToNativePlayback();
        return { ok: false, reason: "capacity" };
      }
      if (
        !result.ok &&
        (result.reason === "fallback" || result.reason === "error") &&
        !isTornDown() &&
        isTranscodeHlsMode(transcodeMode.value)
      ) {
        transcodeMode.value = "native";
        onModeMenuChange();
        requestUrlSync();
        if (formatNeedsTranscode.value) {
          // offer transcoding again so the user can retry.
          transcodeOfferEmitted = false;
          await loadAndShowTranscodeOffer();
        } else {
          // restore native playback before attaching it.
          await switchToNativePlayback();
        }
        return { ok: false, reason: result.reason };
      }
      if (result.ok) {
        onModeMenuChange();
        requestUrlSync({ transcodeMode: activeMode });
      }
      return { ok: result.ok === true, reason: result.reason };
    } finally {
      if (manageSwitch) {
        releaseSwitch();
        const urlSync = pendingUrlSync;
        if (!applyPendingModeRequest(startGeneration) && urlSync) {
          syncPlaybackUrl(urlSync);
        }
      }
    }
  }

  function onVideoPlaybackError(event: Event): void {
    if (transcodeSwitchInProgress.value || transcodeStartInProgress.value || isTranscodePlaybackMode.value) {
      return;
    }
    if (transcodeOfferEmitted || getters.isShare()) {
      return;
    }
    const code = (event.target instanceof HTMLMediaElement ? event.target.error?.code : undefined);
    if (code !== MediaError.MEDIA_ERR_DECODE && code !== MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED) {
      return;
    }
    void offerTranscodePlayback({ confirmedFailure: true });
  }

  async function switchToNativePlayback({ autoplay }: { autoplay?: boolean } = {}): Promise<void> {
    const currentTime = player.value?.currentTime ?? 0;
    const shouldPlay = autoplay ?? !!player.value?.playing;
    transcodeLog("switchToNativePlayback", { currentTime, shouldPlay });
    const generation = fileGeneration;

    // Release the HLS source even when a shared controller owns it.
    teardownTranscodePlayback({ stopSession: true });
    lockedNativeVideoSrc.value = null;
    videoStreamAttached.value = false;
    // hls.js leaves its (now revoked) MediaSource blob URL on the element, and
    // attachVideoStreamAndPlay only binds when no src is present.
    const media = getPlaybackMedia();
    if (media?.hasAttribute("src")) {
      media.removeAttribute("src");
    }
    await nextTick();
    if (generation !== fileGeneration || isTornDown()) return;

    // Restore the playhead through the resume path, which waits for the
    // new source to report metadata before seeking
    const canResume = Number.isFinite(currentTime) && currentTime > 0;
    if (canResume) {
      resumeSeekSeconds.value = currentTime;
    }
    resumeSnapshot.value = {
      currentTime: canResume ? currentTime : 0,
      wasPlaying: shouldPlay,
    };

    await attachVideoStreamAndPlay();
    if (!canResume && shouldPlay && player.value) {
      await Promise.resolve(player.value.play()).catch(() => {});
    }
  }

  async function switchTranscodeMode(
    requested: TranscodeMode,
    { playPrime }: { playPrime?: PlayPrime } = {},
  ): Promise<void> {
    const mode = normalizeTranscodeMode(requested);
    if (transcodeSwitchInProgress.value) {
      pendingModeRequest = { mode, playPrime };
      return;
    }
    if (mode === transcodeMode.value) {
      return;
    }
    if (
      isTranscodeHlsMode(mode) &&
      (!getReq()?.source || !getReq()?.path || !transcodeEnabled.value || !resolveViewTokenForTranscode())
    ) {
      syncPlaybackUrl({ transcodeMode: transcodeMode.value });
      return;
    }
    pendingModeRequest = null;
    invalidateOfferRequest();
    const previousMode = transcodeMode.value;
    const wasPlaying = !!player.value?.playing;
    profileChangePaused.value = !wasPlaying;
    transcodeLog("switchTranscodeMode start", {
      mode,
      previousMode,
      wasPlaying,
      playerPaused: player.value?.paused,
      mediaPaused: getPlaybackMedia()?.paused,
      shouldAutoplay: shouldAutoplay.value,
      currentTime: player.value?.currentTime,
    });
    const switchToken = Symbol("modeSwitch");
    modeSwitchToken = switchToken;
    transcodeSwitchInProgress.value = true;
    const releaseModeSwitch = () => {
      if (modeSwitchToken === switchToken) {
        modeSwitchToken = null;
        transcodeSwitchInProgress.value = false;
      }
    };
    const generation = fileGeneration;
    // Capture before mode/teardown changes so same-file profile switches always
    // send replaceSessionId even if the controller is mid-start.
    const replaceSessionId = transcodePlayback.value?.sessionController?.session?.id ?? (await resolveReuseSessionId());
    if (generation !== fileGeneration) {
      releaseModeSwitch();
      return;
    }
    transcodeMode.value = mode;
    if (isTranscodeHlsMode(mode)) {
      savePreferredTranscodeMode(mode);
    }
    try {
      if (isTranscodeHlsMode(mode)) {
        const started = await startTranscodePlayback({
          replaceSessionId,
          startSec: player.value?.currentTime ?? 0,
          mode,
          autoplay: wasPlaying,
          playPrime: wasPlaying ? playPrime : null,
          manageSwitchState: false,
        });
        if (!started.ok && started.reason === "preconditions" && generation === fileGeneration) {
          transcodeMode.value = previousMode;
          if (previousMode === "native") {
            await switchToNativePlayback({ autoplay: wasPlaying });
          }
        }
      } else if (needsTranscodeFirst(transcodeProbeParams())) {
        let status: TranscodeStatus | null = null;
        try {
          status = await getTranscodeSessions();
        } catch (err) {
          console.error("Failed to check transcode sessions:", err);
        }
        if (generation === fileGeneration) {
          transcodeOfferEmitted = false;
          if (status) {
            showTranscodeOffer(normalizeTranscodeSessionStatus(status, {
                source: getReq()?.source,
                path: getReq()?.path,
              }),
            );
          } else if (transcodeActive.value) {
            transcodeMode.value = previousMode;
          } else {
            showTranscodeOffer({ enabled: true, canStart: true, sessions: [] });
          }
        }
      } else {
        await switchToNativePlayback({ autoplay: wasPlaying });
      }
      onModeMenuChange();
    } catch (err) {
      console.error("Transcode mode switch failed:", err);
      transcodeMode.value = previousMode;
      onModeMenuChange();
      if (previousMode === "native") {
        await switchToNativePlayback();
      } else {
        await startTranscodePlayback({
          startSec: player.value?.currentTime ?? 0,
          manageSwitchState: false,
        });
      }
    } finally {
      releaseModeSwitch();
      if (generation === fileGeneration && !applyPendingModeRequest(generation)) {
        syncPlaybackUrl({ transcodeMode: transcodeMode.value });
      }
      transcodeLog("switchTranscodeMode done", {
        mode: transcodeMode.value,
        transcodeActive: transcodeActive.value,
        playerPaused: player.value?.paused,
        mediaPaused: getPlaybackMedia()?.paused,
        videoPlaybackLoading: isPlaybackLoading(),
      });
    }
  }

  // --- Controller ownership ---
  function ensureTranscodePlaybackController(): TranscodePlaybackController {
    if (!transcodePlayback.value || transcodePlayback.value.disposed) {
      transcodePlayback.value = new TranscodePlaybackController();
    }
    return transcodePlayback.value;
  }

  /** Stops the HLS session and detaches from the element. The controller stays usable for a later start. */
  function teardownTranscodePlayback({
    stopSession = false,
    keepAlive = false,
    keepalive = false,
  }: TeardownTranscodeOptions = {}): void {
    cancelNativeResume?.();
    invalidateOfferRequest();
    startPreparationId++;
    const controller = transcodePlayback.value;
    if (!controller || keepAlive) {
      return;
    }
    controller.teardownLocal({ stopSession, keepalive });
  }

  /** Final release of the controller (component teardown). */
  function disposeTranscodeController({ stopSession = true, keepalive = false } = {}): void {
    cancelNativeResume?.();
    invalidateOfferRequest();
    startPreparationId++;
    const controller = transcodePlayback.value;
    if (!controller) {
      return;
    }
    controller.dispose({ stopSession, keepalive });
    transcodePlayback.value = null;
  }

  // --- Offer ("transcode this file?") ---
  function clearTranscodeFromUrl(): void {
    const route = router.currentRoute.value;
    const hasTranscode = parseTranscodeModeFromQuery(route?.query);
    const hasTime = parsePlaybackTimeFromQuery(route?.query) !== null;
    if (!hasTranscode && !hasTime) {
      return;
    }
    const nextQuery = buildPlaybackQueryPatch(route.query, {
      transcodeMode: null,
      time: null,
    });
    if (!playbackQueryChanged(route.query, nextQuery)) {
      return;
    }
    router.replace({
      path: route.path,
      query: nextQuery,
      hash: route.hash,
    });
  }
  /** Native playback failed or the container is unsupported: release the session and show the offer. */
  function showTranscodeOffer(rawStatus: TranscodeStatus): void {
    if (getters.isShare()) {
      return;
    }
    teardownTranscodePlayback({ stopSession: true });
    const status = normalizeTranscodeSessionStatus(rawStatus, {
      source: getReq()?.source,
      path: getReq()?.path,
    });
    let mode: TranscodeOffer["mode"] = "offer";
    if (status.enabled === false) {
      mode = "unsupported";
    } else if (!status.canStart) {
      if (status.blockReason === "user_limit") {
        mode = "user_limit";
      } else if (status.blockReason === "global_limit") {
        mode = "system_limit";
      }
    }
    startRequested.value = false;
    transcodeOffer.value = {
      mode,
      sessions: status.sessions || [],
      systemLimit: status.globalLimit,
      userLimit: status.userLimit,
    };
    onOfferShown();
    if (mode === "offer") {
      clearTranscodeFromUrl();
    }
  }

  /** The offer's button: verifies a session can start and hides the offer. Returns whether the player should mount. */
  async function startVideoTranscode(isCurrent: () => boolean = () => true): Promise<boolean> {
    if (startRequested.value) {
      transcodeOffer.value = null;
      return false;
    }
    const offer = transcodeOffer.value;
    if (offer && pendingAcceptance === offer) {
      return false;
    }
    pendingAcceptance = offer;
    try {
      const status = normalizeTranscodeSessionStatus(await getTranscodeSessions(), {
        source: getReq()?.source,
        path: getReq()?.path,
      });
      if (!isCurrent()) {
        return false;
      }
      if (!status.canStart) {
        showTranscodeOffer(status);
        return false;
      }
      ensureTranscodePlaybackController();
      onPlaybackStart();
      transcodeOffer.value = null;
      startRequested.value = true;
      return true;
    } catch (err) {
      console.error("Failed to verify transcode availability:", err);
      return false;
    } finally {
      if (pendingAcceptance === offer) {
        pendingAcceptance = null;
      }
    }
  }

  /** Back to the per-file defaults (new file, or the offer was accepted). */
  function resetFileState(): void {
    cancelNativeAttach();
    cancelNativeResume?.();
    invalidateOfferRequest();
    startPreparationId++;
    modeSwitchToken = null;
    pendingModeRequest = null;
    transcodeSwitchOwner = null;
    transcodeSwitchInProgress.value = false;
    videoStreamAttached.value = false;
    resumeSnapshot.value = null;
    resumeSeekSeconds.value = null;
    lockedNativeVideoSrc.value = null;
    profileChangePaused.value = false;
    pendingAcceptance = null;
    transcodeOfferEmitted = false;
    transcodeFallbackAttempted = false;
  }

  watch(
    () => getRoute().query,
    (newQuery, oldQuery) => {
      const urlTranscode = parseTranscodeModeFromQuery(newQuery);
      if (urlTranscode === parseTranscodeModeFromQuery(oldQuery)) {
        return;
      }
      void switchTranscodeMode(urlTranscode || "native");
    },
  );

  return {
    // state
    transcodeMode,
    transcodeSwitchInProgress,
    transcodeOffer,
    transcodePlayback,
    startRequested,
    videoStreamAttached,
    lockedNativeVideoSrc,
    // derived
    transcodeEnabled,
    transcodeActive,
    transcodeStartInProgress,
    isTranscodePlaybackMode,
    wantsTranscodePlayback,
    formatNeedsTranscode,
    shouldProactivelyOfferTranscode,
    shouldAttachVideoStream,
    nativeVideoSrc,
    videoElementAutoplay,
    // actions
    resetFileState,
    initTranscodeModeFromRoute,
    ensureTranscodePlaybackController,
    offerTranscodePlayback,
    startTranscodePlayback,
    switchTranscodeMode,
    teardownTranscodePlayback,
    disposeTranscodeController,
    attachVideoStreamAndPlay,
    clearAttachVideoStreamWait,
    cancelNativeAttach,
    disarmResumeOnNextGesture,
    syncPlaybackUrl,
    startVideoTranscode,
    // video element handlers
    onVideoLoadedMetadata,
    onVideoPlaybackError,
    // exposed for tests
    resolveReuseSessionId,
  };
}
