<template>
  <div class="scroll-wrapper" :class="{ 'halloween-theme': eventTheme === 'halloween' }" ref="wrapper">
    <slot />
    <div
      class="custom-scrollbar"
      ref="scrollbar"
      :class="{ ready: isReady, visible: isVisible && isScrollable }"
      @mouseenter="handleMouseEnter"
      @mouseleave="handleMouseLeave"
    >
      <div
        class="thumb no-select"
        ref="thumb"
        :class="{ ready: isReady, visible: isVisible && isScrollable }"
        @mousedown="startDrag"
        @touchstart.prevent="startDrag"
      >
        <div v-if="isNotListing" class="thumb-letters">
          <hr />
        </div>
        <div v-else class="thumb-letters no-select">
          <i class="material-icons" :class="{ 'primary-icons': isFolder }"> {{ isFolder ? "folder" : "description" }} </i> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        </div>
      </div>
      <div
        :class="{ hidden: isNotListing }"
        class="thumb-section-id no-select"
        ref="sectionId"
      >
        {{ this.letter() }}
      </div>
    </div>
  </div>
</template>

<script>
import { state, mutations, getters } from "@/store";

const offsetFromBottom = 75;

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
  computed: {
    eventTheme() {
      return getters.eventTheme();
    },
    isScrollable() {
      return getters.isScrollable();
    },
    isNotListing() {
      return getters.currentView() != "listingView";
    },
    isFolder() {
      return this.category() === "folders";
    },
  },
  methods: {
    handleResize() {
      if (!this.isReady) return;
      // Force scroll event to re-compute thumb position
      const content = this.$refs.wrapper;
      this.updateThumbPosition(content.scrollTop);
    },
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
          mutations.updateListing({ ...state.listing, scrolling: false });
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
    updateThumbPosition(scrollTop) {
      const content = this.$refs.wrapper;
      const scrollbar = this.$refs.scrollbar;
      const thumb = this.$refs.thumb;
      const sectionId = this.$refs.sectionId;

      const scrollRatio = scrollTop / (content.scrollHeight - content.clientHeight);
      const thumbHeight = thumb.clientHeight;
      const maxThumbTop = scrollbar.clientHeight - thumbHeight - offsetFromBottom;
      const thumbPosition = scrollRatio * maxThumbTop;

      thumb.style.transform = `translateY(${thumbPosition}px)`;
      sectionId.style.transform = `translateY(${thumbPosition}px)`;
    },
    handleScroll() {
      if (!this.isReady) return;
      const content = this.$refs.wrapper;
      this.isVisible = true;
      this.scheduleHide();
      mutations.setPreviewSource("");
      this.updateThumbPosition(content.scrollTop);
      mutations.updateListing({
        ...state.listing,
        scrolling: true,
        scrollRatio: Math.trunc(
          (content.scrollTop / (content.scrollHeight - content.clientHeight)) * 100
        ),
      });
    },
    startDrag(e) {
      const clientY = e.touches ? e.touches[0].clientY : e.clientY;
      this.isDragging = true;
      this.startY = clientY;
      this.startScrollTop = this.$refs.wrapper.scrollTop;
      this.clearHideTimeout();

      document.addEventListener("mousemove", this.onDrag);
      document.addEventListener("mouseup", this.stopDrag);
      document.addEventListener("touchmove", this.onDrag, { passive: false });
      document.addEventListener("touchend", this.stopDrag);
    },
    onDrag(e) {
      if (!this.isDragging) return;
      const clientY = e.touches ? e.touches[0].clientY : e.clientY;
      const content = this.$refs.wrapper;
      const scrollbar = this.$refs.scrollbar;
      const thumb = this.$refs.thumb;

      const deltaY = clientY - this.startY;
      const scrollableHeight = content.scrollHeight - content.clientHeight;
      const scrollbarHeight =
        scrollbar.clientHeight - thumb.clientHeight - offsetFromBottom;
      const scrollRatio = scrollableHeight / scrollbarHeight;

      let newScrollTop = this.startScrollTop + deltaY * scrollRatio;

      // Clamp scrollTop within bounds
      newScrollTop = Math.max(0, Math.min(newScrollTop, scrollableHeight));
      content.scrollTop = newScrollTop;

      this.updateThumbPosition(newScrollTop);
    },
    stopDrag() {
      this.isDragging = false;
      this.scheduleHide();
      document.removeEventListener("mousemove", this.onDrag);
      document.removeEventListener("mouseup", this.stopDrag);
      document.removeEventListener("touchmove", this.onDrag);
      document.removeEventListener("touchend", this.stopDrag);
    },
  },
  mounted() {
    setTimeout(() => {
      this.isReady = true;
    }, 100);
    this.$refs.wrapper.addEventListener("mousemove", this.handleMouseMove);
    this.$refs.wrapper.addEventListener("scroll", this.handleScroll, { passive: true });
    window.addEventListener("resize", this.handleResize);
  },
  beforeUnmount() {
    window.removeEventListener("resize", this.handleResize);
    this.$refs.wrapper.removeEventListener("mousemove", this.handleMouseMove);
    this.$refs.wrapper.removeEventListener("scroll", this.handleScroll);
  },
};
</script>

<style scoped>
.scroll-wrapper {
  position: relative;
  width: 100%;
  height: 100%;
}

.custom-scrollbar {
  position: absolute;
  top: 0;
  right: 0;
  height: 100%;
  display: none;
  width: 2em;
  z-index: 1000;
  pointer-events: none;
  transition: right 0.25s ease;
}

.custom-scrollbar.ready {
  display: block;
}

.thumb.ready {
  display: flex;
}

.thumb.visible {
  right: 0.25em;
}

.thumb {
  right: -5em;
  /* <- Start hidden */
  display: none;
  border-style: solid;
  border-color: var(--background);
  position: fixed;
  top: 4em;
  height: 6em;
  background-color: var(--alt-background);
  border-radius: 1em;
  cursor: pointer;
  pointer-events: auto;
  justify-content: center;
  align-items: center;
  transition: right 0.25s ease, opacity 0.2s;
}

@supports (backdrop-filter: none) {
  .thumb,
  .thumb-section-id {
    background-color: rgba(237, 237, 237, 0.1) !important;
    backdrop-filter: blur(10px) invert(0.1);
  }
}

.thumb-letters {
  width: 2em;
  height: 2em;
  border-radius: 3em;
  color: var(--textPrimary);
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
  width: 3em;
  height: 2.75em;
  background-color: var(--alt-background);
  border-radius: 3em;
  border-style: solid;
  border-color: var(--background);
  font-size: 1em;
  justify-content: center;
  align-items: center;
  pointer-events: none;
  transition: opacity 0.2s;
  position: fixed;
  display: none;
}

.custom-scrollbar.visible .thumb-section-id {
  display: flex;
}
</style>
