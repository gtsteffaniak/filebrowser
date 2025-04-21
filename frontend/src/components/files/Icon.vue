<template>
  <span v-if="isPreviewImg">
    <img
      @mouseenter="handleMouseEnter($event)"
      @mouseleave="handleMouseLeave($event)"
      v-lazy="thumbnailUrl"
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
import { mutations,state } from "@/store";
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
    };
  },
  computed: {
    isMaterialIcon() {
      return this.materialIcon !== "";
    },
    isPreviewImg() {
      // todo support webp previews
      if (this.mimetype == "text/csv") {
        return false;
      }
      if (this.getIconForType().simpleType === "image" && state.user.preview?.image) {
        return true;
      }
      if (this.getIconForType().simpleType === "video" && state.user.preview?.video && mediaAvailable) {
        return true;
      }
      if (this.getIconForType().simpleType === "document" && state.user.preview?.office && onlyOfficeUrl != "") {
        return true;
      }
      if (this.getIconForType().simpleType === "pdf" && state.user.preview?.office && onlyOfficeUrl != "") {
        return true;
      }
      return false;
    },
  },
  methods: {
    handleMouseEnter() {
      mutations.setPreviewSource(this.thumbnailUrl);
    },
    handleMouseLeave() {
      mutations.setPreviewSource("");
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
