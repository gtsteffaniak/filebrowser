<template>
  <!-- Unified preview container for all types -->
  <span v-if="hasPreviewImage || shouldUse3DPreview" class="image-preview" @mouseenter="handleMouseEnter" @mouseleave="handleMouseLeave">
    <!-- Overlay icons (folder/animation) positioned top-left -->
    <i v-if="hasPreviewImage && hasMotion && isFile" class="material-icons overlay-icon muted">animation</i>
    <i v-else-if="hasPreviewImage && !isFile" class="material-icons overlay-icon muted">folder</i>
    <i v-if="isShared" class="material-icons overlay-icon">group</i>
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
    <i :class="[classes, { active: active, clickable: clickable }]"> {{ materialIcon }} </i>
    <i v-if="isShared" class="material-icons overlay-icon">group</i>
  </span>
</template>

<script>
import { globalVars } from "@/utils/constants";
import { getTypeInfo } from "@/utils/mimetype";
import { mutations, state, getters } from "@/store";
import { setImageLoaded } from "@/utils/imageCache";
import ThreeJs from "@/views/files/ThreeJs.vue";

// NEW: Define placeholder and error image URLs for easy configuration
const PLACEHOLDER_URL = globalVars.baseURL + "public/static/img/placeholder.png"; // A generic loading placeholder
const ERROR_URL = globalVars.baseURL + "public/static/img/placeholder.png";

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
      materialIcon: "",
      classes: "",
      svgPath: "",
      previewTimeouts: [],
      // UPDATED: Manage image state directly
      imageState: "loading", // Can be 'loading', 'loaded', or 'error'
      imageTargetSrc: "", // This is now a data property
      currentThumbnail: "", // Add currentThumbnail to data
      color: "", // Add color to data
      threeJsError: false, // Track if 3D preview failed
    };
  },
  computed: {
    isFile() {
      return this.mimetype !== "directory";
    },
    hasPreviewImage() {
      if (state.shareInfo?.disableThumbnails) {
        return false;
      }
      if (this.thumbnailUrl == "") {
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
      // office files
      if ((simpleType === "document" || simpleType === "text") && !getters.previewPerms().office) {
        return false;
      }
      if (!getters.previewPerms().folder && this.mimetype == "directory") {
        return false;
      }
      // 3D models - show preview thumbnails (if backend provides them)
      if (simpleType === "3d-model" && !getters.previewPerms().image) {
        return false;
      }
      return this.imageState !== 'error' && !this.disablePreviewExt && !this.officeFileDisabled;
    },
    disablePreviewExt() {
      const ext = "." + (this.filename.split(".").pop() || "").toLowerCase(); // Ensure lowercase and dot
      // @ts-ignore
      return state.user?.disablePreviewExt?.includes(ext);
    },
    officeFileDisabled() {
      const ext = "." + (this.filename.split(".").pop() || "").toLowerCase(); // Ensure lowercase and dot
      // @ts-ignore
      return state.user?.disablePreviewExt?.includes(ext);
    },
    pdfConvertable() {
      if (!globalVars.muPdfAvailable) {
        return false; // If muPDF is not available
      }
      const ext = "." + (this.filename.split(".").pop() || "").toLowerCase(); // Ensure lowercase and dot
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
        return ERROR_URL;
      }
      // Show placeholder only for the initial load, not during hover animations
      if (this.imageState === "loading") {
        return PLACEHOLDER_URL;
      }
      return this.imageTargetSrc;
    },
    showLargeIcon() {
      return getters.viewMode() === "gallery";
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
    isMaterialIcon() {
      return this.materialIcon !== "";
    },
    shouldUse3DPreview() {
      // Check if we should use 3D preview instead of regular icon
      if (!this.isFile || !this.size || !this.path) return false;
      
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
  },
  methods: {
    handle3DError() {
      // When 3D preview fails, fall back to material icon
      this.threeJsError = true;
    },
    // NEW: Centralized method to load any image and handle its state
    /**
     * @param {string} url
     */
    loadImage(url) {
      if (!url) {
        this.imageState = "error";
        return;
      }

      this.imageState = "loading";
      const targetImage = new Image();

      targetImage.onload = () => {
        // Prevent race conditions: only update if this is the image we still want.
        if (this.imageTargetSrc === url) {
          this.imageState = "loaded";
          // Mark this image as loaded in our cache tracker
          if (this.path) {
            // For shares, use shareInfo.hash as the source; otherwise use this.source or state.req.source
            const source = getters.isShare() ? state.shareInfo?.hash : (this.source || state.req.source);
            const size = this.showLargeIcon ? 'large' : 'small';
            // Use file's modified date if available, otherwise fall back to state.req.modified
            const modified = this.modified || state.req.modified;
            setImageLoaded(source, this.path, size, modified, url);
          }
        }
      };

      targetImage.onerror = () => {
        // Prevent race conditions: only show an error if this is the image that failed.
        if (this.imageTargetSrc === url) {
          this.imageState = "error";
        }
      };

      targetImage.src = url;
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

      // Image (and other preview types): use thumbnail URL
      const imageUrl = this.thumbnailUrl + "&size=large";
      if (this.imageState === "loaded") {
        mutations.setPreviewSource(imageUrl);
        state.popupPreviewSourceInfo = { source, path: this.path, size: "large", url: imageUrl, modified };
      }
      // Motion preview: video (atPercentage) or folder (cycle next previewable image)
      const useVideoMotion = getters.previewPerms().motionVideoPreview && this.hasMotion;
      const useFolderMotion = getters.previewPerms().popup && this.hasFolderMotion;
      if (!useVideoMotion && !useFolderMotion) {
        return;
      }

      const sequence = [
        imageUrl,
        imageUrl + "&atPercentage=25",
        imageUrl + "&atPercentage=50",
        imageUrl + "&atPercentage=75",
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
          mutations.setPreviewSource(sequence[index]);
        } else {
          this.currentThumbnail = sequence[index];
        }

        // Preload the next image
        const nextIndex = (index + 1) % sequence.length;
        const preloadImg = new Image();
        preloadImg.onload = () => {
          // Track that this image was loaded (motion preview frames use the same path)
          if (this.path) {
            // For shares, use shareInfo.hash as the source; otherwise use this.source or state.req.source
            const source = getters.isShare() ? state.shareInfo?.hash : (this.source || state.req.source);
            const frameUrl = sequence[nextIndex];
            // Use file's modified date if available, otherwise fall back to state.req.modified
            const modified = this.modified || state.req.modified;
            setImageLoaded(source, this.path, 'large', modified, frameUrl);
          }
        };
        preloadImg.src = sequence[nextIndex];

        // Schedule next update
        index = nextIndex;
        const timeoutId = setTimeout(updateThumbnail, 750);
        // @ts-ignore
        this.previewTimeouts.push(timeoutId);
      };
      updateThumbnail();
    },
    handleMouseLeave() {
      this.previewTimeouts.forEach(clearTimeout);
      this.previewTimeouts = [];
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
      let newSrc = this.thumbnailUrl || PLACEHOLDER_URL;
      // If we need large thumbnails and have a thumbnail URL, append &size=large
      // Otherwise use the URL as-is (defaults to small)
      if (this.thumbnailUrl && this.showLargeIcon) {
        newSrc = this.thumbnailUrl + "&size=large";
      }
      
      if (this.imageTargetSrc !== newSrc) {
        this.imageTargetSrc = newSrc;
      }
    },
  },
  watch: {
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
    this.classes = result.classes || "material-icons";
    // @ts-ignore
    this.color = result.color || "lightgray";
    this.materialIcon = result.materialIcon || "";
    // @ts-ignore
    this.svgPath = result.svgPath || "";
    this.updateImageTargetSrc();
  },
  beforeUnmount() {
    // Clean up any pending animation timeouts
    this.previewTimeouts.forEach(clearTimeout);
    this.previewTimeouts = [];
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
}

.overlay-icon.muted {
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
  /* Default size */
  fill: currentColor;
  /* Uses inherited color */
  border-radius: 0.2em;
  padding: 0.1em;
  background: var(--iconBackground);
  /* Performance optimization */
  will-change: auto;
  transform: translateZ(0);
}

.icon.active {
  background: var(--background);
}

.purple-icons {
  color: purple;
}

/* Icon Colors */
.blue-icons {
  color: var(--icon-blue);
}

/* Icon Colors */
.primary-icons {
  color: var(--primaryColor);
}

.primary-icons.active {
  text-shadow: 0px 0px 1px #000;
}

.lightblue-icons {
  color: lightskyblue;
}

.orange-icons {
  color: lightcoral;
}

.tan-icons {
  color: tan;
}

.plum-icons {
  color: plum;
}

.red-icons {
  color: rgb(246, 70, 70);
}

.white-icons {
  color: white;
}

#listingView.gallery .listing-item i.white-icons,
.active.white-icons {
  color: var(--activeWhiteIcon);
}

.deep-blue-icons {
  color: rgb(29, 95, 191);
}

.green-icons {
  color: rgb(23, 128, 74);
}

.red-orange-icons {
  color: rgb(255, 147, 111);
}

.gray-icons {
  color: gray;
}

.skyblue-icons {
  color: rgb(42, 170, 242);
}

.lightgray-icons {
  color: rgb(176, 176, 176);
}

.yellow-icons {
  color: yellow;
}

.simple-icons {
  color: white;
  font-size: 1.5em !important;
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
