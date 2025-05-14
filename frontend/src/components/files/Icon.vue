<template>
  <span v-if="isPreviewImg">
    <i
      v-if="hasMotion"
      class="material-icons"
      :class="{ larger: showLarger, smaller: !showLarger }"
      >animation</i
    >
    <img
      @mouseenter="handleMouseEnter($event)"
      @mouseleave="handleMouseLeave($event)"
      v-lazy="thumbnailUrl"
      :src="currentThumbnail"
      class="icon"
      ref="thumbnail"
    />
  </span>
  <span v-else>
    <!-- Material Icon -->
    <i :class="[classes, { active: active }]" class="icon"> {{ materialIcon }} </i>
  </span>
</template>

<script>
import { onlyOfficeUrl, mediaAvailable } from "@/utils/constants";
import { getTypeInfo } from "@/utils/mimetype";
import { mutations, state } from "@/store";
export default {
  name: "Icon",
  props: {
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
      previewTimeouts: [], // Store timeout IDs
      currentThumbnail: this.thumbnailUrl,
    };
  },
  computed: {
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
      // todo support webp previews
      if (this.mimetype == "text/csv") {
        return false;
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
      if (
        this.getIconForType().simpleType === "pdf" &&
        state.user.preview?.office &&
        onlyOfficeUrl != ""
      ) {
        return true;
      }
      return false;
    },
  },
  methods: {
    handleMouseEnter() {
      if (state.user.viewMode === "gallery" && !state.user.preview.highQuality) {
        return;
      }

      mutations.setPreviewSource(this.thumbnailUrl);

      if (!state.user.preview.motionVideoPreview) {
        return;
      }

      const sequence = [
        this.thumbnailUrl,
        this.thumbnailUrl + "&atPercentage=25",
        this.thumbnailUrl + "&atPercentage=50",
        this.thumbnailUrl + "&atPercentage=75",
      ];

      let index = 0;

      const updateThumbnailUrl = () => {
        if (state.popupPreviewSource === "") {
          this.previewTimeouts.forEach(clearTimeout);
          this.previewTimeouts = [];
          return;
        }

        const currentUrl = sequence[index];

        const img = new Image();
        img.onload = () => {
          // Set the thumbnail or popup preview
          if (state.user.preview.popup) {
            mutations.setPreviewSource(currentUrl);
          } else {
            this.currentThumbnail = currentUrl;
          }

          // Preload the next image if it exists
          const nextIndex = (index + 1) % sequence.length;
          const nextUrl = sequence[nextIndex];
          const preloadImg = new Image();
          preloadImg.src = nextUrl;

          // Schedule next update
          index = nextIndex;
          const timeoutId = setTimeout(updateThumbnailUrl, 750);
          this.previewTimeouts.push(timeoutId);
        };

        img.src = currentUrl;
      };
      updateThumbnailUrl();
    },
    handleMouseLeave() {
      this.previewTimeouts.forEach(clearTimeout);
      this.previewTimeouts = [];
      mutations.setPreviewSource("");
      this.currentThumbnail = this.thumbnailUrl;
    },
    getIconForType() {
      return getTypeInfo(this.mimetype);
    },
  },
  mounted() {
    const result = this.getIconForType();
    this.classes = result.classes || "material-icons"; // Default class
    this.color = result.color || "lightgray"; // Default color
    this.materialIcon = result.materialIcon || "";
    this.svgPath = result.svgPath || ""; // For SVG file paths
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
