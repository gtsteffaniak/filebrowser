<template>
  <div class="media-player">
    <div
      ref="gestureRoot"
      class="audio-player-container audio-player-container--gestures"
      :class="{ 'audio-player-container--lyrics-open': mobileLyricsActive }"
    >
      <!-- Desktop panel button, will auto‑hide only when panel is closed -->
      <FloatingActionButton
        v-if="!isMobile"
        :icon="showDesktopPanel ? 'close' : 'queue_music'"
        :badge="showDesktopPanel ? undefined : queueCount"
        :auto-hide="!showDesktopPanel"
        :offset="{ zIndex: '9999' }"
        @press="showDesktopPanel = !showDesktopPanel"
        :label="desktopPanelToggleLabel"
      />
      <!-- Two‑column layout -->
      <div class="audio-player-content" :class="{ 'panel-open': !isMobile && showDesktopPanel }">
        <!-- Left column: art + meta -->
        <div class="audio-left-column">
          <!-- Album art with a generic icon if no image/metadata -->
          <div class="album-art-container"
              :class="{ 'no-artwork': !artworkUrl }"
              :style="{
                width: `${displayArtSize}em`,
                maxWidth: '100%',
                aspectRatio: '1 / 1'
              }"
              @mouseenter="onAlbumArtHover"
              @mouseleave="onAlbumArtLeave"
              @wheel="onAlbumArtScroll">
            <img class="no-select album-art" v-if="artworkUrl" :src="artworkUrl" :alt="metadata?.album || 'Album art'" />
            <div v-else class="album-art-fallback">
              <i class="material-symbols">music_note</i>
            </div>
          </div>
          <!-- Metadata info -->
          <div class="audio-metadata" v-if="metadata">
            <div class="audio-title">
              {{ metadata.title }}
            </div>
            <div class="audio-artist" v-if="metadata.artist">
              {{ formattedArtist }}
            </div>
            <div class="audio-album" v-if="metadata.album">
              {{ metadata.album }}
            </div>
            <div class="audio-year" v-if="metadata.album">
              {{ metadata.year }}
              <span class="filetype-badge">{{ filetype }}</span>
            </div>
          </div>
        </div>
        <!-- Right column: panel with Queue & Lyrics tabs (desktop) -->
        <Transition name="panel-slide">
          <AudioPanel
            v-if="!isMobile && showDesktopPanel"
            :lyrics="lyrics"
            :player="player"
            :audio-context="audioContext"
            :audio-source="audioSource"
            class="lyrics-panel"
            @needs-audio-graph="setupAudioVisualizer"
          />
        </Transition>
      </div>
      <template v-if="isMobile && lyrics.length">
        <FloatingActionButton
          icon="lyrics"
          position="top-left"
          zone-height="8.5em"
          group="media-controls"
          :label="t('player.toggleLyrics')"
          @press="showMobileLyrics = !showMobileLyrics"
        />
        <div v-if="showMobileLyrics" class="lyrics-mobile">
          <LyricsList compact :lyrics="lyrics" :player="player" />
        </div>
      </template>
      <!-- Audio controls -->
      <div class="audio-controls-container">
        <div class="media-audio-container">
          <audio :src="raw" :type="req.type" :autoplay="shouldAutoplay" @play="handlePlay" ref="audioElement"></audio>
        </div>
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
    <FloatingActionButton
      v-if="isMobile"
      icon="queue_music"
      :badge="queueCount"
      group="media-controls"
      @press="showQueuePrompt"
      :label="t('player.QueueButtonHint')"
    />
  </div>
</template>

<script setup lang="ts">
import type Plyr from "plyr";
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, useTemplateRef, watch } from "vue";
import AudioPanel from "@/components/files/AudioPanel.vue";
import LyricsList from "@/components/files/LyricsList.vue";
import FloatingActionButton from "@/components/settings/FloatingActionButton.vue";
import PlayerOverlays from "@/components/player/PlayerOverlays.vue";
import { getSettingsElements } from "@/composables/player/settingsElements.ts";
import {
  readAlbumArtSize,
  readDesktopPanelOpen,
  readMobileLyricsOpen,
  writeAlbumArtSize,
  writeDesktopPanelOpen,
  writeMobileLyricsOpen,
} from "@/composables/player/playerStorage.ts";
import { useAudioVisualizer } from "@/composables/player/useAudioVisualizer.ts";
import { useEmbeddedArtwork } from "@/composables/player/useEmbeddedArtwork.ts";
import { useDoubleTapSeek } from "@/composables/player/useDoubleTapSeek.ts";
import { useGestureState } from "@/composables/player/useGestureState.ts";
import { useMediaKeyboard } from "@/composables/player/useMediaKeyboard.ts";
import {
  type PlayerEmits,
  type PlayerFile,
  resolveMountedFileKey,
  stripPlaybackTimeFromUrl,
  useMediaPlayer,
} from "@/composables/player/useMediaPlayer.ts";
import { buildMediaSessionInfo, useMediaSession } from "@/composables/player/useMediaSession.ts";
import { usePlaybackQueue } from "@/composables/player/usePlaybackQueue.ts";
import { useSwipeNavigationGestures } from "@/composables/player/useSwipeNavigationGestures.ts";
import { useI18n } from "@/i18n/index.ts";
import { enableSeekOnRelease } from "@/player/seekOnRelease.js";
import { getters } from "@/store/index.ts";
import type { FileListItem } from "@/store/types.ts";
import { getTypeFromMime } from "@/utils/files.js";
import type { LyricLines } from "@/utils/lyrics.ts";
import { formatArtist } from "@/utils/playbackQueue.js";

interface AudioMetadata {
  title?: string;
  artist?: string;
  album?: string;
  year?: string | number;
  /** Base64 encoded embedded art. */
  albumArt?: string;
}

interface AudioFile extends PlayerFile {
  metadata?: AudioMetadata;
}

const props = withDefaults(
  defineProps<{
    req: AudioFile;
    raw: string;
    /** Identity of the file; a change swaps the file on the same player. */
    fileKey?: string;
    lyrics?: LyricLines;
    /** Folder listing the playback queue is built from. */
    listing?: FileListItem[];
    autoPlayEnabled?: boolean;
  }>(),
  { lyrics: () => [], listing: () => [], autoPlayEnabled: false },
);

const emit = defineEmits<PlayerEmits>();

const { t } = useI18n();

// The Plyr instance is held in a shallow ref, it's a third-party object and must not be deep-proxied.
const player = shallowRef<Plyr | null>(null);
const audioElement = useTemplateRef<HTMLAudioElement>("audioElement");
const gestureRoot = useTemplateRef<HTMLElement>("gestureRoot");

// --- Metadata and album art ---
// Storage keys (plyrAlbumArtSize, plyrShowDesktopPanel, plyrShowMobileLyrics)
const albumArtSize = ref(readAlbumArtSize());
const isHovering = ref(false);
const showDesktopPanel = ref(readDesktopPanelOpen());
const showMobileLyrics = ref(readMobileLyricsOpen());

const isMobile = computed(() => getters.isMobile());
const filetype = computed(() => getTypeFromMime(props.req.type || ""));
const formattedArtist = computed(() => formatArtist(metadata.value?.artist ?? ""));
const fileName = computed(() => (props.req.name ? props.req.name.replace(/\.[^/.]+$/, "") : ""));
const { metadata, albumArtUrl, loadMediaMetadata, cleanupAlbumArt } = useEmbeddedArtwork({
  getMetadata: () => props.req.metadata,
  getFileName: () => fileName.value,
});
const artworkUrl = computed(() => albumArtUrl.value || undefined);
const desktopPanelToggleLabel = computed(() =>
  showDesktopPanel.value ? t("player.closePanel") : t("player.openPanel"),
);
const mobileLyricsActive = computed(() => isMobile.value && showMobileLyrics.value && props.lyrics.length > 0);
const displayArtSize = computed(() => (mobileLyricsActive.value ? 5 : albumArtSize.value));

function onAlbumArtHover(): void {
  isHovering.value = true;
}

function onAlbumArtLeave(): void {
  isHovering.value = false;
}

function onAlbumArtScroll(event: WheelEvent): void {
  if (!isHovering.value) return;
  event.preventDefault();
  const scrollDelta = event.deltaY > 0 ? -5 : 5; // Scroll down decreases, scroll up increases
  // Apply size constraints (minimum 10em, maximum 50em)
  albumArtSize.value = Math.max(10, Math.min(50, albumArtSize.value + scrollDelta));
}

watch(showDesktopPanel, writeDesktopPanelOpen);
watch(albumArtSize, writeAlbumArtSize);
watch(showMobileLyrics, writeMobileLyricsOpen);

const { audioContext, audioSource, setupAudioVisualizer, resumeAudioGraph, cleanupAudioVisualizer } =
  useAudioVisualizer({ getMediaElement: () => audioElement.value });
const {
  setupMediaSession,
  updateMediaSessionMetadata,
  updateMediaSessionNavHandlers,
  updateMediaSessionPlaybackState,
  clearMediaSession,
} = useMediaSession(player);
const gestures = useGestureState();
const playbackQueueState = usePlaybackQueue({
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
} = playbackQueueState;

const shouldAutoplay = computed(() => props.autoPlayEnabled || playbackQueue.value.length > 1);

let seekOnReleaseCleanup: (() => void) | null = null;

const doubleTap = useDoubleTapSeek({
  player,
  gestures,
  getPreviewType: () => "audio",
  getAudioGestureRoot: () => gestureRoot.value,
  requestPlay: () => {
    void Promise.resolve(player.value?.play()).catch(() => {});
  },
});
const {
  skipFeedbackVisible,
  skipFeedbackSide,
  skipFeedbackIcon,
  skipFeedbackKey,
  flashSkipFeedback,
  setupDoubleTapSeek,
  teardownDoubleTapSeek,
  setupRewindForwardFeedback,
  clearRewindForwardFeedback,
} = doubleTap;
const swipe = useSwipeNavigationGestures({
  player,
  gestures,
  getPreviewType: () => "audio",
  isMobileLyricsActive: () => mobileLyricsActive.value,
  getGestureSurface: doubleTap.getGestureSurface,
  isControlOrMenuTarget: doubleTap.isControlOrMenuTarget,
  onNavigatePrevious: () => emit("navigate-previous"),
  onNavigateNext: () => emit("navigate-next"),
  onClosePreview: () => emit("close-preview"),
});
const { speedToastVisible, speedToastMessage, setupSwipeGestures, teardownSwipeGestures, clearDismissAnimTimers } = swipe;
const {
  lifecycle,
  createPlayer,
  setupSharedEvents,
  getActiveMediaElement,
  stopMediaElement,
  releaseFile,
  setMountedFile,
  reapplyPlaybackRate,
  destroyPlayer,
} = useMediaPlayer({
  kind: "audio",
  player,
  getMediaElement: () => audioElement.value,
  getRequestedFile: () => props.req,
  onPlaybackState: updateMediaSessionPlaybackState,
  destroy,
  restoreSource: restoreFileSource,
});
const { attachKeyboard, detachKeyboard } = useMediaKeyboard({
  player,
  getPreviewType: () => "audio",
  cyclePlaybackModes,
  toggleSingleLoop,
  showQueuePrompt,
  flashSkipFeedback,
  toggleDesktopPanel: () => {
    showDesktopPanel.value = !showDesktopPanel.value;
  },
});

/** Plyr-specific: builds the hand-made playback and loop menus (innerHTML); rewritten with the Video.js 10 migration. */
function applyPlayerSettings(plyr: Plyr): void {
  try {
    applyPlaybackMenu(plyr);
    applyLoopMenu(plyr);
  } catch (error) {
    console.error("Error applying custom settings:", error);
  }
}

/** Rebuilds the custom menus when Plyr re-rendered them or a mode changed while a menu is open. */
function ensurePlaybackModeApplied(): void {
  if (!player.value) return;
  try {
    const settings = getSettingsElements(player.value);
    const settingsMenu = settings?.menu;
    const menuOpen =
      settingsMenu && settingsMenu.style.display !== "none" && settingsMenu.getAttribute("hidden") === null;
    const needPlayback = settings?.buttons?.playback && !playbackMenuInitialized.value;
    const needLoop = settings?.buttons?.loop && !loopMenuInitialized.value;
    if (menuOpen || needPlayback || needLoop) {
      applyPlayerSettings(player.value);
    }
  } catch (error) {
    console.error("Error ensuring playback mode applied:", error);
  }
}

// --- Mount / teardown ---
function setupPlayerEvents(): void {
  if (!player.value) return;
  setupSharedEvents();
  ensurePlaybackModeApplied();
  setupDoubleTapSeek();
  setupSwipeGestures();
  setupRewindForwardFeedback();
}

const mediaSessionInfo = () => buildMediaSessionInfo(metadata.value, fileName.value, artworkUrl.value);

let swapGeneration = 0;

function restoreFileSource(): void {
  const media = audioElement.value;
  if (lifecycle.tornDown || !media || !props.raw || media.getAttribute("src") === props.raw) {
    return;
  }
  media.setAttribute("src", props.raw);
  media.load();
  reapplyPlaybackRate();
}

/** A new file on the same player: only the per-file data changes (pre-flush, before the new `:src` renders). */
async function onFileSwap(newKey?: string, oldKey?: string): Promise<void> {
  if (!newKey || newKey === oldKey || lifecycle.tornDown) return;
  const generation = ++swapGeneration;
  releaseFile();
  setMountedFile(props.req?.source, props.req?.path);
  loadMediaMetadata();
  if (!player.value) {
    void initializePlayer();
    return;
  }
  updateMediaSessionMetadata(mediaSessionInfo());
  await nextTick(); // the template sets the new src
  if (generation !== swapGeneration || lifecycle.tornDown || !player.value) return;
  reapplyPlaybackRate();
}

watch(() => props.fileKey, onFileSwap, { flush: "pre" });

async function mountPlayer(): Promise<void> {
  if (!audioElement.value || player.value || lifecycle.mountInProgress || lifecycle.tornDown) return;
  lifecycle.mountInProgress = true;
  try {
    lifecycle.mountedFile = resolveMountedFileKey(props.req?.source, props.req?.path);
    await stripPlaybackTimeFromUrl();
    if (!audioElement.value || lifecycle.tornDown || player.value) return;
    createPlayer();
    if (!player.value) return;
    setupMediaSession(mediaSessionInfo());
    setupPlayerEvents();
    seekOnReleaseCleanup = enableSeekOnRelease(player.value);
  } finally {
    lifecycle.mountInProgress = false;
  }
}

async function initializePlayer(): Promise<void> {
  if (!audioElement.value || player.value || lifecycle.tornDown) return;
  await nextTick();
  if (!audioElement.value || player.value || lifecycle.tornDown) return;
  void mountPlayer();
}

function handlePlay(): void {
  if (lifecycle.tornDown) return;
  emit("playback-start");
  resumeAudioGraph();
}

/** Stops playback and releases the player, listeners and album art */
function destroy(): void {
  if (lifecycle.tornDown) return;
  lifecycle.tornDown = true;
  cleanupAlbumArt();
  if (seekOnReleaseCleanup) {
    seekOnReleaseCleanup();
    seekOnReleaseCleanup = null;
  }
  lifecycle.mountedFile = null;
  const media = getActiveMediaElement();

  if (media) {
    stopMediaElement(media);
  }
  if (player.value) {
    teardownSwipeGestures();
    teardownDoubleTapSeek();
    clearRewindForwardFeedback();
    cleanupAudioVisualizer();
    clearMediaSession();
  }
  destroyPlayer(media);
  resetPlaybackMenuState();
}

function onPageHide(event: PageTransitionEvent): void {
  if (event.persisted) return;
  cleanupAudioVisualizer({ close: true });
  stopMediaElement();
}

onMounted(async () => {
  lifecycle.mountedFile = resolveMountedFileKey(props.req?.source, props.req?.path);
  loadMediaMetadata();
  await initializePlayer();
  if (lifecycle.unmounted || lifecycle.tornDown) return;
  attachKeyboard();
  window.addEventListener("pagehide", onPageHide);
});

onBeforeUnmount(() => {
  lifecycle.unmounted = true;
  clearDismissAnimTimers();
  try {
    destroy();
  } finally {
    cleanupAudioVisualizer({ close: true });
    clearMediaSession();
    detachKeyboard();
    window.removeEventListener("pagehide", onPageHide);
  }
});

defineExpose({ destroy });
</script>

<style scoped>
.audio-player-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1em;
  margin: 0 auto;
  padding: 1em;
  width: 100%;
  box-sizing: border-box;
  height: 100%;
  justify-content: center;
}

.lyrics-mobile {
  position: relative;
  display: flex;
  flex-direction: column;
  flex: 1 1 0%;
  min-height: 0;
}

/* When mobile lyrics are open: push art+meta to top */
.audio-player-container--lyrics-open {
  justify-content: flex-start;
  gap: 0;
}

.audio-player-container--gestures {
  position: relative;
  touch-action: manipulation;
}

.audio-player-content {
  width: 100%;
  max-width: 1500px;
  margin: 0 auto;
  padding: 0 3.5em;
  padding-bottom: 0;
  box-sizing: border-box;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  overflow: hidden;
  position: relative;
}

.audio-player-container--lyrics-open .audio-player-content {
  height: auto;
  flex: none;
}

/* Left column (album art + metadata) */
.audio-left-column {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  width: 100%;
  padding: 0 2em;
  box-sizing: border-box;
  transition: width 0.4s cubic-bezier(0.25, 0.8, 0.25, 1);
  min-width: 0;
  flex-shrink: 0;
}

.panel-open .audio-left-column {
  width: 50%;
}

/* Right panel (lyrics / queue) */
.lyrics-panel {
  width: 50%;
  flex-shrink: 0;
  height: 100%;
  overflow-y: auto;
  padding: 0.5em 2em;
  color: var(--textPrimary);
  scroll-behavior: smooth;
  text-align: center;
  background: transparent;
  border-radius: var(--borderRadius);
  box-sizing: border-box;
  scrollbar-width: none;
}

.panel-slide-enter-active {
  transition: opacity 0.4s cubic-bezier(0.25, 0.8, 0.25, 1);
}

.panel-slide-leave-active {
  transition: none;
}

.panel-slide-enter-from,
.panel-slide-leave-to {
  opacity: 0;
}

.album-art-container {
  flex-shrink: 0;
  border-radius: 1em;
  overflow: hidden;
  box-shadow: 0 6px 20px rgb(0 0 0 / 20%);
  transition: width 0.3s ease;
  will-change: transform;
}

.audio-player-container--lyrics-open .album-art-container {
  width: 5em;
  height: 5em;
}

.album-art {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.album-art-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  border-radius: 18px;
  background: linear-gradient(115deg, var(--primaryColor), rgb(2 0 36 / 90%));
  filter: brightness(0.85);
}

.album-art-fallback i.material-symbols {
  font-size: 5rem;
  color: white;
  opacity: 0.8;
  user-select: none;
}

.album-art-container.no-artwork {
  box-shadow: 0 4px 12px rgb(0 0 0 / 15%);
  height: auto;
  aspect-ratio: 1 / 1;
}

/* Metadata */
.audio-title {
  font-size: max(1.4rem, 3.1vmin);
  font-weight: 700;
  margin-bottom: 8px;
  overflow-wrap: break-word;
}

.audio-metadata {
   text-align: center;
   color: whitesmoke;
   box-sizing: border-box;
   padding-bottom: 0;
   margin-bottom: 0;
   padding-top: 1.2em;
   overflow-wrap: break-word;
}

.audio-artist,
.audio-album,
.audio-year {
  font-size: max(1.2rem, 2.5vmin);
  opacity: 0.8;
  margin-bottom: 5px;
  overflow-wrap: break-word;
}

.filetype-badge {
  display: inline-block;
  background: var(--primaryColor);
  color: white;
  padding: 2px 8px;
  border-radius: var(--borderRadius);
  font-size: 0.8em;
  margin-left: 0.5em;
  vertical-align: middle;
}

.audio-controls-container {
  width: 100%;
  border-radius: var(--borderRadius);
  margin: -2px;
}

/* For small tablets and phones with big screen */
@media (width <= 740px) {
  .audio-player-container {
    padding: 0;
    padding-top: 1em;
  }
  .album-art-container {
    margin-top: 1em;
    max-width: min(71vw);
  }
  .audio-player-container--lyrics-open .album-art-container {
    transition: none !important;
  }
}

@media (width <= 550px) {
  .audio-left-column {
    padding: 0;
    margin: 0;
  }
}

/* For small screens in landscape orientation (Like a phone) */
@media (height <= 600px) and (orientation: landscape) {
  .album-art-container {
    width: min(100px, 30vh);
    height: min(100px, 30vh);
    margin: 0;
    flex-shrink: 0;
  }
}
</style>

<style>
.plyr.plyr--audio {
  border-radius: var(--borderRadius);
  backdrop-filter: var(--panel-blur);
  background-color: var(--alt-background);
}

.audio-controls-container .plyr .plyr__controls {
  color: var(--textPrimary);
}

@media (width <= 768px) {
  .plyr--audio .plyr__control {
    min-width: 44px;
    min-height: 44px;
  }
  .plyr--audio .plyr__progress__container {
    margin: 10px 0;
  }
  .plyr--audio .plyr__controls__items {
    justify-content: center;
    gap: 12px;
  }
  .plyr--audio .plyr__control--play {
    transform: scale(1.25);
  }
}

/* For small tablets and phones with big screen */
@media (width <= 740px) {
  .plyr.plyr--audio {
    padding: 1em;
    border-radius: 0;
  }
  .plyr--audio .plyr__controls {
    padding: 0;
    gap: 5px;
  }
}

@media (width <= 550px) {
  /* Time playing */
  .plyr--audio .plyr__time {
    font-size: 14px;
    margin: 0 5px;
  }
}
</style>
