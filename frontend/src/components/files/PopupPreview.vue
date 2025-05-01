<template>
  <div class="popup-preview" v-show="source" ref="popup" :style="popupStyle">
    <img :src="source" alt="Popup image" />
  </div>
</template>

<script>
import { state } from "@/store";

export default {
  name: "PopupPreview",
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
    source(newVal) {
      if (newVal) {
        this.$nextTick(() => {
          this.positionPopup();
        });
      }
    },
  },
  computed: {
    source() {
      return state.popupPreviewSource;
    },
  },
  mounted() {
    window.addEventListener("mousemove", this.updateCursorPosition);
  },
  beforeUnmount() {
    window.removeEventListener("mousemove", this.updateCursorPosition);
  },
  methods: {
    updateCursorPosition(event) {
      this.cursorX = event.clientX;
      this.cursorY = event.clientY;
      if (!state.isMobile) this.positionPopup();
    },
    positionPopup() {
      const popup = this.$refs.popup;
      if (!popup) return;

      if (state.isMobile) {
        // Mobile: center in parent
        this.popupStyle = {
          left: "50%",
          width: "90%",
          transform: "translate(-50%, 10em)",
        };
        return;
      }

      // Desktop behavior
      const { innerWidth, innerHeight } = window;
      const width = popup.offsetWidth;
      const height = popup.offsetHeight;

      const minLeft = 320; // 20em ≈ 320px
      let left = this.cursorX - width / 2;
      let top = this.cursorY;

      // Clamp left
      if (left + width > innerWidth) {
        left = innerWidth - width;
      }
      if (left < minLeft) {
        left = minLeft;
      }

      // Clamp top
      if (top + height > innerHeight) {
        top = innerHeight - height;
      }
      if (top < 0) {
        top = 0;
      }

      this.popupStyle = {
        top: `${top}px`,
        left: `${left}px`,
        transform: "none",
      };
    },
  },
};
</script>

<style scoped>
.popup-preview {
  height: unset !important;
  position: fixed;
  pointer-events: none;
  border-radius: 1em;
  border-style: solid;
  border-width: 0.2em;
  box-shadow: 0 0 0.5em black;
  border-color: var(--primaryColor);
  overflow: hidden;

  max-height: 80vh;
  max-width: 80vw;
  z-index: 1000;
  transition: all 0.3s ease-in-out;
}

.popup-preview img {
  pointer-events: none;
  width: auto;
  height: auto;
  max-width: 100%;
  max-height: 100%;
  display: block;
}
</style>
