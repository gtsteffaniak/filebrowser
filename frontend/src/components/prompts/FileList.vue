<template>

  <div v-if="isDisplayMode" class="card-title">
    <h2>{{ effectiveTitle }}</h2>
  </div>
  <div class="card-content">
    <!-- Source Selection Dropdown (only show if multiple sources available and not in display mode) -->
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

    <div v-if="!isDisplayMode" aria-label="filelist-path" class="searchContext button clickable">{{ $t('general.path', { suffix: ':' }) }}
      {{ sourcePath.path }}</div>

    <div v-if="loading" class="loading-spinner-wrapper">
      <LoadingSpinner size="small" mode="placeholder" />
    </div>

    <ul v-else class="file-list">
      <li @click="itemClick" @touchstart="touchstart" @dblclick="next" role="button" tabindex="0"
        :aria-label="item.name" :aria-selected="selected == item.path" :key="item.name" v-for="item in items"
        :data-path="item.path" class="file-item">
        <Icon :filename="item.name"
          :mimetype="item.originalItem?.type || 'directory'"
          class="file-icon" 
        />
        <span class="file-name">{{ item.name }}</span>
      </li>
    </ul>
  </div>

  <!-- Cancel/Close button for display mode -->
  <div v-if="isDisplayMode" class="card-action">
    <button @click="closeModal" class="button button--flat" :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')">
      {{ $t('general.cancel') }}
    </button>
  </div>

</template>

<script>
import { state, mutations, getters } from "@/store";
import { url } from "@/utils";
import { filesApi, publicApi } from "@/api";
import Icon from "@/components/files/Icon.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";

export default {
  name: "file-list",
  components: {
    Icon,
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
    fileList: {
      type: Array,
      default: null,
    },
    mode: {
      type: String,
      default: "browse", // 'browse', 'navigate-up', 'quick-jump'
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
  },
  data: function () {
    const initialSource = this.browseSource || state.req.source;
    // Use current path if browsing the same source as current, otherwise start at root
    const initialPath = (this.browseSource && this.browseSource !== state.req.source) || this.browseShare ? "/" : state.req.path;
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
      if (getters.isShare()) {
        return { source: this.source, path: this.path };
      }
      return { source: this.source, path: this.path };
    },
    effectiveSource() {
      return this.browseSource || state.req.source;
    },
    availableSources() {
      // Get all available sources from state.sources.info
      return state.sources && state.sources.info ? Object.keys(state.sources.info) : [state.req.source];
    },
    isDisplayMode() {
      // Display mode when fileList prop is provided (drag-triggered navigation)
      return this.fileList !== null;
    },
    showSourceSelector() {
      return this.availableSources.length > 1 && !this.isDisplayMode && !getters.isShare() && !this.browseShare;
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
    if (this.isDisplayMode) {
      // Display mode: use provided fileList
      this.withLoading(async () => {
        await new Promise(resolve => setTimeout(resolve, 0)); // Make it async
        this.fillOptionsFromList();
      });
    } else if (this.browseShare) {
      // Browse a specific share
      this.withLoading(() => publicApi.fetchPub("/", this.browseShare).then(this.fillOptions));
    } else {
      // Normal browse mode: fetch files
      const sourceToUse = this.currentSource;
      const pathToUse = this.currentSource !== state.req.source ? "/" : state.req.path;
      const initialReq = {
        ...state.req,
        source: sourceToUse,
        path: pathToUse,
      };
      // Fetch the initial data for the source
      if (this.currentSource !== state.req.source) {
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
      this.current = req.path;
      this.source = req.source || null; // For shares, source might be null/undefined
      this.items = [];

      // Emit both path and source
      // For shares, source will be null, which is handled by MoveCopy
      this.$emit("update:selected", {
        path: this.current,
        source: this.source
      });

      // If the path isn't the root path,
      // show a button to navigate to the previous
      // directory (unless we are only displaying files).
      if (req.path !== "/" && !this.showFolders) {
        this.items.push({
          name: "..",
          path: url.removeLastDir(req.path) + "/",
          source: req.source,
          type: "directory",
        });
      }

      // If this folder is empty, finish here.
      if (req.items === null) return;
      for (let item of req.items) {
        if (!this.showFolders && item.type === "directory") continue;
        if (!this.showFiles && item.type !== "directory") continue;
        // If showFiles is true and showFolders is false -- show only files
        // If showFolders is true and showFiles is false -- show only directories
        // If both are true -- show files and folders
        // If both are false -- show nothing
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
      if (this.browseShare || getters.isShare()) {
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
    itemClick: function (event) {
      if (this.isDisplayMode) {
        // In display mode, navigate directly to the item
        this.navigateToItem(event);
      } else if (state.user.singleClick) {
        this.next(event);
      } else {
        this.select(event);
      }
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

    // Display mode methods (for drag-triggered navigation)
    fillOptionsFromList() {
      // Use the provided fileList, filtering out directories to show only files
      const allItems = this.fileList || [];
      this.items = allItems.filter(item => !item.isDirectory && item.type !== 'directory');
      this.current = this.title || "Navigation";
      this.source = state.req.source;

      // Emit the current info
      this.$emit("update:selected", {
        path: this.current,
        source: this.source
      });
    },


    navigateToItem(event) {
      const path = event.currentTarget.dataset.path;
      const item = this.items.find(item => item.path === path);

      if (!item) return;

      // Close the file list modal
      mutations.closeHovers();

      // Navigate to the item's URL
      const itemUrl = url.buildItemUrl(item.source || state.req.source, item.path);

      // Use router to navigate
      this.$router.replace({ path: itemUrl });
    },

    closeModal() {
      // Close the file list modal
      mutations.closeHovers();
    },
  },
};
</script>

<style scoped>
.file-item {
  display: flex;
  align-items: center;
  padding: 0.5rem;
  cursor: pointer;
  user-select: none;
}

.file-item:hover {
  background-color: var(--surfaceSecondary, rgba(0, 0, 0, 0.05));
}

.file-icon {
  margin-right: 0.75rem;
  flex-shrink: 0;
}

.file-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  user-select: none;
}

.file-list {
  padding: 0;
  margin: 0;
}

.file-list li[aria-selected=true] {
  background: var(--primaryColor) !important;
  color: #fff !important;
  transition: .1s ease all;
}

.loading-spinner-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 2em 0;
}

</style>
