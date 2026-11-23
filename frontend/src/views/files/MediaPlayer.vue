<template>
  <PreviewShell
    :preview-class="{
      'media-background-light': !isDarkMode && committed?.kind === 'audio',
      'media-background-dark': isDarkMode && committed?.kind === 'audio',
    }"
  >
    <div v-if="previewType === 'audio' || previewType === 'video'" class="av-preview-wrap">
      <AudioPlayer v-if="committed?.kind === 'audio'"
        ref="audioRef"
        :file-key="committed.key"
        :req="committed.req"
        :raw="committed.raw"
        :lyrics="committed.lyrics"
        :listing="listing"
        :autoPlayEnabled="autoPlay"
        class="media-background"
        @playback-start="playbackStarted = true"
        @navigate-previous="navigatePrevious"
        @navigate-next="navigateNext"
        @close-preview="exitPreviewFromImageGesture"
      />
      <VideoPlayer v-else-if="committed?.kind === 'video'"
        ref="videoRef"
        :file-key="committed.key"
        :raw="committed.raw"
        :subtitlesList="committed.subtitles"
        :req="committed.req"
        :listing="listing"
        :autoPlayEnabled="autoPlay"
        @playback-start="playbackStarted = true"
        @navigate-previous="navigatePrevious"
        @navigate-next="navigateNext"
        @close-preview="exitPreviewFromImageGesture"
      />
      <div v-if="avMetadataLoading" class="av-preview-loading">
        <LoadingSpinner size="medium" />
      </div>
    </div>
  </PreviewShell>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, shallowRef, useTemplateRef, watch } from "vue";
import { mediaApi, resourcesApi } from "@/api/index.js";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import PreviewShell from "@/components/files/PreviewShell.vue";
import { usePreviewNavigation } from "@/composables/preview/usePreviewNavigation.ts";
import { getters, mutations, state } from "@/store/index.ts";
import type { ReqObject } from "@/store/types.ts";
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import { type LyricLines, parseLyrics } from "@/utils/lyrics.ts";
import { shouldAutoPlayPreview } from "@/utils/previewAutoplay.js";
import { convertToVTT, getSubtitleFormatExtension } from "@/utils/subtitles.js";
import { removeLastDir } from "@/utils/url.js";

interface SubtitleSource {
  name: string;
  src: string;
  srclang?: string;
}

interface CommittedSource {
  key: string;
  kind: "audio" | "video";
  req: ReqObject;
  raw: string;
  subtitles: SubtitleSource[];
  lyrics: LyricLines;
}

function revokeSubtitleUrls(tracks: readonly SubtitleSource[]): void {
  for (const subtitle of tracks) {
    if (subtitle.src.startsWith("blob:")) URL.revokeObjectURL(subtitle.src);
  }
}

const AudioPlayer = createAsyncComponent(() => import("@/components/player/AudioPlayer.vue"));
const VideoPlayer = createAsyncComponent(() => import("@/components/player/VideoPlayer.vue"));

const videoRef = useTemplateRef<{ destroy?: () => void }>("videoRef");
const audioRef = useTemplateRef<{ destroy?: () => void }>("audioRef");

/** What the players show; only replaced once everything for the next file is loaded. */
const committed = shallowRef<CommittedSource | null>(null);
const avMetadataLoading = ref(false);
/** Skip duplicate media-metadata fetch when patchRequestFileMediaMetadata updates `req` for same file */
let mediaEnrichDoneForPath: string | null = null;
let loadOwner = 0;
let loadIdentity: string | null = null;
let inFlightIdentity: string | null = null;
/** enables autoplay for queue navigation even when autoplayMedia pref is off */
const playbackStarted = ref(false);

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
    videoRef.value?.destroy?.();
  },
});

const isDarkMode = computed(() => getters.isDarkMode());
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

function buildRaw(): string {
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
}

function commitSource(next: CommittedSource | null) {
  const previous = committed.value;
  committed.value = next;
  void nextTick().then(() => {
    const kept = new Set(next?.subtitles.map((subtitle) => subtitle.src));
    revokeSubtitleUrls((previous?.subtitles ?? []).filter((subtitle) => !kept.has(subtitle.src)));
  });
}

onBeforeUnmount(() => {
  loadOwner += 1;
  if (committed.value) {
    revokeSubtitleUrls(committed.value.subtitles);
  }
});

watch(
  () => state.req,
  async () => {
    await loadMediaForReq();
  },
  { immediate: true },
);

function mediaIdentity(): string {
  const shareHash = getters.isShare() ? state.shareInfo.hash : "";
  return `${shareHash}|${state.req.source ?? ""}|${state.req.path ?? ""}`;
}

async function attachDirMediaMetadata(
  dirListing: NonNullable<typeof listing.value>,
  dirPath: string,
  isStale: () => boolean,
) {
  if (!dirListing?.length) return;
  try {
    const metaMap = await mediaApi.getDirectoryMetadataMap(dirPath, { source: state.req.source });
    if (isStale() || listing.value !== dirListing) return;
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
  const identity = mediaIdentity();
  if (identity !== loadIdentity) {
    loadIdentity = identity;
    loadOwner += 1;
  } else if (inFlightIdentity === identity) {
    // the running load commits `state.req` as it is at the end, so it sees this update too
    return;
  }
  const owner = loadOwner;
  const isStale = () => owner !== loadOwner;
  inFlightIdentity = identity;
  if (committed.value) {
    avMetadataLoading.value = true;
  }
  try {
    await loadAndCommit(identity, isStale);
  } finally {
    if (!isStale() && inFlightIdentity === identity) {
      inFlightIdentity = null;
    }
    if (!isStale()) {
      avMetadataLoading.value = false;
    }
  }
}

async function loadAndCommit(identity: string, isStale: () => boolean) {
  await loadListingContext();
  if (isStale()) {
    return;
  }

  const isAv =
    !getters.fileViewingDisabled(state.req.name ?? "") &&
    state.req.type !== "directory" &&
    (previewType.value === "audio" || previewType.value === "video");

  if (isAv) {
    avMetadataLoading.value = true;
    if (mediaEnrichDoneForPath !== identity) {
      const applied = await enrichAvFromMediaApi(isStale);
      if (isStale()) {
        return;
      }
      if (applied) {
        mediaEnrichDoneForPath = identity;
      }
    }
  } else {
    avMetadataLoading.value = false;
    mediaEnrichDoneForPath = null;
  }

  await updatePreview();
  if (isStale()) {
    return;
  }
  if (!isAv) {
    commitSource(null);
    selectCurrentItem();
    return;
  }
  if (listing.value) {
    const directoryPath = removeLastDir(state.req.path) || "/";
    await attachDirMediaMetadata(listing.value, directoryPath, isStale);
    if (isStale()) {
      return;
    }
  }

  const kind = previewType.value as "audio" | "video";
  // the same file again (e.g. a late metadata patch) keeps its subtitles and lyrics
  const current = committed.value;
  const reuse = current?.key === identity && current.kind === kind ? current : null;
  const loadedSubtitles = reuse ? reuse.subtitles : await subtitles();
  if (isStale()) {
    if (!reuse) revokeSubtitleUrls(loadedSubtitles);
    return;
  }
  const loadedLyrics = reuse ? reuse.lyrics : kind === "audio" ? await loadLyrics() : [];
  if (isStale()) {
    if (!reuse) revokeSubtitleUrls(loadedSubtitles);
    return;
  }
  commitSource({
    key: identity,
    kind,
    req: state.req,
    raw: buildRaw(),
    subtitles: loadedSubtitles,
    lyrics: loadedLyrics,
  });
  avMetadataLoading.value = false;
  selectCurrentItem();
}

async function loadLyrics(): Promise<LyricLines> {
  if (!state.req.metadata?.hasLyrics) {
    return [];
  }
  try {
    const { lyrics: rawLyrics, format } = await mediaApi.getLyrics(state.req.source as string, state.req.path as string);
    return parseLyrics(rawLyrics, format);
  } catch (err) {
    console.warn("Failed to fetch lyrics:", err);
    return [];
  }
}

/** GET /api/media/metadata?albumArt=true - subtitles, duration, embedded cover for the viewer. */
async function enrichAvFromMediaApi(isStale: () => boolean): Promise<boolean> {
  if (isStale()) {
    return false;
  }
  const currentReq = state.req;
  if (!getters.isShare() && !getters.isLoggedIn()) {
    return false;
  }
  try {
    const enriched = await mediaApi.getItemMediaMetadata(currentReq.path as string, {
      source: currentReq.source,
      albumArt: true,
    });
    if (isStale()) {
      return false;
    }
    if (enriched && enriched.type !== "directory") {
      mutations.patchRequestFileMediaMetadata(
        enriched as Parameters<typeof mutations.patchRequestFileMediaMetadata>[0],
      );
      return true;
    }
  } catch (e) {
    console.warn("Preview: media metadata fetch failed", e);
  }
  return false;
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

</script>

<style src="plyr/dist/plyr.css"></style>

<style>
.plyr,
.plyr__video-wrapper,
.plyr video {
  -webkit-tap-highlight-color: transparent;
}

.plyr__video-wrapper:focus,
.plyr video:focus {
  outline: none;
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
  --plyr-video-controls-background: linear-gradient(
    transparent,
    rgb(0 0 0 / 70%)
  );

  overflow: visible;
  background-color: rgb(216 216 216);
  box-shadow: 0 2px 6px rgb(88 88 88 / 45%);
}

.plyr__controls {
  color: black;
}

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

.plyr .plyr__control {
  transition:
    background 0.2s ease,
    color 0.2s ease,
    box-shadow 0.2s ease;
  flex-shrink: 0;
  display: flex;
  min-width: 2em;
  justify-content: center;
  align-items: center;
}

.plyr .plyr__progress__container {
  flex: 100%;
  margin: 0;
}

@media (width <= 768px) {
  .plyr .plyr__control[data-plyr="rewind"],
  .plyr .plyr__control[data-plyr="fast-forward"] {
    display: none;
  }
}

@media (width <= 550px) {
  .plyr__volume {
    display: none;
  }
}

.av-preview-loading {
  position: absolute;
  inset: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgb(0 0 0 / 35%);
}
</style>

<style scoped>
:deep(.media-background-dark) {
  background: radial-gradient(#3b3b3b, black);
}

:deep(.media-background-light) {
  background: radial-gradient(#262626, #e2e2e2);
}

.av-preview-wrap {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 12rem;
}
</style>
