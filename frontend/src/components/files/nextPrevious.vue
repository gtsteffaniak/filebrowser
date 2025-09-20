<template>
  <!-- Left edge detection zone -->
  <div
    v-if="enabled && hasPrevious"
    class="nav-zone nav-zone-left"
    :class="{ moveWithSidebar: moveWithSidebar }"
    @mousemove="toggleNavigation"
    @touchstart="(e) => { handleTouchStart(e); toggleNavigation(e); }"
    @touchmove="handleTouchMove"
    @click="handleClick"
  ></div>

  <!-- Right edge detection zone -->
  <div
    v-if="enabled && hasNext"
    class="nav-zone nav-zone-right"
    @mousemove="toggleNavigation"
    @touchstart="(e) => { handleTouchStart(e); toggleNavigation(e); }"
    @touchmove="handleTouchMove"
    @click="handleClick"
  ></div>

  <!-- Previous button -->
  <button
    v-if="enabled && hasPrevious"
    @click.stop="handlePrevClick"
    @mousedown="startDrag($event, 'previous')"
    @touchstart="handleTouchStart($event, 'previous')"
    @touchmove="handleButtonTouchMove"
    @touchend="handleTouchEnd"
    @mouseover="setHoverNav(true)"
    @mouseleave="setHoverNav(false)"
    class="nav-button nav-previous"
    :class="{
      moveWithSidebar: moveWithSidebar,
      hidden: !showNav,
      disabled: !hasPrevious,
      dragging: dragState.type === 'previous',
      active: dragState.atFullExtent && dragState.type === 'previous',
      'dark-mode': isDarkMode,
  }"
    :style="dragState.type === 'previous' ? { transform: `translateY(-50%) translate(${dragState.deltaX}px, 0)` } : {}"
    :aria-label="$t('buttons.previous')"
    :title="$t('buttons.previous')"
  >
    <i class="material-icons">
      {{ dragState.type === 'previous' && dragState.atFullExtent ? 'list_alt' : 'chevron_left' }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
    </i>
  </button>

  <!-- Next button -->
  <button
    v-if="enabled && hasNext"
    @click.stop="handleNextClick"
    @mousedown="startDrag($event, 'next')"
    @touchstart="handleTouchStart($event, 'next')"
    @touchmove="handleButtonTouchMove"
    @touchend="handleTouchEnd"
    @mouseover="setHoverNav(true)"
    @mouseleave="setHoverNav(false)"
    class="nav-button nav-next"
    :class="{ hidden: !showNav, dragging: dragState.type === 'next', active: dragState.atFullExtent && dragState.type === 'next','dark-mode': isDarkMode}"
    :style="dragState.type === 'next' ? { transform: `translateY(-50%) translate(${dragState.deltaX}px, 0)` } : {}"
    :aria-label="$t('buttons.next')"
    :title="$t('buttons.next')"
  >
    <i class="material-icons">
      {{ dragState.type === 'next' && dragState.atFullExtent ? 'list_alt' : 'chevron_right' }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
    </i>
  </button>

  <!-- Prefetch links for better performance -->
  <link v-if="previousRaw" rel="prefetch" :href="previousRaw" />
  <link v-if="nextRaw" rel="prefetch" :href="nextRaw" />
</template>

<script>
import { state, getters, mutations } from "@/store";
import throttle from "@/utils/throttle";
import { previewViews } from "@/utils/constants";
import { url } from "@/utils";
import { filesApi, publicApi } from "@/api";

export default {
  name: "NextPrevious",
  data() {
    return {
      hoverNav: false,
      dragState: {
        isDragging: false,
        type: null, // 'previous' or 'next'
        startX: 0,
        startY: 0,
        deltaX: 0,
        deltaY: 0,
        threshold: 0, // Will be calculated as 10em in pixels
        atFullExtent: false,
        triggered: false,
      },
      // State tracking
      navigationTimeout: null,
      isSwipe: false,
      touchStartX: 0,
      touchStartY: 0,
      // Button touch handling
      touchState: {
        isButtonTouch: false,
        buttonType: null,
        startTime: 0,
        hasMoved: false,
        tapTimeout: null,
        triggered: false
      },
    };
  },
  computed: {
    isDarkMode() { return getters.isDarkMode(); },
    moveWithSidebar() {
      return getters.isSidebarVisible() && getters.isStickySidebar();
    },
    enabled() {
      return state.navigation.enabled;
    },
    showNav() {
      const shouldShow = state.navigation.show || this.hoverNav;
      return shouldShow;
    },
    hasPrevious() {
      const has = state.navigation.previousLink !== "";
      return has;
    },
    hasNext() {
      const has = state.navigation.nextLink !== "";
      return has;
    },
    previousRaw() {
      return state.navigation.previousRaw;
    },
    nextRaw() {
      return state.navigation.nextRaw;
    },
    currentView() {
      const view = getters.currentView();
      return view;
    }
  },
  watch: {
    currentView() {
      this.updateNavigationEnabled();

      // Also trigger navigation setup if we're now in a preview view
      this.$nextTick(() => {
        if (this.enabled && state.req) {
          this.setupNavigationForCurrentItem();
        }
      });
    },
    'state.req': {
      handler() {
        this.updateNavigationEnabled();
        // Auto-setup navigation when request changes and we're enabled
        if (this.enabled) {
          this.$nextTick(() => {
            this.setupNavigationForCurrentItem();
          });
        }
      },
      deep: true,
      immediate: false
    },
    enabled(newEnabled) {
      if (newEnabled && state.req) {
        this.$nextTick(() => {
          this.setupNavigationForCurrentItem();
        });
      }
    },
    '$route'() {
      // Give time for state.req to be updated, then setup navigation
      setTimeout(() => {
        this.$nextTick(() => {
          if (this.enabled && state.req) {
            this.setupNavigationForCurrentItem();
          }
        });
      }, 100);
    },
    // Watch for when navigation links are set up
    'state.navigation.previousLink'() {
      this.showInitialNavigation();
    },
    'state.navigation.nextLink'() {
      this.showInitialNavigation();
    },
  },
  mounted() {
    window.addEventListener("keydown", this.keyEvent);
    window.addEventListener("mousemove", this.handleDrag);
    window.addEventListener("mouseup", this.endDrag);
    window.addEventListener("touchmove", this.handleDrag, { passive: false });
    window.addEventListener("touchend", this.endDrag);

    // Calculate 10em threshold in pixels
    const emSize = parseFloat(getComputedStyle(document.documentElement).fontSize);
    this.dragState.threshold = 10 * emSize;

    this.updateNavigationEnabled();

    // Setup navigation if enabled and we have a current item
    if (this.enabled && state.req) {
      this.$nextTick(() => {
        this.setupNavigationForCurrentItem();
      });
    } else {
      this.$nextTick(() => {
        this.showInitialNavigation();
      });
    }
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
    window.removeEventListener("mousemove", this.handleDrag);
    window.removeEventListener("mouseup", this.endDrag);
    window.removeEventListener("touchmove", this.handleDrag);
    window.removeEventListener("touchend", this.endDrag);

    // Clear our local timeout
    if (this.navigationTimeout) {
      clearTimeout(this.navigationTimeout);
      this.navigationTimeout = null;
    }

    // Clean up touch state
    this.resetTouchState();

    mutations.clearNavigation();
  },
  methods: {
    updateNavigationEnabled() {
      const shouldEnable = previewViews.includes(this.currentView);
      mutations.setNavigationEnabled(shouldEnable);
    },
    async setupNavigationForCurrentItem() {
      if (!this.enabled || !state.req || state.req.type === 'directory') {
        // Clear navigation when not applicable
        mutations.clearNavigation();
        return;
      }

      const directoryPath = url.removeLastDir(state.req.path);
      let listing = null;

      // Try to get listing from current request first
      if (state.req.items) {
        listing = state.req.items;
      } else {
        // Fetch the directory listing
        try {
          let res;
          if (getters.isShare()) {
            res = await publicApi.fetchPub(directoryPath, state.share.hash);
          } else {
            res = await filesApi.fetchFiles(state.req.source, directoryPath);
          }
          listing = res.items;
        } catch (error) {
          listing = [state.req]; // Fallback to current item only
        }
      }

      mutations.setupNavigation({
        listing: listing,
        currentItem: state.req,
        directoryPath: directoryPath
      });
    },
    showInitialNavigation() {
      // Show navigation initially for 3 seconds when navigation is set up
      if (this.enabled && (this.hasPrevious || this.hasNext)) {
        mutations.setNavigationShow(true);

        // Clear any existing timeout
        if (this.navigationTimeout) {
          clearTimeout(this.navigationTimeout);
          this.navigationTimeout = null;
        }

        this.navigationTimeout = setTimeout(() => {
          if (!this.hoverNav) {
            mutations.setNavigationShow(false);
          }
          this.navigationTimeout = null;
        }, 3000);
      }
    },
    prev() {
      if (this.hasPrevious) {
        this.hoverNav = false;
        this.$router.replace({ path: state.navigation.previousLink });
      }
    },
    next() {
      if (this.hasNext) {
        this.hoverNav = false;
        this.$router.replace({ path: state.navigation.nextLink });
      }
    },
    keyEvent(event) {
      // Only handle navigation if enabled and no prompt is active
      if (!this.enabled || state.prompts.length > 0) {
        return;
      }

     // Check if any media element is currently playing
     const mediaElements = document.querySelectorAll('audio, video');
     let mediaActive = false;
  
     mediaElements.forEach(media => {
       if (!media.paused || 
           document.activeElement === media  || 
           document.activeElement.closest('.plyr__controls')) {
         mediaActive = true;
       }
     });
  
     // If media is playing don't handle arrow keys and let use fastfoward and rewind of the player
     if (mediaActive) {
       return;
     }

      const { key } = event;

      switch (key) {
        case "ArrowRight":
          if (this.hasNext) {
            event.preventDefault();
            this.next();
          }
          break;
        case "ArrowLeft":
          if (this.hasPrevious) {
            event.preventDefault();
            this.prev();
          }
          break;
      }
    },
    handleClick() {
      // Don't show navigation if this is part of a swipe gesture
      if (this.isSwipe) {
        return;
      }

      // Simplified: clicking anywhere in the CSS zones shows navigation
      if (!this.enabled || (!this.hasPrevious && !this.hasNext)) {
        return;
      }

      this.showNavigation();
    },
    showNavigation() {
      mutations.setNavigationShow(true);
      mutations.clearNavigationTimeout();

      // Clear our local timeout too
      if (this.navigationTimeout) {
        clearTimeout(this.navigationTimeout);
        this.navigationTimeout = null;
      }

      this.navigationTimeout = setTimeout(() => {
        if (!this.hoverNav) {
          mutations.setNavigationShow(false);
        }
        mutations.clearNavigationTimeout();
        this.navigationTimeout = null;
      }, 3000); // Show for 3 seconds

      mutations.setNavigationTimeout(this.navigationTimeout);
    },
    toggleNavigation: throttle(function () {
      if (!this.enabled) {
        return;
      }
      if (this.isSwipe) {
        return;
      }
      this.showNavigation();
    }, 100),
    setHoverNav(value) {
      this.hoverNav = value;
      mutations.setNavigationHover(value);
    },

    // Touch handling and swipe detection (similar to ListingItem.vue)
    handleTouchStart(event, buttonType = null) {
      if (event.touches && event.touches.length > 0) {
        const touch = event.touches[0];
        this.touchStartX = touch.clientX;
        this.touchStartY = touch.clientY;
        this.isSwipe = false;

        // Handle button-specific touch
        if (buttonType) {
          this.touchState = {
            isButtonTouch: true,
            buttonType: buttonType,
            startTime: Date.now(),
            hasMoved: false,
            tapTimeout: null,
            triggered: false
          };
          
          // Set a longer timeout to allow for drag intent detection
          this.touchState.tapTimeout = setTimeout(() => {
            // If we haven't moved significantly and not dragging, treat as tap
            if (!this.touchState.hasMoved && !this.dragState.isDragging) {
              this.handleButtonTap(buttonType);
            }
          }, 300); // Increased to 300ms to allow for drag detection
        }
      }
    },

    handleTouchMove(event) {
      if (!event.touches || event.touches.length === 0) return;

      const touch = event.touches[0];
      const deltaX = Math.abs(touch.clientX - this.touchStartX);
      const deltaY = Math.abs(touch.clientY - this.touchStartY);
      const movementThreshold = 10;

      if (deltaX > movementThreshold || deltaY > movementThreshold) {
        this.isSwipe = true;
        this.cancelNavigationTimeout();
      }
    },

    // Handle touch movement specifically for navigation buttons
    handleButtonTouchMove(event) {
      if (!event.touches || event.touches.length === 0) return;
      if (!this.touchState.isButtonTouch) return;

      event.preventDefault(); // Prevent scrolling while dragging

      const touch = event.touches[0];
      const deltaX = Math.abs(touch.clientX - this.touchStartX);
      const deltaY = Math.abs(touch.clientY - this.touchStartY);
      const movementThreshold = 10;

      // Check if user has moved enough to start dragging
      if (deltaX > movementThreshold || deltaY > movementThreshold) {
        this.touchState.hasMoved = true;
        
        // Cancel tap timeout since user is dragging
        if (this.touchState.tapTimeout) {
          clearTimeout(this.touchState.tapTimeout);
          this.touchState.tapTimeout = null;
        }
        
        // Initialize drag state if not already dragging
        if (!this.dragState.isDragging) {
          // Calculate 10em threshold in pixels if not set
          if (!this.dragState.threshold) {
            const emSize = parseFloat(getComputedStyle(document.documentElement).fontSize);
            this.dragState.threshold = 10 * emSize;
          }
          
          this.dragState = {
            isDragging: true,
            type: this.touchState.buttonType,
            startX: this.touchStartX,
            startY: this.touchStartY,
            deltaX: 0,
            deltaY: 0,
            threshold: this.dragState.threshold,
            atFullExtent: false,
            triggered: false,
          };
        }
        
        // Update drag position - implement drag logic directly
        if (this.dragState.isDragging) {
          let dragDeltaX = touch.clientX - this.dragState.startX;
          const maxDrag = this.dragState.threshold; // 10em
          
          // Constrain drag to correct direction and max distance
          if (this.dragState.type === 'previous') {
            // Left button: only allow rightward drag (positive deltaX)
            dragDeltaX = Math.max(0, Math.min(maxDrag, dragDeltaX));
          } else if (this.dragState.type === 'next') {
            // Right button: only allow leftward drag (negative deltaX)
            dragDeltaX = Math.min(0, Math.max(-maxDrag, dragDeltaX));
          }

          this.dragState.deltaX = dragDeltaX;
          
          // Check if we've reached the full extent
          const atFullExtent = Math.abs(dragDeltaX) >= maxDrag;
          this.dragState.atFullExtent = atFullExtent;
        }
      }
    },
    handleTouchEnd() {
      // Handle touch end for buttons
      if (this.touchState.isButtonTouch) {
        const touchDuration = Date.now() - this.touchState.startTime;
        
        // If it was a short touch without movement, and we haven't already navigated, treat as tap
        if (!this.touchState.hasMoved && touchDuration < 300 && this.touchState.tapTimeout) {
          clearTimeout(this.touchState.tapTimeout);
          this.touchState.tapTimeout = null;
          this.handleButtonTap(this.touchState.buttonType);
        }
        
        // Reset touch state
        this.resetTouchState();
      }
      
      // Reset navigation swipe state
      this.isSwipe = false;
      
      // Let endDrag handle the drag cleanup
      if (this.dragState.isDragging) {
        this.endDrag();
      }
    },

    cancelNavigationTimeout() {
      if (this.navigationTimeout) {
        clearTimeout(this.navigationTimeout);
        this.navigationTimeout = null;
      }
    },

    // Handle immediate button tap (mobile-friendly)
    handleButtonTap(buttonType) {
      // Prevent double navigation if already triggered
      if (this.touchState.triggered) {
        return;
      }
      
      // Clear any pending timeouts
      if (this.touchState.tapTimeout) {
        clearTimeout(this.touchState.tapTimeout);
        this.touchState.tapTimeout = null;
      }
      
      // Mark as triggered to prevent double navigation
      this.touchState.triggered = true;
      
      // Navigate immediately on tap
      if (buttonType === 'previous' && this.hasPrevious) {
        this.prev();
      } else if (buttonType === 'next' && this.hasNext) {
        this.next();
      }
      
      // Reset touch state
      this.resetTouchState();
    },

    resetTouchState() {
      if (this.touchState.tapTimeout) {
        clearTimeout(this.touchState.tapTimeout);
      }
      this.touchState = {
        isButtonTouch: false,
        buttonType: null,
        startTime: 0,
        hasMoved: false,
        tapTimeout: null,
        triggered: false
      };
    },

    // Drag functionality for navigation buttons
    startDrag(event, type) {
      event.preventDefault();

      const clientX = event.touches ? event.touches[0].clientX : event.clientX;
      const clientY = event.touches ? event.touches[0].clientY : event.clientY;

      this.dragState = {
        isDragging: true,
        type: type,
        startX: clientX,
        startY: clientY,
        deltaX: 0,
        deltaY: 0,
        threshold: this.dragState.threshold,
        atFullExtent: false,
        triggered: false,
      };

    },

    handleDrag(event) {
      if (!this.dragState.isDragging) return;

      const clientX = event.touches ? event.touches[0].clientX : event.clientX;

      let deltaX = clientX - this.dragState.startX;

      // Constrain drag to correct direction and max distance
      const maxDrag = this.dragState.threshold; // 10em
      if (this.dragState.type === 'previous') {
        // Left button: only allow rightward drag (positive deltaX)
        deltaX = Math.max(0, Math.min(maxDrag, deltaX));
      } else if (this.dragState.type === 'next') {
        // Right button: only allow leftward drag (negative deltaX)
        deltaX = Math.min(0, Math.max(-maxDrag, deltaX));
      }

      this.dragState.deltaX = deltaX;

      // Check if we've reached the full extent
      const atFullExtent = Math.abs(deltaX) >= maxDrag;
      this.dragState.atFullExtent = atFullExtent;

      // Prevent default to avoid text selection during drag
      event.preventDefault();
    },

    endDrag() {
      if (!this.dragState.isDragging && !this.touchState.isButtonTouch) return;

      // Only show file list if user released at full extent
      if (this.dragState.atFullExtent) {
        this.showFileList(this.dragState.type);
      }

      this.resetDragState();
      this.resetTouchState();
    },

    resetDragState() {
      this.dragState = {
        isDragging: false,
        type: null,
        startX: 0,
        startY: 0,
        deltaX: 0,
        deltaY: 0,
        threshold: this.dragState.threshold,
        atFullExtent: false,
        triggered: false,
      };
    },

    handlePrevClick() {
      // Only navigate if this wasn't a drag
      if (!this.dragState.triggered) {
        this.prev();
      }
      this.resetDragState();
    },

    handleNextClick() {
      // Only navigate if this wasn't a drag
      if (!this.dragState.triggered) {
        this.next();
      }
      this.resetDragState();
    },

    showFileList(type) {
      // Hide navigation buttons when showing file list
      mutations.setNavigationShow(false);
      
      // Determine what list to show based on drag type
      if (type === 'previous') {
        // Show parent directories for navigating up
        this.showParentDirectories();
      } else if (type === 'next') {
        // Show current listing items for quick jumping
        this.showCurrentListing();
      }
    },

    showParentDirectories() {
      // Show files in the current directory (same directory as the previewed file)
      const currentItems = this.getCurrentListingItems();
      mutations.showHover({
        name: "file-list",
        props: {
          fileList: currentItems,
          mode: "navigate-siblings",
          title: this.$t("prompts.quickJump")
        }
      });
    },

    showCurrentListing() {
      const currentItems = this.getCurrentListingItems();
      mutations.showHover({
        name: "file-list",
        props: {
          fileList: currentItems,
          mode: "quick-jump",
          title: this.$t("prompts.quickJump")
        }
      });
    },

    getParentDirectories() {
      // Build array of parent directories from current path
      const currentPath = state.req.path || "/";
      const pathParts = currentPath.split("/").filter(part => part);
      const parentDirs = [];

      // Add root
      parentDirs.push({
        name: "/",
        path: "/",
        source: state.req.source,
        isDirectory: true
      });

      // Add each level up to current
      let buildPath = "";
      for (let i = 0; i < pathParts.length; i++) {
        buildPath += "/" + pathParts[i];
        parentDirs.push({
          name: pathParts[i],
          path: buildPath,
          source: state.req.source,
          isDirectory: true
        });
      }

      return parentDirs.reverse(); // Show deepest first
    },

    getCurrentListingItems() {
      // Get items from the current navigation listing (files in same directory)
      const listing = state.navigation.listing || [];
      return listing.map(item => ({
        name: item.name, // Keep original names without emojis
        path: item.path,
        source: item.source || state.req.source,
        type: item.type,
        isDirectory: item.type === 'directory',
        originalItem: item
      }));
    }
  },
};
</script>

<style scoped>
/* Thin edge detection zones - minimal interference with content */
.nav-zone {
  position: fixed;
  top: 25%; /* Start at 25% from top */
  bottom: 25%; /* End at 25% from bottom (so middle 50%) */
  width: 3em; /* Reduced width to minimize content interference */
  pointer-events: auto;
  z-index: 5; /* Lower z-index so content can appear above */
  background: transparent; /* Invisible zones for mouse/touch detection */
}


.nav-zone-left {
  left: 0;
}

.nav-zone-right {
  right: 0;
}

/* Removed navigation-buttons container to prevent content interaction blocking */

.nav-button {
  position: fixed;
  top: 50%;
  transform: translateY(-50%);
  width: 50px;
  height: 50px;
  border: none;
  border-radius: 50%;
  background: var(--background);
  color: var(--textPrimary);
  cursor: pointer;
  transition: opacity 0.4s ease, transform 0.3s ease, background-color 0.3s ease, box-shadow 0.3s ease;
  pointer-events: auto;
  z-index: 1001;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
  opacity: 1;
}

.nav-button.dark-mode {
  background: var(--surfacePrimary);
  color: var(--textPrimary);
}

.nav-button:hover,
.nav-button.active {
  background: var(--primaryColor);
  transform: translateY(-50%) scale(1.1);
  box-shadow:
        inset 0 -3em 3em rgba(217, 217, 217, 0.211),
        0 0 0 2px var(--alt-background),
        0 4px 20px rgba(0, 0, 0, 0.4);
  color: white;
  opacity: 1;
}


.nav-previous {
  left: 20px;
}

.nav-next {
  right: 20px;
}

.nav-button.hidden {
  opacity: 0;
  transform: translateY(-50%) scale(0.9);
  pointer-events: none !important; /* Ensure no interaction when hidden */
  z-index: -1; /* Move behind content when hidden */
}

/* Smooth show animation for better UX */
.nav-button:not(.hidden) {
  animation: nav-button-show 0.4s ease-out;
}

@keyframes nav-button-show {
  0% {
    opacity: 0;
    transform: translateY(-50%) scale(0.8);
  }
  100% {
    opacity: 1;
    transform: translateY(-50%) scale(1);
  }
}

.nav-button.dragging {
  z-index: 1002;
  cursor: grabbing;
  transition: none; /* Disable transitions during drag for immediate response */
}

.nav-button i.material-icons {
  font-size: 24px;
  line-height: 1;
  transition: transform 0.2s ease;
}

.nav-button:hover i.material-icons,
.nav-button.active i.material-icons {
  transform: scale(1.1);
}

/* Mobile styles */
@media (max-width: 768px) {
  .nav-button {
    width: 44px;
    height: 44px;
  }

  .nav-previous {
    left: 10px;
  }

  .nav-next {
    right: 10px;
  }

  .nav-button i.material-icons {
    font-size: 20px;
  }

  /* Reduce animation intensity on mobile for better performance */
  .nav-button:not(.hidden) {
    animation-duration: 0.3s;
  }
}

/* Ensure buttons don't interfere with scrollbars */
@media (max-width: 480px) {
  .nav-previous {
    left: 8px;
  }

  .nav-next {
    right: 8px;
  }
}
.moveWithSidebar {
  margin-left: 20em;
}
</style>
