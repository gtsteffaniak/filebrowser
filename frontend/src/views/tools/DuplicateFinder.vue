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

        <ToggleSwitch v-model="useChecksumsValue" :name="$t('duplicateFinder.useChecksums')"
          :description="$t('duplicateFinder.useChecksumsDescription')" aria-label="Use checksums toggle" />

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

                <div class="duplicate-groups">
                  <div v-for="(group, index) in duplicateGroups" :key="index" class="duplicate-group">
                    <div class="group-header">
                      <span class="group-title">{{ $t('general.group') }} {{ index + 1 }}</span>
                      <span class="group-size">{{ getGroupSizeText(group) }}</span>
                      <span class="wasted-space">{{ $t('duplicateFinder.wasted', { suffix: ': ' }) }} {{ humanSize(group.size * (group.count - 1)) }}</span>
                    </div>
                    <div class="group-files">
                      <div v-for="(file, fileIndex) in group.files" :key="fileIndex"
                           class="item listing-item clickable"
                           :class="{ 'last-item': fileIndex === group.files.length - 1 }"
                           @click="handleFileClick(file)">
                        <div>
                          <Icon
                            :mimetype="file.type"
                            :filename="getFileName(file.path)"
                            :hasPreview="true"
                            :thumbnailUrl="getThumbnailUrl(file)"
                          />
                        </div>
                        <div class="text">
                          <p class="name">{{ getFileName(file.path) }}</p>
                          <p class="size">{{ humanSize(file.size) }}</p>
                          <p class="modified path-text">{{ getFullPath(file.path) }}</p>
                        </div>
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
import { findDuplicates } from "@/api/search";
import { filesApi } from "@/api";
import { state, mutations } from "@/store";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { getTypeInfo } from "@/utils/mimetype";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { eventBus } from "@/store/eventBus";
import Icon from "@/components/files/Icon.vue";
import { globalVars } from "@/utils/constants";
import * as url from "@/utils/url";

export default {
  name: "DuplicateFinder",
  components: {
    ToggleSwitch,
    Icon,
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
      useChecksumsValue: false,
      loading: false,
      error: null,
      duplicateGroups: [],
      isInitializing: true,
    };
  },
  computed: {
    sourceInfo() {
      return state.sources.info || {};
    },
    totalWastedSpace() {
      return this.duplicateGroups.reduce((sum, group) => {
        // Wasted space = size × (count - 1)
        // We keep one copy, so the rest is wasted
        return sum + (group.size * (group.count - 1));
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
    useChecksumsValue() {
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
      this.loading = true;
      this.error = null;

      try {
        // API now expects minSizeMb directly (in megabytes)
        this.duplicateGroups = await findDuplicates(
          this.searchPath,
          this.selectedSource,
          this.minSizeValue,
          this.useChecksumsValue
        );
        console.log('[DuplicateFinder] Fetched duplicate groups:', this.duplicateGroups);
        console.log('[DuplicateFinder] First file in first group:', this.duplicateGroups[0]?.files[0]);
      } catch (err) {
        this.error = err.message || "Failed to find duplicates";
        this.duplicateGroups = [];
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

      if (query.useChecksums !== undefined && query.useChecksums !== null) {
        const value = String(query.useChecksums);
        this.useChecksumsValue = value === 'true' || value === '1';
      } else if (this.useChecksums !== undefined) {
        this.useChecksumsValue = this.useChecksums;
      }
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

        if (this.useChecksumsValue) {
          query.useChecksums = 'true';
        }

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
    getFullPath(itemPath) {
      let basePath = this.searchPath || "/";

      if (basePath !== "/" && !basePath.endsWith("/")) {
        basePath += "/";
      }

      let relativePath = itemPath.startsWith("/") ? itemPath.slice(1) : itemPath;

      let fullPath = basePath === "/" ? "/" + relativePath : basePath + relativePath;

      fullPath = fullPath.replace(/\/+/g, "/");
      if (!fullPath.startsWith("/")) {
        fullPath = "/" + fullPath;
      }

      return fullPath;
    },
    handleFileClick(file) {
      const previousHistoryItem = {
        name: "Duplicate Finder",
        source: this.selectedSource,
        path: this.$route.path,
      };
      url.goToItem(this.selectedSource, file.path, previousHistoryItem);
    },
    canHavePreview(mimetype) {
      // Check if this file type can have a preview/thumbnail
      const typeInfo = getTypeInfo(mimetype);
      const simpleType = typeInfo.simpleType;
      
      // Only these types can have previews
      return simpleType === 'image' || 
             simpleType === 'video' || 
             simpleType === 'document' || 
             simpleType === 'text' ||
             simpleType === 'directory';
    },
    getFileIcon(type) {
      const typeInfo = getTypeInfo(type);
      const iconMap = {
        "video": "movie",
        "image": "image",
        "audio": "audiotrack",
        "archive": "archive",
        "pdf": "picture_as_pdf",
        "document": "description",
        "text": "description",
        "binary": "settings",
        "font": "font_download",
      };
      return iconMap[typeInfo.simpleType] || "insert_drive_file";
    },
    getGroupSizeText(group) {
      return `${this.humanSize(group.size)} × ${group.count} = ${this.humanSize(group.size * group.count)}`;
    },
    getFileMetaText(file) {
      return `${this.humanSize(file.size)} • ${file.type}`;
    },
    getFileName(path) {
      const parts = path.split("/").filter(p => p);
      return parts[parts.length - 1] || path;
    },
        getThumbnailUrl(file) {
          if (!globalVars.enableThumbs) {
            return "";
          }

          // Don't generate URLs for files that can't have previews
          if (!this.canHavePreview(file.type)) {
            return "";
          }

          // Use file.path directly - getPreviewURL expects path relative to source
          // Don't use getFullPath which adds the source prefix
          const url = filesApi.getPreviewURL(this.selectedSource, file.path, file.modified || new Date().toISOString());
          return url;
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

.searchContext {
  margin-bottom: 1em;
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

