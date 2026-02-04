<template>
  <div class="tool">
    <div class="card-title">
      <p>{{ $t('tools.materialIconPicker.description') }}</p>

      <!-- External Link Banner -->
      <a class="button" href="https://fonts.google.com/icons" target="_blank" rel="noopener noreferrer">
        <i class="material-icons">open_in_new</i>
        <span>{{ $t('tools.materialIconPicker.browseFullLibrary') }}</span>
      </a>
    </div>

    <!-- Search Controls -->
    <div class="controls">
      <div class="search-box">
        <i class="material-icons search-icon">search</i>
        <input v-model="searchQuery" type="text" :placeholder="$t('tools.materialIconPicker.searchPlaceholder')"
          class="input" style="padding-left: 2.5em;" />
        <button v-if="searchQuery" @click="searchQuery = ''" class="button button--flat clear-button">
          <i class="material-icons">close</i>
        </button>
      </div>
    </div>

    <!-- Results count -->
    <div class="results-info">
      <span v-if="searchQuery">
        {{ $t('tools.materialIconPicker.showingResults', { count: visibleIcons.length }) }}
      </span>
      <span v-else>
        {{ $t('tools.materialIconPicker.popularIcons', { count: allMaterialIcons.length }) }}
      </span>
    </div>

    <!-- Icon Grid -->
    <div class="icon-grid" :key="searchQueryKey">
      <!-- Custom Icon Preview (if searching and doesn't exactly match existing) -->
      <div v-if="showCustomPreview" class="icon-card clickable custom-icon" @click="copyIconName(searchQuery.trim())"
        :title="$t('tools.materialIconPicker.useCustomIcon', { name: searchQuery.trim() })">
        <div class="icon-display">
          <i :class="getIconClass(searchQuery.trim())">{{ searchQuery.trim() }}</i>
        </div>
        <div class="icon-name">{{ searchQuery.trim() }}</div>
        <div class="custom-badge">{{ $t('tools.materialIconPicker.custom') }}</div>
      </div>

      <!-- Popular Icons -->
      <div v-for="(iconName, index) in visibleIcons" :key="`${searchQueryKey}-${iconName}-${index}`"
        class="icon-card clickable" @click="copyIconName(iconName)" :title="iconName">
        <div class="icon-display">
          <i :class="getIconClass(iconName)">{{ iconName }}</i>
        </div>
        <div class="icon-name">{{ iconName }}</div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-if="!showCustomPreview && visibleIcons.length === 0" class="empty-state">
      <i class="material-icons">search_off</i>
      <p>{{ $t('tools.materialIconPicker.noResults') }}</p>
      <p class="empty-hint">{{ $t('tools.materialIconPicker.tryDifferentSearch') }}</p>
    </div>

  </div>
</template>

<script>
import {
  allMaterialIcons,
  getIconClass,
} from "@/utils/material-icons";
import { notify } from "@/notify";

export default {
  name: "MaterialIconPicker",
  data() {
    return {
      searchQuery: "",
    };
  },
  computed: {
    allMaterialIcons() {
      // Return the icon list as a computed property to avoid reactivity issues
      return allMaterialIcons;
    },
    searchQueryKey() {
      // Create a unique key based on search query to force re-render when search changes
      return this.searchQuery.trim();
    },
    filteredIcons() {
      // Apply search filter - always return a new array to avoid caching issues
      const query = this.searchQuery.trim().toLowerCase();
      if (!query) {
        return [...this.allMaterialIcons];
      }

      // Create a fresh filtered array
      const filtered = [];
      for (const icon of this.allMaterialIcons) {
        if (icon.toLowerCase().includes(query)) {
          filtered.push(icon);
        }
      }
      return filtered;
    },
    showCustomPreview() {
      // Show custom preview if:
      // 1. User is searching
      // 2. The search term doesn't exactly match any existing icon
      const trimmed = this.searchQuery.trim();
      if (!trimmed) return false;

      // Check if it's an exact match
      const exactMatch = this.allMaterialIcons.some(
        (icon) => icon.toLowerCase() === trimmed.toLowerCase()
      );

      return !exactMatch;
    },
    visibleIcons() {
      // Always return the filtered icons array
      return this.filteredIcons;
    },
  },
  methods: {
    getIconClass,
    async copyIconName(iconName) {
      try {
        await navigator.clipboard.writeText(iconName);
        notify.showSuccessToast(
          this.$t('tools.materialIconPicker.copiedToClipboard', { name: iconName })
        );
      } catch (err) {
        console.error("Failed to copy icon name:", err);
        // Fallback for older browsers
        this.fallbackCopy(iconName);
      }
    },
    fallbackCopy(text) {
      const textArea = document.createElement("textarea");
      textArea.value = text;
      textArea.style.position = "fixed";
      textArea.style.left = "-999999px";
      document.body.appendChild(textArea);
      textArea.focus();
      textArea.select();

      try {
        document.execCommand("copy");
        notify.showSuccessToast(
          this.$t('tools.materialIconPicker.copiedToClipboard', { name: text })
        );
      } catch (err) {
        console.error("Fallback copy failed:", err);
        notify.showErrorToast(this.$t('tools.materialIconPicker.copyFailed'));
      }

      document.body.removeChild(textArea);
    },
  },
};
</script>

<style scoped>
/* External Link Banner */
a.button {
  display: flex;
  align-items: center;
  margin: auto;
  width: 500px;
  max-width: 100%;
  justify-content: center;
}

/* Controls - uses .input class from _inputs.css */
.controls {
  margin: 1.5em;
}

.search-box {
  position: relative;
  margin-bottom: 1em;
}

.search-box .search-icon {
  position: absolute;
  left: 0.5em;
  top: 0.25em;
}

.clear-button {
  position: absolute;
  right: 0.5em;
  top: 50%;
  transform: translateY(-50%);
  min-width: auto !important;
  padding: 0.25em !important;
}

/* Results Info */
.results-info {
  margin-bottom: 1em;
  color: var(--textSecondary);
  font-size: 0.9em;
}

/* Icon Grid */
.icon-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
  gap: 0.75em;
  margin-bottom: 2em;
}

/* Uses .clickable hover effect from styles.css */
.icon-card {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 1em 0.6em;
  background: var(--surfaceSecondary);
  border: 1px solid var(--borderColor);
  border-radius: 1em;
  /* Use button border-radius */
}

/* Custom Icon Styling */
.icon-card.custom-icon {
  border-color: var(--primaryColor);
  border-style: dashed;
}

.icon-display {
  font-size: 2em;
  color: var(--primaryColor);
  margin-bottom: 0.4em;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 1em;
}

.icon-name {
  font-size: 0.75em;
  color: var(--textSecondary);
  text-align: center;
  word-break: break-word;
  line-height: 1.2;
}

.custom-badge {
  position: absolute;
  top: 0.5em;
  right: 0.5em;
  background: var(--primaryColor);
  color: white;
  padding: 0.25em 0.6em;
  border-radius: 0.5em;
  /* Smaller rounded corners consistent with design */
  font-size: 0.7em;
  font-weight: 600;
  text-transform: uppercase;
}

/* Empty State */
.empty-state {
  text-align: center;
  padding: 4em 2em;
  color: var(--textSecondary);
}

.empty-state .material-icons {
  font-size: 4em;
  margin-bottom: 0.5em;
  opacity: 0.5;
}

.empty-state p {
  font-size: 1.2em;
  margin-bottom: 0.5em;
}

.empty-hint {
  font-size: 0.9em;
  opacity: 0.7;
}

/* Responsive */
@media (max-width: 768px) {
  #tool-material-icon-picker {
    padding: 1em;
  }

  .icon-grid {
    grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
    gap: 0.75em;
  }

  .icon-card {
    padding: 1em 0.5em;
  }

  .icon-display {
    font-size: 2em;
  }
}
</style>
