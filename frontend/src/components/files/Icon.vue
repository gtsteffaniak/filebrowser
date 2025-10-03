<template>
  <span
    @mouseenter="handleMouseEnter"
    @mouseleave="handleMouseLeave"
    v-if="hasPreviewImage"
    :class="{ 'image-preview': hasPreviewImage }"
  >
    <i
      v-if="hasMotion && isFile"
      class="material-icons"
      :class="{ larger: showLarger, smaller: !showLarger }"
      >animation</i
    >
    <i
      v-else-if="!isFile"
      class="material-icons"
      :class="{ larger: showLarger, smaller: !showLarger }"
      >folder</i
    >
    <img
      :key="imageTargetSrc"
      :src="imageDisplaySrc"
      class="icon"
      ref="thumbnail"
    />
  </span>
  <span v-else>
    <i :class="[classes, { active: active, clickable: clickable }]" class="icon"> {{ materialIcon }} </i>
  </span>
</template>

<script>
import { globalVars, shareInfo } from "@/utils/constants";
import { getTypeInfo } from "@/utils/mimetype";
import { mutations, state, getters } from "@/store";

// NEW: Define placeholder and error image URLs for easy configuration
const PLACEHOLDER_URL = globalVars.baseURL + "static/img/placeholder.png"; // A generic loading placeholder
const ERROR_URL = globalVars.baseURL + "static/img/placeholder.png";

export default {
  name: "Icon",
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
    clickable: {
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
    };
  },
  computed: {
    isFile() {
      return this.mimetype !== "directory";
    },
    hasPreviewImage() {
      if (shareInfo.disableThumbnails) {
        return false;
      }
      if (this.thumbnailUrl == "") {
        return false;
      }
      if (!this.hasPreview) {
        return false;
      }
      const simpleType = this.getIconForType().simpleType;
      if (simpleType === "video" && !state.user.preview?.video) {
        return false;
      }
      if (simpleType === "image" && !state.user.preview?.image) {
        return false;
      }
      // office files
      if ((simpleType === "document" || simpleType === "text") && !state.user.preview?.office) {
        return false;
      }
      if (!state.user.preview.folder && this.mimetype == "directory") {
        return false;
      }
      return this.imageState !== 'error' && !this.disablePreviewExt && !this.officeFileDisabled
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
      return getters.viewMode() === "gallery" && state.user.preview.highQuality;
    },
    showLarger() {
      return getters.viewMode() === "gallery" || getters.viewMode() === "normal";
    },
    hasMotion() {
      return (
        this.getIconForType().simpleType === "video" &&
        state.user.preview?.video &&
        globalVars.mediaAvailable &&
        // @ts-ignore
        state.user.preview.motionVideoPreview
      );
    },
    isMaterialIcon() {
      return this.materialIcon !== "";
    },
  },
  methods: {
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
      const imageUrl = this.thumbnailUrl + "&size=large";
      if (this.imageState == "loaded") {
        mutations.setPreviewSource(imageUrl);
      }
      // @ts-ignore
      if (!state.user.preview.motionVideoPreview || !this.hasMotion) {
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
        if (state.popupPreviewSource === "") {
          this.previewTimeouts.forEach(clearTimeout);
          this.previewTimeouts = [];
          return;
        }
        // Set the thumbnail or popup preview
        if (state.user.preview.popup) {
          mutations.setPreviewSource(sequence[index]);
        } else {
          this.currentThumbnail = sequence[index];
        }

        // Preload the next image
        const nextIndex = (index + 1) % sequence.length;
        const preloadImg = new Image();
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
      // UPDATED: Reset to the base thumbnail URL. The watcher will handle reloading it.
      this.updateImageTargetSrc();
    },
    getIconForType() {
      return getTypeInfo(this.mimetype);
    },
    updateImageTargetSrc() {
      let newSrc = this.thumbnailUrl || PLACEHOLDER_URL;
      if (this.showLargeIcon) {
        newSrc = (this.thumbnailUrl ? this.thumbnailUrl + "&size=large" : "") || PLACEHOLDER_URL;
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
    // UPDATED: Added a check for hasPreviewImage
    imageTargetSrc: {
      handler(newSrc) {
        // ONLY trigger the image loader if the component is meant to show a preview.
        if (this.hasPreviewImage) {
          this.loadImage(newSrc);
        }
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
};
</script>

<style>
.larger {
  position: absolute;
  opacity: 0.5;
  padding: 0.1em !important;
  font-size: 2em !important;
}

.smaller {
  position: absolute;
  opacity: 0.5;
  padding: 0.1em !important;
  font-size: 1em !important;
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

#listingView.gallery .item i.white-icons,
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

.image-preview {
  height: 100%;
  width: 100%;
}
</style>
