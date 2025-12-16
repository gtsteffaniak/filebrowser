<template>
  <div class="duplicate-finder">
    <div class="card duplicate-finder-config">
      <div class="card-title">
        <h2>{{ $t('duplicateFinder.title') }}</h2>
      </div>
      <div class="card-content">
        <h3>{{ $t('general.source') }}</h3>
        <select v-model="selectedSource" class="input">
          <option v-for="(info, name) in sourceInfo" :key="name" :value="name">
            {{ name }}
          </option>
        </select>
        <h3>{{ $t('general.path') }}</h3>
        <div aria-label="duplicate-finder-path" class="searchContext clickable button" @click="openPathPicker">
          {{ $t('general.path', { suffix: ':' }) }} {{ searchPath }}
        </div>
        <h3>{{ $t('duplicateFinder.minSize') }}</h3>
        <input v-model.number="minSizeValue" type="number" min="0" placeholder="1" class="input" />
        <p class="hint">{{ $t('duplicateFinder.minSizeHint') }}</p>
        <button @click="fetchData" class="button" :disabled="loading">
          <i v-if="loading" class="material-icons spin">autorenew</i>
          <span v-else>{{ $t('duplicateFinder.findDuplicates') }}</span>
        </button>
      </div>
    </div>

    <div class="card duplicate-finder-results">
      <div v-if="error" class="error-message">
        {{ error }}
      </div>

      <div v-if="duplicateGroups.length > 0">
        <div class="card-title">
          <h2>{{ $t('general.results') }}</h2>
        </div>
        <div class="card-content">
          <div class="stats">
            <span>{{ $t('duplicateFinder.groupsFound', { suffix: ': ' }) }}<strong>{{ duplicateGroups.length }}</strong></span>
            <span>{{ $t('duplicateFinder.totalWastedSpace', { suffix: ': ' }) }}<strong>{{ humanSize(totalWastedSpace) }}</strong></span>
          </div>

          <!-- Show timeout/limit warning first if applicable -->
          <div v-if="isIncomplete" class="warning-message">
            <i class="material-icons">warning</i>
            <div>
              <strong>{{ $t('fileSizeAnalyzer.incompleteResults') }}</strong> {{ incompleteReason }}
            </div>
          </div>
          <!-- Show complete/maxGroups warning if no timeout -->
          <div v-else-if="duplicateGroups.length < maxGroups" class="success-message">
            <i class="material-icons">check_circle</i>
            <div>
              <strong>{{ $t('fileSizeAnalyzer.completeResults') }}</strong>
            </div>
          </div>
          <div v-else class="warning-message">
            <i class="material-icons">warning</i>
            <div>
              <strong>{{ $t('fileSizeAnalyzer.incompleteResults') }}</strong> {{ $t('messages.incompleteResultsDetails', { max: maxGroups }) }}
            </div>
          </div>

                <div class="duplicate-groups">
                  <div v-for="(group, index) in duplicateGroups" :key="index" class="duplicate-group">
                    <div class="group-header">
                      <span class="group-title">{{ $t('general.group') }} {{ index + 1 }}</span>
                      <span class="group-size">{{ getGroupSizeText(group) }}</span>
                      <span class="wasted-space">{{ $t('duplicateFinder.wasted', { suffix: ': ' }) }} {{ humanSize(group.size * (group.count - 1)) }}</span>
                    </div>
                    <div class="group-files">
                      <ListingItem
                        v-for="(file, fileIndex) in group.files"
                        :key="`${index}-${fileIndex}`"
                        :name="getFileName(file.path)"
                        :isDir="file.type === 'directory'"
                        :source="selectedSource"
                        :type="file.type"
                        :size="file.size"
                        :modified="file.modified"
                        :index="getUniqueIndex(index, fileIndex)"
                        :path="getFullPath(file.path)"
                        :hasPreview="file.hasPreview"
                        :reducedOpacity="false"
                        :displayFullPath="true"
                        @click="handleFileClick($event, file, index, fileIndex)"
                      />
                    </div>
                  </div>
                </div>
        </div>
      </div>

      <div v-else-if="!loading" class="empty-state">
        <i class="material-icons">content_copy</i>
        <p>{{ $t('duplicateFinder.emptyState') }}</p>
      </div>
    </div>
  </div>
</template>

<script>
import { findDuplicates } from "@/api/search";
import { state, mutations } from "@/store";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { eventBus } from "@/store/eventBus";
import ListingItem from "@/components/files/ListingItem.vue";
import * as url from "@/utils/url";

export default {
  name: "DuplicateFinder",
  components: {
    ListingItem,
  },
  props: {
    path: {
      type: String,
      default: "/",
    },
    source: {
      type: String,
      default: "",
    },
    minSize: {
      type: Number,
      default: 1,
    },
    useChecksums: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {
      searchPath: "/",
      selectedSource: "",
      minSizeValue: 1,
      useChecksumsValue: false, // Always false - checksum feature disabled for performance
      loading: false,
      error: null,
      duplicateGroups: [],
      isIncomplete: false, // Track if results are incomplete due to timeout/limits
      incompleteReason: "", // Reason for incomplete results
      isInitializing: true,
      lastRequestTime: 0, // Track last request to prevent rapid-fire
      clickTracker: {}, // Track clicks for double-click detection
      maxGroups: 500,
    };
  },
  computed: {
    sourceInfo() {
      return state.sources.info || {};
    },
    selectedIndexes() {
      // Force reactivity by accessing state.selected
      return state.selected;
    },
    totalWastedSpace() {
      return this.duplicateGroups.reduce((sum, group) => {
        // Wasted space = size × (count - 1)
        // We keep one copy, so the rest is wasted
        return sum + (group.size * (group.count - 1));
      }, 0);
    },
    totalItems() {
      return this.duplicateGroups.reduce((sum, group) => {
        return sum + (group.files ? group.files.length : group.count);
      }, 0);
    },
  },
  watch: {
    searchPath() {
      if (!this.isInitializing) {
        this.updateUrl();
      }
    },
    selectedSource() {
      if (!this.isInitializing) {
        this.updateUrl();
      }
    },
    minSizeValue() {
      if (!this.isInitializing) {
        this.updateUrl();
      }
    },
    '$route.query'() {
      if (!this.isInitializing) {
        this.initializeFromQuery();
      }
    },
  },
  mounted() {
    this.initializeFromQuery();
    // Set default source if not provided
    if (!this.selectedSource) {
      if (state.sources.current) {
        this.selectedSource = state.sources.current;
      } else if (Object.keys(this.sourceInfo).length > 0) {
        this.selectedSource = Object.keys(this.sourceInfo)[0];
      }
    }
    // Mark initialization as complete
    this.isInitializing = false;
    this.updateUrl();

    // Listen for path selection
    eventBus.on('pathSelected', this.handlePathSelected);
  },
  beforeUnmount() {
    eventBus.off('pathSelected', this.handlePathSelected);
  },
  methods: {
    openPathPicker() {
      mutations.showHover({
        name: "pathPicker",
        props: {
          currentPath: this.searchPath,
          currentSource: this.selectedSource,
        }
      });
    },
    handlePathSelected(data) {
      if (data && data.path !== undefined) {
        this.searchPath = data.path;
      }
      if (data && data.source !== undefined) {
        this.selectedSource = data.source;
      }
      mutations.closeHovers();
    },
    async fetchData() {
      // Prevent rapid-fire requests - require at least 1 second between requests
      const now = Date.now();
      if (this.loading) {
        return; // Already loading
      }
      if (now - this.lastRequestTime < 1000) {
        console.log("Duplicate search throttled - please wait before searching again");
        return;
      }

      this.loading = true;
      this.error = null;
      this.lastRequestTime = now;

      try {
        // API now expects minSizeMb directly (in megabytes)
        // Always use false for checksums due to performance issues
        const result = await findDuplicates(
          this.searchPath,
          this.selectedSource,
          this.minSizeValue,
          false // Checksum disabled
        );

        // Handle new response format with incomplete metadata
        this.duplicateGroups = result.groups || [];
        this.isIncomplete = result.incomplete || false;
        this.incompleteReason = result.reason || "";

        // Reset selection when new results arrive
        mutations.resetSelected();
      } catch (err) {
        this.error = err.message || "Failed to find duplicates";
        this.duplicateGroups = [];
        this.isIncomplete = false;
        this.incompleteReason = "";
      } finally {
        this.loading = false;
      }
    },
    initializeFromQuery() {
      const query = this.$route.query;

      if (query.path !== undefined && query.path !== null) {
        this.searchPath = String(query.path);
      } else if (this.path) {
        this.searchPath = this.path;
      }

      if (query.source !== undefined && query.source !== null) {
        this.selectedSource = String(query.source);
      } else if (this.source) {
        this.selectedSource = this.source;
      }

      if (query.minSize !== undefined && query.minSize !== null) {
        const parsed = parseInt(String(query.minSize), 10);
        if (!isNaN(parsed)) {
          this.minSizeValue = parsed;
        }
      } else if (this.minSize !== undefined) {
        this.minSizeValue = this.minSize;
      }

      // Checksum feature removed - always false
    },
    updateUrl() {
      this.$nextTick(() => {
        const query = {};

        if (this.searchPath && this.searchPath !== "/") {
          query.path = this.searchPath;
        }

        if (this.selectedSource) {
          query.source = this.selectedSource;
        }

        if (this.minSizeValue !== 1) {
          query.minSize = String(this.minSizeValue);
        }

        // Checksum feature removed

        const newQueryString = new URLSearchParams(query).toString();
        const currentQuery = this.$route.query || {};
        const currentQueryString = new URLSearchParams(
          Object.entries(currentQuery).reduce((acc, [key, value]) => {
            if (value !== null && value !== undefined) {
              acc[key] = String(value);
            }
            return acc;
          }, {})
        ).toString();

        if (newQueryString !== currentQueryString) {
          this.$router.replace({
            path: this.$route.path,
            query: Object.keys(query).length > 0 ? query : undefined,
          }).catch(() => {});
        }
      });
    },
    humanSize(size) {
      return getHumanReadableFilesize(size);
    },
    getGroupSizeText(group) {
      return `${this.humanSize(group.size)} × ${group.count} = ${this.humanSize(group.size * group.count)}`;
    },
    getFileName(path) {
      const parts = path.split("/").filter(p => p);
      return parts[parts.length - 1] || path;
    },
    ensureLeadingSlash(path) {
      // Ensure path starts with / for proper URL generation in ListingItem
      return path.startsWith('/') ? path : '/' + path;
    },
    getFullPath(filePath) {
      // Ensure the file path includes the full path from root
      // If searching in a subpath, the backend may return relative paths
      const searchPath = this.searchPath || '/';

      // Normalize search path (ensure trailing slash is removed)
      const normalizedSearchPath = searchPath === '/' || searchPath === ''
        ? ''
        : (searchPath.endsWith('/') ? searchPath.slice(0, -1) : searchPath);

      // Check if the path already includes the search path prefix
      if (filePath.startsWith(normalizedSearchPath + '/') ||
          filePath === normalizedSearchPath ||
          (normalizedSearchPath === '' && filePath.startsWith('/'))) {
        // Path already has full context, just ensure leading slash
        return this.ensureLeadingSlash(filePath);
      }

      // Path is relative to search path - prepend it
      if (normalizedSearchPath === '') {
        // Searching from root
        return this.ensureLeadingSlash(filePath);
      }

      // Remove leading slash from file path if present before combining
      const cleanFilePath = filePath.startsWith('/') ? filePath.substring(1) : filePath;
      return normalizedSearchPath + '/' + cleanFilePath;
    },
    getUniqueIndex(groupIndex, fileIndex) {
      // Create a unique index for each file across all groups
      // This ensures selections work correctly even with multiple groups
      return groupIndex * 1000 + fileIndex;
    },
    handleFileClick(event, file, groupIndex, fileIndex) {
      // Prevent default ListingItem navigation since state.req isn't populated
      event.preventDefault();
      event.stopPropagation();

      // Respect single-click vs double-click setting
      if (event.button === 0) {
        const quickNav = state.user.singleClick && !state.multiple;

        if (quickNav) {
          // Single-click navigation enabled - go immediately
          this.navigateToFile(file);
        } else {
          // Double-click navigation - select on first click, navigate on second

          // First click always selects the item using state.selected for proper CSS styling
          const uniqueIndex = this.getUniqueIndex(groupIndex, fileIndex);
          mutations.resetSelected();
          mutations.addSelected(uniqueIndex);

          // Track clicks for double-click detection
          if (!this.clickTracker) {
            this.clickTracker = {};
          }

          const fileKey = file.path;
          if (!this.clickTracker[fileKey]) {
            this.clickTracker[fileKey] = { count: 0, timeout: null };
          }

          const tracker = this.clickTracker[fileKey];
          tracker.count++;

          if (tracker.count >= 2) {
            // Double-click detected - navigate
            this.navigateToFile(file);
            tracker.count = 0;
            if (tracker.timeout) {
              clearTimeout(tracker.timeout);
              tracker.timeout = null;
            }
          } else {
            // First click - wait for potential second click
            if (tracker.timeout) {
              clearTimeout(tracker.timeout);
            }
            tracker.timeout = setTimeout(() => {
              tracker.count = 0;
              tracker.timeout = null;
            }, 500);
          }
        }
      }
    },
    navigateToFile(file) {
      const previousHistoryItem = {
        name: "Duplicate Finder",
        source: this.selectedSource,
        path: this.$route.path,
      };

      // Get the full path including search path context
      const filePath = this.getFullPath(file.path);
      url.goToItem(this.selectedSource, filePath, previousHistoryItem);
    },
  },
};
</script>

<style scoped>
.duplicate-finder {
  padding: 2rem;
  max-width: 100%;
  margin: 0 auto;
}

.duplicate-finder-results {
  max-width: 1200px;
  margin-bottom: 2em;
}

.button {
  margin-top: 1rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.button .material-icons {
  font-size: 1.2rem;
}

.error-message {
  background: #fee;
  color: #c33;
  padding: 1rem;
  border-radius: 4px;
  margin-bottom: 1rem;
  border: 1px solid #fcc;
}

.hint {
  font-size: 0.85rem;
  color: var(--textSecondary);
  margin-top: 0.25rem;
  margin-bottom: 1rem;
}

.stats {
  display: flex;
  gap: 2rem;
  margin-bottom: 1.5rem;
  padding: 1rem;
  background: var(--surfaceSecondary);
  border-radius: 4px;
}

.duplicate-groups {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.duplicate-group {
  border: 1px solid var(--borderPrimary);
  border-radius: 4px;
  overflow: hidden;
}

.group-header {
  background: var(--surfaceSecondary);
  padding: 1rem;
  display: flex;
  align-items: center;
  gap: 1rem;
  border-bottom: 1px solid var(--borderPrimary);
}

.group-title {
  font-weight: 600;
  font-size: 1rem;
}

.group-size {
  color: var(--textSecondary);
  font-size: 0.9rem;
}

.wasted-space {
  margin-left: auto;
  color: #f5576c;
  font-weight: 600;
  font-size: 0.9rem;
}

.group-files {
  display: flex;
  flex-direction: column;
  padding: 0;
}

.group-files .listing-item {
  width: 100%;
  margin: 0;
  border: 1px solid rgba(0, 0, 0, 0.1);
  border-top: 0;
  padding: 0.5em;
  border-radius: 0;
}

.group-files .listing-item:first-child {
  border-top: 1px solid rgba(0, 0, 0, 0.1);
}

.group-files .listing-item.last-item {
  border-bottom-left-radius: 1em;
  border-bottom-right-radius: 1em;
}

.path-text {
  font-size: 0.75rem !important;
  color: var(--textSecondary) !important;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.empty-state {
  text-align: center;
  padding: 4rem 2rem;
  color: var(--textSecondary);
}

.empty-state .material-icons {
  font-size: 4rem;
  opacity: 0.3;
  margin-bottom: 1rem;
}

.empty-state p {
  font-size: 1.1rem;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }

  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 768px) {
  .duplicate-finder {
    padding: 1rem;
  }

  .stats {
    flex-direction: column;
    gap: 0.5rem;
  }

  .group-header {
    flex-wrap: wrap;
  }

  .wasted-space {
    margin-left: 0;
    width: 100%;
  }
}
</style>

