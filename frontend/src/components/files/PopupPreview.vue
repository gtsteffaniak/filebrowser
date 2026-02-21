<template>
  <div class="popup-preview" v-show="sourceInfo" ref="popup" :style="popupStyle"
    :class="{ 'popup-preview--3d': sourceInfo && sourceInfo.type === '3d' }">
    <div v-if="sourceInfo && sourceInfo.type === '3d' && sourceInfo.fbdata" class="popup-preview__3d">
      <ThreeJs
        :key="sourceInfo.path"
        :fbdata="sourceInfo.fbdata"
        :is-thumbnail="true"
      />
    </div>
    <img v-else-if="sourceInfo && sourceInfo.url" :src="sourceInfo.url" alt="Popup image" @load="onImageLoad" />
  </div>
</template>

<script>
import { state, getters } from "@/store";
import { setImageLoaded } from "@/utils/imageCache";
import ThreeJs from "@/views/files/ThreeJs.vue";

export default {
  name: "PopupPreview",
  components: { ThreeJs },
  data() {
    return {
      popupStyle: {
        top: "0px",
        left: "0px",
      },
      cursorX: 0,
      cursorY: 0,
    };
  },
  watch: {
    sourceInfo(newVal) {
      if (newVal) {
        this.$nextTick(() => {
          this.positionPopup();
        });
      }
    },
  },
  computed: {
    sourceInfo() {
      return state.popupPreviewSourceInfo;
    },
  },
  mounted() {
    window.addEventListener("mousemove", this.updateCursorPosition);
  },
  beforeUnmount() {
    window.removeEventListener("mousemove", this.updateCursorPosition);
  },
  methods: {
    onImageLoad() {
      if (!this.sourceInfo || this.sourceInfo.type === "3d") return;
      const { source, path, size, url, modified } = this.sourceInfo;
      setImageLoaded(source, path, size, modified, url);
    },
    updateCursorPosition(event) {
      this.cursorX = event.clientX;
      this.cursorY = event.clientY;
      if (!state.isMobile) this.positionPopup();
    },
    positionPopup() {
      if (!this.sourceInfo) return;
      const popup = this.$refs.popup;
      if (!popup) return;

      const { innerWidth, innerHeight } = window;
      // 3D popup has fixed dimensions; img popup sizes to content
      const size3d = 512;
      const width = popup.offsetWidth || (this.sourceInfo.type === "3d" ? size3d : 0);
      const height = popup.offsetHeight || (this.sourceInfo.type === "3d" ? size3d : 0);
      const padding = 10;

      const minLeft = getters.isSidebarVisible() ? 320 : padding;
      const minTop = padding + 100;

      if (state.isMobile) {
        this.popupStyle = {
          top: `${Math.min(this.cursorY + padding, innerHeight - padding - 200)}px`,
          left: "5%",
          right: "5%",
          margin: "0 auto",
          "max-width": "90vw",
          "max-height": "75vh",
          transform: "translate(0, 4em)",
        };
        return;
      }

      // Position near cursor (prefer center horizontally)
      let left = this.cursorX - width / 2;
      left = Math.max(minLeft, Math.min(left, innerWidth - width));

      // Prefer below or above cursor based on Y position
      let top;
      const isBottomHalf = this.cursorY > innerHeight / 2;

      if (isBottomHalf) {
        // Place above
        top = this.cursorY - height - padding;
        top = Math.max(minTop, top);
      } else {
        // Place below
        top = this.cursorY + padding;
        if (top + height > innerHeight) {
          top = innerHeight - height;
          top = Math.max(minTop, top); // Enforce minTop again
        }
      }

      const maxW = Math.min(innerWidth - minLeft - padding, innerWidth * 0.5);
      const maxH = innerHeight - minTop - padding;
      this.popupStyle = {
        top: `${top}px`,
        left: `${left}px`,
        "max-width": `${maxW}px`,
        "max-height": `${maxH}px`,
        transform: "none",
      };
    },
  },
};
</script>

<style scoped>
.popup-preview {
  position: fixed;
  pointer-events: none;
  border-radius: 1em;
  border-style: solid;
  border-width: 0.2em;
  box-shadow: 0 0 0.5em black;
  border-color: var(--primaryColor);
  overflow: hidden;
  z-index: 1000;
  transition: all 0.3s ease-in-out;
  background: gray;
  display: flex;
  align-items: center;
  justify-content: center;
  /* Fallback if JS hasn't set max dimensions yet */
  max-width: 50vw;
  max-height: 85vh;
}

.popup-preview--3d .popup-preview__3d {
  width: 512px;
  height: 512px;
  min-width: 512px;
  min-height: 512px;
}

.popup-preview__3d :deep(.threejs-viewer) {
  width: 100%;
  height: 100%;
  min-width: 512px;
  min-height: 512px;
}

.popup-preview img {
  pointer-events: none;
  max-width: 100%;
  max-height: 100%;
  width: auto;
  height: auto;
  object-fit: contain;
  display: block;
}
</style>
