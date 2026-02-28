<template>
  <div class="card-content">
    <p>{{ $t('sidebar.pickIconDescription') }}</p>

    <!-- External Link -->
    <div class="external-link-banner">
      <i class="material-icons">open_in_new</i>
      <span>{{ $t('tools.materialIconPicker.browseFullLibrary') }}</span>
      <a href="https://fonts.google.com/icons" target="_blank" rel="noopener noreferrer">
        {{ $t('tools.materialIconPicker.iconLibraryUrl') }}
      </a>
    </div>

    <!-- Search Box -->
    <div class="search-box">
      <i class="material-icons search-icon">search</i>
      <input
        v-model="searchQuery"
        type="text"
        :placeholder="$t('tools.materialIconPicker.searchPlaceholder')"
        class="input"
        ref="searchInput"
        style="padding-left: 2.5em;"
      />
      <button v-if="searchQuery" @click="searchQuery = ''" class="button button--flat clear-button">
        <i class="material-icons">close</i>
      </button>
    </div>

    <!-- Results Info -->
    <div class="results-info">
      <span v-if="searchQuery">
        {{ $t('tools.materialIconPicker.showingResults', { count: visibleIcons.length }) }}
      </span>
      <span v-else>
        {{ $t('tools.materialIconPicker.popularIcons', { count: filteredIcons.length }) }}
      </span>
    </div>

    <!-- Icon Grid -->
    <div class="icon-grid">
      <!-- Custom Icon Preview (if searching and doesn't exactly match existing) -->
      <div
        v-if="showCustomPreview"
        class="icon-card clickable custom-icon"
        @click="selectIcon(searchQuery.trim())"
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
        @click="selectIcon(iconName)"
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
    </div>
  </div>

  <div class="card-actions">
    <button
      @click="closeTopHover"
      class="button button--flat button--grey"
      :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')"
    >
      {{ $t("general.cancel") }}
    </button>
  </div>
</template>

<script>
import {
  allMaterialIcons,
  getIconClass,
} from "@/utils/material-icons";
import { mutations } from "@/store";

export default {
  name: "IconPicker",
  props: {
    onSelect: {
      type: Function,
      required: true,
    },
  },
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
      // Limit results for performance (more now that cards are smaller)
      return this.filteredIcons.slice(0, 150);
    },
  },
  mounted() {
    // Focus the search input when the picker opens
    this.$nextTick(() => {
      this.$refs.searchInput?.focus();
    });
  },
  methods: {
    getIconClass,
    selectIcon(iconName) {
      this.onSelect(iconName);
      this.closeTopHover();
    },
    closeTopHover() {
      mutations.closeTopHover();
    },
  },
};
</script>

<style scoped>
.card-content {
  max-height: 60vh;
  overflow-y: auto;
}

/* External Link Banner */
.external-link-banner {
  display: flex;
  align-items: center;
  gap: 0.5em;
  padding: 0.5em 1em; /* Use button padding */
  background: var(--surfaceSecondary);
  border: 1px solid var(--borderColor);
  border-radius: 1em; /* Use button border-radius */
  margin-bottom: 1em;
  font-size: 0.9em;
  color: var(--textSecondary);
}

.external-link-banner .material-icons {
  font-size: 1.2em;
  color: var(--primaryColor);
}

.external-link-banner a {
  color: var(--primaryColor);
  text-decoration: none;
  font-weight: 500;
  margin-left: auto;
}

.external-link-banner a:hover {
  text-decoration: underline;
}

/* Search Box - uses .input class from _inputs.css */
.search-box {
  position: relative;
  margin: 1em 0;
}

.search-box .search-icon {
  position: absolute;
  left: 0.75em;
  top: 50%;
  transform: translateY(-50%);
  color: var(--textSecondary);
  pointer-events: none;
  font-size: 1.2em;
}

.clear-button {
  position: absolute;
  right: 0.25em;
  top: 50%;
  transform: translateY(-50%);
  min-width: auto !important;
  padding: 0.25em !important;
}

.clear-button .material-icons {
  font-size: 1.2em;
}

/* Results Info */
.results-info {
  margin-bottom: 1em;
  color: var(--textSecondary);
  font-size: 0.85em;
}

/* Icon Grid */
.icon-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(70px, 1fr));
  gap: 0.4em;
}

/* Uses .clickable hover effect from styles.css */
.icon-card {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 0.6em 0.3em;
  background: var(--surfaceSecondary);
  border: 1px solid var(--borderColor);
  border-radius: 1em; /* Use button border-radius */
}

.icon-display {
  font-size: 1.75em;
  color: var(--primaryColor);
  margin-bottom: 0.3em;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 1em;
}

.icon-name {
  font-size: 0.65em;
  color: var(--textSecondary);
  text-align: center;
  word-break: break-word;
  line-height: 1.1;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

/* Custom Icon Badge */
.icon-card.custom-icon {
  border-color: var(--primaryColor);
  border-style: dashed;
}

.custom-badge {
  position: absolute;
  top: 0.25em;
  right: 0.25em;
  background: var(--primaryColor);
  color: white;
  padding: 0.15em 0.4em;
  border-radius: 0.5em; /* Smaller rounded corners consistent with design */
  font-size: 0.6em;
  font-weight: 600;
  text-transform: uppercase;
}

/* Empty State */
.empty-state {
  text-align: center;
  padding: 3em 2em;
  color: var(--textSecondary);
}

.empty-state .material-icons {
  font-size: 3em;
  margin-bottom: 0.5em;
  opacity: 0.5;
}

.empty-state p {
  font-size: 1em;
}
</style>

