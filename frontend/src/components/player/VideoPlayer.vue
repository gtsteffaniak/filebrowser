<template>
  <div class="media-player">
    <div
      class="video-player-container"
      :class="{
        'no-captions': !hasSubtitles,
        'video-player-container--transcode-switching': transcodeSwitchInProgress,
      }"
    >
      <div class="media-video-container">
        <video
          ref="videoElement"
          :type="req.type"
          :src="nativeVideoSrc ?? undefined"
          :poster="artworkUrl"
          :autoplay="videoElementAutoplay"
          @play="handlePlay"
          @error="onVideoPlaybackError"
          @loadedmetadata="onVideoLoadedMetadata"
          playsinline
        >
          <track kind="captions" v-for="(sub, index) in subtitlesList" :key="index" :src="sub.src"
            :label="sub.name || subtitleTrackLabel(sub)" :srclang="sub.srclang || sub.language" />
        </video>
      </div>
      <div
        v-if="videoPlaybackLoading || transcodeSwitchInProgress"
        class="video-loading-overlay"
        :class="{ 'video-loading-overlay--blocking': transcodeSwitchInProgress }"
        :aria-label="transcodeSwitchInProgress ? t('player.transcodeLoading') : undefined"
        aria-hidden="true"
        aria-busy="true"
      >
        <LoadingSpinner size="large" />
      </div>
      <div
        class="video-skip-feedback-layer"
        :class="{
          'video-skip-feedback-layer--visible': skipFeedbackVisible,
          'video-skip-feedback-layer--left': skipFeedbackSide === 'left',
          'video-skip-feedback-layer--right': skipFeedbackSide === 'right',
        }"
        aria-hidden="true"
      >
        <i :key="skipFeedbackKey" class="material-symbols video-skip-feedback-layer__icon">{{ skipFeedbackIcon }}</i>
      </div>
    </div>
    <!-- Queue button (hidden in fullscreen) -->
    <FloatingActionButton
      v-if="showQueueButton"
      icon="queue_music"
      :badge="queueCount"
      group="media-controls"
      @press="showQueuePrompt"
      :label="t('player.QueueButtonHint')"
    />
    <!-- Toast when you change playback modes in the media player -->
    <div :class="['playback-toast', toastVisible ? 'visible' : '']">
      <!-- Loop icon for "single playback", "loop single file" and "loop all files" -->
      <i class="material-symbols">{{ toastIcon }}</i>
      <span>{{ playbackModeMessage }}</span>

      <!-- Status indicator for loop -->
      <span v-if="toastType === 'loop'" :class="[
          'status-indicator', loop !== 'off' ? 'status-on' : 'status-off',]"></span>
    </div>
    <!-- Speed toast (long-press) -->
    <div v-if="speedToastVisible" class="playback-toast visible">
      <i class="material-symbols">speed</i>
      <span>{{ speedToastMessage }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import type Plyr from "plyr";
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, useTemplateRef, watch } from "vue";
import { useRoute } from "vue-router";
import { getPreviewURL, getPreviewURLPublic } from "@/api/resources.js";
import { getTranscodeSessions } from "@/api/transcode.js";
import { getCachedViewToken } from "@/api/viewToken.js";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import FloatingActionButton from "@/components/settings/FloatingActionButton.vue";
import { getPlayerI18n, getSettingsElements } from "@/composables/player/plyrSettingsElements.ts";
import {
  CAPTION_SIZE_IDS,
  getCaptionSizeLabel,
  getStoredCaptionSize,
  setStoredCaptionSize,
} from "@/composables/player/playerStorage.ts";
import { useCaptionSize } from "@/composables/player/useCaptionSize.ts";
import { useDoubleTapSeek } from "@/composables/player/useDoubleTapSeek.ts";
import { useEmbeddedArtwork } from "@/composables/player/useEmbeddedArtwork.ts";
import { useFullscreenState } from "@/composables/player/useFullscreenState.ts";
import { useGestureState } from "@/composables/player/useGestureState.ts";
import { useMediaKeyboard } from "@/composables/player/useMediaKeyboard.ts";
import { type MountedFileKey, resolveMountedFileKey, useMediaPlayer } from "@/composables/player/useMediaPlayer.ts";
import { useMediaSession } from "@/composables/player/useMediaSession.ts";
import { usePlaybackQueue } from "@/composables/player/usePlaybackQueue.ts";
import { useSwipeNavigationGestures } from "@/composables/player/useSwipeNavigationGestures.ts";
import { useI18n } from "@/i18n/index.ts";
import { notify } from "@/notify/index.ts";
import { appendMediaFragment, isPlayBlockedError, primePlay } from "@/player/playbackIntent.js";
import { enableScrubPreview } from "@/player/scrubPreview.js";
import { blockSeekOnInput, enableSeekOnRelease } from "@/player/seekOnRelease.js";
import { transcodeLog, transcodeWarn } from "@/player/transcodeDebug.js";
import {
  isTranscodeAvailable,
  isTranscodeHlsMode,
  normalizeTranscodeMode,
  normalizeTranscodeSessionStatus,
  TRANSCODE_HLS_MODES,
  TRANSCODE_MODES,
} from "@/player/transcodeMode.js";
import { TranscodePlaybackController } from "@/player/transcodePlaybackController.js";
import { createTranscodeClientId } from "@/player/transcodeSession.js";
import { enableVideoLoadingIndicator } from "@/player/videoLoading.js";
import { router } from "@/router/index.ts";
import { getters, mutations, state } from "@/store/index.ts";
import { needsTranscodeFirst } from "@/utils/canBrowserPlayNative.js";
import { globalVars } from "@/utils/constants.ts";
import {
  buildPlaybackQueryPatch,
  parseTranscodeModeFromQuery,
  playbackQueryChanged,
} from "@/utils/playbackQuery.js";
import { getSubtitleFormatExtension } from "@/utils/subtitles.js";
import { resolveTranscodeModeForPlayback, savePreferredTranscodeMode } from "@/utils/transcodePreference.js";

interface VideoFile {
  name?: string;
  type?: string;
  path?: string;
  source?: string;
  modified?: string;
  hasPreview?: boolean;
  viewToken?: string;
  subtitles?: unknown[];
  metadata?: {
    title?: string;
    artist?: string;
    album?: string;
    year?: string | number;
    albumArt?: string;
    duration?: number;
    videoCodec?: string;
    audioCodec?: string;
  };
}

interface SubtitleTrack {
  src?: string;
  name?: string;
  srclang?: string;
  language?: string;
}

/** Playhead restore (transcode -> native); applied when the video stream attaches. */
interface ResumeSnapshot {
  currentTime: number;
  wasPlaying?: boolean;
}

interface RecoverContext {
  startSec?: number;
  sessionId?: string;
  mode?: string;
  shouldAutoplay?: boolean;
  playPrime?: ReturnType<typeof primePlay> | null;
}

/** Disposer returned by enableVideoLoadingIndicator; `expectPlayback` is absent on its no-op variant. */
interface LoadingIndicatorHandle {
  (): void;
  expectPlayback?: () => void;
}

interface StartTranscodeOptions {
  replaceSessionId?: string;
  startSec?: number;
  mode?: string;
  autoplay?: boolean;
  playPrime?: ReturnType<typeof primePlay> | null;
  manageSwitchState?: boolean;
}

interface TeardownTranscodeOptions {
  stopSession?: boolean;
  keepAlive?: boolean;
  keepalive?: boolean;
}

type TranscodeOffer = ReturnType<typeof normalizeTranscodeSessionStatus>;

const props = withDefaults(
  defineProps<{
    raw: string;
    req: VideoFile;
    subtitlesList?: SubtitleTrack[];
    autoPlayEnabled?: boolean;
    listing?: unknown[];
    startTranscode?: boolean;
    /** Controller owned by MediaPlayer.vue so it survives this component being re-keyed. */
    sharedTranscodePlayback?: TranscodePlaybackController | null;
  }>(),
  {
    subtitlesList: () => [],
    autoPlayEnabled: false,
    listing: () => [],
    startTranscode: false,
    sharedTranscodePlayback: null,
  },
);

const emit = defineEmits<{
  "playback-start": [];
  "navigate-previous": [];
  "navigate-next": [];
  "close-preview": [];
  "needs-transcode": [status: TranscodeOffer];
}>();

const { t } = useI18n();
const route = useRoute();

// The Plyr instance is held in a shallow ref: it is a third-party object and must not be deep-proxied.
const player = shallowRef<Plyr | null>(null);
const videoElement = useTemplateRef<HTMLVideoElement>("videoElement");

// --- State ---
/** Native video: defer stream URL until play so opening preview does not range-fetch the file. */
const videoStreamAttached = ref(false);
/** Centered spinner while the video is loading or buffering. */
const videoPlaybackLoading = ref(false);
const resumeSnapshot = ref<ResumeSnapshot | null>(null);
/** Seek target passed via Media Fragments URI (#t=) when re-attaching the native stream. */
const resumeSeekSeconds = ref<number | null>(null);
/** Keeps :src stable after a resume attach so Vue does not rebind without #t=. */
const lockedNativeVideoSrc = ref<string | null>(null);
const transcodePlayback = ref<TranscodePlaybackController | null>(null);
const transcodeSwitchInProgress = ref(false);
const transcodeMode = ref("native");
const transcodeClientId = createTranscodeClientId();

/** Source+path this instance mounted for; guards stale instances during route transitions. */
let mountedFile: MountedFileKey | null = null;
let tornDown = false;
let mountInProgress = false;
let seekOnReleaseCleanup: (() => void) | null = null;
let scrubPreviewCleanup: (() => void) | null = null;
let videoLoadingCleanup: LoadingIndicatorHandle | null = null;
let attachVideoStreamResume: (() => void) | null = null;
let transcodeFallbackAttempted = false;
let transcodeOfferEmitted = false;
/** Armed when autoplay is blocked so the next user gesture can resume playback. */
let resumeOnGestureArmed = false;
let resumeOnGestureHandler: (() => void) | null = null;
// Plyr-specific menu state (the custom caption size and transcode menus are built with innerHTML).
let captionSizeMenuInitialized = false;
let transcodeMenuInitialized = false;
let captionSizeButtons: NodeListOf<HTMLButtonElement> | null = null;
let captionSizeValueSpan: HTMLElement | null = null;
let transcodeModeButtons: NodeListOf<HTMLButtonElement> | null = null;
let transcodeModeValueSpan: HTMLElement | null = null;

// --- Derived state ---
const isMobile = computed(() => getters.isMobile());
const hasSubtitles = computed(() => props.subtitlesList.length > 0);
const fileName = computed(() => (props.req.name ? props.req.name.replace(/\.[^/.]+$/, "") : ""));
const { metadata, albumArtUrl, loadMediaMetadata, cleanupAlbumArt } = useEmbeddedArtwork({
  getMetadata: () => props.req.metadata,
  getFileName: () => fileName.value,
});

const scrubPreviewEnabled = computed(() => Boolean(props.req?.hasPreview) && getters.previewPerms().video);
const artworkUrl = computed(() => {
  if (albumArtUrl.value) return albumArtUrl.value;
  if (!scrubPreviewEnabled.value) return undefined;
  return getters.isShare()
    ? getPreviewURLPublic(props.req.path ?? "", "original")
    : `${getPreviewURL(props.req.source, props.req.path, props.req.modified)}&size=original`;
});

const transcodeEnabled = computed(() => isTranscodeAvailable());
const transcodeActive = computed(() => transcodePlayback.value?.active === true);
const transcodeStartInProgress = computed(() => transcodePlayback.value?.startInProgress === true);
const transcodeMenuAvailable = computed(
  () => transcodeEnabled.value && getters.isLoggedIn() && !getters.isShare(),
);
const isTranscodePlaybackMode = computed(
  () => transcodeEnabled.value && TRANSCODE_HLS_MODES.includes(transcodeMode.value),
);
const wantsTranscodePlayback = computed(() => isTranscodePlaybackMode.value);
const formatNeedsTranscode = computed(() => needsTranscodeFirst(transcodeProbeParams()));
const transcodeProfileOptions = computed<string[]>(() => {
  const live = state.mediaSettings?.profiles;
  const configured: string[] = Array.isArray(live) ? live : TRANSCODE_HLS_MODES;
  return configured.filter((id) => TRANSCODE_HLS_MODES.includes(id));
});
const shouldProactivelyOfferTranscode = computed(() => {
  if (props.startTranscode) {
    return false;
  }
  if (isTranscodePlaybackMode.value || getters.isShare()) {
    return false;
  }
  return formatNeedsTranscode.value;
});

function transcodeProbeParams() {
  const meta = props.req?.metadata || {};
  return {
    videoCodec: meta.videoCodec,
    audioCodec: meta.audioCodec,
    mimeType: props.req?.type,
    fileName: props.req?.name,
  };
}

const { applyCaptionSizeClass, syncCaptionSizeSettingsVisibility } = useCaptionSize({
  player,
  isVisible: () => props.subtitlesList.length > 0,
});
const { setupMediaSession, updateMediaSessionNavHandlers, updateMediaSessionPlaybackState, clearMediaSession } =
  useMediaSession(player);
const gestures = useGestureState();
const queue = usePlaybackQueue({
  getReq: () => props.req,
  getListing: () => props.listing,
});
const {
  queueCount,
  playbackQueue,
  currentQueueIndex,
  playbackMode,
  loop,
  toastVisible,
  toastType,
  toastIcon,
  playbackModeMessage,
  showToast,
  showQueuePrompt,
  toggleSingleLoop,
  cyclePlaybackModes,
  playbackMenuInitialized,
  loopMenuInitialized,
  applyPlaybackMenu,
  applyLoopMenu,
  resetPlaybackMenuState,
} = queue;

const shouldAutoplay = computed(() => props.autoPlayEnabled || playbackQueue.value.length > 1);
const showQueueButton = computed(() => !isFullscreen.value);

const doubleTap = useDoubleTapSeek({
  player,
  gestures,
  getPreviewType: () => "video",
  getAudioGestureRoot: () => null,
  requestPlay,
});
const {
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
} = doubleTap;
const swipe = useSwipeNavigationGestures({
  player,
  gestures,
  getPreviewType: () => "video",
  isMobileLyricsActive: () => false,
  getGestureSurface: doubleTap.getGestureSurface,
  isControlOrMenuTarget: doubleTap.isControlOrMenuTarget,
  onNavigatePrevious: () => emit("navigate-previous"),
  onNavigateNext: () => emit("navigate-next"),
  onClosePreview: () => emit("close-preview"),
});
const { speedToastVisible, speedToastMessage, setupSwipeGestures, teardownSwipeGestures, clearDismissAnimTimers } = swipe;
const { isFullscreen, onFullscreenEnter, onFullscreenExit } = useFullscreenState({
  onEnter: swipe.resetEdgeGestureImmediate,
});
const {
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
  handleMediaEnd,
} = useMediaPlayer({
  player,
  getMediaElement: () => videoElement.value,
  getRequestedFile: () => props.req,
  getMountedFile: () => mountedFile,
  isTornDown: () => tornDown,
  destroy,
  beforeStopMedia: clearAttachVideoStreamWait,
});
const { attachKeyboard, detachKeyboard } = useMediaKeyboard({
  player,
  getPreviewType: () => "video",
  cyclePlaybackModes,
  toggleSingleLoop,
  showQueuePrompt,
  flashSkipFeedback,
  toggleDesktopPanel: () => undefined, // the desktop panel exists only in AudioPlayer
});

// --- Native stream source ---
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
    return appendMediaFragment(props.raw, resumeTime ?? 0);
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
  if (formatNeedsTranscode.value) {
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

/** Plyr only builds settings panels at mount; applyCustomSettings toggles visibility when media/live settings settle. */
const settingsMenu = computed(() => {
  const base = ["captions", "captionSize", "quality", "speed", "playback", "loop"];
  // Register the slot whenever authenticated video can show it later.
  if (getters.isLoggedIn() && !getters.isShare()) {
    return [...base, "transcodeMode"];
  }
  return base;
});

/** Rewind / fast-forward in the control bar only on non-mobile (gestures stay as elsewhere). */
const playerOptions = computed<Plyr.Options>(() => {
  const controlsDesktop = [
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
    "fullscreen",
  ];
  const controlsMobile = [
    "play-large",
    "play",
    "progress",
    "current-time",
    "duration",
    "mute",
    "volume",
    "captions",
    "settings",
    "fullscreen",
  ];
  return {
    controls: isMobile.value ? controlsMobile : controlsDesktop,
    settings: settingsMenu.value,
    i18n: {
      playback: t("player.playbackMode"),
      captionSize: t("player.captionSize"),
      loop: t("player.loop"),
      transcodeMode: t("general.transcode"),
    },
    speed: {
      selected: 1,
      options: [0.25, 0.5, 0.75, 1, 1.25, 1.5, 2],
    },
    disableContextMenu: true,
    seekTime: 10,
    hideControls: true,
    keyboard: { focused: true, global: true },
    tooltips: { controls: true, seek: !scrubPreviewEnabled.value },
    loop: { active: false },
    blankVideo: "",
    muted: false,
    autoplay: false,
    playsinline: true,
    clickToPlay: false, // we manage this ourselves with the gestures, plyr has a issue where this doesn't work in mobile.
    resetOnEnd: false,
    preload: "none",
    fullscreen: {
      enabled: true,
      fallback: true,
      // iPhone Safari has no element Fullscreen API; only video.webkitEnterFullscreen() hides the browser UI.
      iosNative: true,
      container: ".media-player",
    },
    iconUrl: `${globalVars.baseURL}public/static/img/plyr.svg`,
    // Blob/async tracks need addtrack → captions.update; otherwise meta never fills and toggle CC throws (track undefined).
    // Do not call toggleCaptions() here — Plyr already applies `plyr` localStorage for captions on/off.
    captions: {
      active: false,
      language: "auto",
      update: true,
    },
    listeners: {
      seek: blockSeekOnInput,
    },
  };
});

/** Plyr captions menu: show format only (e.g. `.srt`, `.ass`), not the video basename. */
function subtitleTrackLabel(sub: SubtitleTrack): string {
  const ext = getSubtitleFormatExtension(sub?.name || "");
  return ext || sub?.name || "";
}

/** The element Plyr plays on; falls back to the template ref before the player exists. */
function getPlaybackMedia(): HTMLMediaElement | null | undefined {
  return (player.value as unknown as { media?: HTMLMediaElement } | null)?.media || videoElement.value;
}

function resetFilePlaybackState(): void {
  videoStreamAttached.value = false;
  resumeSeekSeconds.value = null;
  lockedNativeVideoSrc.value = null;
  hasStartedPlayback.value = false;
  transcodeOfferEmitted = false;
  transcodeFallbackAttempted = false;
  resetOverlaidHints();
}

// --- Watchers ---
watch(playbackMode, async (newMode, oldMode) => {
  if (newMode === oldMode) return;
  if (oldMode !== undefined) {
    // can only be "single" via clearPlaybackQueue
    toastType.value = newMode === "single" ? "cleared" : "mode";
    showToast();
  }
  updateMediaSessionNavHandlers();
  await nextTick();
  ensurePlaybackModeApplied();
});
watch(loop, async (newVal, oldVal) => {
  if (newVal === oldVal) return;
  toastType.value = "loop";
  showToast();
  updateMediaSessionNavHandlers();
  await nextTick();
  ensurePlaybackModeApplied();
});
watch(currentQueueIndex, updateMediaSessionNavHandlers);
watch(
  () => state.playbackQueue.shouldTogglePlayPause || false,
  (newVal, oldVal) => {
    if (newVal !== oldVal) togglePlayPause();
  },
);
watch(
  () => state.playbackQueue.isPlaying || false,
  (isPlaying) => syncOverlaidHintPlayback(isPlaying),
);
watch(() => props.req.path, resetFilePlaybackState);
watch(
  () => props.req.source,
  (newSource, oldSource) => {
    if (!newSource || newSource === oldSource) {
      return;
    }
    resetFilePlaybackState();
    destroy();
    tornDown = false;
    void initializePlayer();
  },
);
watch(() => state.route.path, stopIfRouteLeftThisFile);
watch(
  () => props.subtitlesList,
  async (newSubs, oldSubs) => {
    const gained = newSubs && newSubs.length > 0 && (!oldSubs || oldSubs.length === 0);
    const lost = (!newSubs || newSubs.length === 0) && oldSubs && oldSubs.length > 0;
    if (gained || lost) {
      captionSizeMenuInitialized = false;
    }
    if (gained) {
      if (!player.value) {
        await nextTick();
        void initializePlayer();
      } else {
        await nextTick();
        applyCustomSettings(player.value);
        syncCaptionSizeSettingsVisibility();
        applyCaptionSizeClass();
      }
    } else if (player.value) {
      await nextTick();
      syncCaptionSizeSettingsVisibility();
      captionSizeMenuInitialized = false;
      applyCustomSettings(player.value);
    }
  },
  { deep: true },
);
watch(
  () => route.query,
  (newQuery, oldQuery) => {
    const urlTranscode = parseTranscodeModeFromQuery(newQuery);
    const oldUrlTranscode = parseTranscodeModeFromQuery(oldQuery);
    if (urlTranscode !== oldUrlTranscode) {
      const requestedMode = urlTranscode || "native";
      if (requestedMode !== transcodeMode.value) {
        void switchTranscodeMode(requestedMode);
      }
    }
  },
);
watch(transcodeSwitchInProgress, (val) => setTranscodeModeMenuDisabled(val));
watch(transcodeMode, updateTranscodeModeMenuSelection);
watch(transcodeMenuAvailable, () => {
  if (player.value) {
    applyCustomSettings(player.value);
  }
});

// --- Mount / teardown ---
function onPageHide(event: PageTransitionEvent): void {
  if (event.persisted) return;
  teardownTranscodePlayback({ stopSession: true, keepalive: true });
  stopMediaElement();
}

/** Stops playback and releases the player, listeners, transcode session and album art. Safe to call more than once. */
function destroy(): void {
  if (tornDown) {
    return;
  }
  tornDown = true;
  cleanupAlbumArt();

  if (scrubPreviewCleanup) {
    scrubPreviewCleanup();
    scrubPreviewCleanup = null;
  }
  if (seekOnReleaseCleanup) {
    seekOnReleaseCleanup();
    seekOnReleaseCleanup = null;
  }
  if (videoLoadingCleanup) {
    videoLoadingCleanup();
    videoLoadingCleanup = null;
  }
  videoPlaybackLoading.value = false;
  teardownOverlaidHintController();
  mountedFile = null;
  lockedNativeVideoSrc.value = null;
  const media = getActiveMediaElement();
  const isTransition = state.navigation.isTransitioning;

  // Stop the element before controller/player teardown. Those paths can
  // throw or await, and a failure there must never leave audio playing on
  // an element that is about to be detached from the DOM.
  if (media) {
    stopMediaElement(media);
  }

  // Any deliberate teardown of this viewer (route leave, source swap, close)
  // must stop the server session.
  teardownTranscodePlayback({ stopSession: true });
  if (!usesSharedTranscodePlayback()) {
    transcodePlayback.value = null;
  }
  disarmResumeOnNextGesture();

  if (player.value) {
    if (isTransition) {
      rememberFullscreenForTransition();
    }
    restoreNativePlay();
    teardownSwipeGestures();
    teardownDoubleTapSeek();
    clearRewindForwardFeedback();
    clearMediaSession();
  }
  destroyPlayer(media);

  resetPlaybackMenuState();
  captionSizeMenuInitialized = false;
  transcodeMenuInitialized = false;
  captionSizeButtons = null;
  captionSizeValueSpan = null;
  transcodeModeButtons = null;
  transcodeModeValueSpan = null;
}

/** Starts playback from the gesture surface or the overlaid play button. */
function requestPlay(): void {
  // Only offer transcoding when the source cannot play natively and no
  // transcode is already running. Otherwise a transcoded `.wmv` could never
  // be resumed from the player surface or the overlaid play button.
  if (formatNeedsTranscode.value && !isTranscodePlaybackMode.value && !transcodeActive.value) {
    void offerTranscodePlayback();
    return;
  }
  void Promise.resolve(player.value?.play()).catch((playErr: Error) => {
    transcodeWarn("player.play failed", { message: playErr?.message });
    if (playErr?.name === "NotSupportedError" && !transcodeActive.value) {
      void offerTranscodePlayback();
    }
  });
}

function handlePlay(): void {
  if (tornDown) {
    return;
  }
  emit("playback-start");
  if (isTranscodePlaybackMode.value && !transcodeActive.value && !transcodeStartInProgress.value) {
    void startTranscodePlayback({ autoplay: true });
  }
}

function ensurePlaybackModeApplied(): void {
  if (!player.value) return;
  try {
    const settings = getSettingsElements(player.value);
    const settingsMenuEl = settings?.menu;
    const buttons = settings?.buttons;
    const menuOpen =
      settingsMenuEl && settingsMenuEl.style.display !== "none" && settingsMenuEl.getAttribute("hidden") === null;
    const needPlayback = buttons?.playback && !playbackMenuInitialized.value;
    const needCaptionSize = buttons?.captionSize && !captionSizeMenuInitialized;
    const needLoop = buttons?.loop && !loopMenuInitialized.value;
    const needTranscodeMode = buttons?.transcodeMode && !transcodeMenuInitialized;

    if (menuOpen || needPlayback || needCaptionSize || needLoop || needTranscodeMode) {
      applyCustomSettings(player.value);
    }
  } catch (error) {
    console.error("Error ensuring playback mode applied:", error);
  }
}

async function preparePlayerMount(): Promise<void> {
  // For videos with subtitle metadata, wait for subtitles to load before initializing Plyr
  // This prevents Plyr from trying to access tracks before they have valid blob URLs
  const hasSubtitleMetadata = (props.req?.subtitles?.length ?? 0) > 0;
  const subtitlesNotLoaded = props.subtitlesList.length === 0;

  if (hasSubtitleMetadata && subtitlesNotLoaded) {
    // Wait for subtitles to be loaded (watcher will call initializePlayer)
    return;
  }

  void initializePlayer();
}

async function initializePlayer(): Promise<void> {
  if (!videoElement.value || player.value || tornDown) {
    return;
  }
  await nextTick();
  if (!videoElement.value || player.value || tornDown) {
    return;
  }
  void mountPlayer();
}

function usesSharedTranscodePlayback(): boolean {
  return Boolean(props.sharedTranscodePlayback && transcodePlayback.value === props.sharedTranscodePlayback);
}

function bindTranscodePlaybackController(): void {
  if (props.sharedTranscodePlayback && !props.sharedTranscodePlayback.disposed) {
    transcodePlayback.value = props.sharedTranscodePlayback;
    return;
  }
  if (!transcodePlayback.value || transcodePlayback.value.disposed) {
    transcodePlayback.value = new TranscodePlaybackController();
  }
}

async function mountPlayer(): Promise<void> {
  if (!videoElement.value || player.value || mountInProgress || tornDown) {
    return;
  }
  mountInProgress = true;
  try {
    bindTranscodePlaybackController();
    mountedFile = resolveMountedFileKey(props.req?.source, props.req?.path);
    initTranscodeModeFromRoute();
    await stripPlaybackTimeFromUrl();
    if (!videoElement.value || tornDown || player.value) {
      return;
    }
    if (!props.startTranscode && !wantsTranscodePlayback.value && shouldProactivelyOfferTranscode.value) {
      await offerTranscodePlayback();
      if (!videoElement.value || tornDown || player.value) {
        return;
      }
    }
    createPlayer(playerOptions.value);
    if (!player.value) {
      return;
    }
    if (!shouldAttachVideoStream.value) {
      overridePlay((nativePlay) => {
        if (!videoStreamAttached.value) {
          void attachVideoStreamAndPlay();
          return Promise.resolve();
        }
        return nativePlay();
      });
    }
    setupMediaSession({
      title: metadata.value?.title || fileName.value,
      artist: metadata.value?.artist || globalVars.name || "Filebrowser Quantum",
      album: metadata.value?.album || "",
      // Embedded artwork (if any), otherwise the thumbnail/preview url.
      artworkSrc: artworkUrl.value,
    });
    setupPlayerEvents();
    seekOnReleaseCleanup = enableSeekOnRelease(player.value);
    setupScrubPreview();
    setupVideoLoadingIndicator();
    if (props.startTranscode || (wantsTranscodePlayback.value && !shouldProactivelyOfferTranscode.value)) {
      await startTranscodePlayback({
        autoplay: shouldAutoplay.value,
      });
    }
    resumeFullscreenAfterNavigation();
  } finally {
    mountInProgress = false;
  }
}

async function stripPlaybackTimeFromUrl(): Promise<void> {
  const query = route.query as Record<string, string | undefined>;
  const nextQuery = buildPlaybackQueryPatch(query, { time: null });
  if (!playbackQueryChanged(route.query, nextQuery)) {
    return;
  }
  await router.replace({
    path: route.path,
    query: nextQuery,
    hash: route.hash,
  });
}

function syncPlaybackUrl({ transcodeMode: explicit }: { transcodeMode?: string } = {}): void {
  if (transcodeSwitchInProgress.value) {
    return;
  }
  const explicitMode = explicit !== undefined;
  const resolvedMode = normalizeTranscodeMode(explicitMode ? explicit : transcodeMode.value);
  let modeForQuery: string | null = null;
  if (TRANSCODE_HLS_MODES.includes(resolvedMode)) {
    if (explicitMode || transcodeActive.value) {
      modeForQuery = resolvedMode;
    }
  }
  const nextQuery = buildPlaybackQueryPatch(route.query as Record<string, string | undefined>, {
    time: null,
    transcodeMode: modeForQuery,
  });
  if (!playbackQueryChanged(route.query, nextQuery)) {
    return;
  }
  void router.replace({
    path: route.path,
    query: nextQuery,
    hash: route.hash,
  });
}

// --- Resume and native stream attach ---
function applyResumeSeek(el: HTMLVideoElement | null | undefined, snapshot: ResumeSnapshot | null, onDone?: () => void): void {
  if (!el || !snapshot) {
    onDone?.();
    return;
  }
  const targetTime = Number.isFinite(snapshot.currentTime) ? snapshot.currentTime : 0;
  let finished = false;
  let resumeCompleted = false;
  const completeResume = () => {
    if (resumeCompleted) {
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
    onDone?.();
  };
  const finish = () => {
    if (finished) {
      return;
    }
    finished = true;
    if (snapshot.wasPlaying !== false && el.readyState < HTMLMediaElement.HAVE_FUTURE_DATA) {
      el.addEventListener("canplay", () => completeResume(), { once: true });
      window.setTimeout(() => completeResume(), 8000);
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
    const onSeeked = () => {
      if (nearTarget()) {
        finish();
        return;
      }
      el.addEventListener(
        "canplay",
        () => {
          try {
            writeTime(targetTime);
          } catch (_) {
            finish();
            return;
          }
          el.addEventListener(
            "seeked",
            () => {
              if (nearTarget()) {
                finish();
              }
            },
            { once: true },
          );
        },
        { once: true },
      );
    };
    el.addEventListener("seeked", onSeeked, { once: true });
    window.setTimeout(() => {
      if (!finished && nearTarget()) {
        finish();
      }
    }, 5000);
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
    el.addEventListener("canplay", seek, { once: true });
  };
  if (el.readyState >= HTMLMediaElement.HAVE_METADATA) {
    whenSeekable();
  } else {
    el.addEventListener("loadedmetadata", whenSeekable, { once: true });
  }
}

async function attachVideoStreamAndPlay(): Promise<void> {
  if (tornDown || videoStreamAttached.value || !props.raw) {
    return;
  }
  videoStreamAttached.value = true;
  videoLoadingCleanup?.expectPlayback?.();
  await nextTick();
  await nextTick();
  const el = videoElement.value;
  if (tornDown || !el) {
    return;
  }
  const snap = resumeSnapshot.value;
  const resumeTime = snap?.currentTime ?? resumeSeekSeconds.value;
  const streamUrl = appendMediaFragment(props.raw, resumeTime ?? 0);
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
      el.addEventListener("canplay", onCanPlay, { once: true });
    }
  }

  const startDefault = () => {
    clearAttachVideoStreamWait();
    if (tornDown) {
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
      el.addEventListener("loadedmetadata", onMeta, { once: true });
    }
  };

  const start = snap ? startWithResume : startDefault;
  attachVideoStreamResume = start;
  if (snap) {
    if (el.readyState >= HTMLMediaElement.HAVE_METADATA) {
      start();
    } else {
      el.addEventListener("loadedmetadata", start, { once: true });
    }
  } else {
    start();
  }
}

function clearAttachVideoStreamWait(): void {
  const el = videoElement.value;
  if (el && attachVideoStreamResume) {
    el.removeEventListener("canplay", attachVideoStreamResume);
    el.removeEventListener("loadedmetadata", attachVideoStreamResume);
  }
  attachVideoStreamResume = null;
}

function attemptResumePlay(el: HTMLVideoElement | null | undefined, snapshot: ResumeSnapshot | null): void {
  if (tornDown || !el || snapshot?.wasPlaying === false) {
    return;
  }
  const tryPlay = () => {
    if (tornDown) {
      return;
    }
    const playCall = getNativePlay() ?? (() => player.value?.play?.() ?? el.play());
    void Promise.resolve(playCall())
      .then(() => {
        mutations.setPlaybackState(true);
        emit("playback-start");
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
  el.addEventListener("canplay", tryPlay, { once: true });
}

// --- Transcode ---
function resolveViewTokenForTranscode(): string {
  return props.req?.viewToken || getCachedViewToken(props.req?.source) || "";
}

function ensureTranscodePlaybackController(): TranscodePlaybackController {
  bindTranscodePlaybackController();
  return transcodePlayback.value as TranscodePlaybackController;
}

async function offerTranscodePlayback(): Promise<void> {
  if (tornDown || transcodeSwitchInProgress.value || isTranscodePlaybackMode.value) {
    return;
  }
  if (transcodeOfferEmitted) {
    return;
  }
  if (getters.isShare() || !formatNeedsTranscode.value) {
    return;
  }
  transcodeOfferEmitted = true;
  if (!transcodeEnabled.value) {
    // Transcoding is off on this server: show the unsupported notice without
    // asking for sessions, which would answer 503.
    emit("needs-transcode", { enabled: false, canStart: false, sessions: [] });
    return;
  }
  try {
    const status = await getTranscodeSessions();
    emit(
      "needs-transcode",
      normalizeTranscodeSessionStatus(status, {
        source: props.req?.source,
        path: props.req?.path,
      }),
    );
  } catch (err) {
    console.error("Failed to check transcode sessions:", err);
    emit("needs-transcode", { enabled: false, canStart: false, sessions: [] });
  }
}

function onVideoLoadedMetadata(): void {
  if (transcodeOfferEmitted || getters.isShare()) {
    return;
  }
  const video = videoElement.value;
  if (!video || video.videoWidth > 0 || video.videoHeight > 0) {
    return;
  }
  if (!Number.isFinite(video.duration) || video.duration <= 0) {
    return;
  }
  void offerTranscodePlayback();
}

function syncPlayerAfterTranscodeReady(): void {
  if (!transcodeActive.value || !player.value) {
    return;
  }
  const video = getPlaybackMedia() as HTMLVideoElement | null | undefined;
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
  if (tornDown) {
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
 * Autoplay was blocked (Safari requires the play() call to come directly from
 * a gesture handler). Arm a one-shot resume so a single tap or keypress
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
  if (tornDown) {
    return;
  }
  mutations.setPlaybackState(true);
  emit("playback-start");
}

function initTranscodeModeFromRoute(): void {
  if (!transcodeEnabled.value) {
    transcodeMode.value = "native";
    return;
  }
  if (props.startTranscode) {
    transcodeMode.value = resolveTranscodeModeForPlayback(route.query);
    return;
  }
  if (needsTranscodeFirst(transcodeProbeParams())) {
    transcodeMode.value = "native";
    return;
  }
  const urlTranscode = parseTranscodeModeFromQuery(route.query);
  if (urlTranscode) {
    transcodeMode.value = urlTranscode;
    return;
  }
  transcodeMode.value = "native";
}

function resolveActiveTranscodeMode(): string {
  const urlMode = parseTranscodeModeFromQuery(route.query);
  if (urlMode) {
    return urlMode;
  }
  if (TRANSCODE_HLS_MODES.includes(transcodeMode.value)) {
    return transcodeMode.value;
  }
  return resolveTranscodeModeForPlayback(route.query);
}

async function resolveReuseSessionId(): Promise<string | undefined> {
  if (!props.req?.source || !props.req?.path) {
    return undefined;
  }
  try {
    const status = await getTranscodeSessions();
    const sameFile = (status.sessions || []).filter(
      (session: { source?: string; path?: string }) =>
        session.source === props.req?.source && session.path === props.req?.path,
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
}: StartTranscodeOptions = {}): Promise<void> {
  if (tornDown) {
    return;
  }
  transcodeLog("startTranscodePlayback called", {
    source: props.req?.source,
    path: props.req?.path,
    transcodeEnabled: transcodeEnabled.value,
    transcodeMode: transcodeMode.value,
    requestedMode: mode,
    hasPlayer: Boolean(player.value),
    replaceSessionId,
    startSec,
  });
  if (!props.req?.source || !props.req?.path || !transcodeEnabled.value) {
    transcodeWarn("startTranscodePlayback skipped — preconditions failed", {
      source: props.req?.source,
      path: props.req?.path,
      transcodeEnabled: transcodeEnabled.value,
    });
    return;
  }
  const activeMode = isTranscodeHlsMode(mode) ? normalizeTranscodeMode(mode) : resolveActiveTranscodeMode();
  if (!isTranscodeHlsMode(activeMode)) {
    transcodeWarn("startTranscodePlayback skipped — not an HLS mode", { activeMode });
    return;
  }
  const viewToken = resolveViewTokenForTranscode();
  if (!viewToken) {
    transcodeWarn("startTranscodePlayback skipped — missing view token");
    return;
  }
  const manageSwitch = manageSwitchState && !transcodeSwitchInProgress.value;
  if (manageSwitch) {
    transcodeSwitchInProgress.value = true;
  }
  const effectiveReplaceSessionId =
    replaceSessionId ??
    transcodePlayback.value?.sessionController?.session?.id ??
    (await resolveReuseSessionId());
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

  try {
    transcodeLog("preparing video element for HLS", {
      replaceSessionId: effectiveReplaceSessionId,
      seekSec,
    });
    videoStreamAttached.value = true;
    transcodeMode.value = activeMode;
    lockedNativeVideoSrc.value = null;

    const startParams = {
      source: props.req.source,
      path: props.req.path,
      viewToken,
      mode: activeMode,
      clientId: transcodeClientId,
      replaceSessionId: effectiveReplaceSessionId,
      startSec: seekSec,
      playPrime,
      getVideoElement: () => getPlaybackMedia(),
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
      onRecoverPlayback: async (ctx?: RecoverContext) => {
        transcodeLog("recovering transcode playback after HLS failure", {
          startSec: ctx?.startSec,
          sessionId: ctx?.sessionId,
        });
        await startTranscodePlayback({
          mode: ctx?.mode ?? resolveActiveTranscodeMode(),
          startSec: ctx?.startSec ?? player.value?.currentTime ?? 0,
          replaceSessionId: ctx?.sessionId,
          autoplay: ctx?.shouldAutoplay ?? shouldAutoplay.value,
          manageSwitchState: true,
          playPrime: ctx?.playPrime,
        });
        return true;
      },
      onCapacityStatus: (status: unknown) => {
        emit(
          "needs-transcode",
          normalizeTranscodeSessionStatus(status, {
            source: props.req?.source,
            path: props.req?.path,
          }),
        );
      },
      onShowError: (err?: Error) => {
        notify.showError(err?.message || t("player.transcodeFailed"));
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
      updateTranscodeModeMenuSelection();
      syncPlaybackUrl();
      await switchToNativePlayback();
      return;
    }
    if (result.ok) {
      updateTranscodeModeMenuSelection();
      syncPlaybackUrl({ transcodeMode: activeMode });
    }
  } finally {
    if (manageSwitch) {
      transcodeSwitchInProgress.value = false;
    }
  }
}

function teardownTranscodePlayback({
  stopSession = false,
  keepAlive = false,
  keepalive = false,
}: TeardownTranscodeOptions = {}): void {
  const controller = transcodePlayback.value;
  if (!controller || keepAlive) {
    return;
  }
  if (usesSharedTranscodePlayback()) {
    controller.teardownLocal({ stopSession, keepalive });
    return;
  }
  controller.dispose({ stopSession, keepalive });
}

function onVideoPlaybackError(event: Event): void {
  if (transcodeSwitchInProgress.value || transcodeStartInProgress.value || isTranscodePlaybackMode.value) {
    return;
  }
  if (transcodeOfferEmitted || getters.isShare()) {
    return;
  }
  const code = (event.target as HTMLMediaElement | null)?.error?.code;
  if (code !== MediaError.MEDIA_ERR_DECODE && code !== MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED) {
    return;
  }
  void offerTranscodePlayback();
}

function getTranscodeModeLabel(mode: string): string {
  switch (normalizeTranscodeMode(mode)) {
    case "datasaver":
      return t("general.dataSaver");
    case "balanced":
      return t("general.balanced");
    case "quality":
      return t("general.quality");
    default:
      return t("general.off");
  }
}

function updateTranscodeModeMenuSelection(): void {
  if (!transcodeModeButtons) {
    return;
  }
  const label = getTranscodeModeLabel(transcodeMode.value);
  for (const btn of transcodeModeButtons) {
    btn.setAttribute("aria-checked", String(btn.getAttribute("value") === transcodeMode.value));
  }
  if (transcodeModeValueSpan) {
    // Only the profile name is shown; the resolved encoder/accel is a
    // backend/ops detail and does not belong in the player's profile picker.
    transcodeModeValueSpan.textContent = label;
  }
}

function setTranscodeModeMenuDisabled(disabled: boolean): void {
  if (!transcodeModeButtons) {
    return;
  }
  for (const btn of transcodeModeButtons) {
    btn.disabled = disabled;
    if (disabled) {
      btn.setAttribute("aria-disabled", "true");
    } else {
      btn.removeAttribute("aria-disabled");
    }
  }
}

async function switchToNativePlayback({ autoplay }: { autoplay?: boolean } = {}): Promise<void> {
  const currentTime = player.value?.currentTime ?? 0;
  const shouldPlay = autoplay ?? !!player.value?.playing;
  transcodeLog("switchToNativePlayback", { currentTime, shouldPlay });

  // Release the HLS source even when a shared controller owns it. hls.js
  // otherwise keeps its MediaSource attached to the element and the native
  // source never loads, leaving the player frozen on the last frame.
  const controller = transcodePlayback.value;
  if (controller) {
    if (usesSharedTranscodePlayback()) {
      controller.teardownLocal({ stopSession: true });
    } else {
      controller.dispose({ stopSession: true });
    }
  }
  lockedNativeVideoSrc.value = null;
  videoStreamAttached.value = false;
  // hls.js leaves its (now revoked) MediaSource blob URL on the element, and
  // attachVideoStreamAndPlay only binds when no src is present.
  const media = getPlaybackMedia();
  if (media?.hasAttribute("src")) {
    media.removeAttribute("src");
  }
  await nextTick();

  // Restore the playhead through the resume path, which waits for the
  // new source to report metadata before seeking. Setting currentTime any
  // earlier is discarded and playback restarts at 0:00.
  const canResume = Number.isFinite(currentTime) && currentTime > 0;
  if (canResume) {
    resumeSeekSeconds.value = currentTime;
    resumeSnapshot.value = {
      currentTime,
      wasPlaying: shouldPlay,
    };
  }

  await attachVideoStreamAndPlay();
  if (!canResume && shouldPlay && player.value) {
    await Promise.resolve(player.value.play()).catch(() => {});
  }
}

async function switchTranscodeMode(
  requested: string,
  { playPrime }: { playPrime?: ReturnType<typeof primePlay> | null } = {},
): Promise<void> {
  const mode = normalizeTranscodeMode(requested);
  if (!TRANSCODE_MODES.includes(mode)) {
    return;
  }
  if (mode === transcodeMode.value || transcodeSwitchInProgress.value) {
    return;
  }
  const previousMode = transcodeMode.value;
  const wasPlaying = !!player.value?.playing;
  transcodeLog("switchTranscodeMode start", {
    mode,
    previousMode,
    wasPlaying,
    playerPaused: player.value?.paused,
    mediaPaused: getPlaybackMedia()?.paused,
    shouldAutoplay: shouldAutoplay.value,
    currentTime: player.value?.currentTime,
  });
  transcodeSwitchInProgress.value = true;
  // Capture before mode/teardown changes so same-file profile switches always
  // send replaceSessionId even if the controller is mid-start.
  const replaceSessionId = transcodePlayback.value?.sessionController?.session?.id ?? (await resolveReuseSessionId());
  transcodeMode.value = mode;
  if (TRANSCODE_HLS_MODES.includes(mode)) {
    savePreferredTranscodeMode(mode);
  }
  try {
    if (TRANSCODE_HLS_MODES.includes(mode)) {
      await startTranscodePlayback({
        replaceSessionId,
        startSec: player.value?.currentTime ?? 0,
        mode,
        autoplay: wasPlaying || shouldAutoplay.value,
        playPrime: wasPlaying ? playPrime : null,
        manageSwitchState: false,
      });
    } else if (needsTranscodeFirst(transcodeProbeParams())) {
      transcodeOfferEmitted = false;
      if (!usesSharedTranscodePlayback()) {
        teardownTranscodePlayback({ stopSession: true });
      }
      try {
        const status = await getTranscodeSessions();
        emit(
          "needs-transcode",
          normalizeTranscodeSessionStatus(status, {
            source: props.req?.source,
            path: props.req?.path,
          }),
        );
      } catch (err) {
        console.error("Failed to check transcode sessions:", err);
      }
    } else {
      await switchToNativePlayback({ autoplay: wasPlaying || shouldAutoplay.value });
    }
    updateTranscodeModeMenuSelection();
  } catch (err) {
    console.error("Transcode mode switch failed:", err);
    transcodeMode.value = previousMode;
    updateTranscodeModeMenuSelection();
    if (previousMode === "native") {
      await switchToNativePlayback();
    } else {
      await startTranscodePlayback({
        startSec: player.value?.currentTime ?? 0,
        manageSwitchState: false,
      });
    }
  } finally {
    transcodeSwitchInProgress.value = false;
    syncPlaybackUrl({ transcodeMode: transcodeMode.value });
    transcodeLog("switchTranscodeMode done", {
      mode: transcodeMode.value,
      transcodeActive: transcodeActive.value,
      playerPaused: player.value?.paused,
      mediaPaused: getPlaybackMedia()?.paused,
      videoPlaybackLoading: videoPlaybackLoading.value,
    });
  }
}

// --- Player features ---
function getPlaybackDuration(): number {
  const fromPlayer = player.value?.duration;
  if (fromPlayer && Number.isFinite(fromPlayer) && fromPlayer > 0) {
    return fromPlayer;
  }
  const meta = props.req?.metadata?.duration;
  if (meta && Number.isFinite(meta) && meta > 0) {
    return meta;
  }
  return 0;
}

function setupVideoLoadingIndicator(): void {
  if (!player.value) {
    return;
  }
  videoLoadingCleanup?.();
  videoLoadingCleanup = enableVideoLoadingIndicator(player.value, (loading: boolean) => {
    videoPlaybackLoading.value = loading;
  });
}

function setupScrubPreview(): void {
  if (!scrubPreviewEnabled.value || !player.value) {
    return;
  }
  scrubPreviewCleanup?.();
  scrubPreviewCleanup = enableScrubPreview(player.value, {
    buildPreviewUrl: (atPercentage: number) => {
      const base = getters.isShare()
        ? getPreviewURLPublic(props.req.path ?? "", "large")
        : `${getPreviewURL(props.req.source, props.req.path, props.req.modified)}&size=large`;
      return `${base}&atPercentage=${atPercentage}`;
    },
    getDuration: () => getPlaybackDuration(),
  });
}

function setupPlayerEvents(): void {
  const current = player.value;
  if (!current) return;
  mutations.setPlaybackState(current.playing);
  const events: [Plyr.StandardEvent | Plyr.Html5Event, () => void][] = [
    ["ended", handleMediaEnd],
    [
      "play",
      () => {
        hasStartedPlayback.value = true;
        mutations.setPlaybackState(true);
        updateMediaSessionPlaybackState();
        transcodePlayback.value?.updatePlayback({
          currentTime: player.value?.currentTime,
          paused: false,
        });
      },
    ],
    [
      "pause",
      () => {
        mutations.setPlaybackState(false);
        updateMediaSessionPlaybackState();
        transcodePlayback.value?.updatePlayback({
          currentTime: player.value?.currentTime,
          paused: true,
        });
      },
    ],
    [
      "timeupdate",
      () => {
        updateMediaSessionPlaybackState();
        if (transcodePlayback.value?.sessionController) {
          transcodePlayback.value.updatePlayback({
            currentTime: player.value?.currentTime,
            paused: !player.value?.playing,
          });
        }
      },
    ],
    ["seeked", updateMediaSessionPlaybackState],
    ["loadedmetadata", updateMediaSessionPlaybackState],
    ["ratechange", updateMediaSessionPlaybackState],
    ["canplay", updateMediaSessionPlaybackState],
    ["enterfullscreen", onFullscreenEnter],
    ["exitfullscreen", onFullscreenExit],
    ["enterfullscreen", applyCaptionSizeClass],
    ["exitfullscreen", applyCaptionSizeClass],
  ];
  for (const [event, handler] of events) {
    onPlayerEvent(event, handler);
  }
  ensurePlaybackModeApplied();
  applyCaptionSizeClass();
  syncCaptionSizeSettingsVisibility();
  setupDoubleTapSeek();
  setupSwipeGestures();
  setupRewindForwardFeedback();
  setupOverlaidHintController();
}

/** Plyr-specific: builds the caption size and transcode menus once (innerHTML), then only updates their state. */
function applyCustomSettings(plyr: Plyr): void {
  try {
    const settings = getSettingsElements(plyr);
    const captionSizeBtn = settings?.buttons?.captionSize;
    const captionSizePanel = settings?.panels?.captionSize;

    // --- Playback and loop menus (usePlaybackQueue) ---
    applyPlaybackMenu(plyr);
    applyLoopMenu(plyr);
    // --- Caption size menu ---
    if (captionSizeBtn && captionSizePanel) {
      if (!hasSubtitles.value) {
        captionSizeBtn.setAttribute("hidden", "");
        captionSizeMenuInitialized = true;
      } else {
        captionSizeBtn.removeAttribute("hidden");
        const title = getPlayerI18n(plyr)?.captionSize || "Caption size";
        const currentSize = getStoredCaptionSize();
        const currentSizeLabel = getCaptionSizeLabel(currentSize);

        if (!captionSizeMenuInitialized) {
          const menu = captionSizePanel.querySelector<HTMLElement>('div[role="menu"]');
          if (!menu) throw new Error("caption size menu element missing");
          menu.innerHTML = CAPTION_SIZE_IDS.map(
            (id) => `<button type="button" data-plyr="caption-size" role="menuitemradio" class="plyr__control" value="${id}">
                        <span>${getCaptionSizeLabel(id)}</span>
                      </button>`,
          ).join("");

          const buttons = menu.querySelectorAll<HTMLButtonElement>('button[data-plyr="caption-size"]');
          captionSizeButtons = buttons;
          // Set initial checked state
          for (const btn of buttons) {
            btn.setAttribute("aria-checked", String(btn.getAttribute("value") === currentSize));
          }
          // Add click listeners
          for (const btn of buttons) {
            btn.addEventListener("click", (event) => {
              const value = (event.currentTarget as HTMLElement).getAttribute("value");
              if (!value || !(CAPTION_SIZE_IDS as readonly string[]).includes(value)) return;
              setStoredCaptionSize(value as (typeof CAPTION_SIZE_IDS)[number]);
              applyCaptionSizeClass();
              // Update checked states and label
              for (const b of buttons) {
                b.setAttribute("aria-checked", String(b.getAttribute("value") === value));
              }
              // Update label in button
              const label = getCaptionSizeLabel(value as (typeof CAPTION_SIZE_IDS)[number]);
              if (captionSizeValueSpan) captionSizeValueSpan.textContent = label;
            });
          }
          const valueSpan = captionSizeBtn.querySelector<HTMLElement>("span .plyr__menu__value");
          if (valueSpan) {
            valueSpan.textContent = currentSizeLabel;
            captionSizeValueSpan = valueSpan;
          } else {
            const labelSpan = captionSizeBtn.querySelector("span");
            if (labelSpan) {
              labelSpan.innerHTML = `${title}: <span class="plyr__menu__value">${currentSizeLabel}</span>`;
            }
            captionSizeValueSpan = captionSizeBtn.querySelector<HTMLElement>("span .plyr__menu__value");
          }
          captionSizeMenuInitialized = true;
        } else {
          // Update checked states and label
          if (captionSizeButtons) {
            for (const btn of captionSizeButtons) {
              btn.setAttribute("aria-checked", String(btn.getAttribute("value") === currentSize));
            }
          }
          if (captionSizeValueSpan) {
            captionSizeValueSpan.textContent = currentSizeLabel;
          }
        }
        applyCaptionSizeClass();
      }
    }

    // --- Transcode mode menu (video + transcode enabled) ---
    const transcodeBtn = settings?.buttons?.transcodeMode;
    const transcodePanel = settings?.panels?.transcodeMode;
    if (transcodeBtn && transcodePanel) {
      // Stable hook for tests — Plyr stamps data-plyr="settings" on every menu item.
      transcodeBtn.setAttribute("data-plyr-menu", "transcodeMode");
      if (!transcodeMenuAvailable.value) {
        transcodeBtn.setAttribute("hidden", "");
        // Availability flipped off after init (logout, live settings
        // sync): the built buttons are stale, so a later availability flip
        // must rebuild them.
        transcodeMenuInitialized = false;
        transcodeModeButtons = null;
        transcodeModeValueSpan = null;
      } else {
        transcodeBtn.removeAttribute("hidden");
        const currentLabel = getTranscodeModeLabel(transcodeMode.value);

        if (!transcodeMenuInitialized) {
          const menu = transcodePanel.querySelector<HTMLElement>('div[role="menu"]');
          if (!menu) throw new Error("transcode menu element missing");
          const modeButtons = [{ value: "native", label: t("general.off") }].concat(
            transcodeProfileOptions.value.map((id) => ({
              value: id,
              label: getTranscodeModeLabel(id),
            })),
          );
          menu.innerHTML = modeButtons
            .map(
              (opt) => `
                <button data-plyr="transcode-mode" type="button" role="menuitemradio" class="plyr__control" value="${opt.value}">
                  <span>${opt.label}</span>
                </button>`,
            )
            .join("");
          const buttons = menu.querySelectorAll<HTMLButtonElement>('button[data-plyr="transcode-mode"]');
          transcodeModeButtons = buttons;
          for (const btn of buttons) {
            btn.setAttribute("aria-checked", String(btn.getAttribute("value") === transcodeMode.value));
          }
          for (const btn of buttons) {
            btn.addEventListener("click", (event) => {
              if (transcodeSwitchInProgress.value) {
                return;
              }
              const value = (event.currentTarget as HTMLElement).getAttribute("value");
              if (!value || !TRANSCODE_MODES.includes(value)) {
                return;
              }
              const wasPlaying = !!player.value?.playing;
              const el = getPlaybackMedia();
              transcodeLog("transcode menu selection", {
                value,
                wasPlaying,
                playerPaused: player.value?.paused,
                mediaPaused: el?.paused,
                currentTime: player.value?.currentTime,
                shouldAutoplay: shouldAutoplay.value,
              });
              const playPrime = wasPlaying ? primePlay(el) : null;
              void switchTranscodeMode(value, { playPrime });
            });
          }
          const valueSpan = transcodeBtn.querySelector<HTMLElement>("span .plyr__menu__value");
          if (valueSpan) {
            valueSpan.textContent = currentLabel;
            transcodeModeValueSpan = valueSpan;
          } else {
            const labelSpan = transcodeBtn.querySelector("span");
            if (labelSpan) {
              labelSpan.innerHTML = `<span class="plyr__menu__value">${currentLabel}</span>`;
            }
            transcodeModeValueSpan = transcodeBtn.querySelector<HTMLElement>("span .plyr__menu__value");
          }
          transcodeMenuInitialized = true;
        } else if (transcodeModeButtons) {
          for (const btn of transcodeModeButtons) {
            btn.setAttribute("aria-checked", String(btn.getAttribute("value") === transcodeMode.value));
          }
          if (transcodeModeValueSpan) {
            transcodeModeValueSpan.textContent = currentLabel;
          }
        }
      }
    }
  } catch (error) {
    console.error("Error applying custom settings:", error);
  }
}

onMounted(async () => {
  // Record the mounted file before any async prepare step so route-leave
  // detection works while Plyr is still waiting on subtitles/metadata.
  mountedFile = resolveMountedFileKey(props.req?.source, props.req?.path);
  loadMediaMetadata();
  await preparePlayerMount();
  attachKeyboard();
  window.addEventListener("pagehide", onPageHide);
});

onBeforeUnmount(() => {
  clearDismissAnimTimers();
  // Always finish listener/media-session cleanup even if a teardown step throws,
  // so an orphaned element cannot keep playing.
  disarmResumeOnNextGesture();
  try {
    destroy();
  } finally {
    clearMediaSession();
    detachKeyboard();
    window.removeEventListener("pagehide", onPageHide);
  }
});

defineExpose({ destroy });
</script>

<style >

/**********************************
*** STYLES FOR THE MEDIA PLAYER ***
**********************************/

.media-video-container {
  width: 100%;
  height: 100%;
}



/* this is just a workaround, the own plyr poster was hiding the video poster and showing black
   when it didn't loaded (somehow happens); but summarizing, this a bug of plyr, it also loads the poster image 2 times
   cuz it sets the URL on the html video tags and in their own */
.plyr__poster {
  display: none !important;
}

/* Scrub preview popup (mounted on Plyr container / fullscreen root while scrubbing) */
.fb-scrub-preview {
  position: fixed;
  z-index: 2147483646;
  pointer-events: none;
  transform: translate(-50%, calc(-100% - 14px));
  opacity: 0;
  visibility: hidden;
  transition: opacity 0.12s ease;
}

.fb-scrub-preview--visible {
  opacity: 1;
  visibility: visible;
}

.fb-scrub-preview__frame {
  overflow: hidden;
  position: relative;
  max-height: 600px;
  max-width: 600px;
  border-radius: 14px;
  background: #000;
  border: 2px solid var(--primaryColor);
  box-shadow:
    0 0 0 1px rgb(0 0 0 / 35%),
    0 6px 20px rgb(0 0 0 / 55%),
    0 0 12px color-mix(in srgb, var(--primaryColor) 35%, transparent);
}

.fb-scrub-preview__frame::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 44%;
  background: linear-gradient(to top, rgb(0 0 0 / 60%), rgb(0 0 0 / 0%));
  pointer-events: none;
}

.fb-scrub-preview__frame img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.fb-scrub-preview__loading {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgb(0 0 0 / 45%);
}

.fb-scrub-preview__loading[hidden] {
  display: none;
}

/* Spinner sits above the previous frame while the next preview loads. */
.fb-scrub-preview__frame--loading:not(.fb-scrub-preview__frame--empty) .fb-scrub-preview__loading {
  background: rgb(0 0 0 / 50%);
}

.fb-scrub-preview__frame--loading:not(.fb-scrub-preview__frame--empty) img {
  opacity: 1;
}

.fb-scrub-preview__loading .loader {
  position: relative;
  z-index: 1;
}

.fb-scrub-preview__time {
  position: absolute;
  left: 50%;
  bottom: 8px;
  transform: translateX(-50%);
  z-index: 1;
  font-size: 13px;
  font-weight: 600;
  line-height: 1;
  color: #fff;
  white-space: nowrap;
  text-shadow:
    0 1px 3px rgb(0 0 0 / 90%),
    0 0 8px rgb(0 0 0 / 60%);
  pointer-events: none;
}

.fb-scrub-preview__arrow {
  position: absolute;
  top: calc(100% - 2px);
  transform: translateX(-50%);
  width: 14px;
  height: 8px;
  pointer-events: none;
}

.fb-scrub-preview__arrow::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 0;
  height: 0;
  border-left: 7px solid transparent;
  border-right: 7px solid transparent;
  border-top: 8px solid var(--primaryColor);
}

.plyr__control--overlaid {
  background: var(--plyr-video-control-background-hover, var(--primaryColor));
  border: 0;
  display: none;
  position: absolute;
  transition:
    opacity 0.2s ease-out,
    visibility 0.2s ease-out,
    transform 0.3s ease,
    box-shadow 0.3s ease !important;
  z-index: 5;
  height: 4em;
  inset: 50% auto auto 50%;
  transform: translate(-50%, -50%) !important;
  width: 4em !important;
  margin: 0 !important;
  border-radius: 5em !important;
  pointer-events: auto;
  cursor: pointer;
  outline: none;
  box-shadow: 0 2px 10px rgb(0 0 0 / 30%);
}

.plyr--fullscreen-active .plyr__control--overlaid {
  top: 50% !important;
  left: 50% !important;
  transform: translate(-50%, -50%) !important;
}


/* Big play button when pause/start the video */
.plyr--full-ui.plyr--video .plyr__control--overlaid {
  display: flex;
  justify-content: center;
  align-items: center;
}

.plyr--video .plyr__control--overlaid:hover,
.plyr--video .plyr__control--overlaid:focus-visible,
.plyr--fullscreen-active .plyr__control--overlaid:hover,
.plyr--fullscreen-active .plyr__control--overlaid:focus-visible {
  transform: translate(-50%, -50%) scale(1.05) !important;
  opacity: 1 !important;
  outline: none;
  box-shadow:
    0 0 0 2px rgb(255 255 255 / 90%),
    0 8px 25px color-mix(in srgb, var(--primaryColor) 30%, transparent),
    0 4px 12px rgb(0 0 0 / 20%);
}

.plyr.fb-overlaid--fade-in .plyr__control--overlaid {
  transition:
    opacity 0.4s ease-in-out,
    transform 0.3s ease,
    box-shadow 0.3s ease !important;
}

.plyr.fb-overlaid--fade-out .plyr__control--overlaid {
  opacity: 0 !important;
  visibility: visible !important;
  pointer-events: none !important;
  transition:
    opacity 0.4s ease-in-out,
    transform 0.3s ease,
    box-shadow 0.3s ease !important;
}

.plyr--playing:not(.plyr--hide-controls) .plyr__control--overlaid,
.plyr--playing.fb-overlaid--shown:not(.fb-overlaid--fade-out) .plyr__control--overlaid {
  opacity: 1 !important;
  visibility: visible !important;
  pointer-events: auto !important;
  transition:
    opacity 0.4s ease-in-out,
    transform 0.3s ease,
    visibility 0.2s ease-out,
    box-shadow 0.3s ease !important;
}

/* Hide center button while playing unless shown or fading out */
.plyr--playing.plyr--hide-controls:not(.fb-overlaid--shown, .fb-overlaid--fade-out)
  .plyr__control--overlaid {
  opacity: 0 !important;
  visibility: hidden !important;
  pointer-events: none !important;
}

/************
*** VIDEO ***
************/

/* Video container size */
.video-player-container {
  position: relative;
  width: 100%;
  height: 100%;
  background-color: #000;
}

.video-loading-overlay {
  position: absolute;
  inset: 0;
  z-index: 6;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
  background: rgb(0 0 0 / 35%);
}

/* Letterboxing and Plyr chrome: match cinema-style black */
.video-player-container .plyr {
  background-color: #000;
  box-shadow: none;
}

.video-player-container .plyr .plyr__controls {
  color: #fff;
}

/* Video size in the container */
.plyr.plyr--video {
  width: 100%;
  height: 100%;
}

.plyr .plyr__video-wrapper {
  touch-action: manipulation;
  cursor: pointer;
}



/* Hide captions button when there are no subtitle tracks */
.video-player-container.no-captions .plyr__control[data-plyr="captions"] {
  display: none !important;
}

/*
 * Caption size: --fb-captions-font-size on .plyr (caption-size--* from JS; inherits to .plyr__captions).
 * Video: cqmin replaces vmin when supported so small players don’t use the full viewport scale.
 */
.plyr__captions {
  pointer-events: none;
  font-size: var(--fb-captions-font-size, max(20px, 4vmin));
  line-height: 150%;
  font-weight: 700;
  -webkit-font-smoothing: antialiased;
  color: #fff;
  -webkit-text-stroke: 0.1em #000;
  paint-order: stroke fill;
  text-shadow:
    0.0625em 0.0625em 0 #000,
    -0.0625em 0.0625em 0 #000,
    -0.0625em -0.0625em 0 #000,
    0.0625em -0.0625em 0 #000,
    0 0.08em 0.2em rgb(0 0 0 / 55%);
}

.plyr.caption-size--small {
  --fb-captions-font-size: max(1em, 2.5vmin);
}

.plyr.caption-size--medium {
  --fb-captions-font-size: max(1.5em, 4vmin);
}

.plyr.caption-size--large {
  --fb-captions-font-size: max(2em, 5vmin);
}

.plyr.caption-size--xlarge {
  --fb-captions-font-size: max(2.5em, 5.5vmin);
}

.video-player-container .plyr--fullscreen-fallback .plyr__captions,
.video-player-container .plyr:fullscreen .plyr__captions {
  font-size: var(--fb-captions-font-size);
}

.plyr__caption {
  background: transparent;
  line-height: 150%;
}

</style>
