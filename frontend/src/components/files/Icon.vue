<template>
  <!-- Unified preview container for all types -->
  <span v-if="hasPreviewImage || shouldUse3DPreview" class="image-preview" @mouseenter="handleMouseEnter" @mouseleave="handleMouseLeave">
    <!-- Overlay icons (folder/animation) positioned top-left -->
    <i v-if="hasPreviewImage && hasMotion && isFile" class="material-symbols-outlined overlay-icon">animation</i>
    <i v-else-if="hasPreviewImage && !isFile" class="material-symbols overlay-icon">folder</i>
    <i v-if="isShared" class="material-symbols overlay-icon">group</i>
    <!-- Preview content: image, 3D, or fallback -->
    <img v-if="hasPreviewImage" :key="imageTargetSrc" :src="imageDisplaySrc" ref="thumbnail" />
    <ThreeJs v-else-if="shouldUse3DPreview && !threeJsError"
      :key="`3d-${path}-${gallerySizeKey}`"
      :fbdata="{
        name: filename,
        path: path,
        source: source,
        size: size,
        type: mimetype
      }"
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
import { fetchPreviewImage } from "@/api/resources";
import { globalVars } from "@/utils/constants";
import { getTypeInfo } from "@/utils/mimetype";
import { getObjectProperty } from '@/utils/object.js';
import { mutations, state, getters } from "@/store";
import { setImageLoaded } from "@/utils/imageCache";
import ThreeJs from "@/views/files/ThreeJs.vue";

export default {
  name: "Icon",
  components: {
    ThreeJs,
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
  },
  data() {
    return {
      materialSymbol: "",
      classes: "",
      svgPath: "",
      previewTimeouts: [],
      previewAbortController: null,
      preloadAbortControllers: [],
      imageBlobUrl: null,
      // UPDATED: Manage image state directly
      imageState: "loading", // Can be 'loading', 'loaded', or 'error'
      imageTargetSrc: "", // API URL being loaded
      currentThumbnail: "", // Add currentThumbnail to data
      color: "", // Add color to data
      threeJsError: false, // Track if 3D preview failed
    };
  },
  computed: {
    placeholderUrl() {
      return `${globalVars.baseURL}public/static/img/placeholder.png`;
    },
    errorUrl() {
      return `${globalVars.baseURL}public/static/img/placeholder.png`;
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
      const simpleType = this.getIconForType().simpleType;
      if (simpleType === "audio" && !getters.previewPerms().audio) {
        return false;
      }
      if (simpleType === "video" && !getters.previewPerms().video) {
        return false;
      }
      if (simpleType === "image" && !getters.previewPerms().image) {
        return false;
      }
      if (simpleType === "ebook" && !getters.previewPerms().image) {
        return false;
      }
      // office files
      if ((simpleType === "document" || simpleType === "text") && !getters.previewPerms().office) {
        return false;
      }
      if (!getters.previewPerms().folder && this.mimetype === "directory") {
        return false;
      }
      // 3D models - show preview thumbnails (if backend provides them)
      if (simpleType === "3d-model" && !getters.previewPerms().models) {
        return false;
      }
      return this.imageState !== 'error' && !this.disablePreviewExt && !this.officeFileDisabled;
    },
    disablePreviewExt() {
      const ext = `.${(this.filename.split(".").pop() || "").toLowerCase()}`; // Ensure lowercase and dot
      return state.user?.disablePreviewExt?.includes(ext);
    },
    officeFileDisabled() {
      const ext = `.${(this.filename.split(".").pop() || "").toLowerCase()}`; // Ensure lowercase and dot
      return state.user?.disablePreviewExt?.includes(ext);
    },
    pdfConvertable() {
      if (!globalVars.muPdfAvailable) {
        return false; // If muPDF is not available
      }
      const ext = `.${(this.filename.split(".").pop() || "").toLowerCase()}`; // Ensure lowercase and dot
      const pdfConvertCompatibleFileExtensions = {
        ".pdf": true,
        ".xps": true,
        ".epub": true,
        ".mobi": true,
        ".fb2": true,
        ".cbz": true,
        ".svg": true,
        ".txt": true,
        ".docx": true,
        ".pptx": true,
        ".xlsx": true,
        ".hwp": true,
        ".hwpx": true,
        ".md": true,
      };
      return ext in pdfConvertCompatibleFileExtensions;
    },
    // NEW: A single computed property to determine the final image src
    imageDisplaySrc() {
      if (this.imageState === "error") {
        return this.errorUrl;
      }
      // Show placeholder only for the initial load, not during hover animations
      if (this.imageState === "loading") {
        return this.placeholderUrl;
      }
      return this.imageBlobUrl || this.placeholderUrl;
    },
    showLargeIcon() {
      return getters.viewMode() === "gallery";
    },
    popupPreviewUrl() {
      if (!this.thumbnailUrl) {
        return "";
      }
      return `${this.thumbnailUrl}&size=xlarge`;
    },
    showLarger() {
      return getters.viewMode() === "gallery" || getters.viewMode() === "normal";
    },
    hasMotion() {
      return (
        this.getIconForType().simpleType === "video" &&
        getters.previewPerms().video &&
        globalVars.mediaAvailable &&
        getters.previewPerms().motionVideoPreview
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
      if (!getters.previewPerms().models) return false;

      const MAX_SIZE = 250 * 1024; // 250KB in bytes
      if (this.size > MAX_SIZE) return false;

      const typeInfo = this.getIconForType();
      return typeInfo.simpleType === '3d-model';
    },
    gallerySizeKey() {
      // Returns gallery size to force 3D preview re-initialization on size change
      // Also includes view mode to handle view changes
      return `${state.user?.gallerySize || 1}-${getters.viewMode()}`;
    },
    routePath() {
      return state.route.path ?? "";
    },
  },
  methods: {
    handle3DError() {
      // When 3D preview fails, fall back to material icon
      this.threeJsError = true;
    },
    cancelPreloadRequests() {
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
        URL.revokeObjectURL(this.imageBlobUrl);
        this.imageBlobUrl = null;
      }
    },
    // Load preview via fetch so requests can be aborted on navigation
    /**
     * @param {string} url
     */
    loadImage(url) {
      if (!url) {
        this.imageState = "error";
        return;
      }

      this.cancelActivePreviews();
      this.imageState = "loading";

      const controller = new AbortController();
      this.previewAbortController = controller;
      const loadUrl = url;

      fetchPreviewImage(loadUrl, controller.signal)
        .then((blobUrl) => {
          if (this.previewAbortController !== controller) {
            URL.revokeObjectURL(blobUrl);
            return;
          }
          if (this.imageTargetSrc !== loadUrl) {
            URL.revokeObjectURL(blobUrl);
            return;
          }
          this.imageBlobUrl = blobUrl;
          this.imageState = "loaded";
          this.previewAbortController = null;
          if (this.path) {
            const source = getters.isShare() ? state.shareInfo?.hash : (this.source || state.req.source);
            const size = this.showLargeIcon ? "large" : "small";
            const modified = this.modified || state.req.modified;
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
        } else {
          this.currentThumbnail = getObjectProperty(sequence, index);
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
            if (this.path) {
              const source = getters.isShare() ? state.shareInfo?.hash : (this.source || state.req.source);
              const modified = this.modified || state.req.modified;
              setImageLoaded(source, this.path, "xlarge", modified, frameUrl);
            }
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
      this.cancelPreloadRequests();
      mutations.setPreviewSource("");
      // Clear popup preview source info when mouse leaves
      state.popupPreviewSourceInfo = null;
      // UPDATED: Reset to the base thumbnail URL. The watcher will handle reloading it.
      this.updateImageTargetSrc();
    },
    getIconForType() {
      return getTypeInfo(this.mimetype);
    },
    updateImageTargetSrc() {
      let newSrc = this.thumbnailUrl || this.placeholderUrl;
      // If we need large thumbnails and have a thumbnail URL, append &size=large
      // Otherwise use the URL as-is (defaults to small)
      if (this.thumbnailUrl && this.showLargeIcon) {
        newSrc = `${this.thumbnailUrl}&size=large`;
      }

      if (this.imageTargetSrc !== newSrc) {
        this.imageTargetSrc = newSrc;
      }
    },
  },
  watch: {
    routePath() {
      this.cancelActivePreviews();
      this.imageState = "loading";
    },
    thumbnailUrl() {
      this.updateImageTargetSrc();
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
        const simpleType = this.getIconForType().simpleType;
        // Check preview settings (share or user)
        if (simpleType === "video" && !getters.previewPerms().video) {
          return;
        }
        if (simpleType === "image" && !getters.previewPerms().image) {
          return;
        }
        if ((simpleType === "document" || simpleType === "text") && !getters.previewPerms().office) {
          return;
        }
        if (!getters.previewPerms().folder && this.mimetype === "directory") {
          return;
        }
        if (this.disablePreviewExt || this.officeFileDisabled) {
          return;
        }
        // All checks passed, load the image
        this.loadImage(newSrc);
      },
      immediate: true, // Run this watcher on component mount
    },
  },
  mounted() {
    const result = this.getIconForType();
    this.classes = result.classes || "material-symbols";
    this.color = result.color || "lightgray";
    this.materialSymbol = result.materialSymbol || "";
    this.svgPath = result.svgPath || "";
    this.updateImageTargetSrc();
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
  text-shadow: 0 0 3px rgba(0, 0, 0, 0.8);
  z-index: 2;
  color: white;
  opacity: 0.7;
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
  fill: currentColor;
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
  text-shadow: 0px 0px 1px #000;
}

/* blue variations */
.blue-icons {
  color: var(--icon-blue);
}

.deep-blue-icons {
  color: rgb(29, 95, 191);
}

.lightblue-icons {
  color: lightskyblue;
}

.skyblue-icons {
  color: rgb(42, 170, 242);
}

/* purple variations */
.purple-icons {
  color: purple;
}

.plum-icons {
  color: plum;
}

/* yellow */
.yellow-icons {
  color: yellow;
}

/* orange/red variations */
.orange-icons {
  color: orange;
}

.red-icons {
  color: rgb(211, 16, 16);
}

.deep-orange-icons {
  color: rgb(255, 111, 0);
}

.brown-icons {
  color: brown;
}

.coral-icons {
  color: lightcoral;
}

.tan-icons {
  color: tan;
}

/* green variations */
.green-icons {
  color: rgb(23, 128, 74);
}

.light-green-icons {
  color: rgb(48, 207, 117);
}

/* white variations */
.white-icons {
  color: white;
}

.gray-icons {
  color: gray;
}

.lightgray-icons {
  color: rgb(176, 176, 176);
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
