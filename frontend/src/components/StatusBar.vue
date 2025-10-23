<template>
  <div v-if="showStatusBar" id="status-bar" :class="{ 'dark-mode-header': isDarkMode }">
    <div class="status-content">
      <div class="status-info">
        <span v-if="selectedCount > 0" class="selection-info">
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          {{ selectedCount }} {{ $t(selectedCount === 1 ? 'files.itemSelected' : 'files.itemsSelected') }} ({{ displayTotalSize }})
        </span>
        <span v-else class="directory-info">
          {{ numDirs }} {{ $t(numDirs === 1 ? 'general.folder' : 'general.folders') }}
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          {{ numFiles }} {{ $t(numFiles === 1 ? 'general.file' : 'general.files') }} ({{ displayTotalSize }})
        </span>
      </div>

      <div class="status-controls">
        <div v-if="showGalleryToggle" class="gallery-toggle">
          <action
            class="menu-button"
            icon="grid_view"
            :title="$t('buttons.switchView')"
            @action="toggleGalleryView"
          />
        </div>

        <div v-if="showGallerySize" class="gallery-size-control">
          <span class="size-label">{{ $t("files.size") }}</span>
          <input
            v-model="gallerySize"
            type="range"
            id="gallery-size"
            name="gallery-size"
            min="1"
            max="8"
            @input="updateGallerySize"
            @change="commitGallerySize"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { state, getters, mutations } from "@/store";
import Action from "@/components/Action.vue";
import { getHumanReadableFilesize } from "@/utils/filesizes";

export default {
  name: "StatusBar",
  components: {
    Action,
  },
  data() {
    return {
      gallerySize: state.user.gallerySize,
    };
  },
  computed: {
    showStatusBar() {
      return getters.currentView() === "listingView" && !getters.isShare();
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    selectedCount() {
      return getters.selectedCount();
    },
    numDirs() {
      return getters.reqNumDirs();
    },
    numFiles() {
      return getters.reqNumFiles();
    },
    showGallerySize() {
      return getters.isCardView() || getters.isListingViewMode() && state.req?.items?.length > 0;
    },
    showGalleryToggle() {
      const viewMode = getters.viewMode();
      return viewMode === "gallery" || viewMode === "icons";
    },
    isListingViewMode() {
      const viewMode = getters.viewMode();
      return viewMode === "list" || viewMode === "compact";
    },
    sliderConfig() {
      if (this.isListingViewMode) {
        return {
          min: 0,
          max: 2,
          step: 1,
          values: { 0: 'compact', 2: 'list' }
        };
      } else {
        return {
          min: 0,
          max: 9,
          step: 1,
          values: null // Normal gallery size
        };
      }
    },
    currentSliderValue() {
      if (this.isListingViewMode) {
        // Map view mode to slider value
        return getters.viewMode() === 'compact' ? 0 : 2;
      } else {
        // Normal gallery size
        return state.user.gallerySize;
      }
    },
    // Calculate total size of current directory
    totalDirectorySize() {
      if (!state.req?.items) return 0;
      return state.req.items.reduce((total, item) => total + (item.size || 0), 0);
    },
    // Calculate total size of selected items
    totalSelectedSize() {
      if (this.selectedCount === 0) return 0;
      
      let total = 0;
      state.selected.forEach(index => {
        const item = state.req.items[index];
        if (item && item.size) {
          total += item.size;
        }
      });
      return total;
    },
    // Total size
    displayTotalSize() {
      const size = this.selectedCount > 0 ? this.totalSelectedSize : this.totalDirectorySize;
      return getHumanReadableFilesize(size);
    },
  },
  methods: {
    updateGallerySize(event) {
      const newValue = parseInt(event.target.value, 10);
      
      if (this.isListingViewMode) {
        // Switch between list and compact views
        const newViewMode = newValue === 1 ? "compact" : "list";
        mutations.updateDisplayPreferences({ viewMode: newViewMode });
        mutations.updateCurrentUser({ viewMode: newViewMode });
      } else {
        // Normal gallery size behavior
        this.gallerySize = newValue;
        mutations.setGallerySize(newValue);
      }
    },
    commitGallerySize() {
      mutations.setGallerySize(this.gallerySize);
    },
    toggleGalleryView() {
      const currentMode = getters.viewMode();
      const newMode = currentMode === "gallery" ? "icons" : "gallery";
      mutations.updateDisplayPreferences({ viewMode: newMode });
      mutations.updateCurrentUser({ viewMode: newMode });
    },
  },
};
</script>

<style scoped>
#status-bar {
  background-color: var(--alt-background);
  border-top: 1px solid var(--divider);
  font-size: 0.875em;
  height: 2.5em;
  display: flex;
  align-items: center;
  position: sticky;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 10;
  border-radius: 2px;
}

.status-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 0 1em;
  height: 100%;
}

.status-info {
  display: flex;
  align-items: center;
  color: var(--textSecondary);
  font-weight: 500;
}

.selection-info {
  color: var(--primaryColor);
}

.status-controls {
  display: flex;
  align-items: center;
  gap: 1.5em;
}

.gallery-toggle .menu-button {
  width: 2em;
  height: 2em;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.gallery-toggle .menu-button i {
  font-size: 1.2em;
  margin: 0;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
}

.gallery-size-control {
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.size-label {
  color: var(--textSecondary);
  font-size: 0.875em;
  white-space: nowrap;
}

input[type="range"] {
  accent-color: var(--primaryColor);
  width: 8em;
}

/* Backdrop filter support */
@supports (backdrop-filter: none) {
  #status-bar {
    backdrop-filter: blur(16px) invert(0.1);
    background-color: rgb(37 49 55 / 5%) !important;
  }
}

/* Mobile styles */
@media (max-width: 800px) {
  #status-bar {
    height: 3em;
    font-size: 0.9em;
    box-shadow: 0 -2px 10px rgba(0, 0, 0, 0.1);
  }
  
  .status-content {
    padding: 0 0.8em;
  }
  
  .status-controls {
    gap: 1.2em;
  }
  
  input[type="range"] {
    width: 7em;
    height: 1.5em; /* For better touch */
  }
  
  .gallery-toggle .menu-button {
    width: 2.2em;
    height: 2.2em;
  }
  
  .gallery-toggle .menu-button i {
    font-size: 1.3em;
  }
  
  .status-info {
    font-size: 1em;
  }
  
  .size-label {
    font-size: 0.9em;
  }
}
</style>