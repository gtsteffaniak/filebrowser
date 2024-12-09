<template>
  <span v-if="isMaterialIcon">
    <!-- Material Icon -->
    <i :class="classes" class="icon">
      {{ materialIcon }}
    </i>
  </span>
  <span v-else-if="svgPath">
    <!-- SVG Icon -->
    <svg
      class="icon"
      :class="['svg-icon', customClass]"
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
    >
      <use :href="svgPath" />
    </svg>
  </span>
</template>

<script>
export default {
  name: "Icon",
  props: {
    mimetype: {
      type: String,
      required: true,
    },
    customClass: {
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
  },
  methods: {
    getIconForType(mimeType) {
      if (mimeType === "directory" || mimeType === "application/vnd.google-apps.folder") {
        return {
          classes: "blue-icons material-icons",
          materialIcon: "folder",
        };
      }

      if (mimeType.startsWith("image/")) {
        return {
          classes: "orange-icons material-icons",
          materialIcon: "photo",
        };
      }

      if (
        mimeType.startsWith("audio/") ||
        mimeType === "application/vnd.google-apps.audio"
      ) {
        return {
          classes: "plum-icons material-icons",
          materialIcon: "volume_up",
        };
      }

      if (
        mimeType.startsWith("video/") ||
        mimeType === "application/vnd.google-apps.video"
      ) {
        return {
          classes: "skyblue-icons material-icons",
          materialIcon: "movie",
        };
      }

      if (mimeType.startsWith("font/")) {
        return {
          classes: "gray-icons material-icons",
          materialIcon: "font_download",
        };
      }

      if (
        mimeType === "application/zip" ||
        mimeType === "application/x-7z-compressed" ||
        mimeType === "application/x-bzip" ||
        mimeType === "application/x-rar-compressed" ||
        mimeType === "application/x-tar" ||
        mimeType === "application/gzip"
      ) {
        return {
          classes: "tan-icons material-symbols-outlined",
          materialIcon: "archive",
        };
      }

      if (mimeType === "application/pdf") {
        return {
          classes: "red-icons material-icons",
          materialIcon: "picture_as_pdf",
        };
      }

      if (
        mimeType === "application/msword" ||
        mimeType ===
          "application/vnd.openxmlformats-officedocument.wordprocessingml.document" ||
        mimeType === "application/vnd.google-apps.document" ||
        mimeType === "text/rtf" ||
        mimeType === "application/rtf"
      ) {
        return {
          classes: "deep-blue-icons material-icons",
          materialIcon: "description",
        };
      }

      if (
        mimeType === "application/vnd.ms-excel" ||
        mimeType ===
          "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" ||
        mimeType === "application/vnd.google-apps.spreadsheet" ||
        mimeType === "text/csv"
      ) {
        return {
          classes: "green-icons material-icons",
          materialIcon: "border_all",
        };
      }

      if (
        mimeType === "application/vnd.ms-powerpoint" ||
        mimeType ===
          "application/vnd.openxmlformats-officedocument.presentationml.presentation" ||
        mimeType === "application/vnd.google-apps.presentation"
      ) {
        return {
          classes: "red-orange-icons material-icons",
          materialIcon: "slideshow",
        };
      }

      if (mimeType === "text/plain" || mimeType === "text/markdown") {
        return {
          classes: "beige-icons material-symbols-outlined",
          materialIcon: "description",
        };
      }

      if (mimeType === "application/json" || mimeType === "application/xml") {
        return {
          classes: "yellow-icons material-icons",
          materialIcon: "code",
        };
      }

      if (
        mimeType === "application/octet-stream" ||
        mimeType === "application/x-executable" ||
        mimeType === "application/vnd.google-apps.unknown"
      ) {
        return {
          classes: "gray-icons material-icons",
          materialIcon: "memory",
        };
      }

      if (mimeType === "application/javascript" || mimeType === "text/javascript") {
        return {
          classes: "yellow-icons material-symbols-outlined",
          materialIcon: "javascript",
        };
      }

      if (
        mimeType === "application/x-python" ||
        mimeType === "text/html" ||
        mimeType === "text/css" ||
        mimeType === "application/vnd.google-apps.sites"
      ) {
        return {
          classes: "gray-icons material-symbols-outlined",
          materialIcon: "code_blocks",
        };
      }

      if (
        mimeType === "application/x-disk-image" ||
        mimeType === "application/x-iso-image" ||
        mimeType === "application/x-apple-diskimage"
      ) {
        return {
          classes: "gray-icons material-symbols-outlined",
          materialIcon: "deployed_code",
        };
      }

      if (mimeType === "invalid_link") {
        return {
          classes: "lightgray-icons material-icons",
          materialIcon: "link_off",
        };
      }

      // Default fallback
      return {
        classes: "lightgray-icons material-symbols-outlined",
        materialIcon: "draft",
      };
    },
  },
  mounted() {
    const result = this.getIconForType(this.mimetype);
    this.classes = result.classes || "material-icons"; // Default class
    this.color = result.color || "lightgray"; // Default color
    this.materialIcon = result.materialIcon || "";
    this.svgPath = result.svgUrl || ""; // For SVG file paths
  },
};
</script>

<style scoped>
.file-icons [aria-label^="."] {
  opacity: 0.33;
}

.file-icons [aria-label$=".bak"] {
  opacity: 0.33;
}

.icon {
  font-size: 1.5rem;
  /* Default size */
  fill: currentColor;
  /* Uses inherited color */
}

.purple-icons {
  color: purple;
}

/* Icon Colors */
.blue-icons {
  color: var(--icon-blue);
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

.beige-icons {
  color: beige;
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
  color: lightgray;
}
.yellow-icons {
  color: yellow;
}
</style>
