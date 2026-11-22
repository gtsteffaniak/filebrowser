<template>
  <div class="media-player">
    <!-- Native playback is not possible: offer transcoding instead of the player -->
    <PreviewNotice
      v-if="transcodeOffer"
      icon="feedback"
      :title="transcodeOfferTitle"
      :download-url="canDownload ? downloadUrl : undefined"
    >
      <p v-if="showTranscodeOfferDetail" class="transcode-offer-message">{{ transcodeOfferMessage }}</p>
      <ul v-if="showTranscodeSessionList" class="transcode-session-list">
        <li v-for="sess in transcodeOffer.sessions" :key="sess.id">
          {{ sess.fileName || sess.path }}
          <span v-if="sess.source" class="transcode-session-source">{{ `(${sess.source})` }}</span>
        </li>
      </ul>
      <template #actions>
        <button
          v-if="transcodeOffer.mode === 'offer'"
          type="button"
          class="button button--flat preview-action--icon-only"
          :aria-label="t('general.transcode', { suffix: '' })"
          :title="t('general.transcode', { suffix: '' })"
          @click="onStartTranscodeClick"
        >
          <i class="material-symbols">sync</i>
        </button>
      </template>
    </PreviewNotice>
    <template v-else>
    <div v-if="offerCheckPending" class="av-preview-loading">
      <LoadingSpinner size="medium" />
    </div>
    <div
      v-show="!offerCheckPending"
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
      <PlayerOverlays
        :skip-visible="skipFeedbackVisible"
        :skip-side="skipFeedbackSide"
        :skip-icon="skipFeedbackIcon"
        :skip-key="skipFeedbackKey"
        :toast-visible="toastVisible"
        :toast-icon="toastIcon"
        :toast-message="playbackModeMessage"
        :toast-loop-state="toastType === 'loop' ? (loop !== 'off' ? 'on' : 'off') : undefined"
        :speed-toast-visible="speedToastVisible"
        :speed-toast-message="speedToastMessage"
      />
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
    </template>
  </div>
</template>

<script setup lang="ts">
import type Plyr from "plyr";
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, useTemplateRef, watch } from "vue";
import { useRoute } from "vue-router";
import { getPreviewURL, getPreviewURLPublic } from "@/api/resources.js";
import { resourcesApi } from "@/api/index.js";
import PreviewNotice from "@/components/files/PreviewNotice.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import FloatingActionButton from "@/components/settings/FloatingActionButton.vue";
import PlayerOverlays from "@/components/player/PlayerOverlays.vue";
import { getPlayerI18n, getSettingsElements } from "@/composables/player/settingsElements.ts";
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
import {
  type PlayerEmits,
  type PlayerFile,
  resolveMountedFileKey,
  stripPlaybackTimeFromUrl,
  useMediaPlayer,
} from "@/composables/player/useMediaPlayer.ts";
import { useMediaSession } from "@/composables/player/useMediaSession.ts";
import { usePlaybackQueue } from "@/composables/player/usePlaybackQueue.ts";
import { useSwipeNavigationGestures } from "@/composables/player/useSwipeNavigationGestures.ts";
import { useTranscodePlayback } from "@/composables/player/useTranscodePlayback.ts";
import { useI18n } from "@/i18n/index.ts";
import { primePlay } from "@/player/playbackIntent.js";
import { enableScrubPreview } from "@/player/scrubPreview.js";
import { enableSeekOnRelease } from "@/player/seekOnRelease.js";
import { transcodeLog, transcodeWarn } from "@/player/transcodeDebug.js";
import {
  normalizeTranscodeMode,
  TRANSCODE_HLS_MODES,
  TRANSCODE_MODES,
} from "@/player/transcodeMode.js";
import { enableVideoLoadingIndicator } from "@/player/videoLoading.js";
import { getters, state } from "@/store/index.ts";
import type { FileListItem, SubtitleTrack as FileSubtitleTrack } from "@/store/types.ts";
import { globalVars } from "@/utils/constants.ts";
import { parseTranscodeModeFromQuery } from "@/utils/playbackQuery.js";
import { getSubtitleFormatExtension } from "@/utils/subtitles.js";

interface VideoFile extends PlayerFile {
  modified?: string;
  hasPreview?: boolean;
  viewToken?: string;
  subtitles?: FileSubtitleTrack[];
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

/** Disposer returned by enableVideoLoadingIndicator; `expectPlayback` is absent on its no-op variant. */
interface LoadingIndicatorHandle {
  (): void;
  expectPlayback?: () => void;
}

const props = withDefaults(
  defineProps<{
    raw: string;
    req: VideoFile;
    subtitlesList?: SubtitleTrack[];
    autoPlayEnabled?: boolean;
    listing?: FileListItem[];
  }>(),
  {
    subtitlesList: () => [],
    autoPlayEnabled: false,
    listing: () => [],
  },
);

const emit = defineEmits<PlayerEmits>();

const { t } = useI18n();
const route = useRoute();

// The Plyr instance is held in a shallow ref: it is a third-party object and must not be deep-proxied.
const player = shallowRef<Plyr | null>(null);
const videoElement = useTemplateRef<HTMLVideoElement>("videoElement");

/** Centered spinner while the video is loading or buffering. */
const videoPlaybackLoading = ref(false);

let seekOnReleaseCleanup: (() => void) | null = null;
let scrubPreviewCleanup: (() => void) | null = null;
let videoLoadingCleanup: LoadingIndicatorHandle | null = null;
// Plyr-specific menu state (the custom caption size and transcode menus are built with innerHTML).
let captionSizeMenuInitialized = false;
let transcodeMenuInitialized = false;
let captionSizeButtons: NodeListOf<HTMLButtonElement> | null = null;
let captionSizeValueSpan: HTMLElement | null = null;
let transcodeModeButtons: NodeListOf<HTMLButtonElement> | null = null;
let transcodeModeValueSpan: HTMLElement | null = null;

// --- Derived state ---
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
  onQueueChange: updateMediaSessionNavHandlers,
  applyMenus: ensurePlaybackModeApplied,
});
const {
  queueCount,
  playbackQueue,
  loop,
  toastVisible,
  toastType,
  toastIcon,
  playbackModeMessage,
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

const {
  transcodeMode,
  transcodeSwitchInProgress,
  transcodeOffer,
  transcodePlayback,
  startRequested,
  lockedNativeVideoSrc,
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
  videoStreamAttached,
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
  disarmResumeOnNextGesture,
  startVideoTranscode,
  onVideoLoadedMetadata,
  onVideoPlaybackError,
} = useTranscodePlayback({
  player,
  shouldAutoplay,
  getReq: () => props.req,
  getRaw: () => props.raw,
  getRoute: () => route,
  getVideoElement: () => videoElement.value,
  getPlaybackMedia: () => getActiveMediaElement(),
  getNativePlay: () => getNativePlay(),
  isTornDown: () => lifecycle.tornDown,
  isPlaybackLoading: () => videoPlaybackLoading.value,
  onStreamAttach: () => videoLoadingCleanup?.expectPlayback?.(),
  onPlaybackStart: () => emit("playback-start"),
  ensurePlaybackModeApplied,
  onModeMenuChange: updateTranscodeModeMenuSelection,
  // The offer replaces the player in the template, so release it right away.
  onOfferShown: teardownPlayer,
});
const transcodeMenuAvailable = computed(
  () => transcodeEnabled.value && getters.isLoggedIn() && !getters.isShare(),
);
const transcodeProfileOptions = computed<string[]>(() => {
  const live = state.mediaSettings?.profiles;
  const configured: string[] = Array.isArray(live) ? live : TRANSCODE_HLS_MODES;
  return configured.filter((id) => TRANSCODE_HLS_MODES.includes(id));
});

// --- Transcode offer (shown instead of the player when native playback is not possible) ---
/** True while the pre-mount "does this need transcoding?" check runs, so the player is not flashed first. */
const offerCheckPending = ref(false);
const canDownload = computed(() => getters.sourcePermissions().download);
const downloadUrl = computed(() => {
  if (getters.isShare()) {
    return resourcesApi.getDownloadURLPublic(
      { path: state.shareInfo.subPath, hash: state.shareInfo.hash },
      [props.req.path as string],
    );
  }
  return resourcesApi.getDownloadURL(props.req.source, props.req.path);
});
const transcodeOfferMessage = computed(() => {
  if (!transcodeOffer.value) {
    return "";
  }
  switch (transcodeOffer.value.mode) {
    case "user_limit":
      return t("player.transcodeUserLimitReached");
    case "system_limit":
      return t("player.transcodeSystemLimitReached", { limit: transcodeOffer.value.systemLimit });
    default:
      return t("player.transcodeUnsupportedFormat");
  }
});
const transcodeOfferTitle = computed(() => {
  if (!transcodeOffer.value) {
    return "";
  }
  return transcodeOffer.value.mode === "offer" ? t("player.transcodeUnsupportedFormat") : t("files.noPreview");
});
const showTranscodeOfferDetail = computed(() => transcodeOffer.value?.mode !== "offer");
const showTranscodeSessionList = computed(() => {
  const mode = transcodeOffer.value?.mode;
  if (!mode || mode === "offer") {
    return false;
  }
  return (transcodeOffer.value?.sessions?.length ?? 0) > 0;
});

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
  lifecycle,
  createPlayer,
  setupSharedEvents,
  overridePlay,
  getNativePlay,
  restoreNativePlay,
  onPlayerEvent,
  getActiveMediaElement,
  stopMediaElement,
  rememberFullscreenForTransition,
  resumeFullscreenAfterNavigation,
  destroyPlayer,
} = useMediaPlayer({
  kind: "video",
  player,
  getMediaElement: () => videoElement.value,
  getRequestedFile: () => props.req,
  onPlaybackState: updateMediaSessionPlaybackState,
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

/** Plyr only builds settings panels at mount; applyCustomSettings toggles visibility when media/live settings settle. */
const settingsMenu = computed(() => {
  const base = ["captions", "captionSize", "quality", "speed", "playback", "loop"];
  // Register the slot whenever authenticated video can show it later.
  if (getters.isLoggedIn() && !getters.isShare()) {
    return [...base, "transcodeMode"];
  }
  return base;
});

/** The Plyr options only the video player needs */
const videoPlyrOptions = computed<Plyr.Options>(() => ({
  settings: settingsMenu.value,
  i18n: {
    captionSize: t("player.captionSize"),
    transcodeMode: t("general.transcode"),
  },
  tooltips: { controls: true, seek: !scrubPreviewEnabled.value },
  captions: {
    active: false,
    language: "auto",
    update: true,
  },
}));

/** Plyr captions menu: show format only (e.g. `.srt`, `.ass`), not the video basename. */
function subtitleTrackLabel(sub: SubtitleTrack): string {
  const ext = getSubtitleFormatExtension(sub?.name || "");
  return ext || sub?.name || "";
}

function resetFilePlaybackState(): void {
  resetFileState();
  hasStartedPlayback.value = false;
  resetOverlaidHints();
}

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
    teardownPlayer();
    lifecycle.tornDown = false;
    void initializePlayer();
  },
);
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
    } else if (newSubs !== oldSubs && (props.req?.subtitles?.length ?? 0) > 0) {
      await nextTick();
      void initializePlayer();
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

watch(
  () => transcodeProfileOptions.value.join(","),
  () => {
    transcodeMenuInitialized = false;
    transcodeModeButtons = null;
    transcodeModeValueSpan = null;
    if (player.value) {
      applyCustomSettings(player.value);
    }
  },
);

function onPageHide(event: PageTransitionEvent): void {
  if (event.persisted) return;
  teardownTranscodePlayback({ stopSession: true, keepalive: true });
  stopMediaElement();
}

/**
 * Stops playback and releases the player, listeners, transcode session and album art.
 * The transcode controller stays usable, so the player can be created again (source change, offer accepted).
 */
function teardownPlayer(): void {
  if (lifecycle.tornDown) {
    return;
  }
  lifecycle.tornDown = true;
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
  lifecycle.mountedFile = null;
  lockedNativeVideoSrc.value = null;
  const media = getActiveMediaElement();
  const isTransition = state.navigation.isTransitioning;

  if (media) {
    stopMediaElement(media);
  }
  teardownTranscodePlayback({ stopSession: true });
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

/** Final teardown: also releases the transcode controller */
function destroy(): void {
  teardownPlayer();
  disposeTranscodeController({ stopSession: true });
}

/** Creates the player again after the offer was accepted (the offer had released it) */
async function remountPlayer(): Promise<void> {
  await nextTick(); // the <video> element is rendered again
  resetFilePlaybackState();
  lifecycle.tornDown = false;
  lifecycle.mountedFile = resolveMountedFileKey(props.req?.source, props.req?.path);
  loadMediaMetadata();
  await preparePlayerMount();
}

async function onStartTranscodeClick(): Promise<void> {
  if (await startVideoTranscode()) {
    await remountPlayer();
  }
}

/** Starts playback from the gesture surface or the overlaid play button */
function requestPlay(): void {
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
  if (lifecycle.tornDown) {
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
  if (!videoElement.value || player.value || lifecycle.tornDown) {
    return;
  }
  await nextTick();
  if (!videoElement.value || player.value || lifecycle.tornDown) {
    return;
  }
  void mountPlayer();
}

async function mountPlayer(): Promise<void> {
  if (!videoElement.value || player.value || lifecycle.mountInProgress || lifecycle.tornDown) {
    return;
  }
  lifecycle.mountInProgress = true;
  try {
    ensureTranscodePlaybackController();
    lifecycle.mountedFile = resolveMountedFileKey(props.req?.source, props.req?.path);
    initTranscodeModeFromRoute();
    await stripPlaybackTimeFromUrl();
    if (!videoElement.value || lifecycle.tornDown || player.value) {
      return;
    }
    if (!startRequested.value && !wantsTranscodePlayback.value && shouldProactivelyOfferTranscode.value) {
      await offerTranscodePlayback();
      if (!videoElement.value || lifecycle.tornDown || player.value) {
        return;
      }
    }
    createPlayer(videoPlyrOptions.value);
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
    if (startRequested.value || (wantsTranscodePlayback.value && !shouldProactivelyOfferTranscode.value)) {
      await startTranscodePlayback({
        autoplay: shouldAutoplay.value,
      });
    }
    resumeFullscreenAfterNavigation();
  } finally {
    lifecycle.mountInProgress = false;
  }
}

// --- Transcode ---
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
  if (!player.value) return;
  const syncTranscode = (paused: boolean) =>
    transcodePlayback.value?.updatePlayback({ currentTime: player.value?.currentTime, paused });
  setupSharedEvents({
    play: () => {
      hasStartedPlayback.value = true;
      syncTranscode(false);
    },
    pause: () => syncTranscode(true),
    timeupdate: () => {
      if (transcodePlayback.value?.sessionController) {
        syncTranscode(!player.value?.playing);
      }
    },
  });
  for (const [event, handler] of [
    ["enterfullscreen", onFullscreenEnter],
    ["exitfullscreen", onFullscreenExit],
    ["enterfullscreen", applyCaptionSizeClass],
    ["exitfullscreen", applyCaptionSizeClass],
  ] as const) {
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

/** Plyr-specific: builds the caption size and transcode menus once (innerHTML), then only updates their state */
function applyCustomSettings(plyr: Plyr): void {
  try {
    const settings = getSettingsElements(plyr);
    const captionSizeBtn = settings?.buttons?.captionSize;
    const captionSizePanel = settings?.panels?.captionSize;

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
      // Stable hook for tests - Plyr stamps data-plyr="settings" on every menu item.
      transcodeBtn.setAttribute("data-plyr-menu", "transcodeMode");
      if (!transcodeMenuAvailable.value) {
        transcodeBtn.setAttribute("hidden", "");
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
              const el = getActiveMediaElement();
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
  lifecycle.mountedFile = resolveMountedFileKey(props.req?.source, props.req?.path);
  loadMediaMetadata();
  if (shouldProactivelyOfferTranscode.value) {
    // Decide before the player appears: an unsupported format gets the offer, not a blank player.
    offerCheckPending.value = true;
    try {
      await offerTranscodePlayback();
    } finally {
      offerCheckPending.value = false;
    }
  }
  await preparePlayerMount();
  if (lifecycle.unmounted) return;
  attachKeyboard();
  window.addEventListener("pagehide", onPageHide);
});

onBeforeUnmount(() => {
  lifecycle.unmounted = true;
  clearDismissAnimTimers();
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

<style scoped>
.media-video-container {
  width: 100%;
  height: 100%;
}

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

/* Transcode offer (shown instead of the player; layout comes from PreviewNotice). */
.transcode-offer-message {
  margin: 1rem 0;
  text-align: center;
}

.transcode-session-list {
  list-style: none;
  padding: 0;
  margin: 0 0 1rem;
  text-align: center;
}

.transcode-session-list li {
  margin: 0.25rem 0;
}

.transcode-session-source {
  opacity: 0.7;
}
</style>

<style>
/* this is just a workaround, the own plyr poster was hiding the video poster and showing black
   when it didn't loaded (somehow happens); but summarizing, this a bug of plyr, it also loads the poster image 2 times
   cuz it sets the URL on the html video tags and in their own */
.plyr__poster {
  display: none !important;
}

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

/* Letterboxing and Plyr chrome */
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
