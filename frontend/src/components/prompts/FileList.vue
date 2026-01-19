<template>
  <div class="card-content">
    <!-- Source Selection Dropdown -->
    <div v-if="showSourceSelector" class="source-selector" style="margin-bottom: 1rem;">
      <label for="destinationSource" style="display: block; margin-bottom: 0.5rem; font-weight: bold;">
        {{ $t("prompts.destinationSource") }}
      </label>
      <select id="destinationSource" v-model="currentSource" @change="onSourceChange" class="input">
        <option v-for="source in availableSources" :key="source" :value="source">
          {{ source }}
        </option>
      </select>
    </div>

    <!-- Current Path Display -->
    <div aria-label="filelist-path" class="searchContext button clickable">
      {{ $t('general.path', { suffix: ':' }) }} {{ sourcePath.path }}
    </div>

    <!-- Loading Spinner -->
    <div v-if="loading" class="loading-spinner-wrapper">
      <LoadingSpinner size="small" mode="placeholder" />
    </div>

    <!-- File List -->
    <div v-else class="listing-items list">
      <ListingItem
        v-for="(item, index) in items"
        :key="item.path"
        :name="item.name"
        :isDir="item.type === 'directory' || item.originalItem?.isDir"
        :source="item.source"
        :type="item.type"
        :size="item.originalItem?.size || 0"
        :modified="item.originalItem?.modified || new Date().toISOString()"
        :index="index"
        :path="item.path"
        :hasPreview="item.originalItem?.hasPreview || false"
        :metadata="item.originalItem?.metadata"
        :hasDuration="item.originalItem?.hasDuration || false"
        :updateGlobalState="false"
        :isSelectedProp="selected === item.path"
        :clickable="false"
        :forceFilesApi="!!browseSource"
        @click.prevent="(event) => handleItemClick(item, index, event)"
        @dblclick.prevent="(event) => handleItemDblClick(item, index, event)"
      />
    </div>
  </div>
</template>

<script>
import { state, mutations, getters } from "@/store";
import { url } from "@/utils";
import { filesApi, publicApi } from "@/api";
import ListingItem from "@/components/files/ListingItem.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";

export default {
  name: "file-list",
  components: {
    ListingItem,
    LoadingSpinner,
  },
  props: {
    browseSource: {
      type: String,
      default: null,
    },
    browseShare: {
      type: String,
      default: null, // Share hash to browse
    },
    title: {
      type: String,
      default: null,
    },
    showFiles: {
      type: Boolean,
      default: false,
    },
    showFolders: {
      type: Boolean,
      default: true,
    },
    allowedFileTypes: {
      type: Array,
      default: null, // Array of allowed file extensions (e.g., ['.jpg', '.png', '.gif'])
    },
    browsePath: {
      type: String,
      default: null, // Optional initial path to start browsing from
    },
  },
  data: function () {
    const initialSource = this.browseSource || state.req.source;
    // If browsePath is provided, use it; otherwise use current path or root
    let initialPath;
    if (this.browsePath) {
      initialPath = this.browsePath;
    } else if ((this.browseSource && this.browseSource !== state.req.source) || this.browseShare) {
      initialPath = "/";
    } else {
      initialPath = state.req.path;
    }
    return {
      items: [],
      path: initialPath,
      source: initialSource,
      shareHash: this.browseShare || null,
      touches: {
        id: "",
        count: 0,
      },
      selected: null,
      selectedSource: null,
      current: window.location.pathname,
      currentSource: initialSource,
      loading: false,
    };
  },
  computed: {
    effectiveTitle() {
      return this.title || this.$t("general.files");
    },
    sourcePath() {
      return { source: this.source, path: this.path };
    },
    availableSources() {
      // Get all available sources from state.sources.info
      return state.sources && state.sources.info ? Object.keys(state.sources.info) : [state.req.source];
    },
    showSourceSelector() {
      return this.availableSources.length > 1 && !getters.isShare() && !this.browseShare;
    },
  },
  watch: {
    browseSource(newSource) {
      if (newSource && newSource !== this.source) {
        this.currentSource = newSource;
        this.resetToSource(newSource);
      }
    },
    browseShare(newHash) {
      if (newHash && newHash !== this.shareHash) {
        this.resetToShare(newHash);
      }
    },
    currentSource(newSource) {
      if (newSource && newSource !== this.source) {
        this.resetToSource(newSource);
      }
    },
  },
  mounted() {
    if (this.browseShare) {
      // Browse a specific share
      this.withLoading(() => publicApi.fetchPub("/", this.browseShare).then(this.fillOptions));
    } else {
      // Normal browse mode: fetch files
      const sourceToUse = this.currentSource;
      const pathToUse = this.path; // Use the path initialized in data() which respects browsePath
      const initialReq = {
        ...state.req,
        source: sourceToUse,
        path: pathToUse,
      };
      // Fetch the initial data for the source
      // Always fetch if browsing a different source or if browsePath was specified
      if (this.currentSource !== state.req.source || this.browsePath) {
        this.withLoading(() => filesApi.fetchFiles(sourceToUse, pathToUse).then(this.fillOptions));
      } else {
        this.fillOptions(initialReq);
      }
    }
  },
  methods: {
    // Helper method to ensure loading spinner shows for minimum 200ms
    async withLoading(operation) {
      const startTime = Date.now();
      this.loading = true;
      try {
        await operation();
      } finally {
        const elapsed = Date.now() - startTime;
        const remaining = Math.max(0, 200 - elapsed);
        await new Promise(resolve => setTimeout(resolve, remaining));
        this.loading = false;
      }
    },
    // Check if file matches allowed file types
    isFileTypeAllowed(fileName) {
      if (!this.allowedFileTypes || this.allowedFileTypes.length === 0) {
        return true; // No filter, allow all
      }
      const lowerFileName = fileName.toLowerCase();
      return this.allowedFileTypes.some(ext => lowerFileName.endsWith(ext.toLowerCase()));
    },
    resetToSource(newSource) {
      // Use current path if browsing the same source as current, otherwise start at root
      const newPath = newSource === state.req.source ? state.req.path : "/";
      // Reset to the appropriate path for the new source
      this.path = newPath;
      this.source = newSource;
      this.shareHash = null;
      this.selected = null;
      this.selectedSource = null;
      // Fetch files for the new source
      this.withLoading(() => filesApi.fetchFiles(newSource, newPath).then(this.fillOptions));
    },
    resetToShare(newHash) {
      // Reset to the share root
      this.path = "/";
      this.shareHash = newHash;
      this.source = null;
      this.selected = null;
      this.selectedSource = null;
      // Fetch files for the share
      this.withLoading(() => publicApi.fetchPub("/", newHash).then(this.fillOptions));
    },
    fillOptions(req) {
      // Sets the current path and resets
      // the current items.
      // Use this.path (the path we're browsing) instead of req.path (which may be relative)
      this.current = this.path;
      this.source = req.source || this.source; // Preserve the source we're browsing
      this.items = [];

      // Emit both path and source
      this.$emit("update:selected", {
        path: this.current,
        source: this.source
      });

      // If the path isn't the root path,
      // show a button to navigate to the previous
      // directory (unless we are only displaying files).
      if (this.path !== "/" && this.showFolders) {
        this.items.push({
          name: "..",
          path: url.removeLastDir(this.path) + "/",
          source: this.source,
          type: "directory",
        });
      }

      // If this folder is empty, finish here.
      if (req.items === null) return;
      for (let item of req.items) {
        if (!this.showFolders && item.type === "directory") continue;
        if (!this.showFiles && item.type !== "directory") continue;
        // Filter by file type if specified (only for files, not directories)
        if (item.type !== "directory" && !this.isFileTypeAllowed(item.name)) continue;
        this.items.push({
          name: item.name,
          path: item.path,
          source: item.source || req.source,
          type: item.type, // Store type for file selection
          originalItem: item, // Store original item for Icon component
        });
      }
    },
    next: function (event) {
      // Retrieves the URL of the directory the user
      // just clicked in and fill the options with its
      // content.
      let path = event.currentTarget.dataset.path;
      let clickedItem = this.items.find(item => item.path === path);
      let sourceToUse = clickedItem ? clickedItem.source : this.source;
      
      // If showFiles and showFolders is true, and clicked item is a file (not a directory), select it directly
      if (this.showFiles && clickedItem && clickedItem.type !== "directory") {
        this.selected = path;
        this.selectedSource = sourceToUse;
        this.$emit("update:selected", {
          path: path,
          source: sourceToUse,
          type: clickedItem.type
        });
        return;
      }
      
      this.path = path;
      // Priority: browseSource > browseShare > isShare
      if (this.browseSource) {
        // Explicitly browsing a source - use files API
        this.source = sourceToUse;
        this.withLoading(() => filesApi.fetchFiles(sourceToUse, path).then(this.fillOptions));
      } else if (this.browseShare || getters.isShare()) {
        // Browsing a share - use public API
        const hashToUse = this.browseShare || state.shareInfo?.hash;
        this.withLoading(() => publicApi.fetchPub(path, hashToUse).then(this.fillOptions));
      } else {
        this.source = sourceToUse;
        this.withLoading(() => filesApi.fetchFiles(sourceToUse, path).then(this.fillOptions));
      }

    },
    touchstart(event) {
      let url = event.currentTarget.dataset.path;

      // In 300 milliseconds, we shall reset the count.
      setTimeout(() => {
        this.touches.count = 0;
      }, 300);

      // If the element the user is touching
      // is different from the last one he touched,
      // reset the count.
      if (this.touches.id !== url) {
        this.touches.id = url;
        this.touches.count = 1;
        return;
      }

      this.touches.count++;

      // If there is more than one touch already,
      // open the next screen.
      if (this.touches.count > 1) {
        this.next(event);
      }
    },
    handleItemClick(item, index, event) {
      event.preventDefault();
      event.stopPropagation();
      
      // Create a synthetic event-like object for compatibility with existing methods
      const syntheticEvent = {
        currentTarget: {
          dataset: {
            path: item.path
          }
        },
        preventDefault: () => {},
        stopPropagation: () => {},
      };

      if (state.user.singleClick) {
        this.next(syntheticEvent);
      } else {
        this.select(syntheticEvent);
      }
    },
    handleItemDblClick(item, index, event) {
      event.preventDefault();
      event.stopPropagation();
      
      // Create a synthetic event for double-click
      const syntheticEvent = {
        currentTarget: {
          dataset: {
            path: item.path
          }
        },
        preventDefault: () => {},
        stopPropagation: () => {},
      };
      this.next(syntheticEvent);
    },
    select: function (event) {
      let path = event.currentTarget.dataset.path;
      // If the element is already selected, unselect it.
      if (this.selected === path) {
        this.selected = null;
        this.selectedSource = null;
        this.$emit("update:selected", {
          path: this.current,
          source: this.source
        });
        return;
      }

      // Otherwise select the element.
      this.selected = path;
      let clickedItem = this.items.find(item => item.path === path);
      this.selectedSource = clickedItem ? clickedItem.source : this.source;
      this.$emit("update:selected", {
        path: this.selected,
        source: this.selectedSource
      });
    },
    createDir: async function () {
      mutations.showHover({
        name: "newDir",
        action: null,
        confirm: null,
        props: {
          redirect: false,
          base: this.current === this.path ? null : this.current,
        },
      });
    },
    onSourceChange() {
      this.resetToSource(this.currentSource);
    },
  },
};
</script>

<style scoped>
/* File picker specific: make non-link items interactive */
.listing-items :deep(.listing-item.clickable) {
  cursor: pointer;
}

/* Highlight selected items with primary color */
.listing-items :deep(.listing-item.activebutton) {
  background: var(--primaryColor) !important;
  color: #fff !important;
}

/* Loading spinner (not part of listing.css) */
.loading-spinner-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 2em 0;
}

</style>
