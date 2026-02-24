<template>
  <div class="duplicate-finder">
    <div class="card duplicate-finder-config padding-normal">
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

    <div class="card duplicate-finder-results padding-normal">
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
                      <div
                        v-for="(file, fileIndex) in group.files"
                        :key="`${index}-${fileIndex}`"
                        class="file-item-wrapper"
                        :class="{ 'deleted': isDeleted(file), 'failed': isFailed(file) }"
                      >
                        <ListingItem
                          :name="getFileName(file.path)"
                          :isDir="file.type === 'directory'"
                          :source="file.source"
                          :type="file.type"
                          :size="file.size"
                          :modified="file.modified"
                          :index="getUniqueIndex(index, fileIndex)"
                          :path="getFullPath(file.path)"
                          :hasPreview="shouldHavePreview(file)"
                          :reducedOpacity="isDeleted(file)"
                          :displayFullPath="true"
                          :updateGlobalState="false"
                          :isSelectedProp="selectedIndices.has(getUniqueIndex(index, fileIndex))"
                          :clickable="false"
                          @select="handleItemSelect"
                          @clearSelection="clearSelection"
                          @selectRange="handleSelectRange"
                        />
                      </div>
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
import { toolsApi, resourcesApi } from "@/api";
import { state, mutations } from "@/store";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { eventBus } from "@/store/eventBus";
import ListingItem from "@/components/files/ListingItem.vue";
import * as url from "@/utils/url";
import { getTypeInfo } from "@/utils/mimetype";

export default {
  name: "DuplicateFinder",
  components: {
    ListingItem,
  },
  data() {
    return {
      searchPath: "/",
      selectedSource: "",
      minSizeValue: 1,
      loading: false,
      error: null,
      duplicateGroups: [],
      isIncomplete: false,
      incompleteReason: "",
      lastRequestTime: 0,
      maxGroups: 500,
      selectedIndices: new Set(),
      deletedFiles: new Set(),
      failedFiles: new Map(),
      deleting: false,
    };
  },
  computed: {
    sourceInfo() {
      return state.sources.info || {};
    },
    selectedCount() {
      return this.selectedIndices.size;
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
      this.updateUrl();
    },
    selectedSource() {
      this.updateUrl();
    },
    minSizeValue() {
      this.updateUrl();
    },
  },
  mounted() {
    // Initialize from URL query params or use defaults
    const query = this.$route.query;
    
    this.searchPath = (typeof query.path === 'string' ? query.path : null) || "/";
    this.selectedSource = (typeof query.source === 'string' ? query.source : null) || state.sources.current || Object.keys(this.sourceInfo)[0] || "";
    
    if (query.minSize) {
      const parsed = parseInt(String(query.minSize), 10);
      if (!isNaN(parsed)) {
        this.minSizeValue = parsed;
      }
    }

    // Listen for events
    eventBus.on('pathSelected', this.handlePathSelected);
    eventBus.on('itemsDeleted', this.handleItemsDeleted);
    eventBus.on('duplicateFinderDeleteRequested', this.showDeleteConfirm);
    eventBus.on('duplicateFinderClearRequested', this.clearSelection);
  },
  beforeUnmount() {
    // Clear local selection when leaving
    this.selectedIndices.clear();
    
    eventBus.off('pathSelected', this.handlePathSelected);
    eventBus.off('itemsDeleted', this.handleItemsDeleted);
    eventBus.off('duplicateFinderDeleteRequested', this.showDeleteConfirm);
    eventBus.off('duplicateFinderClearRequested', this.clearSelection);
    
    // Notify Files.vue that selection is cleared
    eventBus.emit('duplicateFinderSelectionChanged', 0);
    eventBus.emit('duplicateFinderDeletingChanged', false);
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
      mutations.closeTopHover();
    },
    handleItemsDeleted(data) {
      // Update local state when items are deleted from the delete prompt
      if (data && data.succeeded) {
        data.succeeded.forEach(item => {
          const key = `${item.source}::${item.path}`;
          this.deletedFiles.add(key);
          this.failedFiles.delete(key);
        });
      }
      if (data && data.failed) {
        data.failed.forEach(item => {
          const key = `${item.source}::${item.path}`;
          this.failedFiles.set(key, item.message || 'Unknown error');
        });
      }
      // Clear selection after processing
      this.selectedIndices.clear();
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

        // Reset selection, deleted, and failed files when new results arrive
        this.selectedIndices.clear();
        this.deletedFiles.clear();
        this.failedFiles.clear();
      } catch (err) {
        this.error = err.message || "Failed to find duplicates";
        this.duplicateGroups = [];
        this.isIncomplete = false;
        this.incompleteReason = "";
        this.selectedIndices.clear();
        this.deletedFiles.clear();
        this.failedFiles.clear();
      } finally {
        this.loading = false;
      }
    },
    updateUrl() {
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

      this.$router.replace({
        path: this.$route.path,
        query: Object.keys(query).length > 0 ? query : undefined,
      }).catch(() => {});
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
    shouldHavePreview(file) {
      // Compute hasPreview based on file type if backend doesn't provide it
      if (file.hasPreview || file.HasPreview) {
        return true;
      }
      // Check if file type supports previews using getTypeInfo
      const type = file.type || '';
      const typeInfo = getTypeInfo(type);
      const simpleType = typeInfo.simpleType;
      
      // Files that typically have previews
      if (simpleType === 'image' || simpleType === 'video') {
        return true;
      }
      // Office files and PDFs
      if (simpleType === 'document' || simpleType === 'text' || type.includes('pdf')) {
        return true;
      }
      return false;
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
    getFileKey(file) {
      // Create a unique key for the file based on source and path
      return `${this.selectedSource}::${this.getFullPath(file.path)}`;
    },
    handleItemSelect({ index }) {
      // Toggle selection - if already selected, remove it; if not selected, add it
      if (this.selectedIndices.has(index)) {
        this.selectedIndices.delete(index);
      } else {
        this.selectedIndices.add(index);
      }
      // Notify Files.vue of selection change
      eventBus.emit('duplicateFinderSelectionChanged', this.selectedIndices.size);
    },
    handleSelectRange({ startIndex, endIndex }) {
      // Select all indices between start and end
      const start = Math.min(startIndex, endIndex);
      const end = Math.max(startIndex, endIndex);
      for (let i = start; i <= end; i++) {
        this.selectedIndices.add(i);
      }
      // Notify Files.vue of selection change
      eventBus.emit('duplicateFinderSelectionChanged', this.selectedIndices.size);
    },
    clearSelection() {
      this.selectedIndices.clear();
      // Notify Files.vue of selection change
      eventBus.emit('duplicateFinderSelectionChanged', 0);
    },
    isDeleted(file) {
      return this.deletedFiles.has(this.getFileKey(file));
    },
    isFailed(file) {
      return this.failedFiles.has(this.getFileKey(file));
    },
    getFailureMessage(file) {
      return this.failedFiles.get(this.getFileKey(file)) || '';
    },
    showDeleteConfirm() {
      if (this.selectedIndices.size === 0 || this.deleting) {
        return;
      }

      // Build items array with preview URLs from selected indices
      const items = [];
      for (const selectedIndex of this.selectedIndices) {
        // Find the file corresponding to this index
        for (const group of this.duplicateGroups) {
          for (let fileIndex = 0; fileIndex < group.files.length; fileIndex++) {
            const uniqueIndex = this.getUniqueIndex(this.duplicateGroups.indexOf(group), fileIndex);
            if (uniqueIndex === selectedIndex) {
              const file = group.files[fileIndex];
              const fullPath = this.getFullPath(file.path);
              const previewUrl = this.shouldHavePreview(file)
                ? resourcesApi.getPreviewURL(this.selectedSource, fullPath, file.modified)
                : null;
              items.push({
                source: this.selectedSource,
                path: fullPath,
                type: file.type,
                size: file.size,
                modified: file.modified,
                previewUrl: previewUrl,
              });
              break;
            }
          }
        }
      }

      mutations.showHover({
        name: "delete",
        props: {
          items: items,
        },
      });
    },
    async deleteSelected() {
      if (this.selectedIndices.size === 0 || this.deleting) {
        return;
      }

      this.deleting = true;
      // Notify Files.vue that deletion is in progress
      eventBus.emit('duplicateFinderDeletingChanged', true);
      
      const itemsToDelete = [];
      
      // Map selected indices to files
      for (const selectedIndex of this.selectedIndices) {
        // Find the file corresponding to this index
        for (const group of this.duplicateGroups) {
          for (let fileIndex = 0; fileIndex < group.files.length; fileIndex++) {
            const uniqueIndex = this.getUniqueIndex(this.duplicateGroups.indexOf(group), fileIndex);
            if (uniqueIndex === selectedIndex) {
              const file = group.files[fileIndex];
              itemsToDelete.push({
                source: this.selectedSource,
                path: this.getFullPath(file.path),
              });
              break;
            }
          }
        }
      }

      try {
        const response = await resourcesApi.bulkDelete(itemsToDelete);
        
        // Process succeeded items
        if (response.succeeded && response.succeeded.length > 0) {
          response.succeeded.forEach(item => {
            const key = `${item.source}::${item.path}`;
            this.deletedFiles.add(key);
            this.failedFiles.delete(key);
          });
        }

        // Process failed items
        if (response.failed && response.failed.length > 0) {
          response.failed.forEach(item => {
            const key = `${item.source}::${item.path}`;
            this.failedFiles.set(key, item.message || 'Unknown error');
          });
        }

        // Clear selection after deletion attempt
        this.selectedIndices.clear();
        eventBus.emit('duplicateFinderSelectionChanged', 0);
      } catch (err) {
        this.error = err.message || 'Failed to delete files';
      } finally {
        this.deleting = false;
        eventBus.emit('duplicateFinderDeletingChanged', false);
      }
    },
  },
};
</script>

<style scoped>
.duplicate-finder {
  max-width: 1200px;
  margin-left: auto;
  margin-right: auto;
  padding: 1em;
}

.duplicate-finder-results {
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

.file-item-wrapper {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  border: 1px solid rgba(0, 0, 0, 0.1);
  border-top: 0;
  padding: 0.5em;
  border-radius: 0;
}

.file-item-wrapper:first-child {
  border-top: 1px solid rgba(0, 0, 0, 0.1);
}

.file-item-wrapper.deleted {
  opacity: 0.5;
  background: var(--surfaceSecondary);
}

.file-item-wrapper.failed {
  border-left: 3px solid #f5576c;
}

.file-item-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}


.group-files .listing-item {
  width: 100%;
  margin: 0.25em;
  cursor: pointer;
}

/* Highlight selected items */
.group-files .listing-item.activebutton {
  background: var(--primaryColor) !important;
  color: #fff !important;
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

