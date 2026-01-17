<template>
  <transition name="shelf-slide">
    <div
      v-show="!isHidden"
      id="shelf"
      :class="{ 'add-padding': addPadding }"
    >
      <slot></slot>
    </div>
  </transition>
  <div class="shelf-placeholder"></div>
</template>

<script>
import { getters } from "@/store";

export default {
  name: "Shelf",
  data() {
    return {
      isHidden: false,
      lastScrollTop: 0,
      scrollElement: null,
      scrollFrame: null,
    };
  },
  computed: {
    addPadding() {
      return getters.isStickySidebar() || getters.isShare();
    },
  },
  mounted() {
    this.$nextTick(() => {
      this.attachScrollListener();
    });
  },
  beforeUnmount() {
    this.detachScrollListener();
  },
  watch: {
    $route() {
      this.isHidden = false;
      this.lastScrollTop = 0;
      this.$nextTick(() => {
        this.detachScrollListener();
        this.attachScrollListener();
      });
    },
  },
  methods: {
    attachScrollListener() {
      // Try to find the scroll wrapper (used by Scrollbar component)
      const scrollWrapper = document.querySelector('.scroll-wrapper');
      if (scrollWrapper) {
        scrollWrapper.addEventListener('scroll', this.handleScroll, { passive: true });
        this.scrollElement = scrollWrapper;
      }
    },

    detachScrollListener() {
      if (this.scrollElement) {
        this.scrollElement.removeEventListener('scroll', this.handleScroll);
        this.scrollElement = null;
      }
      // Cancel any pending animation frame
      if (this.scrollFrame) {
        cancelAnimationFrame(this.scrollFrame);
        this.scrollFrame = null;
      }
    },

    handleScroll(event) {
      // Use requestAnimationFrame to throttle updates (like Scrollbar component)
      if (this.scrollFrame) return;
      
      this.scrollFrame = requestAnimationFrame(() => {
        const scrollTop = event.target.scrollTop;
        
        // Always show when at the top
        if (scrollTop <= 10) {
          this.isHidden = false;
          this.lastScrollTop = scrollTop;
          this.scrollFrame = null;
          return;
        }

        // Calculate scroll difference
        const scrollDiff = scrollTop - this.lastScrollTop;

        // Only trigger if scrolled enough (at least 30px)
        if (Math.abs(scrollDiff) < 30) {
          this.scrollFrame = null;
          return;
        }

        // Hide when scrolling down, show when scrolling up
        if (scrollDiff > 0) {
          // Scrolling down
          this.isHidden = true;
        } else {
          // Scrolling up
          this.isHidden = false;
        }

        this.lastScrollTop = scrollTop;
        this.scrollFrame = null;
      });
    },
  },
};
</script>

<style scoped>
#shelf {
  overflow-y: hidden;
  overflow-x: hidden;
  position: fixed;
  z-index: 1000;
  right: 0;
  left: 0;
  transition: 0.3s ease;
  box-sizing: border-box;
}

.shelf-slide-enter-active,
.shelf-slide-leave-active {
  transition: transform 0.3s ease, opacity 0.3s ease;
}

.shelf-slide-enter-from,
.shelf-slide-leave-to {
  transform: translateY(-100%);
  opacity: 0;
}

.shelf-slide-enter-to,
.shelf-slide-leave-from {
  transform: translateY(0);
  opacity: 1;
}

/* Backdrop-filter support */
@supports (backdrop-filter: none) {
  #shelf {
    backdrop-filter: blur(12px) invert(0.01);
    background-color: color-mix(in srgb, var(--background) 75%, transparent);
  }
}

.shelf-placeholder {
  margin-top: 0.35em;
  visibility: hidden;
  min-height: 3em;
}

#main.moveWithSidebar #shelf {
  padding-left: 20.5em;
}
</style>
