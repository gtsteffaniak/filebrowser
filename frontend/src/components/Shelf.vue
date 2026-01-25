<template>
  <transition name="shelf-slide">
    <div
      v-show="!isHidden && showShelf"
      id="shelf"
      :class="{ 'add-padding': addPadding }"
      :style="moveWithSidebar"
    >
      <breadcrumbs v-if="showBreadcrumbs" :base="isShare ? `/share/${shareHash}` : undefined" />
      <listing-header v-if="showListingHeader" :hasDuration="hasDuration" />
      <duplicate-finder-actions 
        v-if="showDuplicateFinderActions"
        :selectedCount="duplicateFinderSelectedCount"
        :deleting="duplicateFinderDeleting"
        @delete="handleDuplicateFinderDelete"
        @clear="handleDuplicateFinderClear"
      />
    </div>
  </transition>
</template>

<script>
import Breadcrumbs from "@/components/files/Breadcrumbs.vue";
import ListingHeader from "@/components/files/ListingHeader.vue";
import DuplicateFinderActions from "@/components/tools/DuplicateFinderActions.vue";
import { state, getters } from "@/store";
import { eventBus } from "@/store/eventBus";

export default {
  name: "Shelf",
  components: {
    Breadcrumbs,
    ListingHeader,
    DuplicateFinderActions,
  },
  data() {
    return {
      isHidden: false,
      lastScrollTop: 0,
      scrollElement: null,
      scrollFrame: null,
      // Duplicate finder state
      duplicateFinderSelectedCount: 0,
      duplicateFinderDeleting: false,
    };
  },
  computed: {
    addPadding() {
      return getters.isStickySidebar() || getters.isShare();
    },
    showShelf() {
      if (state.loading.length > 0) {
        return false;
      }
      return this.showBreadcrumbs || this.showListingHeader || this.showDuplicateFinderActions;
    },
    showBreadcrumbs() {
      return getters.showBreadCrumbs();
    },
    isShare() {
      return getters.isShare();
    },
    shareHash() {
      return getters.shareHash();
    },
    showListingHeader() {
      // Show listing header when in listing view with items
      return getters.currentView() === 'listingView' && state.req?.items?.length > 0;
    },
    hasDuration() {
      // Check if any file has duration metadata
      if (!state.req?.items) return false;
      return state.req.items.some(item => 
        item.type !== 'directory' && item.metadata && item.metadata.duration
      );
    },
    showDuplicateFinderActions() {
      // Show duplicate finder actions when on that route and there are selected items
      return this.duplicateFinderSelectedCount > 0;
    },
    moveWithSidebar() {
      if (getters.isStickySidebar() && getters.isSidebarVisible()) {
        return {
          left: state.sidebar.width + 'em',
        };
      }
      return {};
    },
  },
  mounted() {
    this.$nextTick(() => {
      this.attachScrollListener();
    });
    eventBus.on('duplicateFinderSelectionChanged', this.handleDuplicateFinderSelectionChanged);
    eventBus.on('duplicateFinderDeletingChanged', this.handleDuplicateFinderDeletingChanged);
  },
  beforeUnmount() {
    this.detachScrollListener();
    
    // Clean up event bus listeners
    eventBus.off('duplicateFinderSelectionChanged', this.handleDuplicateFinderSelectionChanged);
    eventBus.off('duplicateFinderDeletingChanged', this.handleDuplicateFinderDeletingChanged);
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
    showShelf(newValue, oldValue) {
      // When shelf content appears (false -> true), always show it regardless of scroll position
      if (newValue === true && oldValue === false) {
        this.isHidden = false;
      }
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
    
    // Duplicate finder event handlers
    handleDuplicateFinderSelectionChanged(count) {
      this.duplicateFinderSelectedCount = count;
    },
    handleDuplicateFinderDeletingChanged(deleting) {
      this.duplicateFinderDeleting = deleting;
    },
    handleDuplicateFinderDelete() {
      eventBus.emit('duplicateFinderDeleteRequested');
    },
    handleDuplicateFinderClear() {
      eventBus.emit('duplicateFinderClearRequested');
    },
  },
};
</script>

<style scoped>
#shelf {

  overflow-y: visible;
  overflow-x: hidden;
  position: fixed;
  padding: 0.5em;
  z-index: 1000;
  right: 0;
  left: 0;
  transition: 0.2s ease;
  box-sizing: border-box;
  height: auto;
  min-height: 0;
  pointer-events: auto;
}

.shelf-slide-enter-active,
.shelf-slide-leave-active {
  transition: transform 0.3s ease, opacity 0.3s ease !important;
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

</style>
