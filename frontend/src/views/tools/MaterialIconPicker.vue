<template>
  <div id="tool-material-icon-picker">
    <div class="tool-header">
      <h1>{{ $t('tools.materialIconPicker.name') }}</h1>
      <p>{{ $t('tools.materialIconPicker.description') }}</p>

      <!-- External Link Banner -->
      <div class="external-link-banner">
        <i class="material-icons">open_in_new</i>
        <span>{{ $t('tools.materialIconPicker.browseFullLibrary') }}</span>
        <a href="https://fonts.google.com/icons" target="_blank" rel="noopener noreferrer">
          {{ $t('tools.materialIconPicker.iconLibraryUrl') }}
        </a>
      </div>
    </div>

    <!-- Search Controls -->
    <div class="controls">
      <div class="search-box">
        <i class="material-icons search-icon">search</i>
        <input
          v-model="searchQuery"
          type="text"
          :placeholder="$t('tools.materialIconPicker.searchPlaceholder')"
          class="input"
          style="padding-left: 2.5em;"
        />
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
    <div class="icon-grid">
      <!-- Custom Icon Preview (if searching and doesn't exactly match existing) -->
      <div
        v-if="showCustomPreview"
        class="icon-card clickable custom-icon"
        @click="copyIconName(searchQuery.trim())"
        :title="$t('tools.materialIconPicker.useCustomIcon', { name: searchQuery.trim() })"
      >
        <div class="icon-display">
          <i :class="getIconClass(searchQuery.trim())">{{ searchQuery.trim() }}</i>
        </div>
        <div class="icon-name">{{ searchQuery.trim() }}</div>
        <div class="custom-badge">{{ $t('tools.materialIconPicker.custom') }}</div>
      </div>

      <!-- Popular Icons -->
      <div
        v-for="iconName in visibleIcons"
        :key="iconName"
        class="icon-card clickable"
        @click="copyIconName(iconName)"
        :title="iconName"
      >
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
      allMaterialIcons,
    };
  },
  computed: {
    filteredIcons() {
      // Apply search filter
      if (this.searchQuery.trim()) {
        const query = this.searchQuery.toLowerCase();
        return this.allMaterialIcons.filter((icon) =>
          icon.toLowerCase().includes(query)
        );
      }
      return this.allMaterialIcons;
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
      // Limit results for performance (show more in full-page view with smaller cards)
      return this.filteredIcons.slice(0, 300);
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
#tool-material-icon-picker {
  padding: 2em;
  max-width: 1400px;
  margin: 0 auto;
}

/* Header */
.tool-header {
  margin-bottom: 2em;
}

.tool-header h1 {
  font-size: 2em;
  margin-bottom: 0.5em;
  color: var(--textPrimary);
}

.tool-header p {
  color: var(--textSecondary);
  font-size: 1.1em;
  margin-bottom: 1em;
}

/* External Link Banner */
.external-link-banner {
  display: flex;
  align-items: center;
  gap: 0.75em;
  padding: 0.5em 1em; /* Use button padding */
  background: var(--surfaceSecondary);
  border: 1px solid var(--borderColor);
  border-radius: 1em; /* Use button border-radius */
  font-size: 1em;
  color: var(--textSecondary);
  margin-top: 1.5em;
}

.external-link-banner .material-icons {
  font-size: 1.5em;
  color: var(--primaryColor);
}

.external-link-banner a {
  color: var(--primaryColor);
  text-decoration: none;
  font-weight: 600;
  margin-left: auto;
  font-size: 1.05em;
}

.external-link-banner a:hover {
  text-decoration: underline;
}

/* Controls - uses .input class from _inputs.css */
.controls {
  margin-bottom: 1.5em;
}

.search-box {
  position: relative;
  max-width: 600px;
  margin-bottom: 1em;
}

.search-box .search-icon {
  position: absolute;
  left: 1em;
  top: 50%;
  transform: translateY(-50%);
  color: var(--textSecondary);
  pointer-events: none;
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
  border-radius: 1em; /* Use button border-radius */
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
  border-radius: 0.5em; /* Smaller rounded corners consistent with design */
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

