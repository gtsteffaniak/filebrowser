<template>
  <div class="size-viewer">
    <div class="card size-viewer-config padding-normal">
      <div class="card-content">
        <h3>{{ $t('general.source') }}</h3>
        <select v-model="selectedSource" class="input">
          <option v-for="(info, name) in sourceInfo" :key="name" :value="name">
            {{ name }}
          </option>
        </select>

        <h3>{{ $t('general.path') }}</h3>
        <div aria-label="size-viewer-path" class="searchContext clickable button" @click="openPathPicker">
          {{ $t('general.path', { suffix: ':' }) }} {{ searchPath }}
        </div>

        <h3>{{ $t('fileSizeAnalyzer.largerThan') }}</h3>
        <input v-model.number="largerThanValue" type="number" min="0" placeholder="100" class="input" />

        <ToggleSwitch v-model="includeFoldersValue" :name="$t('fileSizeAnalyzer.includeFolders')"
          :description="$t('fileSizeAnalyzer.includeFoldersDescription')" aria-label="Include folders toggle" />

        <button @click="fetchData" class="button" :disabled="loading">
          <i v-if="loading" class="material-icons spin">autorenew</i>
          <span v-else>{{ $t('general.analyze') }}</span>
        </button>
      </div>
    </div>

    <div class="card size-viewer-results padding-normal">
      <div v-if="error" class="error-message">
        {{ error }}
      </div>

      <div v-if="results.length > 0">
        <div class="card-title">
          <h2>{{ $t('general.results') }}</h2>
        </div>
        <div class="card-content">
          <div class="stats">
            <span>{{ $t('fileSizeAnalyzer.totalFiles', { suffix: ': ' }) }}<strong>{{ results.length }}</strong></span>
            <span>{{ $t('fileSizeAnalyzer.totalSize', { suffix: ': ' }) }}<strong>{{ humanSize(totalSize) }}</strong></span>
          </div>

          <div v-if="results.length < 100" class="success-message">
            <i class="material-icons">check_circle</i>
            <div>
              <strong>{{ $t('fileSizeAnalyzer.completeResults') }}</strong>
            </div>
          </div>
          <div v-else class="warning-message">
            <i class="material-icons">warning</i>
            <div>
              <strong>{{ $t('fileSizeAnalyzer.incompleteResults') }}</strong> {{ $t('messages.incompleteResultsDetails', { max: 100 }) }}
            </div>
          </div>

          <div class="treemap" ref="treemap" :class="{ 'has-hover': hoveredItem !== null }">
            <div v-for="(rect, index) in treemapRects" :key="index">
              <!-- Invisible hit area at original position - handles mouse events -->
              <div :class="['treemap-hit-area', { 'active': hoveredItem === rect.item }]" :style="getRectStyle(rect)"
                @mouseenter="handleMouseEnter(rect.item)" @mouseleave="handleMouseLeave"
                @click="handleItemClick(rect.item)">
              </div>

              <!-- Visual item - moves to center when hovered -->
              <div :class="['treemap-item', getTypeClass(rect.item.type), {
                'small-item': isSmallItem(rect.item),
                'hovered': hoveredItem === rect.item,
                'dimmed': hoveredItem !== null && hoveredItem !== rect.item
              }]" :style="getRectStyle(rect)">
                <div class="item-content" v-if="!isSmallItem(rect.item)">
                  <div class="item-name">{{ getDisplayPath(rect.item.path) }}</div>
                  <div class="item-size">{{ humanSize(rect.item.size) }}</div>
                  <div class="item-percentage">{{ $t('fileSizeAnalyzer.percentageOfResults', { percentage: getPercentage(rect.item.size) }) }}</div>
                </div>
                <!-- Expanded hover content - shows when hovered after delay -->
                <div class="item-expanded" v-if="hoveredItem === rect.item">
                  <div class="expanded-field">
                    <span class="field-label">{{ $t('general.name', { suffix: ':' }) }}</span>
                    <span class="field-value">{{ getDisplayPath(rect.item.path) }}</span>
                  </div>
                  <div class="expanded-field">
                    <span class="field-label">{{ $t('general.path', { suffix: ':' }) }}</span>
                    <span class="field-value">{{ getFullPath(rect.item.path) }}</span>
                  </div>
                  <div class="expanded-field">
                    <span class="field-label">{{ $t('general.size', { suffix: ':' }) }}</span>
                    <span class="field-value">{{ humanSize(rect.item.size) }}</span>
                  </div>
                  <div class="expanded-field">
                    <span class="field-label">{{ $t('general.type', { suffix: ':' }) }}</span>
                    <span class="field-value">{{ getTypeLabel(rect.item.type) }}</span>
                  </div>
                  <div class="expanded-field">
                    <span class="field-label">{{ $t('fileSizeAnalyzer.relativePercentage', { suffix: ':' }) }}</span>
                    <span class="field-value">{{ $t('general.percentage', { percentage: getPercentage(rect.item.size) }) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <h4>{{ $t('general.types') }}</h4>
          <div class="legend">
            <div class="legend-item">
              <span class="legend-color type-video"></span>
              <span>{{ $t('fileTypes.video') }}</span>
            </div>
            <div class="legend-item">
              <span class="legend-color type-image"></span>
              <span>{{ $t('fileTypes.image') }}</span>
            </div>
            <div class="legend-item">
              <span class="legend-color type-audio"></span>
              <span>{{ $t('fileTypes.audio') }}</span>
            </div>
            <div class="legend-item">
              <span class="legend-color type-archive"></span>
              <span>{{ $t('fileTypes.archive') }}</span>
            </div>
            <div class="legend-item">
              <span class="legend-color type-document"></span>
              <span>{{ $t('fileTypes.document') }}</span>
            </div>
            <div class="legend-item">
              <span class="legend-color type-binary"></span>
              <span>{{ $t('fileTypes.binary') }}</span>
            </div>
            <div v-if="includeFoldersValue" class="legend-item">
              <span class="legend-color type-directory"></span>
              <span>{{ $t('fileTypes.directory') }}</span>
            </div>
            <div class="legend-item">
              <span class="legend-color type-other"></span>
              <span>{{ $t('fileTypes.other') }}</span>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="!loading" class="empty-state">
        <i class="material-icons">analytics</i>
        <p>{{ $t('fileSizeAnalyzer.emptyState') }}</p>
      </div>
    </div>
  </div>
</template>

<script>
import { toolsApi } from "@/api";
import { state, mutations } from "@/store";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { getTypeInfo } from "@/utils/mimetype";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { eventBus } from "@/store/eventBus";
import { goToItem } from "@/utils/url";

export default {
  name: "SizeViewer",
  components: {
    ToggleSwitch,
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
    largerThan: {
      type: Number,
      default: 100,
    },
    includeFolders: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {
      searchPath: "/",
      selectedSource: "",
      largerThanValue: 100,
      includeFoldersValue: false,
      loading: false,
      error: null,
      results: [],
      isInitializing: true,
      hoveredItem: null,
      hoverTimeout: null,
    };
  },
  computed: {
    sourceInfo() {
      return state.sources.info || {};
    },
    sortedResults() {
      // Sort by size descending for better treemap layout
      return [...this.results].sort((a, b) => b.size - a.size);
    },
    totalSize() {
      return this.results.reduce((sum, item) => sum + item.size, 0);
    },
    treemapRects() {
      if (this.sortedResults.length === 0) return [];
      // Calculate treemap layout
      const width = 1000; // Base width
      const height = 600; // Base height
      return this.squarify(this.sortedResults, { x: 0, y: 0, width, height });
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
    largerThanValue() {
      if (!this.isInitializing) {
        this.updateUrl();
      }
    },
    includeFoldersValue() {
      if (!this.isInitializing) {
        this.updateUrl();
      }
    },
    // Watch route query params in case URL changes externally
    '$route.query'() {
      if (!this.isInitializing) {
        this.initializeFromQuery();
      }
    },
  },
  mounted() {
    this.initializeFromQuery();
    // Set default source if not provided via props or query
    if (!this.selectedSource) {
      if (state.sources.current) {
        this.selectedSource = state.sources.current;
      } else if (Object.keys(this.sourceInfo).length > 0) {
        this.selectedSource = Object.keys(this.sourceInfo)[0];
      }
    }
    // Mark initialization as complete and sync URL
    this.isInitializing = false;
    this.updateUrl();

    // Listen for path selection from FileList picker
    eventBus.on('pathSelected', this.handlePathSelected);
  },
  beforeUnmount() {
    // Clean up timeout
    if (this.hoverTimeout) {
      clearTimeout(this.hoverTimeout);
      this.hoverTimeout = null;
    }
    // Clean up event listener
    eventBus.off('pathSelected', this.handlePathSelected);
  },
  methods: {
    openPathPicker() {
      // Open FileList picker to select path and source
      mutations.showHover({
        name: "pathPicker",
        props: {
          currentPath: this.searchPath,
          currentSource: this.selectedSource,
        }
      });
    },
    handlePathSelected(data) {
      // Handle path selection from FileList picker
      if (data && data.path !== undefined) {
        this.searchPath = data.path;
      }
      if (data && data.source !== undefined) {
        this.selectedSource = data.source;
      }
      // Close the picker
      mutations.closeTopHover();
    },
    async fetchData() {
      this.loading = true;
      this.error = null;

      try {
        let query = `type:largerThan=${this.largerThanValue}`;
        if (!this.includeFoldersValue) {
          query += " type:file";
        }
        this.results = await search(
          this.searchPath,
          this.selectedSource,
          query,
          true // largest=true
        );
      } catch (err) {
        this.error = err.message || "Failed to fetch data";
        this.results = [];
      } finally {
        this.loading = false;
      }
    },
    initializeFromQuery() {
      // Priority: URL query params > props > defaults
      const query = this.$route.query;

      // Initialize searchPath: query > prop > default
      if (query.path !== undefined && query.path !== null) {
        this.searchPath = String(query.path);
      } else if (this.path) {
        this.searchPath = this.path;
      }

      // Initialize selectedSource: query > prop > default
      if (query.source !== undefined && query.source !== null) {
        this.selectedSource = String(query.source);
      } else if (this.source) {
        this.selectedSource = this.source;
      }

      // Initialize largerThanValue: query > prop > default
      if (query.largerThan !== undefined && query.largerThan !== null) {
        const parsed = parseInt(String(query.largerThan), 10);
        if (!isNaN(parsed)) {
          this.largerThanValue = parsed;
        }
      } else if (this.largerThan !== undefined) {
        this.largerThanValue = this.largerThan;
      }

      // Initialize includeFoldersValue: query > prop > default
      if (query.includeFolders !== undefined && query.includeFolders !== null) {
        const value = String(query.includeFolders);
        this.includeFoldersValue = value === 'true' || value === '1';
      } else if (this.includeFolders !== undefined) {
        this.includeFoldersValue = this.includeFolders;
      }
    },
    updateUrl() {
      // Use nextTick to avoid triggering updates during component lifecycle
      this.$nextTick(() => {
        // Update URL query parameters to reflect current state
        // This ensures refreshing the page will restore the same configuration
        const query = {};

        // Include path if it's not the default "/"
        if (this.searchPath && this.searchPath !== "/") {
          query.path = this.searchPath;
        }

        // Include source if set
        if (this.selectedSource) {
          query.source = this.selectedSource;
        }

        // Include largerThan if not the default value of 100
        if (this.largerThanValue !== 100) {
          query.largerThan = String(this.largerThanValue);
        }

        // Include includeFolders if true
        if (this.includeFoldersValue) {
          query.includeFolders = 'true';
        }

        // Build query string for comparison
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
          }).catch(() => {
            // Ignore navigation errors (e.g., if navigating to same route)
          });
        }
      });
    },
    isSmallItem(item) {
      // Calculate if item is too small to display text
      const percentage = (item.size / this.totalSize) * 100;
      // Items smaller than 1.5% of total are considered small
      return percentage < 1.5;
    },
    squarify(items, container) {
      if (items.length === 0) return [];

      const rects = [];
      const totalValue = items.reduce((sum, item) => sum + item.size, 0);

      // Normalize sizes
      const normalized = items.map(item => ({
        item,
        value: (item.size / totalValue) * container.width * container.height
      }));

      this.squarifyRecursive(normalized, [], container, rects);
      return rects;
    },
    squarifyRecursive(items, row, container, rects) {
      if (items.length === 0) {
        this.layoutRow(row, container, rects);
        return;
      }

      const item = items[0];
      const newRow = [...row, item];
      const remainingItems = items.slice(1);

      if (row.length === 0 || this.worst(newRow, container) <= this.worst(row, container)) {
        this.squarifyRecursive(remainingItems, newRow, container, rects);
      } else {
        this.layoutRow(row, container, rects);
        const newContainer = this.cutArea(row, container);
        this.squarifyRecursive(items, [], newContainer, rects);
      }
    },
    worst(row, container) {
      if (row.length === 0) return Infinity;

      const total = row.reduce((sum, item) => sum + item.value, 0);
      const min = Math.min(...row.map(item => item.value));
      const max = Math.max(...row.map(item => item.value));
      const w = Math.min(container.width, container.height);

      return Math.max(
        (w * w * max) / (total * total),
        (total * total) / (w * w * min)
      );
    },
    layoutRow(row, container, rects) {
      const total = row.reduce((sum, item) => sum + item.value, 0);
      const width = container.width;
      const height = container.height;

      if (width >= height) {
        // Horizontal layout
        const rowWidth = total / height;
        let y = container.y;

        row.forEach(item => {
          const itemHeight = item.value / rowWidth;
          rects.push({
            item: item.item,
            x: container.x,
            y: y,
            width: rowWidth,
            height: itemHeight
          });
          y += itemHeight;
        });
      } else {
        // Vertical layout
        const rowHeight = total / width;
        let x = container.x;

        row.forEach(item => {
          const itemWidth = item.value / rowHeight;
          rects.push({
            item: item.item,
            x: x,
            y: container.y,
            width: itemWidth,
            height: rowHeight
          });
          x += itemWidth;
        });
      }
    },
    cutArea(row, container) {
      const total = row.reduce((sum, item) => sum + item.value, 0);
      const width = container.width;
      const height = container.height;

      if (width >= height) {
        const rowWidth = total / height;
        return {
          x: container.x + rowWidth,
          y: container.y,
          width: width - rowWidth,
          height: height
        };
      } else {
        const rowHeight = total / width;
        return {
          x: container.x,
          y: container.y + rowHeight,
          width: width,
          height: height - rowHeight
        };
      }
    },
    getRectStyle(rect) {
      return {
        position: 'absolute',
        left: `${(rect.x / 1000) * 100}%`,
        top: `${(rect.y / 600) * 100}%`,
        width: `${(rect.width / 1000) * 100}%`,
        height: `${(rect.height / 600) * 100}%`,
      };
    },
    getTypeClass(type) {
      const typeInfo = getTypeInfo(type);
      const simpleType = typeInfo.simpleType;

      // Map simple types to CSS classes
      switch (simpleType) {
        case "directory":
          return "type-directory";
        case "video":
          return "type-video";
        case "image":
          return "type-image";
        case "audio":
          return "type-audio";
        case "archive":
          return "type-archive";
        case "pdf":
        case "document":
        case "text":
          return "type-document";
        case "binary":
          return "type-binary";
        case "font":
        default:
          return "type-other";
      }
    },
    getDisplayPath(path) {
      // Show just the filename/directory name for cleaner display
      const parts = path.split("/").filter(p => p);
      return parts[parts.length - 1] || path;
    },
    humanSize(size) {
      return getHumanReadableFilesize(size);
    },
    getFullPath(itemPath) {
      // Combine searchPath with the item's relative path
      let basePath = this.searchPath || "/";

      // Ensure basePath ends with / if it's not root
      if (basePath !== "/" && !basePath.endsWith("/")) {
        basePath += "/";
      }

      // Remove leading slash from itemPath if present (it's relative)
      let relativePath = itemPath.startsWith("/") ? itemPath.slice(1) : itemPath;

      // Combine paths
      let fullPath = basePath === "/" ? "/" + relativePath : basePath + relativePath;

      // Normalize: remove double slashes and ensure it starts with /
      fullPath = fullPath.replace(/\/+/g, "/");
      if (!fullPath.startsWith("/")) {
        fullPath = "/" + fullPath;
      }

      return fullPath;
    },
    handleItemClick(item) {
      // Navigate to the file/directory using full path
      const fullPath = this.getFullPath(item.path);
      goToItem(this.selectedSource, fullPath, {});
    },
    handleMouseEnter(item) {
      // Clear any existing timeout
      if (this.hoverTimeout) {
        clearTimeout(this.hoverTimeout);
        this.hoverTimeout = null;
      }

      // Set timeout to show hover after 500ms
      this.hoverTimeout = setTimeout(() => {
        this.hoveredItem = item;
        this.hoverTimeout = null;
      }, 500);
    },
    handleMouseLeave() {
      // Clear timeout and hover state
      if (this.hoverTimeout) {
        clearTimeout(this.hoverTimeout);
        this.hoverTimeout = null;
      }
      this.hoveredItem = null;
    },
    getPercentage(size) {
      if (this.totalSize === 0) return 0;
      const percentage = (size / this.totalSize) * 100;
      // Round to 1 decimal place, but show as integer if whole number
      return percentage < 0.1 ? percentage.toFixed(2) : percentage.toFixed(1).replace(/\.0$/, '');
    },
    getTypeLabel(type) {
      const typeInfo = getTypeInfo(type);
      const simpleType = typeInfo.simpleType;

      // Map simple types to labels
      const labels = {
        "directory": this.$t('fileTypes.directory'),
        "video": this.$t('fileTypes.video'),
        "image": this.$t('fileTypes.image'),
        "audio": this.$t('fileTypes.audio'),
        "archive": this.$t('fileTypes.archive'),
        "pdf": this.$t('fileTypes.document'),
        "document": this.$t('fileTypes.document'),
        "text": this.$t('fileTypes.document'),
        "binary": this.$t('fileTypes.binary'),
        "font": this.$t('fileTypes.other'),
      };

      return labels[simpleType] || this.$t('fileTypes.other');
    },
  },
};
</script>

<style scoped>
.size-viewer {
  max-width: 1200px;
  margin-left: auto;
  margin-right: auto;
  padding: 1em;
}

.toggle-container {
  padding: 1em;
}

.size-viewer-results {
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

.stats {
  display: flex;
  gap: 2rem;
  margin-bottom: 1rem;
}

.treemap {
  position: relative;
  margin-bottom: 2rem;
  width: 100%;
  height: 600px;
  background: var(--surfacePrimary);
  border-radius: 4px;
  overflow: hidden;
}

.treemap.has-hover {
  overflow: visible;
}

/* Invisible hit area that stays at original position */
.treemap-hit-area {
  position: absolute;
  z-index: 200;
  cursor: pointer;
  pointer-events: auto;
  background: transparent;
  border: none;
  outline: none;
}

.treemap-item {
  border: 1px solid rgba(0, 0, 0, 0.1);
  cursor: pointer;
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1), z-index 0s;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  overflow: visible;
  box-sizing: border-box;
  position: absolute;
  pointer-events: none;
}

.treemap-item.small-item {
  padding: 0;
}

.treemap-item.dimmed {
  opacity: 0.3;
  filter: brightness(0.5);
}

.treemap-item.hovered {
  z-index: 100;
  /* Fixed size: 50% of treemap width, centered */
  width: 50% !important;
  height: 50% !important;
  left: 25% !important;
  top: 25% !important;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.7);
  border: 2px solid rgba(255, 255, 255, 0.9);
}

.treemap-item:not(.hovered):hover {
  z-index: 10;
  border: 2px solid rgba(255, 255, 255, 0.5);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.item-content {
  text-align: center;
  color: white;
  font-weight: 500;
  text-shadow: 1px 1px 2px rgba(0, 0, 0, 0.5);
  width: 100%;
  padding: 0.5rem;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  transition: opacity 0.2s ease;
}

.treemap-item.hovered .item-content {
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.2s ease;
}

.item-name {
  font-size: 0.85rem;
  margin-bottom: 0.25rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}

.item-size {
  font-size: 0.75rem;
  opacity: 0.95;
  white-space: nowrap;
}

.item-percentage {
  font-size: 0.7rem;
  opacity: 0.85;
  margin-top: 0.2rem;
  white-space: nowrap;
}

/* Expanded hover content */
.item-expanded {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  padding: 0.75rem;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 0.4rem;
  opacity: 0;
  transform: scale(0.9);
  transition: opacity 0.2s ease, transform 0.2s ease;
  pointer-events: none;
}

.treemap-item.hovered .item-expanded {
  opacity: 1;
  transform: scale(1);
  pointer-events: auto;
}

.expanded-field {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  text-align: left;
}

.field-label {
  font-size: 0.65rem;
  opacity: 0.85;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: white;
  text-shadow: 1px 1px 2px rgba(0, 0, 0, 0.5);
}

.field-value {
  font-size: 0.8rem;
  color: white;
  font-weight: 500;
  text-shadow: 1px 1px 2px rgba(0, 0, 0, 0.5);
  word-break: break-word;
  overflow-wrap: break-word;
  max-height: 3em;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  line-clamp: 2;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.expanded-field:first-child .field-value {
  max-height: 1.5em;
  line-clamp: 1;
  -webkit-line-clamp: 1;
}

.expanded-field:nth-child(2) .field-value {
  max-height: 4em;
  line-clamp: 3;
  -webkit-line-clamp: 3;
}

/* Type colors - solid colors for utilitarian look */
.type-video {
  background: #667eea;
}

.type-image {
  background: #f5576c;
}

.type-audio {
  background: #4facfe;
}

.type-archive {
  background: #ffa726;
}

.type-document {
  background: #26a69a;
}

.type-binary {
  background: #676767;
}

.type-directory {
  background: #9575cd;
}

.type-other {
  background: #78909c;
}

.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  margin-top: 0.5rem;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.legend-color {
  width: 20px;
  height: 20px;
  border-radius: 3px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
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

/* Responsive adjustments */
@media (max-width: 768px) {
  .size-viewer {
    padding: 1rem;
  }

  .treemap {
    height: 400px;
  }

  .item-path {
    font-size: 0.7rem;
  }

  .item-size {
    font-size: 0.65rem;
  }

  .stats {
    flex-direction: column;
    gap: 0.5rem;
  }

  .treemap-item.hovered {
    width: 50% !important;
    height: 50% !important;
    left: 25% !important;
    top: 25% !important;
  }

  .field-label {
    font-size: 0.6rem;
  }

  .field-value {
    font-size: 0.75rem;
  }
}
</style>
