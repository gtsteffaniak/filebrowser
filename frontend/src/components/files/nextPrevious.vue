<template>
  <!-- Left edge detection zone -->
  <div
    v-if="enabled && hasPrevious"
    class="nav-zone nav-zone-left"
    @mousemove="toggleNavigation"
    @touchstart="toggleNavigation"
    @click="handleClick"
  ></div>

  <!-- Right edge detection zone -->
  <div
    v-if="enabled && hasNext"
    class="nav-zone nav-zone-right"
    @mousemove="toggleNavigation"
    @touchstart="toggleNavigation"
    @click="handleClick"
  ></div>

  <!-- Navigation buttons container -->
  <div
    v-if="enabled && (hasPrevious || hasNext)"
    class="navigation-buttons"
  >
    <button
      v-if="hasPrevious"
      @click.stop="prev"
      @mouseover="setHoverNav(true)"
      @mouseleave="setHoverNav(false)"
      :class="['nav-button', 'nav-previous', { hidden: !showNav }]"
      :aria-label="$t('buttons.previous')"
      :title="$t('buttons.previous')"
    >
      <i class="material-icons">chevron_left</i>
    </button>

    <button
      v-if="hasNext"
      @click.stop="next"
      @mouseover="setHoverNav(true)"
      @mouseleave="setHoverNav(false)"
      :class="['nav-button', 'nav-next', { hidden: !showNav }]"
      :aria-label="$t('buttons.next')"
      :title="$t('buttons.next')"
    >
      <i class="material-icons">chevron_right</i>
    </button>

    <!-- Prefetch links for better performance -->
    <link v-if="previousRaw" rel="prefetch" :href="previousRaw" />
    <link v-if="nextRaw" rel="prefetch" :href="nextRaw" />
  </div>
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
    };
  },
  computed: {
    enabled() {
      console.log("🔍 NextPrevious enabled:", state.navigation.enabled);
      return state.navigation.enabled;
    },
    showNav() {
      const shouldShow = state.navigation.show || this.hoverNav;
      console.log("🔍 NextPrevious showNav:", shouldShow, "state.navigation.show:", state.navigation.show, "hoverNav:", this.hoverNav);
      return shouldShow;
    },
    hasPrevious() {
      const has = state.navigation.previousLink !== "";
      console.log("🔍 NextPrevious hasPrevious:", has, "link:", state.navigation.previousLink);
      return has;
    },
    hasNext() {
      const has = state.navigation.nextLink !== "";
      console.log("🔍 NextPrevious hasNext:", has, "link:", state.navigation.nextLink);
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
      console.log("🔍 NextPrevious currentView:", view);
      return view;
    }
  },
  watch: {
    currentView() {
      console.log("👀 NextPrevious currentView changed to:", this.currentView);
      this.updateNavigationEnabled();
    },
    'state.req'() {
      console.log("👀 NextPrevious state.req changed:", state.req?.name, state.req?.type);
      this.updateNavigationEnabled();
      // Auto-setup navigation when request changes and we're enabled
      if (this.enabled) {
        this.$nextTick(() => {
          this.setupNavigationForCurrentItem();
        });
      }
    },
    enabled(newEnabled) {
      console.log("👀 NextPrevious enabled changed:", newEnabled);
      if (newEnabled && state.req) {
        this.$nextTick(() => {
          this.setupNavigationForCurrentItem();
        });
      }
    },
    // Watch for when navigation links are set up
    'state.navigation.previousLink'() {
      console.log("👀 previousLink changed:", state.navigation.previousLink);
      this.showInitialNavigation();
    },
    'state.navigation.nextLink'() {
      console.log("👀 nextLink changed:", state.navigation.nextLink);
      this.showInitialNavigation();
    },
  },
  mounted() {
    console.log("🚀 NextPrevious mounted, currentView:", this.currentView, "navigation state:", state.navigation);
    window.addEventListener("keydown", this.keyEvent);
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
    console.log("💀 NextPrevious unmounting");
    window.removeEventListener("keydown", this.keyEvent);
    mutations.clearNavigation();
  },
  methods: {
    updateNavigationEnabled() {
      const shouldEnable = previewViews.includes(this.currentView);
      console.log("🔧 NextPrevious updateNavigationEnabled:", {
        currentView: this.currentView,
        previewViews,
        shouldEnable,
        navigationState: state.navigation
      });
      mutations.setNavigationEnabled(shouldEnable);
    },
    async setupNavigationForCurrentItem() {
      if (!this.enabled || !state.req || state.req.type === 'directory') {
        console.log("⏭️ Skipping navigation setup:", { enabled: this.enabled, req: !!state.req, isDirectory: state.req?.type === 'directory' });
        return;
      }

      console.log("🔧 Setting up navigation for:", state.req.name);

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
          console.error("Failed to fetch directory listing:", error);
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
      console.log("🎯 showInitialNavigation called:", {
        enabled: this.enabled,
        hasPrevious: this.hasPrevious,
        hasNext: this.hasNext,
        condition: this.enabled && (this.hasPrevious || this.hasNext)
      });

      // Show navigation initially for 3 seconds when navigation is set up
      if (this.enabled && (this.hasPrevious || this.hasNext)) {
        console.log("✨ Showing initial navigation for 3 seconds");
        mutations.setNavigationShow(true);
        setTimeout(() => {
          console.log("⏰ Auto-hiding navigation, hoverNav:", this.hoverNav);
          if (!this.hoverNav) {
            mutations.setNavigationShow(false);
          }
        }, 3000);
      } else {
        console.log("❌ Not showing navigation - conditions not met");
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
      // Simplified: clicking anywhere in the CSS zones shows navigation
      if (!this.enabled || (!this.hasPrevious && !this.hasNext)) return;

      console.log("✨ Click in navigation zone - showing navigation");
      this.showNavigation();
    },
    showNavigation() {
      console.log("🎯 showNavigation called - showing nav for 3 seconds");
      mutations.setNavigationShow(true);
      mutations.clearNavigationTimeout();

      const timeout = setTimeout(() => {
        if (!this.hoverNav) {
          console.log("⏰ Timeout: hiding navigation (not hovered)");
          mutations.setNavigationShow(false);
        } else {
          console.log("⏰ Timeout: keeping navigation (hovered)");
        }
        mutations.clearNavigationTimeout();
      }, 3000); // Show for 3 seconds instead of 1.5

      mutations.setNavigationTimeout(timeout);
    },
    toggleNavigation: throttle(function (event) {
      console.log("🖱️ toggleNavigation called from zone", {
        enabled: this.enabled,
        showNav: this.showNav,
        eventType: event.type,
        target: event.target.className
      });
      if (!this.enabled) return;
      this.showNavigation();
    }, 100),
    setHoverNav(value) {
      console.log("🖱️ setHoverNav:", { value, currentHover: this.hoverNav });
      this.hoverNav = value;
      mutations.setNavigationHover(value);
    }
  },
};
</script>

<style scoped>
/* Edge detection zones for mouse/touch events */
.nav-zone {
  position: fixed;
  top: 25%; /* Start at 25% from top */
  bottom: 25%; /* End at 25% from bottom (so middle 50%) */
  width: 7em; /* 7em wide zones at screen edges */
  pointer-events: auto;
  z-index: 10;
  background: rgba(255, 0, 0, 0.1); /* Temporary debug - red tint to see zones */
  border: 2px dashed rgba(255, 0, 0, 0.3); /* Temporary debug border */
}

.nav-zone-left {
  left: 0;
}

.nav-zone-right {
  right: 0;
}

.navigation-buttons {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  pointer-events: none; /* Don't block content interaction */
  z-index: 1000;
}

.nav-button {
  position: fixed;
  top: 50%;
  transform: translateY(-50%);
  width: 50px;
  height: 50px;
  border: none;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.7);
  color: white;
  cursor: pointer;
  transition: all 0.3s ease;
  pointer-events: auto;
  z-index: 1001;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
}

.nav-button:hover {
  background: var(--primaryColor, rgba(0, 178, 255, 0.9));
  transform: translateY(-50%) scale(1.1);
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.4);
}

.nav-button:active {
  transform: translateY(-50%) scale(0.95);
}

.nav-previous {
  left: 20px;
}

.nav-next {
  right: 20px;
}

.nav-button.hidden {
  opacity: 0;
  transform: translateY(-50%) scale(0.8);
  pointer-events: none;
}

.nav-button i.material-icons {
  font-size: 24px;
  line-height: 1;
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
</style>
