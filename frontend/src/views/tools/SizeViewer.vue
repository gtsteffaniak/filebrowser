<template>
  <div class="size-viewer">
    <div class="card size-viewer-config">
      <div class="card-title">
        <h2>{{ $t('tools.sizeViewer') }}</h2>
      </div>
      <div class="card-content">
        <h3>Path</h3>
        <input v-model="searchPath" type="text" placeholder="/" class="input" />

        <h3>Source</h3>
        <select v-model="selectedSource" class="input">
          <option v-for="(info, name) in sourceInfo" :key="name" :value="name">
            {{ name }}
          </option>
        </select>

        <h3>Larger Than (MB)</h3>
        <input v-model.number="largerThan" type="number" min="0" placeholder="100" class="input" />

        <ToggleSwitch v-model="includeFolders" name="Include Folders" description="Include directories in the analysis"
          aria-label="Include folders toggle" />

        <button @click="fetchData" class="button" :disabled="loading">
          <i v-if="loading" class="material-icons spin">autorenew</i>
          <span v-else>Analyze</span>
        </button>
      </div>
    </div>

    <div class="card size-viewer-results">
      <div v-if="error" class="error-message">
        {{ error }}
      </div>

      <div v-if="results.length > 0">
        <div class="card-title">
          <h2>Results</h2>
        </div>
        <div class="card-content">
          <div class="stats">
            <span>Total Files: <strong>{{ results.length }}</strong></span>
            <span>Total Size: <strong>{{ humanSize(totalSize) }}</strong></span>
          </div>

          <div class="treemap" ref="treemap">
            <div v-for="(rect, index) in treemapRects" :key="index"
              :class="['treemap-item', getTypeClass(rect.item.type), { 'small-item': isSmallItem(rect.item) }]"
              :style="getRectStyle(rect)" :title="`${rect.item.path}\n${humanSize(rect.item.size)}`"
              @click="handleItemClick(rect.item)">
              <div class="item-content" v-if="!isSmallItem(rect.item)">
                <div class="item-path">{{ getDisplayPath(rect.item.path) }}</div>
                <div class="item-size">{{ humanSize(rect.item.size) }}</div>
              </div>
            </div>
          </div>

          <h4>File Types</h4>
          <div class="legend">
            <div class="legend-item">
              <span class="legend-color type-video"></span>
              <span>Video</span>
            </div>
            <div class="legend-item">
              <span class="legend-color type-image"></span>
              <span>Image</span>
            </div>
            <div class="legend-item">
              <span class="legend-color type-audio"></span>
              <span>Audio</span>
            </div>
            <div class="legend-item">
              <span class="legend-color type-archive"></span>
              <span>Archive</span>
            </div>
            <div class="legend-item">
              <span class="legend-color type-document"></span>
              <span>Document</span>
            </div>
            <div class="legend-item">
              <span class="legend-color type-binary"></span>
              <span>Binary</span>
            </div>
            <div v-if="includeFolders" class="legend-item">
              <span class="legend-color type-directory"></span>
              <span>Directory</span>
            </div>
            <div class="legend-item">
              <span class="legend-color type-other"></span>
              <span>Other</span>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="!loading" class="empty-state">
        <i class="material-icons">analytics</i>
        <p>Enter search criteria and click Analyze to visualize file sizes</p>
      </div>
    </div>
  </div>
</template>

<script>
import { search } from "@/api";
import { state } from "@/store";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { getTypeInfo } from "@/utils/mimetype";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "SizeViewer",
  components: {
    ToggleSwitch,
  },
  data() {
    return {
      searchPath: "/",
      selectedSource: "",
      largerThan: 100,
      includeFolders: false,
      loading: false,
      error: null,
      results: [],
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
  mounted() {
    // Set default source
    if (state.sources.current) {
      this.selectedSource = state.sources.current;
    } else if (Object.keys(this.sourceInfo).length > 0) {
      this.selectedSource = Object.keys(this.sourceInfo)[0];
    }
  },
  methods: {
    async fetchData() {
      this.loading = true;
      this.error = null;

      try {
        let query = `type:largerThan=${this.largerThan}`;
        if (!this.includeFolders) {
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
    handleItemClick(item) {
      // Navigate to the file/directory
      const path = item.path.startsWith("/") ? item.path : "/" + item.path;
      const route = `/files${path}`;
      this.$router.push(route);
    },
  },
};
</script>

<style scoped>

.size-viewer {
  padding: 2rem;
  max-width: 100%;
  margin: 0 auto;
}

.size-viewer-results {
  width: 80vw;
  max-width: 1000px;
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

.treemap-item {
  border: 1px solid rgba(0, 0, 0, 0.1);
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  box-sizing: border-box;
}

.treemap-item.small-item {
  padding: 0;
}

.treemap-item:hover {
  z-index: 10;
  border: 2px solid rgba(255, 255, 255, 0.8);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
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
}

.item-path {
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
}
</style>
