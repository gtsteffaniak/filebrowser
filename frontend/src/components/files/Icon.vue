<template>
  <!-- Unified preview container for all types -->
  <span v-if="hasPreviewImage || (shouldUse3DPreview && !threeJsError)" class="image-preview" @mouseenter="handleMouseEnter" @mouseleave="handleMouseLeave">
    <!-- Overlay icons (folder/animation) positioned top-left -->
    <i v-if="hasPreviewImage && hasMotion && isFile" class="material-symbols-outlined overlay-icon">animation</i>
    <i v-else-if="hasPreviewImage && !isFile" class="material-symbols overlay-icon">folder</i>
    <i v-if="isShared" class="material-symbols overlay-icon">group</i>
    <!-- Preview content: image, 3D, or fallback -->
    <img
      v-if="hasPreviewImage && imageDisplaySrc"
      :src="imageDisplaySrc"
      aria-hidden="true"
      decoding="async"
    />
    <i v-else-if="hasPreviewImage" class="material-symbols placeholder-icon" aria-hidden="true">broken_image</i>
    <ThreeJs v-else-if="shouldUse3DPreview && !threeJsError"
      :key="`3d-${path}-${gallerySizeKey}`"
      :fbdata="threeDFbdata"
      :is-thumbnail="true"
      :add-load-delay="true"
      @error="handle3DError" />
  </span>
  <!-- Regular material icon (no preview) -->
  <span v-else class="image-preview">
    <i :class="[classes, { active: active, clickable: clickable }]"> {{ materialSymbol }} </i>
    <i v-if="isShared" class="material-symbols overlay-icon">group</i>
  </span>
</template>

<script setup lang="ts">
import { computed, inject, onBeforeUnmount, ref, unref, watch, type Ref } from "vue";
import { fetchPreviewImage } from "@/api/resources";
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import { globalVars } from "@/utils/constants";
import { getTypeInfo } from "@/utils/mimetype";
import { getters, mutations, state } from "@/store";
import {
  acquireThumbnailBlob,
  getThumbnailBlobKey,
  releaseThumbnailBlob,
  setImageLoaded,
  storeThumbnailBlob,
} from "@/utils/cache/imageCache.ts";
import { queuedFetchPreview } from "@/utils/previewRequests.ts";

const props = withDefaults(
  defineProps<{
    filename: string;
    mimetype: string;
    hasPreview?: boolean;
    active?: boolean;
    thumbnailUrl?: string;
    source?: string | null;
    path?: string | null;
    modified?: string | null;
    clickable?: boolean;
    size?: number | null;
    isShared?: boolean;
    isDir?: boolean;
    viewToken?: string;
    inView?: boolean;
  }>(),
  {
    hasPreview: false,
    active: false,
    thumbnailUrl: "",
    source: null,
    path: null,
    modified: null,
    clickable: false,
    size: null,
    isShared: false,
    isDir: false,
    viewToken: "",
    inView: true,
  },
);

const POPUP_PREVIEW_HOVER_DELAY_MS = 250;

type ImageState = "loading" | "loaded" | "error";

// a shallowRef from ListingItem, so a visibility change re-renders only the Icon
const injectedInView = inject<Ref<boolean> | null>("inView", null);

const ThreeJs = createAsyncComponent(() => import("@/views/files/ThreeJs.vue"));

const showLargeIcon = computed(() => getters.viewMode() === "gallery");

function buildImageTargetSrc(thumbnailUrl: string): string {
  if (!thumbnailUrl) {
    return "";
  }
  return showLargeIcon.value ? `${thumbnailUrl}&size=large` : thumbnailUrl;
}

const imageBlobUrl = ref<string | null>(null);
const imageState = ref<ImageState>("loading"); // Can be 'loading', 'loaded', or 'error'
const imageTargetSrc = ref(buildImageTargetSrc(props.thumbnailUrl));
const threeJsError = ref(false); // track if 3D preview failed
const everInView = ref<boolean>(unref(injectedInView) ?? props.inView);

// non-reactive on purpose cuz never read by template
let previewTimeouts: ReturnType<typeof setTimeout>[] = [];
let popupHoverTimeoutId: ReturnType<typeof setTimeout> | null = null;
let previewAbortController: AbortController | null = null;
let preloadAbortControllers: AbortController[] = [];
let loadPaused = false;
let blobKey: string | null = null;

const isInView = computed(() => unref(injectedInView) ?? props.inView);
const typeInfo = computed(() => getTypeInfo(props.mimetype));
const classes = computed(() => typeInfo.value.classes || "material-symbols");
const materialSymbol = computed(() => typeInfo.value.materialSymbol || "");
const isFile = computed(() => props.mimetype !== "directory");

const previewExtDisabled = computed(() => {
  const disabled = state.user?.disablePreviewExt;
  if (!disabled) {
    return false;
  }
  const dot = props.filename.lastIndexOf(".");
  if (dot <= 0) return false;
  const ext = `.${(dot === -1 ? props.filename : props.filename.slice(dot + 1)).toLowerCase()}`;
  return disabled.includes(ext);
});

/** Everything that decides whether a thumbnail may be shown */
const previewAllowed = computed(() => {
  if (state.shareInfo?.disableThumbnails) {
    return false;
  }
  if (props.thumbnailUrl === "") {
    return false;
  }
  if (!props.hasPreview) {
    return false;
  }
  const perms = getters.previewPerms();
  const simpleType = typeInfo.value.simpleType;
  if (simpleType === "audio" && !perms.audio) {
    return false;
  }
  if (simpleType === "video" && !perms.video) {
    return false;
  }
  if (simpleType === "image" && !perms.image) {
    return false;
  }
  if (simpleType === "ebook" && !perms.image) {
    return false;
  }
  // office files
  if ((simpleType === "document" || simpleType === "text") && !perms.office) {
    return false;
  }
  if (!perms.folder && props.mimetype === "directory") {
    return false;
  }
  // 3D models - show preview thumbnails (if backend provides them)
  if (simpleType === "3d-model" && !perms.models) {
    return false;
  }
  return !previewExtDisabled.value;
});

const hasPreviewImage = computed(() => previewAllowed.value && imageState.value !== "error");
const imageDisplaySrc = computed(() => (imageState.value === "loaded" ? imageBlobUrl.value : null));

const popupPreviewUrl = computed(() => (props.thumbnailUrl ? `${props.thumbnailUrl}&size=xlarge` : ""));

const hasMotion = computed(() => {
  const perms = getters.previewPerms();
  return Boolean(
    typeInfo.value.simpleType === "video" &&
      perms.video &&
      perms.motionVideoPreview,
  );
});

/** True when this is a folder with preview and we can cycle through folder images (motion preview). */
const hasFolderMotion = computed(
  () => props.mimetype === "directory" && getters.previewPerms().folder && props.hasPreview,
);

const shouldUse3DPreview = computed(() => {
  // Check if we should use 3D preview instead of regular icon
  if (props.mimetype === "directory" || !props.size || !props.path) return false;
  if (!everInView.value) return false;
  if (!getters.previewPerms().models) return false;

  const MAX_SIZE = 250 * 1024; // 250KB in bytes
  if (props.size > MAX_SIZE) return false;

  return typeInfo.value.simpleType === "3d-model";
});

/** Directory listing items for sibling viewToken lookup in 3D previews. */
const parentDirItemsForPreview = computed(() => {
  const req = state.req;
  if (!req) {
    return undefined;
  }
  if (req.type === "directory" && req.items?.length) {
    return req.items;
  }
  return req.parentDirItems;
});

// Returns gallery size to force 3D preview re-initialization on size change
// Also includes view mode to handle view changes
const gallerySizeKey = computed(() => `${state.user?.gallerySize || 1}-${getters.viewMode()}`);

const threeDFbdata = computed(() => ({
  name: props.filename,
  path: props.path,
  source: props.source,
  size: props.size,
  type: props.mimetype,
  viewToken: props.viewToken,
  parentDirItems: parentDirItemsForPreview.value,
}));

function handle3DError(): void {
  // When 3D preview fails, fall back to material icon
  threeJsError.value = true;
}

function clearPopupHoverTimeout(): void {
  if (popupHoverTimeoutId !== null) {
    clearTimeout(popupHoverTimeoutId);
    popupHoverTimeoutId = null;
  }
}

function cancelPreloadRequests(): void {
  clearPopupHoverTimeout();
  previewTimeouts.forEach(clearTimeout);
  previewTimeouts = [];
  for (const controller of preloadAbortControllers) {
    controller.abort();
  }
  preloadAbortControllers = [];
}

function cancelActivePreviews(): void {
  cancelPreloadRequests();
  if (previewAbortController) {
    previewAbortController.abort();
    previewAbortController = null;
  }
  if (imageBlobUrl.value) {
    releaseThumbnailBlob(blobKey);
    blobKey = null;
    imageBlobUrl.value = null;
  }
}

function cacheModified(): string {
  return props.modified || state.req?.modified || "";
}

/** Shows a thumbnail that was already fetched, without a request */
function showCachedThumbnail(url: string): boolean {
  const key = getThumbnailBlobKey(url, cacheModified());
  const blobUrl = acquireThumbnailBlob(key);
  if (!blobUrl) {
    return false;
  }
  blobKey = key;
  imageBlobUrl.value = blobUrl;
  imageState.value = "loaded";
  loadPaused = false;
  return true;
}

function loadImage(url: string): void {
  if (!url) {
    imageState.value = "error";
    return;
  }

  cancelActivePreviews();
  if (showCachedThumbnail(url)) {
    return;
  }
  imageState.value = "loading";
  if (!isInView.value) {
    loadPaused = true;
    return;
  }
  loadPaused = false;

  const controller = new AbortController();
  previewAbortController = controller;
  const loadUrl = url;
  const modified = cacheModified();

  queuedFetchPreview(loadUrl, controller.signal)
    .then(({ blobUrl, bytes }) => {
      // Keep what was fetched even if moved on, another one may need it
      const key = getThumbnailBlobKey(loadUrl, modified);
      const sharedBlobUrl = storeThumbnailBlob(key, blobUrl, bytes);
      if (previewAbortController !== controller || imageTargetSrc.value !== loadUrl) {
        releaseThumbnailBlob(key);
        return;
      }
      blobKey = key;
      imageBlobUrl.value = sharedBlobUrl;
      imageState.value = "loaded";
      previewAbortController = null;
      if (props.path) {
        const source = getters.isShare() ? state.shareInfo?.hash : props.source || state.req?.source;
        const size = showLargeIcon.value ? "large" : "small";
        setImageLoaded(source, props.path, size, modified, loadUrl);
      }
    })
    .catch((err) => {
      if (err?.name === "AbortError") {
        return;
      }
      if (imageTargetSrc.value === loadUrl && previewAbortController === controller) {
        imageState.value = "error";
        previewAbortController = null;
      }
    });
}

function handleMouseEnter(): void {
  if (!getters.previewPerms().popup || !props.path) {
    return;
  }
  clearPopupHoverTimeout();
  popupHoverTimeoutId = setTimeout(() => {
    popupHoverTimeoutId = null;
    showPopupPreview();
  }, POPUP_PREVIEW_HOVER_DELAY_MS);
}

function showPopupPreview(): void {
  const path = props.path;
  if (!path) {
    return;
  }
  const source = (getters.isShare() ? state.shareInfo?.hash : props.source || state.req?.source) ?? "";
  const modified = props.modified || state.req?.modified;

  // 3D model: show ThreeJs in popup
  if (shouldUse3DPreview.value) {
    mutations.setPopupPreview({
      type: "3d",
      source,
      path,
      fbdata: {
        name: props.filename,
        path,
        source,
        size: props.size ?? undefined,
        type: props.mimetype,
        viewToken: props.viewToken,
        parentDirItems: parentDirItemsForPreview.value,
      },
    });
    return;
  }

  // Image (and other preview types): popup always uses xlarge preview
  const imageUrl = popupPreviewUrl.value;
  if (imageState.value === "loaded") {
    mutations.setPopupPreview({ source, path, size: "xlarge", url: imageUrl, modified });
  }
  // Motion preview: video (atPercentage) or folder (cycle next previewable image)
  const useVideoMotion = getters.previewPerms().motionVideoPreview && hasMotion.value;
  const useFolderMotion = getters.previewPerms().popup && hasFolderMotion.value;
  if (!useVideoMotion && !useFolderMotion) {
    return;
  }

  const sequence = [
    imageUrl,
    `${imageUrl}&atPercentage=25`,
    `${imageUrl}&atPercentage=50`,
    `${imageUrl}&atPercentage=75`,
  ];
  let index = 0;

  const updateThumbnail = () => {
    if (!state.popupPreviewSourceInfo || state.popupPreviewSourceInfo.path !== props.path) {
      previewTimeouts.forEach(clearTimeout);
      previewTimeouts = [];
      return;
    }
    // Set the thumbnail or popup preview
    if (getters.previewPerms().popup) {
      mutations.setPreviewSource(sequence[index] ?? "");
    }
    // Preload the next frame (cancellable fetch warms HTTP cache)
    const nextIndex = (index + 1) % sequence.length;
    const frameUrl = sequence.at(nextIndex);
    if (!frameUrl) return;
    const preloadController = new AbortController();
    preloadAbortControllers.push(preloadController);
    fetchPreviewImage(frameUrl, preloadController.signal)
      .then((blobUrl) => {
        URL.revokeObjectURL(blobUrl);
      })
      .catch(() => {});

    // Schedule next update
    index = nextIndex;
    const timeoutId = setTimeout(updateThumbnail, 750);
    previewTimeouts.push(timeoutId);
  };
  updateThumbnail();
}

function handleMouseLeave(): void {
  clearPopupHoverTimeout();
  cancelPreloadRequests();
  mutations.setPreviewSource("");
  updateImageTargetSrc();
}

function updateImageTargetSrc(): void {
  // If we need large thumbnails and have a thumbnail URL, append &size=large
  // Otherwise use the URL as-is (defaults to small)
  const newSrc = buildImageTargetSrc(props.thumbnailUrl);
  if (imageTargetSrc.value !== newSrc) {
    imageTargetSrc.value = newSrc;
  }
}

/** stop the request if the item scrolled away while the thumb was still loading */
function pauseThumbnailLoad(): void {
  if (previewAbortController) {
    previewAbortController.abort();
    previewAbortController = null;
    imageState.value = "loading";
    loadPaused = true;
  }
}

/** request the thumb again if the item came back into view */
function resumeThumbnailLoad(): void {
  if (loadPaused && imageTargetSrc.value) {
    loadImage(imageTargetSrc.value);
  }
}

watch(() => props.thumbnailUrl, updateImageTargetSrc);
watch(showLargeIcon, updateImageTargetSrc);
watch(isInView, (visible) => {
  if (visible) {
    everInView.value = true;
    resumeThumbnailLoad();
  } else {
    pauseThumbnailLoad();
  }
});
watch(
  imageTargetSrc,
  (newSrc) => {
    if (!previewAllowed.value) {
      return;
    }
    loadImage(newSrc);
  },
  { immediate: true }, // Run on component mount
);

onBeforeUnmount(() => {
  cancelActivePreviews();
  if (props.path && state.popupPreviewSourceInfo?.path === props.path) {
    mutations.setPopupPreview(null);
  }
});
</script>

<style>

/* Overlay icons (folder/animation) positioned top-left */
.overlay-icon {
  position: absolute;
  width: 100% !important;
  height: 100% !important;
  top: 0.2em;
  left: 0.2em;
  font-size: 1.2em !important;
  text-shadow: 0 0 3px rgb(0 0 0 / 80%);
  z-index: 2;
  color: white;
  opacity: 0.7;
}

.overlay-icon--bottom {
  inset: auto 0 0 auto;
  width: auto !important;
  height: auto !important;
  pointer-events: auto;
  cursor: pointer;
  border: none;
  padding: 0;
  margin: 0;
  background: none;
  line-height: 1;
}

.overlay-icon--bottom.clickable:hover {
  transform: none;
  box-shadow: none !important;
  opacity: 1;
}

.file-icons [aria-label^="."] {
  opacity: 0.33;
}

.file-icons [aria-label$=".bak"] {
  opacity: 0.33;
}

.svg-icons {
  display: flex;
  max-width: 100px;
}

.icon {
  font-size: 1.5rem;
  fill: currentcolor;
  /* Uses inherited color */
  border-radius: 0.2em;
  padding: 0.1em;
  background: var(--iconBackground);
  will-change: auto;
  transform: translateZ(0);
}

.icon.active {
  background: var(--background);
}

/* -----------
   Icon Colors
   ----------- */
.primary-icons {
  color: var(--primaryColor);
}

.primary-icons.active {
  text-shadow: 0 0 1px #000;
}

/* blue variations */
.blue-icons {
  color: var(--icon-blue);
}

.deep-blue-icons {
  color: rgb(29 95 191);
}

.lightblue-icons {
  color: lightskyblue;
}

/* purple variations */
.purple-icons {
  color: var(--icon-violet);
}

.plum-icons {
  color: var(--icon-plum);
}

.yellow-icons {
  color: var(--icon-yellow)
}

.orange-icons {
  color: var(--icon-orange)
}

.red-icons {
  color: var(--red)
}

.deep-orange-icons {
  color: var(--icon-deep-orange)
}

.brown-icons {
  color: var(--icon-brown)
}

.coral-icons {
  color: var(--icon-coral);
}

.tan-icons {
  color: var(--icon-tan);
}

/* green variations */
.green-icons {
  color: var(--icon-green)
}

/* white variations */
.white-icons {
  color: white;
}

.gray-icons {
  color: var(--divider);
}

.lightgray-icons {
  color: var(--light-gray)
}

#listingView.gallery .listing-item i.white-icons,
.active.white-icons {
  color: var(--activeWhiteIcon);
}

/* Unified .image-preview container - works universally, always square */
.image-preview {
  width: var(--icon-size);
  height: var(--icon-size);
  aspect-ratio: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.5em;
  background: var(--iconBackground);
  overflow: hidden;
  position: relative;
}

.gallery .image-preview {
  border-radius: 0;
}

/* Universal icon styling - uses --icon-font-size variable */
.image-preview i {
  padding: 0.1em;
  box-sizing: border-box;
  transform: translateZ(0);
  font-size: var(--icon-font-size, 3em);
}

/* Images - default */
.image-preview img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center;
  display: block;
}

.image-preview .placeholder-icon {
  color: white;
  opacity: 0.7;
  text-shadow: 0 0 3px rgb(0 0 0 / 80%);
}

/* 3D viewers - universal */
.image-preview .threejs-icon-container {
  width: 100%;
  height: 100%;
  background: #000;
  display: flex;
  align-items: center;
  justify-content: center;
}

.image-preview .threejs-icon-container canvas {
  width: 100% !important;
  height: 100% !important;
  display: block;
}
</style>
