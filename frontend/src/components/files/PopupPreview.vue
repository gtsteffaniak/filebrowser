<template>
  <div class="popup-preview" v-if="source" ref="popup" :style="popupStyle">
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
    },
    positionPopup() {
      const popup = this.$refs.popup;
      if (!popup) return;

      const { innerWidth } = window;
      const width = popup.offsetWidth;

      let left = this.cursorX - width / 2;

      // Apply 100px shift if cursor is in the left half
      if ((this.cursorX < innerWidth / 2) && !state.isMobile) {
        left += 120;
      }

      // Clamp to viewport
      if (left + width > innerWidth) {
        left = innerWidth - width;
      }

      if (left < 0) left = 0;

      this.popupStyle = {
        position: "fixed",
        top: this.cursorY + "px",
        left: `${left}px`,
      };
    },
  },
};
</script>

<style scoped>
.popup-preview {
  pointer-events: none;

  position: fixed;
  border-radius: 1em;
  max-height: 60vh;
  max-width: 50vw;
  transition: all 0.3s ease-in-out;
  z-index: 1000;
}

.popup-preview img {
  pointer-events: none;

  width: 100%;
  height: auto;
  display: block;
  border-radius: 5px;
}
</style>
