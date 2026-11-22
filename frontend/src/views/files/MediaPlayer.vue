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
      <div v-if="previewType === 'audio' || previewType === 'video'" class="av-preview-wrap">
        <div v-if="avMetadataLoading" class="av-preview-loading">
          <LoadingSpinner size="medium" />
        </div>
        <AudioPlayer v-else-if="previewType === 'audio'"
          :key="`audio:${viewerKey}`"
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
          :key="`video:${viewerKey}`"
          ref="videoRef"
          :raw="raw"
          :subtitlesList="subtitlesList"
          :req="req"
          :listing="listing"
          :autoPlayEnabled="autoPlay"
          @playback-start="playbackStarted = true"
          @navigate-previous="navigatePrevious"
          @navigate-next="navigateNext"
          @close-preview="exitPreviewFromImageGesture"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, useTemplateRef, watch } from "vue";
import { mediaApi, resourcesApi } from "@/api/index.js";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { usePreviewNavigation } from "@/composables/preview/usePreviewNavigation.ts";
import { getters, mutations, state } from "@/store/index.ts";
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import { parseLyrics } from "@/utils/lyrics.ts";
import { shouldAutoPlayPreview } from "@/utils/previewAutoplay.js";
import { convertToVTT, getSubtitleFormatExtension } from "@/utils/subtitles.js";
import { removeLastDir } from "@/utils/url.js";

const AudioPlayer = createAsyncComponent(() => import("@/components/player/AudioPlayer.vue"));
const VideoPlayer = createAsyncComponent(() => import("@/components/player/VideoPlayer.vue"));

interface SubtitleSource {
  name: string;
  src: string;
  srclang?: string;
}

const videoRef = useTemplateRef<{ destroy?: () => void }>("videoRef");
const audioRef = useTemplateRef<{ destroy?: () => void }>("audioRef");

const subtitlesList = ref<SubtitleSource[]>([]);
const lyrics = ref<ReturnType<typeof parseLyrics>>([]);
let lyricsFetchedForPath: string | null = null;
const avMetadataLoading = ref(false);
/** Skip duplicate media-metadata fetch when patchRequestFileMediaMetadata updates `req` for same path */
let mediaEnrichDoneForPath: string | null = null;
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
/** Keyed by file so each file gets a fresh player; transcode/native switching happens inside VideoPlayer. */
const viewerKey = computed(() => state.req.path);

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

</script>

<!-- Classes owned by this view; the shared player styles live in src/css/player.css. -->
<style>
/* Background styles for the audio player */
.media-background-dark {
  background: radial-gradient(#3b3b3b, black);
}

.media-background-light {
  background: radial-gradient(#262626, #e2e2e2);
}

/* Also used by VideoPlayer while it checks whether native playback is possible. */
.av-preview-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 12rem;
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

.av-preview-wrap {
  width: 100%;
  height: 100%;
  min-height: 12rem;
}
</style>
