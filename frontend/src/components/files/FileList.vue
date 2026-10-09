<template>
  <div class="card-content" aria-label="file-list-prompt">
    <!-- Source Selection Dropdown (hidden when browsing a fixed source, e.g. user scope row) -->
    <div v-if="!hidePathChrome && showSourceSelector" class="source-selector" style="margin-bottom: 1rem;">
      <label for="destinationSource" style="display: block; margin-bottom: 0.5rem; font-weight: bold;">
        {{ $t("prompts.destinationSource") }}
      </label>
      <ExpandDropdown
        input-id="destinationSource"
        v-model="currentSource"
        :options="sourceOptions"
        :aria-label="$t('prompts.destinationSource')"
      />
    </div>

    <!-- Sortable Column Header (opt-in via sortable prop, e.g. destination pickers) -->
    <div v-if="(!hidePathChrome && !fileList) || sortable || $slots.pinned" class="sticky-header" :class="{ 'header-hidden': headerHidden }">
      <!-- Current Path Display -->
      <div v-if="!hidePathChrome && !fileList" aria-label="filelist-path" class="searchContext button clickable">
        {{ $t('general.path', { suffix: ':' }) }} {{ sourcePath.path }}
      </div>
      <slot name="sticky" />
      <ListingHeader v-if="sortable && !loading" use-picker-sorting />
    </div>

    <!-- Loading Spinner -->
    <div v-if="loading" class="loading-spinner-wrapper">
      <LoadingSpinner size="small" mode="placeholder" />
    </div>

    <!-- File List -->
    <div
      v-if="!loading"
      ref="list"
      class="listing-items list"
      @contextmenu.capture="snapshotSelection"
      @touchstart.capture="snapshotSelection"
    >
      <ListingItem
        v-for="(item, index) in visibleItems"
        :key="item.path"
        :name="item.name"
        :isDir="item.type === 'directory' || item.originalItem?.isDir"
        :source="item.source"
        :type="item.type"
        :size="item.size ?? item.originalItem?.size ?? 0"
        :modified="item.originalItem?.modified || new Date().toISOString()"
        :index="index"
        :class="{ 'zebra-row': index % 2 === 1, 'current-item': isCurrentItem(item), 'context-item': isContextItem(item) }"
        :path="item.path"
        :hasPreview="item.originalItem?.hasPreview && item.type !== 'directory' || false"
        :metadata="item.originalItem?.metadata"
        :hasDuration="item.originalItem?.hasDuration || false"
        :updateGlobalState="false"
        :isSelectedProp="selected === item.path"
        :clickable="false"
        :forceFilesApi="!!browseSource"
        :showLimitedOptions="true"
        :inlinePin="true"
        :pinned="item.pinned"
        @click.prevent="(event) => handleItemClick(item, index, event)"
        @dblclick.prevent="(event) => handleItemDblClick(item, index, event)"
      />
      <h2 v-if="filterQuery && !visibleItems.some((item) => item.name !== '..')" class="no-results">
        <i class="material-symbols-outlined">search_off</i>
        <span>{{ $t("tools.advancedSearch.noResults") }}</span>
      </h2>
    </div>
  </div>
</template>

<script>
import { state, mutations, getters } from "@/store";
import { url } from "@/utils";
import { resourcesApi } from "@/api";
import ListingHeader from "@/components/files/ListingHeader.vue";
import ListingItem from "@/components/files/ListingItem.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import { sortedItems } from "@/utils/sort.js";

export default {
  name: "file-list",
  components: {
    ListingHeader,
    ListingItem,
    LoadingSpinner,
    ExpandDropdown,
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
      default: null, // for example, in "quick jump" we provide the listing from the parent.
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
      default: null, // Array of MIME type prefixes (e.g., ['image/', 'video/']) or full types (e.g., ['image/jpeg', 'image/png'])
    },
    browsePath: {
      type: String,
      default: null, // Optional initial path to start browsing from
    },
    /** When true, never show the destination source dropdown (source is fixed via browseSource). */
    hideDestinationSource: {
      type: Boolean,
      default: false,
    },
    /** Hide the source dropdown and path caption (e.g. when PathPickerButton provides both). */
    hidePathChrome: {
      type: Boolean,
      default: false,
    },
    requireFileSelection: {
      type: Boolean,
      default: false, // If true, only files (not folders) can be selected
    },
    filterQuery: {
      type: String,
      default: "",
    },
    sortable: {
      type: Boolean,
      default: true,
    },
  },
  data() {
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
      selectedType: null, // Track the type of the selected item
      current: window.location.pathname,
      currentSource: initialSource,
      loading: false,
      headerHidden: false,
      lastScrollTop: 0,
      scrollContainer: null,
      ownsContextMenu: false,
    };
  },
  computed: {
    /** Sorting config used by sortEntries; follows the picker sort when sortable. */
    pickerSort() {
      return this.sortable ? getters.pickerSorting() : getters.sorting();
    },
    isMobile() {
      return getters.isMobile();
    },
    visibleItems() {
      const query = this.filterQuery.trim().toLowerCase();
      if (!query) return this.items;
      return this.items.filter(
        (item) => item.name === ".." || item.name.toLowerCase().includes(query)
      );
    },
    contextItemPath() {
      if (state.prompts.length < 2 && !state.prompts.some((prompt) => prompt.name === "ContextMenu")) return null;
      const entry = state.selected.find((selected) => selected && typeof selected === "object");
      return entry ? entry.path : null;
    },
    currentPromptName() {
      return getters.currentPromptName();
    },
    promptCount() {
      return state.prompts.length;
    },
    sourcePath() {
      return { source: this.source, path: this.path };
    },
    availableSources() {
      // Get all available sources from state.sources.info
      return state.sources.info ? Object.keys(state.sources.info) : [state.req.source];
    },
    sourceOptions() {
      return this.availableSources.map((source) => ({
        value: source,
        label: source,
      }));
    },
    showSourceSelector() {
      if (this.hideDestinationSource) {
        return false;
      }
      return this.availableSources.length > 1 && !this.fileList && !getters.isShare() && !this.browseShare;
    },
    isValidSelection() {
      // If file selection is required, check if a file (not folder) is selected
      if (this.requireFileSelection) {
        return this.selected && this.selectedType && this.selectedType !== 'directory';
      }
      // Otherwise, any selection is valid (including folders)
      return !!this.selected;
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
    path() {
      if (this.filterQuery) {
        this.$emit("update:filterQuery", "");
      }
    },
    filterQuery() {
      if (!this.selected) return;
      const stillVisible = this.visibleItems.some((item) => item.path === this.selected);
      if (!stillVisible) {
        this.clearSelection();
      }
    },
    currentPromptName(now) {
      if (now !== "ContextMenu") return;
      this.promptsBeforeMenu = this.promptCount - 1;
      this.ownsContextMenu = !this.$el.closest(".floating-window")?.classList.contains("prompt-behind");
    },
    promptCount(count) {
      if (!this.ownsContextMenu || count > this.promptsBeforeMenu) return;
      this.ownsContextMenu = false;
      const previous = this.selectionBeforeMenu;
      this.selectionBeforeMenu = null;
      mutations.resetSelected();
      if (previous?.entries?.length) {
        for (const entry of previous.entries) {
          mutations.addSelected(entry);
        }
      }
      if (previous?.multiple) {
        mutations.setMultiple(true);
      }
    },
    async loading(isLoading) {
      if (!isLoading && this.fileList) {
        await this.$nextTick();
        this.followCurrentItem();
      }
    },
    // Re-sort local items when the picker header changes the sort config
    pickerSort() {
      if (this.sortable) {
        this.resortItems();
      }
    },
  },
  mounted() {
    if (this.fileList) {
      // When fileList is provided, just display the items
      this.withLoading(async () => {
        await new Promise(resolve => setTimeout(resolve, 0));
        this.fillFromList();
      });
    } else if (this.browseShare) {
      // Browse a specific share
      this.withLoading(() => resourcesApi.fetchFilesPublic("/", this.browseShare).then(this.fillOptions));
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
        this.withLoading(() => resourcesApi.fetchFiles(sourceToUse, pathToUse).then(this.fillOptions));
      } else {
        this.fillOptions(initialReq);
      }
    }
    this.attachScrollListener();
  },
  beforeUnmount() {
    this.scrollContainer?.removeEventListener("scroll", this.handleScroll);
    this.stopFollowingCurrentItem?.();
  },
  methods: {
    // Helper method to ensure loading spinner shows for minimum 200ms
    async withLoading(operation) {
      const startTime = Date.now();
      this.loading = true;
      try {
        await operation();
      } catch (error) {
        // Handle fetch errors gracefully
        // Note: API methods already show error notifications, so we don't need to show another one
        console.error('FileList fetch error:', error);
        // Always provide at least the parent directory option if not at root
        // This allows users to navigate back even if the current directory has issues
        this.items = [];
        if (this.path !== "/" && this.showFolders) {
          this.items.push({
            name: "..",
            path: `${url.removeLastDir(this.path)}/`,
            source: this.source,
            type: "directory",
          });
        }
        // Emit the current (failed) path so parent knows about the state
        this.$emit("update:selected", {
          path: this.path,
          source: this.source,
          type: 'directory',
          isValid: !this.requireFileSelection,
          error: true, // Indicate there was an error
        });
      } finally {
        const elapsed = Date.now() - startTime;
        const remaining = Math.max(0, 200 - elapsed);
        await new Promise(resolve => setTimeout(resolve, remaining));
        this.loading = false;
      }
    },
    // Check if file matches allowed file types
    isFileTypeAllowed(itemType) {
      if (!this.allowedFileTypes || this.allowedFileTypes.length === 0) {
        return true; // No filter, allow all
      }
      // If itemType is not provided or is 'directory', allow it
      if (!itemType || itemType === 'directory') {
        return true;
      }
      // Check if the itemType matches any of the allowed types
      // Supports both prefixes (e.g., 'image/') and full types (e.g., 'image/jpeg')
      return this.allowedFileTypes.some(allowedType => {
        if (allowedType.endsWith('/')) {
          // Prefix match (e.g., 'image/' matches 'image/jpeg', 'image/png')
          return itemType.startsWith(allowedType);
        } else {
          // Exact match (e.g., 'image/jpeg' matches 'image/jpeg')
          return itemType === allowedType;
        }
      });
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
      this.selectedType = null;
      // Fetch files for the new source
      void this.withLoading(() => resourcesApi.fetchFiles(newSource, newPath).then(this.fillOptions));
    },
    resetToShare(newHash) {
      // Reset to the share root
      this.path = "/";
      this.shareHash = newHash;
      this.source = null;
      this.selected = null;
      this.selectedSource = null;
      this.selectedType = null;
      // Fetch files for the share
      void this.withLoading(() => resourcesApi.fetchFilesPublic("/", newHash).then(this.fillOptions));
    },
    fillOptions(req) {
      // Sets the current path and resets the current items.
      // Use this.path (the path we're browsing) instead of req.path (which may be relative)
      this.current = this.path;
      this.source = req.source || this.source; // Preserve the source we're browsing
      this.items = [];

      // Emit both path, source, and validity
      this.$emit("update:selected", {
        path: this.current,
        source: this.source,
        type: 'directory',
        isValid: !this.requireFileSelection, // Folder is only valid if file is not required
      });

      const entries = [];
      if (req.items && Array.isArray(req.items)) {
        for (const item of req.items) {
          if (!this.showFolders && item.type === "directory") continue;
          if (!this.showFiles && item.type !== "directory") continue;
          if (item.type !== "directory" && !this.isFileTypeAllowed(item.type)) continue;
          entries.push({
            name: item.name,
            path: item.path,
            source: item.source || req.source,
            type: item.type,
            pinned: !!item.pinned,
            size: item.size,
            modified: item.modified,
            metadata: item.metadata,
            originalItem: item,
          });
        }
        this.items = this.sortEntries(entries);
      }

      if (this.path !== "/" && this.showFolders) {
        this.items.unshift({
          name: "..",
          path: `${url.removeLastDir(this.path)}/`,
          source: this.source,
          type: "directory",
        });
      }
    },
    sortEntries(entries) {
      const sorting = this.pickerSort;
      const dirs = entries.filter((item) => item.type === "directory");
      const files = entries.filter((item) => item.type !== "directory");
      return [
        ...sortedItems(dirs, sorting.by, sorting.asc),
        ...sortedItems(files, sorting.by, sorting.asc),
      ];
    },
    resortItems() {
      const parentEntry = this.items.find((item) => item.name === "..");
      const rest = this.items.filter((item) => item.name !== "..");
      const sorted = this.sortEntries(rest);
      this.items = parentEntry ? [parentEntry, ...sorted] : sorted;
    },
    next(event) {
      // Retrieves the URL of the directory the user
      // just clicked in and fill the options with its
      // content.
      const path = event.currentTarget.dataset.path;
      const clickedItem = this.items.find(item => item.path === path);
      const sourceToUse = clickedItem ? clickedItem.source : this.source;

      // If showFiles and showFolders is true, and clicked item is a file (not a directory), select it directly
      if (this.showFiles && clickedItem && clickedItem.type !== "directory") {
        this.selected = path;
        this.selectedSource = sourceToUse;
        this.selectedType = clickedItem.type;
        this.$emit("update:selected", {
          path: path,
          source: sourceToUse,
          type: clickedItem.type,
          isValid: true, // File is always valid
        });
        return;
      }

      this.path = path;
      // Reset selected when navigating to a directory
      this.selected = null;
      this.selectedSource = null;
      this.selectedType = null;

      // Priority: browseSource > browseShare > isShare
      if (this.browseSource) {
        // Explicitly browsing a source - use files API
        this.source = sourceToUse;
        void this.withLoading(() => resourcesApi.fetchFiles(sourceToUse, path).then(this.fillOptions));
      } else if (this.browseShare || getters.isShare()) {
        // Browsing a share - use public API
        const hashToUse = this.browseShare || state.shareInfo?.hash;
        void this.withLoading(() => resourcesApi.fetchFilesPublic(path, hashToUse).then(this.fillOptions));
      } else {
        this.source = sourceToUse;
        void this.withLoading(() => resourcesApi.fetchFiles(sourceToUse, path).then(this.fillOptions));
      }

    },
    touchstart(event) {
      const url = event.currentTarget.dataset.path;

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
    handleItemClick(item, _index, event) {
      event.preventDefault();
      event.stopPropagation();

      if (this.fileList) {
        if (this.isCurrentItem(item)) return;
        this.navigateToItem(item);
        return;
      }

      // Browse mode: single click selects only, emit selected, do not navigate
      const syntheticEvent = {
        currentTarget: {
          dataset: {
            path: item.path
          }
        },
        preventDefault: () => {},
        stopPropagation: () => {},
      };
      this.select(syntheticEvent);
    },
    handleItemDblClick(item, _index, event) {
      event.preventDefault();
      event.stopPropagation();

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
    // ListingItem overwrites state.selected when opening the menu (eg: from quick jump)
    // this is to restore the previous selection that the previews use for the overflow menu, otherwise would remain undefined.
    snapshotSelection() {
      if (this.ownsContextMenu || this.currentPromptName === "ContextMenu") return;
      this.selectionBeforeMenu = {
        entries: Array.isArray(state.selected) ? [...state.selected] : [],
        multiple: state.multiple,
      };
    },
    clearSelection() {
      this.selected = null;
      this.selectedSource = null;
      this.selectedType = null;
      this.$emit("update:selected", {
        path: this.current,
        source: this.source,
        type: 'directory',
        isValid: !this.requireFileSelection,
      });
    },
    select(event) {
      const path = event.currentTarget.dataset.path;
      if (this.selected === path) {
        this.clearSelection();
        return;
      }
      // Otherwise select the element.
      this.selected = path;
      const clickedItem = this.items.find(item => item.path === path);
      this.selectedSource = clickedItem ? clickedItem.source : this.source;
      this.selectedType = clickedItem ? clickedItem.type : null;
      const isFile = clickedItem && clickedItem.type !== "directory";
      this.$emit("update:selected", {
        path: this.selected,
        source: this.selectedSource,
        type: this.selectedType,
        isValid: !this.requireFileSelection || isFile,
      });
    },
    async createDir() {
      mutations.showPrompt({
        name: "newDir",
        action: null,
        confirm: null,
        props: {
          redirect: false,
          base: this.current === this.path ? null : this.current,
        },
      });
    },
    /**
     * Jump listing to a source/path (e.g. after PathPickerButton confirms).
     * Normalizes to a directory path with trailing slash (except root).
     */
    jumpTo(source, path) {
      if (!source) {
        return;
      }
      let p = path === null || path === "" ? "/" : String(path);
      if (!p.startsWith("/")) {
        p = `/${p}`;
      }
      if (p !== "/" && !p.endsWith("/")) {
        p = `${p}/`;
      }
      this.currentSource = source;
      this.source = source;
      this.path = p;
      this.selected = null;
      this.selectedSource = null;
      this.selectedType = null;
      if (this.browseShare || getters.isShare()) {
        const hashToUse = this.browseShare || state.shareInfo?.hash;
        void this.withLoading(() => resourcesApi.fetchFilesPublic(p, hashToUse).then(this.fillOptions));
      } else {
        void this.withLoading(() => resourcesApi.fetchFiles(source, p).then(this.fillOptions));
      }
    },
    fillFromList() {
      const allItems = this.fileList || [];
      const items = allItems.filter(item => !item.isDirectory && item.type !== 'directory');
      this.items = this.sortable ? this.sortEntries(items) : items;
    },
    isCurrentItem(item) {
      return !!this.fileList && !!state.req && item.name === state.req.name;
    },
    // Items selected via right click or long press
    isContextItem(item) {
      return this.ownsContextMenu && !!this.contextItemPath && item.path === this.contextItemPath;
    },
    attachScrollListener() {
      const el = this.$el.closest(".floating-window > .card-content");
      if (!el) return;
      el.addEventListener("scroll", this.handleScroll, { passive: true });
      this.scrollContainer = el;
    },
    handleScroll() {
      const top = this.scrollContainer.scrollTop;
      const diff = top - this.lastScrollTop;
      if (top <= 10 || this.stopFollowingCurrentItem) {
        this.headerHidden = false;
      } else if (Math.abs(diff) >= 30) {
        this.headerHidden = diff > 0;
      } else {
        return;
      }
      this.lastScrollTop = top;
    },
    scrollToCurrentItem() {
      const current = this.$refs.list?.querySelector(".current-item");
      if (!current) return null;
      let container = current.parentElement;
      while (container && !/(auto|scroll)/.test(getComputedStyle(container).overflowY)) {
        container = container.parentElement;
      }
      if (!container) return null;
      const itemTop =
        current.getBoundingClientRect().top - container.getBoundingClientRect().top + container.scrollTop;
      container.scrollTo({
        top: itemTop - (container.clientHeight - current.offsetHeight) / 2,
        behavior: "instant",
      });
      return container;
    },
    // Keeps the current item centered while the list is still resizing
    followCurrentItem() {
      this.stopFollowingCurrentItem?.();
      const container = this.scrollToCurrentItem();
      if (!container) return;
      const observer = new ResizeObserver(() => this.scrollToCurrentItem());
      observer.observe(container);
      observer.observe(this.$refs.list);
      const events = ["wheel", "touchstart", "pointerdown", "keydown"];
      const timer = setTimeout(() => this.stopFollowingCurrentItem?.(), 1000);
      this.stopFollowingCurrentItem = () => {
        observer.disconnect();
        clearTimeout(timer);
        for (const name of events) {
          container.removeEventListener(name, this.stopFollowingCurrentItem);
        }
        this.stopFollowingCurrentItem = null;
      };
      for (const name of events) {
        container.addEventListener(name, this.stopFollowingCurrentItem, { passive: true });
      }
    },
    navigateToItem(item) {
      mutations.closeTopPrompt();
      mutations.setNavigationTransitioning(true);
      const isShare = !!(this.browseShare) || getters.isShare();
      const source = isShare ? (state.shareInfo?.hash) : (item.source || state.req.source);
      url.goToItem(source, item.path, undefined, false, isShare);
    },
  },
};
</script>

<style scoped>
/* File picker specific: make non-link items interactive */
.listing-items :deep(.listing-item.clickable) {
  cursor: pointer;
}

/* Current file (quick jump) */
.listing-items :deep(.listing-item.current-item) {
  background: var(--primaryColor) !important;
  color: #fff !important;
}

/* Item that opened the context menu */
.listing-items :deep(.listing-item.context-item) {
  background: color-mix(in srgb, var(--primaryColor) 25%, transparent) !important;
}

/* Highlight selected items with primary color */
.listing-items :deep(.listing-item.activebutton) {
  background: var(--primaryColor) !important;
  color: #fff !important;
}

.sticky-header {
  position: sticky;
  top: calc(-0.5em - 2px);
  z-index: 5;
  display: flex;
  flex-direction: column;
  gap: 0.5em;
  padding: 0.5em 0;
  backdrop-filter: blur(8px);
  transition: transform 0.25s ease, opacity 0.25s ease;
}

.sticky-header.header-hidden {
  transform: translateY(-100%);
  opacity: 0;
  pointer-events: none;
}

.no-results {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.35em;
  padding-top: 2em;
  font-size: 1.2em;
  opacity: 0.6;
}

.no-results i {
  font-size: 2.2em;
}

/* Loading spinner (not part of listing.css) */
.loading-spinner-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 2em 0;
}

</style>
