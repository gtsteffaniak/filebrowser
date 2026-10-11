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
      :key="imageTargetSrc"
      :src="imageDisplaySrc"
      ref="thumbnail"
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

<script>
import { computed } from "vue";
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import { fetchPreviewImage } from "@/api/resources";
import { globalVars } from "@/utils/constants";
import { getTypeInfo } from "@/utils/mimetype";
import { getObjectProperty } from '@/utils/object.js';
import { mutations, state, getters } from "@/store";
import {
  acquireThumbnailBlob,
  getThumbnailBlobKey,
  releaseThumbnailBlob,
  setImageLoaded,
  storeThumbnailBlob
} from "@/utils/imageCache.ts";

const POPUP_PREVIEW_HOVER_DELAY_MS = 250;
const MAX_THUMBNAIL_LOADS = 8;
let activeThumbnailLoads = 0;
const thumbnailQueue = [];

function abortError() {
  return new DOMException("Aborted", "AbortError");
}

function loadThumbnailQueue() {
  while (activeThumbnailLoads < MAX_THUMBNAIL_LOADS && thumbnailQueue.length > 0) {
    const job = thumbnailQueue.shift();
    job.signal.removeEventListener("abort", job.onAbort);
    activeThumbnailLoads++;
    fetchPreviewImage(job.url, job.signal)
      .then(job.resolve, job.reject)
      .finally(() => {
        activeThumbnailLoads--;
        loadThumbnailQueue();
      });
  }
}

/** @param {string} url @param {AbortSignal} signal @returns {Promise<string>} blob URL */
function queuedFetchPreview(url, signal) {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(abortError());
      return;
    }
    const job = { url, signal, resolve, reject, onAbort: null };
    job.onAbort = () => {
      const i = thumbnailQueue.indexOf(job);
      if (i !== -1) {
        thumbnailQueue.splice(i, 1);
        reject(abortError());
      }
    };
    signal.addEventListener("abort", job.onAbort, { once: true });
    thumbnailQueue.push(job);
    loadThumbnailQueue();
  });
}

const viewModeValue = computed(() => getters.viewMode());

function buildImageTargetSrc(thumbnailUrl) {
  if (!thumbnailUrl) {
    return "";
  }
  return viewModeValue.value === "gallery" ? `${thumbnailUrl}&size=large` : thumbnailUrl;
}

export default {
  name: "Icon",
  components: {
    ThreeJs: createAsyncComponent(() => import('@/views/files/ThreeJs.vue')),
  },
  props: {
    filename: {
      type: String,
      required: true,
    },
    mimetype: {
      type: String,
      required: true,
    },
    hasPreview: {
      type: Boolean,
      default: false,
    },
    active: {
      type: Boolean,
    },
    thumbnailUrl: {
      type: String,
      default: "",
    },
    source: {
      type: String,
      default: null,
    },
    path: {
      type: String,
      default: null,
    },
    modified: {
      type: String,
      default: null,
    },
    clickable: {
      type: Boolean,
      default: false,
    },
    size: {
      type: Number,
      default: null,
    },
    isShared: {
      type: Boolean,
      default: false,
    },
    isDir: {
      type: Boolean,
      default: false,
    },
    viewToken: {
      type: String,
      default: "",
    },
    inView: {
      type: Boolean,
      default: true,
    },
    inViewRef: {
      type: Object,
      default: null,
    },
  },
  data() {
    return {
      imageBlobUrl: null,
      imageState: "loading", // Can be 'loading', 'loaded', or 'error'
      imageTargetSrc: buildImageTargetSrc(this.thumbnailUrl),
      threeJsError: false, // Track if 3D preview failed
      everInView: this.inViewRef ? this.inViewRef.value : this.inView,
    };
  },
  computed: {
    isInView() {
      return this.inViewRef ? this.inViewRef.value : this.inView;
    },
    typeInfo() {
      return getTypeInfo(this.mimetype);
    },
    classes() {
      return this.typeInfo.classes || "material-symbols";
    },
    materialSymbol() {
      return this.typeInfo.materialSymbol || "";
    },
    isFile() {
      return this.mimetype !== "directory";
    },
    hasPreviewImage() {
      if (state.shareInfo?.disableThumbnails) {
        return false;
      }
      if (this.thumbnailUrl === "") {
        return false;
      }
      if (!this.hasPreview) {
        return false;
      }
      const perms = getters.previewPerms();
      const simpleType = this.typeInfo.simpleType;
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
      if (!perms.folder && this.mimetype === "directory") {
        return false;
      }
      // 3D models - show preview thumbnails (if backend provides them)
      if (simpleType === "3d-model" && !perms.models) {
        return false;
      }
      return this.imageState !== 'error' && !this.previewExtDisabled;
    },
    previewExtDisabled() {
      const disabled = state.user?.disablePreviewExt;
      if (!disabled) {
        return false;
      }
      const dot = this.filename.lastIndexOf(".");
      const ext = `.${(dot === -1 ? this.filename : this.filename.slice(dot + 1)).toLowerCase()}`;
      return disabled.includes(ext);
    },
    imageDisplaySrc() {
      return this.imageState === "loaded" ? this.imageBlobUrl : null;
    },
    showLargeIcon() {
      return viewModeValue.value === "gallery";
    },
    popupPreviewUrl() {
      if (!this.thumbnailUrl) {
        return "";
      }
      return `${this.thumbnailUrl}&size=xlarge`;
    },
    hasMotion() {
      const perms = getters.previewPerms();
      return (
        this.typeInfo.simpleType === "video" &&
        perms.video &&
        globalVars.mediaAvailable &&
        perms.motionVideoPreview
      );
    },
    /** True when this is a folder with preview and we can cycle through folder images (motion preview). */
    hasFolderMotion() {
      return (
        this.mimetype === "directory" &&
        getters.previewPerms().folder &&
        this.hasPreview
      );
    },
    shouldUse3DPreview() {
      // Check if we should use 3D preview instead of regular icon
      if (this.mimetype === "directory" || !this.size || !this.path) return false;
      if (!this.everInView) return false;
      if (!getters.previewPerms().models) return false;

      const MAX_SIZE = 250 * 1024; // 250KB in bytes
      if (this.size > MAX_SIZE) return false;

      return this.typeInfo.simpleType === '3d-model';
    },
    /** Directory listing items for sibling viewToken lookup in 3D previews. */
    parentDirItemsForPreview() {
      const req = state.req;
      if (!req) {
        return undefined;
      }
      if (req.type === "directory" && req.items?.length) {
        return req.items;
      }
      return req.parentDirItems;
    },
    gallerySizeKey() {
      // Returns gallery size to force 3D preview re-initialization on size change
      // Also includes view mode to handle view changes
      return `${state.user?.gallerySize || 1}-${viewModeValue.value}`;
    },
    threeDFbdata() {
      return {
        name: this.filename,
        path: this.path,
        source: this.source,
        size: this.size,
        type: this.mimetype,
        viewToken: this.viewToken,
        parentDirItems: this.parentDirItemsForPreview,
      };
    },
  },
  methods: {
    handle3DError() {
      // When 3D preview fails, fall back to material icon
      this.threeJsError = true;
    },
    clearPopupHoverTimeout() {
      if (this.popupHoverTimeoutId !== null) {
        clearTimeout(this.popupHoverTimeoutId);
        this.popupHoverTimeoutId = null;
      }
    },
    cancelPreloadRequests() {
      this.clearPopupHoverTimeout();
      this.previewTimeouts.forEach(clearTimeout);
      this.previewTimeouts = [];
      for (const controller of this.preloadAbortControllers) {
        controller.abort();
      }
      this.preloadAbortControllers = [];
    },
    cancelActivePreviews() {
      this.cancelPreloadRequests();
      if (this.previewAbortController) {
        this.previewAbortController.abort();
        this.previewAbortController = null;
      }
      if (this.imageBlobUrl) {
        releaseThumbnailBlob(this.blobKey);
        this.blobKey = null;
        this.imageBlobUrl = null;
      }
    },
    cacheModified() {
      return this.modified || state.req?.modified || "";
    },
    /** Shows a thumbnail that was already fetched, without a request @returns {boolean} */
    showCachedThumbnail(url) {
      const key = getThumbnailBlobKey(url, this.cacheModified());
      const blobUrl = acquireThumbnailBlob(key);
      if (!blobUrl) {
        return false;
      }
      this.blobKey = key;
      this.imageBlobUrl = blobUrl;
      this.imageState = "loaded";
      this.loadPaused = false;
      return true;
    },
    /** @param {string} url */
    loadImage(url) {
      if (!url) {
        this.imageState = "error";
        return;
      }

      this.cancelActivePreviews();
      if (this.showCachedThumbnail(url)) {
        return;
      }
      this.imageState = "loading";
      if (!this.isInView) {
        this.loadPaused = true;
        return;
      }
      this.loadPaused = false;

      const controller = new AbortController();
      this.previewAbortController = controller;
      const loadUrl = url;
      const modified = this.cacheModified();

      queuedFetchPreview(loadUrl, controller.signal)
        .then((blobUrl) => {
          // Keep what was fetched even if moved on, another one may need it
          const key = getThumbnailBlobKey(loadUrl, modified);
          const sharedBlobUrl = storeThumbnailBlob(key, blobUrl);
          if (this.previewAbortController !== controller || this.imageTargetSrc !== loadUrl) {
            releaseThumbnailBlob(key);
            return;
          }
          this.blobKey = key;
          this.imageBlobUrl = sharedBlobUrl;
          this.imageState = "loaded";
          this.previewAbortController = null;
          if (this.path) {
            const source = getters.isShare() ? state.shareInfo?.hash : (this.source || state.req.source);
            const size = this.showLargeIcon ? "large" : "small";
            setImageLoaded(source, this.path, size, modified, loadUrl);
          }
        })
        .catch((err) => {
          if (err?.name === "AbortError") {
            return;
          }
          if (this.imageTargetSrc === loadUrl && this.previewAbortController === controller) {
            this.imageState = "error";
            this.previewAbortController = null;
          }
        });
    },
    handleMouseEnter() {
      if (!getters.previewPerms().popup || !this.path) {
        return;
      }
      this.clearPopupHoverTimeout();
      this.popupHoverTimeoutId = setTimeout(() => {
        this.popupHoverTimeoutId = null;
        this.showPopupPreview();
      }, POPUP_PREVIEW_HOVER_DELAY_MS);
    },
    showPopupPreview() {
      const source = getters.isShare() ? state.shareInfo?.hash : (this.source || state.req.source);
      const modified = this.modified || state.req.modified;

      // 3D model: show ThreeJs in popup
      if (this.shouldUse3DPreview) {
        state.popupPreviewSourceInfo = {
          type: "3d",
          source,
          path: this.path,
          fbdata: {
            name: this.filename,
            path: this.path,
            source,
            size: this.size,
            type: this.mimetype,
            viewToken: this.viewToken,
            parentDirItems: this.parentDirItemsForPreview,
          },
        };
        return;
      }

      // Image (and other preview types): popup always uses xlarge preview
      const imageUrl = this.popupPreviewUrl;
      if (this.imageState === "loaded") {
        state.popupPreviewSourceInfo = { source, path: this.path, size: "xlarge", url: imageUrl, modified };
      }
      // Motion preview: video (atPercentage) or folder (cycle next previewable image)
      const useVideoMotion = getters.previewPerms().motionVideoPreview && this.hasMotion;
      const useFolderMotion = getters.previewPerms().popup && this.hasFolderMotion;
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
        if (!state.popupPreviewSourceInfo || state.popupPreviewSourceInfo.path !== this.path) {
          this.previewTimeouts.forEach(clearTimeout);
          this.previewTimeouts = [];
          return;
        }
        // Set the thumbnail or popup preview
        if (getters.previewPerms().popup) {
          mutations.setPreviewSource(getObjectProperty(sequence, index));
        }
        // Preload the next frame (cancellable fetch warms HTTP cache)
        const nextIndex = (index + 1) % sequence.length;
        const frameUrl = sequence.at(nextIndex);
        if (!frameUrl) return;
        const preloadController = new AbortController();
        this.preloadAbortControllers.push(preloadController);
        fetchPreviewImage(frameUrl, preloadController.signal)
          .then((blobUrl) => {
            URL.revokeObjectURL(blobUrl);
          })
          .catch(() => {});

        // Schedule next update
        index = nextIndex;
        const timeoutId = setTimeout(updateThumbnail, 750);
        this.previewTimeouts.push(timeoutId);
      };
      updateThumbnail();
    },
    handleMouseLeave() {
      this.clearPopupHoverTimeout();
      this.cancelPreloadRequests();
      mutations.setPreviewSource("");
      // Clear popup preview source info when mouse leaves
      state.popupPreviewSourceInfo = null;
      this.updateImageTargetSrc();
    },
    updateImageTargetSrc() {
      // If we need large thumbnails and have a thumbnail URL, append &size=large
      // Otherwise use the URL as-is (defaults to small)
      const newSrc = buildImageTargetSrc(this.thumbnailUrl);
      if (this.imageTargetSrc !== newSrc) {
        this.imageTargetSrc = newSrc;
      }
    },
    /** stop the request if the item scrolled away while the thumb was still loading */
    pauseThumbnailLoad() {
      if (this.previewAbortController) {
        this.previewAbortController.abort();
        this.previewAbortController = null;
        this.imageState = "loading";
        this.loadPaused = true;
      }
    },
    /** request the again thumb again if the item came back into view */
    resumeThumbnailLoad() {
      if (this.loadPaused && this.imageTargetSrc) {
        this.loadImage(this.imageTargetSrc);
      }
    },
  },
  watch: {
    thumbnailUrl() {
      this.updateImageTargetSrc();
    },
    isInView(visible) {
      if (visible) {
        this.everInView = true;
        this.resumeThumbnailLoad();
      } else {
        this.pauseThumbnailLoad();
      }
    },
    showLargeIcon() {
      this.updateImageTargetSrc();
    },
    imageTargetSrc: {
      handler(newSrc) {
        // Check all conditions EXCEPT imageState to avoid circular dependency
        if (!this.hasPreview || !this.thumbnailUrl) {
          return;
        }
        if (state.shareInfo?.disableThumbnails) {
          return;
        }
        const simpleType = this.typeInfo.simpleType;
        const perms = getters.previewPerms();
        if (simpleType === "video" && !perms.video) {
          return;
        }
        if (simpleType === "image" && !perms.image) {
          return;
        }
        if ((simpleType === "document" || simpleType === "text") && !perms.office) {
          return;
        }
        if (!perms.folder && this.mimetype === "directory") {
          return;
        }
        if (this.previewExtDisabled) {
          return;
        }
        // All checks passed, load the image
        this.loadImage(newSrc);
      },
      immediate: true, // Run this watcher on component mount
    },
  },
  beforeCreate() {
    this.previewTimeouts = [];
    this.popupHoverTimeoutId = null;
    this.previewAbortController = null;
    this.preloadAbortControllers = [];
    this.loadPaused = false;
    this.blobKey = null;
  },
  beforeUnmount() {
    this.cancelActivePreviews();
  },
};
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
