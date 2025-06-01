<template>
  <span v-if="isPreviewImg && imageState !== 'error'">
    <i
      v-if="hasMotion"
      class="material-icons"
      :class="{ larger: showLarger, smaller: !showLarger }"
      >animation</i
    >
    <img
      :key="thumbnailUrl"
      :src="imageDisplaySrc"
      @mouseenter="handleMouseEnter($event)"
      @mouseleave="handleMouseLeave($event)"
      class="icon"
      ref="thumbnail"
    />
  </span>
  <span v-else>
    <i :class="[classes, { active: active }]" class="icon"> {{ materialIcon }} </i>
  </span>
</template>

<script>
import { onlyOfficeUrl, mediaAvailable, pdfAvailable, baseURL } from "@/utils/constants";
import { getTypeInfo } from "@/utils/mimetype";
import { mutations, state } from "@/store";

// NEW: Define placeholder and error image URLs for easy configuration
const PLACEHOLDER_URL = baseURL + "static/img/placeholder.png"; // A generic loading placeholder
const ERROR_URL = baseURL + "static/img/placeholder.png";

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
    active: {
      type: Boolean,
    },
    thumbnailUrl: {
      type: String,
      default: "",
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
      imageTargetSrc: this.thumbnailUrl, // The URL we currently want to display
    };
  },
  computed: {
    pdfConvertable() {
      const ext = "." + this.filename.split(".").pop().toLowerCase(); // Ensure lowercase and dot
      const pdfConvertCompatibleFileExtensions = {
        ".pdf": true,
        ".xps": true,
        ".epub": true,
        ".mobi": true,
        ".fb2": true,
        ".cbz": true,
        ".svg": true,
        ".txt": true,
        ".doc": true,
        ".docx": true,
        ".ppt": true,
        ".pptx": true,
        ".xls": true,
        ".xlsx": true,
        ".hwp": true,
        ".hwpx": true, // fix duplication and add this one
      };
      return !!pdfConvertCompatibleFileExtensions[ext];
    },
    // NEW: A single computed property to determine the final image src
    imageDisplaySrc() {
      if (this.imageState === "error") {
        return ERROR_URL;
      }
      // Show placeholder only for the initial load, not during hover animations
      if (this.imageState === "loading" && this.imageTargetSrc === this.thumbnailUrl) {
        return PLACEHOLDER_URL;
      }
      return this.imageTargetSrc;
    },
    showLarger() {
      return state.user.viewMode === "gallery" || state.user.viewMode === "normal";
    },
    hasMotion() {
      return (
        this.getIconForType().simpleType === "video" &&
        state.user.preview?.video &&
        mediaAvailable &&
        state.user.preview.motionVideoPreview
      );
    },
    isMaterialIcon() {
      return this.materialIcon !== "";
    },
    isPreviewImg() {
      if (this.thumbnailUrl == "") {
        return false;
      }
      if (this.mimetype == "text/csv") {
        return false;
      }
      console.log(this.filename, this.pdfConvertable, pdfAvailable);
      if (this.pdfConvertable && pdfAvailable) {
        return true;
      }
      if (this.getIconForType().simpleType === "image" && state.user.preview?.image) {
        return true;
      }
      if (
        this.getIconForType().simpleType === "video" &&
        state.user.preview?.video &&
        mediaAvailable
      ) {
        return true;
      }
      if (
        this.getIconForType().simpleType === "document" &&
        state.user.preview?.office &&
        onlyOfficeUrl != ""
      ) {
        return true;
      }
      return false;
    },
  },
  methods: {
    // NEW: Centralized method to load any image and handle its state
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
      if (this.imageState == "loaded") {
        mutations.setPreviewSource(this.thumbnailUrl);
      }
      if (!state.user.preview.motionVideoPreview || !this.hasMotion) {
        return;
      }

      const sequence = [
        this.thumbnailUrl,
        this.thumbnailUrl + "&atPercentage=25",
        this.thumbnailUrl + "&atPercentage=50",
        this.thumbnailUrl + "&atPercentage=75",
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
        this.previewTimeouts.push(timeoutId);
      };
      updateThumbnail();
    },
    handleMouseLeave() {
      this.previewTimeouts.forEach(clearTimeout);
      this.previewTimeouts = [];
      mutations.setPreviewSource("");
      // UPDATED: Reset to the base thumbnail URL. The watcher will handle reloading it.
      this.imageTargetSrc = this.thumbnailUrl;
    },
    getIconForType() {
      return getTypeInfo(this.mimetype);
    },
  },
  watch: {
    // UPDATED: Added a check for isPreviewImg
    imageTargetSrc: {
      handler(newSrc) {
        // ONLY trigger the image loader if the component is meant to show a preview.
        if (this.isPreviewImg) {
          this.loadImage(newSrc);
        }
      },
      immediate: true, // Run this watcher on component mount
    },
  },
  mounted() {
    const result = this.getIconForType();
    this.classes = result.classes || "material-icons";
    this.color = result.color || "lightgray";
    this.materialIcon = result.materialIcon || "";
    this.svgPath = result.svgPath || "";
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
</style>
