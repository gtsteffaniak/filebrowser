<template>
  <div class="scroll-wrapper" ref="wrapper">
    <div class="scroll-container" ref="content">
      <slot />
    </div>
    <div
      class="custom-scrollbar"
      ref="scrollbar"
      :class="{ ready: isReady, visible: isVisible }"
      @mouseenter="handleMouseEnter"
      @mouseleave="handleMouseLeave"
    >
      <div class="thumb" ref="thumb" @mousedown="startDrag">
        <div class="thumb-letters">{{ this.letter() }}</div>
      </div>
      <div class="thumb-section-id" ref="sectionId">{{ this.category() }}</div>
    </div>
  </div>
</template>

<script>
import { state, mutations } from "@/store";
export default {
  name: "Scrollbar",
  data() {
    return {
      isDragging: false,
      isHovering: false,
      startY: 0,
      startScrollTop: 0,
      scrollTimeout: null,
      isReady: false,
      isVisible: false,
    };
  },
  methods: {
    category() {
      return state.listing.category;
    },
    letter() {
      return state.listing.letter;
    },
    handleMouseEnter() {
      this.isHovering = true;
      this.isVisible = true;
      this.clearHideTimeout();
    },
    handleMouseLeave() {
      this.isHovering = false;
      this.scheduleHide();
    },
    clearHideTimeout() {
      if (this.scrollTimeout) {
        clearTimeout(this.scrollTimeout);
        this.scrollTimeout = null;
      }
    },
    scheduleHide() {
      this.clearHideTimeout();
      this.scrollTimeout = setTimeout(() => {
        if (!this.isDragging && !this.isHovering) {
          this.isVisible = false;
          mutations.updateListing({
            ...state.listing,
            scrolling: false,
          });
        }
      }, 800);
    },
    handleMouseMove(e) {
      if (!this.isReady) return;

      const wrapper = this.$refs.wrapper;
      const bounds = wrapper.getBoundingClientRect();
      const relativeX = e.clientX - bounds.left;

      if (relativeX >= bounds.width - 64) {
        this.isVisible = true;
        this.scheduleHide();
      }
    },
    handleScroll() {
      if (!this.isReady) return;
      const content = this.$refs.content;
      const scrollbar = this.$refs.scrollbar;
      const thumb = this.$refs.thumb;
      const sectionId = this.$refs.sectionId;

      this.isVisible = true;
      this.scheduleHide();

      const scrollRatio =
        content.scrollTop / (content.scrollHeight - content.clientHeight);

      const thumbHeight = thumb.clientHeight;
      const maxThumbTop = scrollbar.clientHeight - thumbHeight;
      const thumbPosition = scrollRatio * maxThumbTop;

      thumb.style.transform = `translateY(${thumbPosition}px)`;
      sectionId.style.transform = `translateY(${thumbPosition}px)`;
      let info = state.listing;
      info.scrolling = true;
      info.scrollRatio = Math.trunc(scrollRatio * 100);
      mutations.updateListing(info);
    },
    startDrag(e) {
      this.isDragging = true;
      this.startY = e.clientY;
      this.startScrollTop = this.$refs.content.scrollTop;
      this.clearHideTimeout();
      document.addEventListener("mousemove", this.onDrag);
      document.addEventListener("mouseup", this.stopDrag);
    },
    onDrag(e) {
      if (!this.isDragging) return;
      const content = this.$refs.content;
      const scrollbar = this.$refs.scrollbar;
      const thumb = this.$refs.thumb;
      const sectionId = this.$refs.sectionId;

      const deltaY = e.clientY - this.startY;
      const scrollableHeight = content.scrollHeight - content.clientHeight;
      const scrollbarHeight = scrollbar.clientHeight - thumb.clientHeight;
      const scrollRatio = scrollableHeight / scrollbarHeight;

      const newScrollTop = this.startScrollTop + deltaY * scrollRatio;
      content.scrollTop = newScrollTop;

      // Also manually move the thumb-section-id in case scroll event hasn't fired
      const scrollRatioNow = newScrollTop / (content.scrollHeight - content.clientHeight);
      const thumbHeight = thumb.clientHeight;
      const maxThumbTop = scrollbar.clientHeight - thumbHeight;
      const thumbPosition = scrollRatioNow * maxThumbTop;

      thumb.style.transform = `translateY(${thumbPosition}px)`;
      sectionId.style.transform = `translateY(${thumbPosition}px)`;
    },

    stopDrag() {
      this.isDragging = false;
      this.scheduleHide();
      document.removeEventListener("mousemove", this.onDrag);
      document.removeEventListener("mouseup", this.stopDrag);
    },
  },
  mounted() {
    setTimeout(() => {
      this.isReady = true;
    }, 100);
    this.$refs.wrapper.addEventListener("mousemove", this.handleMouseMove);
    this.$refs.content.addEventListener("scroll", this.handleScroll, { passive: true });
  },
  beforeDestroy() {
    this.$refs.wrapper.removeEventListener("mousemove", this.handleMouseMove);
    this.$refs.content.removeEventListener("scroll", this.handleScroll);
  },
};
</script>

<style scoped>
.scroll-wrapper {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
}

.scroll-container {
  width: 100%;
  height: 100%;
  overflow-y: scroll;
  scrollbar-width: none;
}

.scroll-container::-webkit-scrollbar {
  display: none;
}

.custom-scrollbar {
  position: absolute;
  top: 0;
  right: -5em;
  display: none;
  width: 2em;
  height: 100%;
  z-index: 1000;
  pointer-events: none;
  transition: right 0.25s ease;
}

.custom-scrollbar.ready {
  display: block;
}

.custom-scrollbar.visible {
  right: 0.25em;
}

.thumb {
  position: absolute;
  top: 0;
  left: 0.25em;
  width: 100%;
  height: 6em;
  background-color: var(--primaryColor, #888);
  border-radius: 1em;
  cursor: pointer;
  pointer-events: auto;
  display: flex;
  justify-content: center;
  align-items: center;
}

.thumb-letters {
  width: 2em;
  height: 2em;
  border-radius: 3em;
  color: #fff;
  font-size: 1em;
  display: flex;
  justify-content: center;
  align-items: center;
  pointer-events: none;
  transition: opacity 0.2s;
}

.thumb-section-id {
  top: 5.5em;
  right: 3em;
  width: 6em;
  height: 2.75em;
  background-color: var(--primaryColor, #444);
  border-radius: 3em;
  color: #fff;
  font-size: 1em;
  display: flex;
  justify-content: center;
  align-items: center;
  pointer-events: none;
  transition: opacity 0.2s;
  border-color: #000;
  border-style: solid;
  position: fixed;
  display: none;
}

.custom-scrollbar.visible .thumb-section-id {
  display: flex;
}
</style>
