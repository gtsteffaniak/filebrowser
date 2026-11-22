<template>
  <div id="previewer">
    <div v-if="isTransitioning" class="transition-loading">
      <LoadingSpinner size="medium" />
    </div>
    <div class="preview" :class="{
      'media-background-light': !isDarkMode && previewType === 'audio',
      'media-background-dark': isDarkMode && previewType === 'audio',
      'transitioning': isTransitioning
    }">
      <div v-if="previewType === 'audio' || (previewType === 'video' && showVideoPlayer)" class="av-preview-wrap">
        <div v-if="avMetadataLoading" class="av-preview-loading">
          <LoadingSpinner size="medium" />
        </div>
        <AudioPlayer v-else-if="previewType === 'audio'"
          :key="viewerKey"
          ref="audioRef"
          :req="req"
          :raw="raw"
          :lyrics="lyrics"
          :listing="listing"
          :autoPlayEnabled="autoPlay"
          class="media-background"
          @playback-start="playbackStarted = true"
          @navigate-previous="navigatePrevious"
          @navigate-next="navigateNext"
          @close-preview="exitPreviewFromImageGesture"
        />
        <VideoPlayer v-else
          :key="viewerKey"
          ref="viewerRef"
          :previewType="previewType"
          :raw="raw"
          :subtitlesList="subtitlesList"
          :req="req"
          :listing="listing"
          :autoPlayEnabled="autoPlay"
          :startTranscode="transcodePlaybackActive"
          :sharedTranscodePlayback="transcodePlaybackController"
          @playback-start="playbackStarted = true"
          @needs-transcode="handleVideoTranscodeOffer"
          @navigate-previous="navigatePrevious"
          @navigate-next="navigateNext"
          @close-preview="exitPreviewFromImageGesture"
        />
      </div>
      <div v-else-if="previewType === 'video' && videoTranscodeOffer" class="info">
        <div class="title">
          <i class="material-symbols">feedback</i>
          {{ transcodeOfferTitle }}
        </div>
        <p v-if="showTranscodeOfferDetail" class="transcode-offer-message">{{ transcodeOfferMessage }}</p>
        <ul v-if="showTranscodeSessionList" class="transcode-session-list">
          <li v-for="sess in videoTranscodeOffer.sessions" :key="sess.id">
            {{ sess.fileName || sess.path }}
            <span v-if="sess.source" class="transcode-session-source">{{ `(${sess.source})` }}</span>
          </li>
        </ul>
        <div class="preview-buttons">
          <button
            v-if="videoTranscodeOffer.mode === 'offer'"
            type="button"
            class="button button--flat preview-action--icon-only"
            :aria-label="t('general.transcode', { suffix: '' })"
            :title="t('general.transcode', { suffix: '' })"
            @click="startVideoTranscode"
          >
            <i class="material-symbols">sync</i>
          </button>
          <a
            v-if="permissions.download"
            target="_blank"
            :href="downloadUrl"
            class="button button--flat preview-action--icon-only"
            :aria-label="t('general.download', { suffix: '' })"
          >
            <i class="material-symbols">file_download</i>
          </a>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, useTemplateRef, watch } from "vue";
import { mediaApi, resourcesApi } from "@/api/index.js";
import { getTranscodeSessions } from "@/api/transcode.js";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { usePreviewNavigation } from "@/composables/preview/usePreviewNavigation.ts";
import { useI18n } from "@/i18n/index.ts";
import { isTranscodeAvailable } from "@/player/transcodeMode.js";
import { TranscodePlaybackController } from "@/player/transcodePlaybackController.js";
import { router } from "@/router/index.ts";
import { getters, mutations, state } from "@/store/index.ts";
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import { needsTranscodeFirst } from "@/utils/canBrowserPlayNative.js";
import { parseLyrics } from "@/utils/lyrics.ts";
import { shouldAutoPlayPreview } from "@/utils/previewAutoplay.js";
import {
  buildPlaybackQueryPatch,
  parsePlaybackTimeFromQuery,
  parseTranscodeModeFromQuery,
  playbackQueryChanged,
} from "@/utils/playbackQuery.js";
import { convertToVTT, getSubtitleFormatExtension } from "@/utils/subtitles.js";
import { removeLastDir } from "@/utils/url.js";

const AudioPlayer = createAsyncComponent(() => import("@/components/player/AudioPlayer.vue"));
const VideoPlayer = createAsyncComponent(() => import("@/components/player/VideoPlayer.vue"));

interface TranscodeSession {
  id?: string;
  source?: string;
  path?: string;
  fileName?: string;
}

interface TranscodeStatus {
  enabled?: boolean;
  canStart?: boolean;
  blockReason?: string;
  sessions?: TranscodeSession[];
  globalLimit?: number;
  userLimit?: number;
}

interface VideoTranscodeOffer {
  mode: string;
  sessions: TranscodeSession[];
  systemLimit?: number;
  userLimit?: number;
}

interface SubtitleSource {
  name: string;
  src: string;
  srclang?: string;
}

const { t } = useI18n();

const viewerRef = useTemplateRef<{ destroyPlyr?: () => void }>("viewerRef");
const audioRef = useTemplateRef<{ destroy?: () => void }>("audioRef");

const subtitlesList = ref<SubtitleSource[]>([]);
const lyrics = ref<ReturnType<typeof parseLyrics>>([]);
let lyricsFetchedForPath: string | null = null;
const avMetadataLoading = ref(false);
/** Skip duplicate media-metadata fetch when patchRequestFileMediaMetadata updates `req` for same path */
let mediaEnrichDoneForPath: string | null = null;
/** enables autoplay for queue navigation even when autoplayMedia pref is off */
const playbackStarted = ref(false);
const videoTranscodeOffer = ref<VideoTranscodeOffer | null>(null);
const transcodePlaybackActive = ref(false);
const transcodePlaybackController = ref<TranscodePlaybackController | null>(null);
let lastVideoPreviewPath: string | null = null;

const {
  listing,
  loadListingContext,
  updatePreview,
  selectCurrentItem,
  navigatePrevious,
  navigateNext,
  exitPreviewFromImageGesture,
} = usePreviewNavigation({
  onBeforeTeardown: () => {
    audioRef.value?.destroy?.();
    viewerRef.value?.destroyPlyr?.();
    disposeTranscodePlaybackController();
  },
});

const permissions = computed(() => getters.sourcePermissions());
const req = computed(() => state.req);
const isDarkMode = computed(() => getters.isDarkMode());
const isTransitioning = computed(() => state.navigation.isTransitioning);
const previewType = computed(() => {
  if (getters.fileViewingDisabled(state.req.name ?? "")) {
    return "preview";
  }
  return getters.previewType();
});
const autoPlay = computed(() =>
  shouldAutoPlayPreview(
    getters.previewPerms().autoplayMedia,
    playbackStarted.value,
    getters.isPreviewPlaybackQueueNavMode(),
  ),
);
const raw = computed(() => {
  const viewToken = state.req.viewToken;
  const typeHint = state.req.type || state.req.name;
  if (getters.isShare()) {
    return resourcesApi.getViewURL(
      state.req.source,
      state.req.path,
      viewToken,
      {
        path: state.shareInfo.subPath,
        hash: state.shareInfo.hash,
      },
      false,
      typeHint,
    );
  }
  return resourcesApi.getViewURL(state.req.source, state.req.path, viewToken, null, false, typeHint);
});
const downloadUrl = computed(() => {
  if (getters.isShare()) {
    return resourcesApi.getDownloadURLPublic(
      {
        path: state.shareInfo.subPath,
        hash: state.shareInfo.hash,
      },
      [state.req.path as string],
    );
  }
  return resourcesApi.getDownloadURL(state.req.source, state.req.path);
});
const showVideoPlayer = computed(() => !videoTranscodeOffer.value);
const viewerKey = computed(
  () => `${state.req.path}-${transcodePlaybackActive.value ? "transcode" : "native"}`,
);
const transcodeOfferMessage = computed(() => {
  if (!videoTranscodeOffer.value) {
    return "";
  }
  switch (videoTranscodeOffer.value.mode) {
    case "user_limit":
      return t("player.transcodeUserLimitReached");
    case "system_limit":
      return t("player.transcodeSystemLimitReached", {
        limit: videoTranscodeOffer.value.systemLimit,
      });
    default:
      return t("player.transcodeUnsupportedFormat");
  }
});
const transcodeOfferTitle = computed(() => {
  if (!videoTranscodeOffer.value) {
    return "";
  }
  if (videoTranscodeOffer.value.mode === "offer") {
    return t("player.transcodeUnsupportedFormat");
  }
  return t("files.noPreview");
});
const showTranscodeOfferDetail = computed(() => videoTranscodeOffer.value?.mode !== "offer");
const showTranscodeSessionList = computed(() => {
  const mode = videoTranscodeOffer.value?.mode;
  if (!mode || mode === "offer") {
    return false;
  }
  return (videoTranscodeOffer.value?.sessions?.length ?? 0) > 0;
});

watch(
  () => state.req,
  async () => {
    await loadMediaForReq();
  },
  { immediate: true },
);

async function attachDirMediaMetadata(dirListing: NonNullable<typeof listing.value>, dirPath: string) {
  if (!dirListing?.length) return;
  try {
    const metaMap = await mediaApi.getDirectoryMetadataMap(dirPath, { source: state.req.source });
    if (listing.value !== dirListing) return;
    mutations.patchListingMetadata(dirListing, metaMap, state.req.path);
    mutations.setupNavigation({
      listing: dirListing,
      currentItem: state.req,
      directoryPath: dirPath,
    });
  } catch (e) {
    console.warn("dir items metadata fetch failed", e);
  }
}

async function loadMediaForReq() {
  if (!getters.isLoggedIn() && !getters.isShare()) {
    return;
  }
  if (!getters.isPreviewPlaybackQueueNavMode()) {
    playbackStarted.value = false;
  }
  const path = state.req.path;
  if (previewType.value === "video" && lastVideoPreviewPath !== path) {
    disposeTranscodePlaybackController();
    videoTranscodeOffer.value = null;
    transcodePlaybackActive.value = false;
    lastVideoPreviewPath = path ?? null;
  }
  await loadListingContext();

  const isAv =
    !getters.fileViewingDisabled(state.req.name ?? "") &&
    state.req.type !== "directory" &&
    (previewType.value === "audio" || previewType.value === "video");

  if (!isAv) {
    avMetadataLoading.value = false;
    mediaEnrichDoneForPath = null;
    lyricsFetchedForPath = null;
  } else {
    if (mediaEnrichDoneForPath !== path) {
      mediaEnrichDoneForPath = path ?? null;
      avMetadataLoading.value = true;
      try {
        await enrichAvFromMediaApi(path);
        if (state.req.path !== path) {
          return;
        }
        await checkProactiveVideoTranscodeOffer(path);
      } finally {
        if (state.req.path === path) {
          avMetadataLoading.value = false;
        }
      }
    } else {
      avMetadataLoading.value = false;
    }
  }

  if (state.req.path !== path) {
    return;
  }
  await updatePreview();
  if (isAv && listing.value) {
    const directoryPath = removeLastDir(state.req.path) || "/";
    await attachDirMediaMetadata(listing.value, directoryPath);
  }
  subtitlesList.value = await subtitles();
  if (previewType.value === "audio" && lyricsFetchedForPath !== state.req.path) {
    lyricsFetchedForPath = state.req.path ?? null;
    if (state.req.metadata?.hasLyrics) {
      try {
        const { lyrics: rawLyrics, format } = await mediaApi.getLyrics(
          state.req.source as string,
          state.req.path as string,
        );
        lyrics.value = parseLyrics(rawLyrics, format);
      } catch (err) {
        console.warn("Failed to fetch lyrics:", err);
        lyrics.value = [];
      }
    } else {
      lyrics.value = [];
    }
  }
  selectCurrentItem();
}

/** GET /api/media/metadata?albumArt=true - subtitles, duration, embedded cover for the viewer. */
async function enrichAvFromMediaApi(expectedPath: string | undefined) {
  if (state.req.path !== expectedPath) {
    return;
  }
  const currentReq = state.req;
  if (!getters.isShare() && !getters.isLoggedIn()) {
    return;
  }
  try {
    const enriched = await mediaApi.getItemMediaMetadata(currentReq.path as string, {
      source: currentReq.source,
      albumArt: true,
    });
    if (state.req.path !== expectedPath) {
      return;
    }
    if (enriched && enriched.type !== "directory") {
      mutations.patchRequestFileMediaMetadata(
        enriched as Parameters<typeof mutations.patchRequestFileMediaMetadata>[0],
      );
    }
  } catch (e) {
    console.warn("Preview: media metadata fetch failed", e);
  }
}

async function subtitles(): Promise<SubtitleSource[]> {
  if (!state.req.subtitles?.length) {
    return [];
  }
  const subs: SubtitleSource[] = [];
  // Fetch subtitle content for each track using the media API
  for (let index = 0; index < state.req.subtitles.length; index++) {
    const subtitleTrack = state.req.subtitles.at(index);
    if (!subtitleTrack) continue;
    try {
      // Fetch subtitle content from API using name and embedded
      const content = await mediaApi.getSubtitleContent(
        state.req.source as string,
        state.req.path as string,
        subtitleTrack.name,
        subtitleTrack.embedded,
      );
      if (!content || content.length === 0) {
        console.warn("Subtitle track has no content:", subtitleTrack.name);
        continue;
      }
      // Convert to VTT if needed
      let vttContent = content;
      if (!content.startsWith("WEBVTT")) {
        const ext = getSubtitleFormatExtension(subtitleTrack.name);
        vttContent = convertToVTT(ext, content);
      }
      if (vttContent.startsWith("WEBVTT")) {
        // Create a virtual file (Blob) and get a URL for it
        const blob = new Blob([vttContent], { type: "text/vtt" });
        const vttURL = URL.createObjectURL(blob);

        subs.push({
          name: subtitleTrack.name,
          src: vttURL,
          srclang: subtitleTrack.srclang || subtitleTrack.language,
        });
      } else {
        console.warn(
          "Skipping subtitle track - no WEBVTT header after conversion:",
          subtitleTrack.name,
        );
      }
    } catch (err) {
      console.error("Failed to load subtitle:", subtitleTrack.name, err);
    }
  }
  return subs;
}

function clearTranscodeFromUrl() {
  const route = router.currentRoute.value;
  const hasTranscode = parseTranscodeModeFromQuery(route?.query);
  const hasTime = parsePlaybackTimeFromQuery(route?.query) !== null;
  if (!hasTranscode && !hasTime) {
    return;
  }
  const nextQuery = buildPlaybackQueryPatch(route.query as Record<string, string | undefined>, {
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

/** Offer transcode UI before mounting the viewer when metadata indicates native playback will fail */
async function checkProactiveVideoTranscodeOffer(expectedPath: string | undefined) {
  if (state.req.path !== expectedPath) {
    return;
  }
  if (previewType.value !== "video" || getters.isShare()) {
    return;
  }
  if (
    transcodePlaybackActive.value ||
    parseTranscodeModeFromQuery(router.currentRoute.value?.query)
  ) {
    return;
  }
  const meta = state.req.metadata || {};
  if (
    !needsTranscodeFirst({
      videoCodec: meta.videoCodec,
      audioCodec: meta.audioCodec,
      mimeType: state.req.type,
      fileName: state.req.name,
    })
  ) {
    return;
  }
  if (!isTranscodeAvailable()) {
    // Transcoding is off on this server: show the unsupported notice without
    // asking for sessions, which would answer 503.
    handleVideoTranscodeOffer({ enabled: false, canStart: false, sessions: [] });
    return;
  }
  try {
    const status = await getTranscodeSessions();
    if (state.req.path !== expectedPath) {
      return;
    }
    handleVideoTranscodeOffer(normalizeTranscodeSessionStatus(status));
  } catch (err) {
    console.warn("Failed to check transcode for unsupported format:", err);
    handleVideoTranscodeOffer({ enabled: false, canStart: false, sessions: [] });
  }
}

function disposeTranscodePlaybackController({ stopSession = true } = {}) {
  if (!transcodePlaybackController.value) {
    return;
  }
  transcodePlaybackController.value.dispose({ stopSession });
  transcodePlaybackController.value = null;
}

/** The viewer failed native playback or detected an unsupported container */
function handleVideoTranscodeOffer(status: TranscodeStatus) {
  if (previewType.value !== "video" || getters.isShare()) {
    return;
  }
  disposeTranscodePlaybackController({ stopSession: true });
  status = normalizeTranscodeSessionStatus(status);
  let mode = "offer";
  if (status.enabled === false) {
    mode = "unsupported";
  } else if (!status.canStart) {
    if (status.blockReason === "user_limit") {
      mode = "user_limit";
    } else if (status.blockReason === "global_limit") {
      mode = "system_limit";
    }
  }
  transcodePlaybackActive.value = false;
  videoTranscodeOffer.value = {
    mode,
    sessions: status.sessions || [],
    systemLimit: status.globalLimit,
    userLimit: status.userLimit,
  };
  if (mode === "offer") {
    clearTranscodeFromUrl();
  }
}

function normalizeTranscodeSessionStatus(status: TranscodeStatus): TranscodeStatus {
  if (!status || status.canStart || status.blockReason !== "user_limit") {
    return status;
  }
  const sameFileActive = (status.sessions || []).some(
    (session) => session.source === state.req.source && session.path === state.req.path,
  );
  if (!sameFileActive) {
    return status;
  }
  return { ...status, canStart: true, blockReason: "" };
}

async function startVideoTranscode() {
  if (transcodePlaybackActive.value) {
    videoTranscodeOffer.value = null;
    return;
  }
  try {
    let status = await getTranscodeSessions();
    status = normalizeTranscodeSessionStatus(status);
    if (!status.canStart) {
      handleVideoTranscodeOffer(status);
      return;
    }
    if (!transcodePlaybackController.value || transcodePlaybackController.value.disposed) {
      transcodePlaybackController.value = new TranscodePlaybackController();
    }
    playbackStarted.value = true;
    videoTranscodeOffer.value = null;
    transcodePlaybackActive.value = true;
  } catch (err) {
    console.error("Failed to verify transcode availability:", err);
  }
}
</script>

<!-- Plyr and playback styles shared by the audio and video players (global, as Plyr renders its own DOM). -->
<style>
@import url("plyr/dist/plyr.css");

/* Remove blue overlay when tapping on mobile */
.plyr,
.plyr__video-wrapper,
.plyr video {
  -webkit-tap-highlight-color: transparent;
}

.plyr__video-wrapper:focus,
.plyr video:focus {
  outline: none;
}

/* Background styles for the audio player */
.media-background-dark {
  background: radial-gradient(#3b3b3b, black);
}

.media-background-light {
  background: radial-gradient(#262626, #e2e2e2);
}

.media-player {
  height: 100%;
  width: 100%;
}

.plyr {
  --plyr-color-main: var(--primaryColor);
  --plyr-video-background: rgb(0 0 0 / 100%);
  --plyr-focus-visible-color: var(--primaryColor);
  --plyr-audio-control-color: #fff;
  --plyr-menu-background: rgb(0 0 0 / 70%);
  --plyr-menu-color: #fff;
  --plyr-menu-border-shadow-color: rgb(0 0 0 / 50%);
  --plyr-menu-radius: 12px;
  --plyr-menu-shadow: 0 1px 2px rgb(0 0 0 / 50%);
  --plyr-control-radius: 12px;
  --plyr-control-icon-size: 16px;
  --plyr-control-spacing: 8px;
  --plyr-control-padding: 6px;
  --plyr-tooltip-background: rgb(0 0 0 / 80%);
  --plyr-tooltip-color: #fff;
  --plyr-video-controls-background: linear-gradient(transparent,
          rgb(0 0 0 / 70%));

  overflow: visible;
  background-color: rgb(216 216 216);
  box-shadow: 0 2px 6px rgb(88 88 88 / 45%);
}

.plyr__controls {
  color: black;
}

/* Position/space of the buttons */
.plyr .plyr__controls {
  display: flex;
  flex-direction: row;
  gap: 8px;
  background-color: transparent;
}

.plyr .plyr__controls__items {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: nowrap;
}

/* Transitions (e.g. how much time take to hide the player UI) */
.plyr .plyr__control {
  transition: background 0.2s ease, color 0.2s ease, box-shadow 0.2s ease;
  flex-shrink: 0;
  display: flex;
  min-width: 2em;
  justify-content: center;
  align-items: center;
}

/* Progress bar with full width (audio and video) */
.plyr .plyr__progress__container {
  flex: 100%;
  margin: 0;
}

/* Hide volume buttons on narrow screens to make more space */
@media (width <= 550px) {
  .plyr__volume {
    display: none;
  }
}

/* Double-tap / double-click seek feedback (left/right third of video) */
.video-skip-feedback-layer {
  position: absolute;
  inset: 0;
  z-index: 8;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.video-skip-feedback-layer--left {
  justify-content: flex-start;
  padding-left: min(22%, 7rem);
}

.video-skip-feedback-layer--right {
  justify-content: flex-end;
  padding-right: min(22%, 7rem);
}

/*
 * Global fonts.css sets `.material-symbols { font-size: 24px }`.
 * Use high specificity and flex-shrink: 0 so the flex parent cannot squeeze the glyph.
 */
.video-skip-feedback-layer i.material-symbols.video-skip-feedback-layer__icon {
  flex-shrink: 0;
  font-size: clamp(2.5rem, 7vmin, 6rem);
  line-height: 1;
  color: rgb(255 255 255 / 96%);
  filter: drop-shadow(0 2px 16px rgb(0 0 0 / 85%));
  opacity: 0;
  transform: scale(0.55);
  font-variation-settings: 'FILL' 1, 'wght' 500, 'GRAD' 0, 'opsz' 40;
}

.video-skip-feedback-layer--visible i.material-symbols.video-skip-feedback-layer__icon {
  animation: video-skip-feedback-pop 0.7s cubic-bezier(0.34, 1.56, 0.64, 1) forwards;
}

@keyframes video-skip-feedback-pop {
  0% {
    opacity: 0;
    transform: scale(0.55);
  }
  28% {
    opacity: 1;
    transform: scale(1.12);
  }
  100% {
    opacity: 0;
    transform: scale(1);
  }
}

/*********************
*** PLAYBACK TOAST ***
**********************/

.playback-toast {
  position: fixed;
  bottom: 50px;
  left: 50%;
  transform: translateX(-50%);
  background: rgb(0 0 0 / 80%);
  color: white;
  padding: 15px 25px;
  border-radius: 8px;
  font-size: 1.1rem;
  display: flex;
  align-items: center;
  gap: 10px;
  z-index: 10000;
  pointer-events: none;
  user-select: none;
  opacity: 0;
  transition: opacity 0.3s ease;
  box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
}

.playback-toast.visible {
  opacity: 1;
}

.playback-toast .material-symbols {
  font-size: 24px;
  color: white;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.status-indicator {
  display: inline-block;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  margin-left: 10px;
}

.status-on {
  background: #4caf50;
}

.status-off {
  background: #f44336;
}
</style>

<style scoped>
.transition-loading {
  position: fixed;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: var(--background);
  z-index: 10000;
  transition: opacity 0.1s ease;
}

.preview.transitioning {
  opacity: 0.3;
  pointer-events: none;
}

.preview-buttons {
  display: flex;
  flex-direction: row;
  gap: 1em;
}

.av-preview-wrap {
  width: 100%;
  height: 100%;
  min-height: 12rem;
}

.av-preview-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 12rem;
}

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
