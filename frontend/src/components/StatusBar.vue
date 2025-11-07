<template>
  <div v-if="showStatusBar" id="status-bar" :class="{ 'dark-mode-header': isDarkMode }" @contextmenu.prevent.stop @touchstart.stop @touchend.stop>
    <div class="status-content" @contextmenu.prevent.stop @touchstart.stop @touchend.stop>
      <div class="status-info">
        <span v-if="selectedCount > 0">
          <span class="button">{{ selectedCount }}</span>
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          {{ $t(selectedCount === 1 ? 'files.itemSelected' : 'files.itemsSelected') }} ({{ displayTotalSize }})
        </span>
        <span v-else class="directory-info">
          <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
          {{ numDirs }} {{ $t(numDirs === 1 ? 'buttons.folder' : 'general.folders') }} | {{ numFiles }} {{ $t(numFiles === 1 ? 'buttons.file' : 'general.files') }} ({{ displayTotalSize }})
        </span>
      </div>
      <div class="status-controls">
        <div v-if="showGallerySize" class="gallery-size-control">
          <span class="size-label">{{ $t("general.size") }}</span>
          <input
            v-model="gallerySize"
            type="range"
            id="gallery-size"
            name="gallery-size"
            min="1"
            max="9"
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
import { getHumanReadableFilesize } from "@/utils/filesizes";

export default {
  name: "StatusBar",
  data() {
    return {
      gallerySize: state.user.gallerySize,
    };
  },
  computed: {
    showStatusBar() {
      return getters.currentView() === "listingView";
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
      return getters.isCardView() && state.req?.items?.length > 0;
    },
    totalDirectorySize() {
      if (!Array.isArray(state.req?.items)) return 0;
      return state.req.items.reduce((total, item) => total + (item.size || 0), 0);
    },
    // Calculate total size of selected items
    totalSelectedSize() {
      if (this.selectedCount === 0) return 0;
      if (!Array.isArray(state.req?.items)) {
        return 0;
      }
      let total = 0;
      state.selected.forEach(index => {
        if (index >= 0 && index < state.req.items.length) {
          const item = state.req.items[index];
          if (item && item.size) {
            total += item.size;
          }
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
      this.gallerySize = newValue;
    },
    commitGallerySize() {
      mutations.setGallerySize(this.gallerySize);
      // Automatically adjust view mode based on gallery size
      this.adjustViewMode();
    },
    adjustViewMode() {
      const currentMode = getters.viewMode();
      let newMode = currentMode;
      const size = this.gallerySize;

      // Gallery/Icons family - switch based on size
      if (currentMode === "gallery" || currentMode === "icons") {
        if (size <= 4) {
          newMode = "icons";
        } else {
          newMode = "gallery";
        }
      }

      // List/Compact family - switch based on size
      if (currentMode === "list" || currentMode === "compact") {
        if (size <= 3) {
          newMode = "compact";
        } else {
          newMode = "list";
        }
      }

      // Only update if the mode actually changed
      if (newMode !== currentMode) {
        mutations.updateDisplayPreferences({ viewMode: newMode });
        mutations.updateCurrentUser({ viewMode: newMode });
      }
    },
  },
};
</script>

<style scoped>
#status-bar {
  background-color: rgb(37 49 55 / 5%) !important;
  border-top: 1px solid var(--divider);
  height: 2.5em;
  display: flex;
  align-items: center;
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 10;
  border-radius: 2px;
  overflow: hidden;
  margin: 0;
  padding: 0;
  transition: 0.5s ease;
}

#main.moveWithSidebar #status-bar {
  margin-left: 20em;
}

.status-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 0 1em;
  height: 100%;
  font-size: 0.85em;
}

.status-info {
  display: flex;
  align-items: center;
  color: var(--textSecondary);
  font-weight: 500;
}

.button {
  padding: 0 0.5em;
  font-size: 0.9em;
  font-weight: bold;
}

.status-controls {
  display: flex;
  align-items: center;
  gap: 1.5em;
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
  }
  #status-bar.dark-mode-header {
    background-color: rgb(37 49 55 / 33%) !important;
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

  .status-info {
    font-size: 1em;
  }

  .size-label {
    font-size: 0.9em;
  }
}
</style>