<template>
  <div ref="root" class="card-content" aria-label="file-list-prompt">
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
    <div v-if="(!hidePathChrome && !fileList) || sortable || $slots.sticky" class="sticky-header" :class="{ 'header-hidden': headerHidden }">
      <!-- Current Path Display -->
      <div v-if="!hidePathChrome && !fileList" aria-label="filelist-path" class="searchContext button clickable">
        {{ $t('general.path', { suffix: ':' }) }} {{ sourcePath.path }}
      </div>
      <slot name="sticky" />
      <ListingHeader v-if="sortable && !loading" :use-picker-sorting="!fileList" :use-navigation-sorting="!!fileList" />
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
      :class="{ virtualized: virtual.active.value }"
      @contextmenu.capture="snapshotSelection"
      @touchstart.capture.passive="snapshotSelection"
    >
      <!-- Long lists only render the rows around the screen, the rest is empty space of the same size -->
      <div data-virtual-group="items" :style="virtual.group('items').style">
        <ListingItem
          v-for="entry in virtual.group('items').items"
          :key="`${entry.item.source}::${entry.item.path}`"
          :name="entry.item.name"
          :isDir="entry.item.type === 'directory' || entry.item.originalItem?.isDir"
          :source="entry.item.source"
          :type="entry.item.type"
          :size="entry.item.size ?? entry.item.originalItem?.size ?? 0"
          :modified="entry.item.originalItem?.modified || fallbackModified"
          :created="entry.item.originalItem?.created"
          :index="entry.index"
          :class="{ 'zebra-row': entry.index % 2 === 1, 'current-item': isCurrentItem(entry.item), 'context-item': isContextItem(entry.item) }"
          :path="entry.item.path"
          :hasPreview="entry.item.originalItem?.hasPreview && entry.item.type !== 'directory' || false"
          :metadata="entry.item.originalItem?.metadata"
          :hasDuration="entry.item.originalItem?.hasDuration || false"
          :updateGlobalState="false"
          :isSelectedProp="selected === entry.item.path"
          :clickable="false"
          :forceFilesApi="!!browseSource"
          :showLimitedOptions="true"
          :inlinePin="true"
          :pinned="entry.item.pinned"
          @click.prevent="(event: MouseEvent) => handleItemClick(entry.item, entry.index, event)"
          @dblclick.prevent="(event: MouseEvent) => handleItemDblClick(entry.item, entry.index, event)"
        />
      </div>
      <h2 v-if="filterQuery && !visibleItems.some((item) => item.name !== '..')" class="no-results">
        <i class="material-symbols-outlined">search_off</i>
        <span>{{ $t("tools.advancedSearch.noResults") }}</span>
      </h2>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useTemplateRef, watch } from "vue";
import { state, mutations, getters } from "@/store/index.ts";
import type { FileListItem, MediaMetadata } from "@/store/types.ts";
import { url } from "@/utils/index.ts";
import { resourcesApi } from "@/api/index.js";
import ListingHeader from "@/components/files/ListingHeader.vue";
import ListingItem from "@/components/files/ListingItem.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import { sortedItems } from "@/utils/sort.ts";
import { findScrollParents, findScroller, useListingVirtualization } from "@/composables/useListingVirtualization.ts";

type ItemInfo = FileListItem & { isDir?: boolean; hasDuration?: boolean };

export interface ListEntry {
  name: string;
  path: string;
  source?: string;
  type?: string;
  pinned?: boolean;
  size?: number;
  modified?: string;
  created?: string;
  metadata?: MediaMetadata;
  isDirectory?: boolean;
  originalItem?: ItemInfo;
}

export interface SelectionPayload {
  path: string | null;
  source: string | null | undefined;
  type: string | null | undefined;
  isValid: boolean;
  error?: boolean;
}

interface SelectionSnapshot {
  entries: (number | FileListItem)[];
  multiple: boolean;
}

const props = withDefaults(
  defineProps<{
    browseSource?: string | null;
    // Share hash to browse
    browseShare?: string | null;
    // for example, in "quick jump" we provide the listing from the parent.
    fileList?: ListEntry[] | null;
    title?: string | null;
    showFiles?: boolean;
    showFolders?: boolean;
    // MIME type prefixes (e.g., ['image/', 'video/']) or full types (e.g., ['image/jpeg', 'image/png'])
    allowedFileTypes?: string[] | null;
    // Optional initial path to start browsing from
    browsePath?: string | null;
    /** When true, never show the destination source dropdown (source is fixed via browseSource). */
    hideDestinationSource?: boolean;
    /** Hide the source dropdown and path caption (e.g. when PathPickerButton provides both). */
    hidePathChrome?: boolean;
    // If true, only files (not folders) can be selected
    requireFileSelection?: boolean;
    filterQuery?: string;
    sortable?: boolean;
  }>(),
  {
    browseSource: null,
    browseShare: null,
    fileList: null,
    title: null,
    showFiles: false,
    showFolders: true,
    allowedFileTypes: null,
    browsePath: null,
    hideDestinationSource: false,
    hidePathChrome: false,
    requireFileSelection: false,
    filterQuery: "",
    sortable: true,
  }
);

const emit = defineEmits<{
  "update:selected": [payload: SelectionPayload];
  "update:filterQuery": [value: string];
}>();

defineSlots<{
  sticky?(): unknown;
}>();

const root = useTemplateRef<HTMLElement>("root");
const list = useTemplateRef<HTMLElement>("list");

const fallbackModified = new Date().toISOString();
const initialSource = props.browseSource || state.req.source;
// If browsePath is provided, use it; otherwise use current path or root
let initialPath: string;
if (props.browsePath) {
  initialPath = props.browsePath;
} else if ((props.browseSource && props.browseSource !== state.req.source) || props.browseShare) {
  initialPath = "/";
} else {
  initialPath = state.req.path ?? "/";
}

const items = ref<ListEntry[]>([]);
const path = ref(initialPath);
const source = ref<string | null | undefined>(initialSource);
const shareHash = ref(props.browseShare || null);
const selected = ref<string | null>(null);
const selectedSource = ref<string | null | undefined>(null);
const selectedType = ref<string | null | undefined>(null); // Track the type of the selected item
const current = ref(window.location.pathname);
const currentSource = ref(initialSource);
const loading = ref(false);
const headerHidden = ref(false);
const ownsContextMenu = ref(false);

let lastScrollTop = 0;
let scrollContainer: HTMLElement | null = null;
let promptsBeforeMenu = 0;
let selectionBeforeMenu: SelectionSnapshot | null = null;
let stopFollowingCurrentItem: (() => void) | null = null;

/** Sorting config used by sortEntries; follows the picker sort when sortable. */
const pickerSort = computed(() => {
  if (props.fileList) return getters.navigationSorting();
  return props.sortable ? getters.pickerSorting() : getters.sorting();
});
const visibleItems = computed(() => {
  const query = props.filterQuery.trim().toLowerCase();
  if (!query) return items.value;
  return items.value.filter((item) => item.name === ".." || item.name.toLowerCase().includes(query));
});
const contextItemPath = computed(() => {
  if (state.prompts.length < 2 && !state.prompts.some((prompt) => prompt.name === "ContextMenu")) return null;
  const entry = state.selected.find((selectedEntry): selectedEntry is FileListItem => typeof selectedEntry === "object");
  return entry ? entry.path : null;
});
const currentPromptName = computed(() => getters.currentPromptName());
const promptCount = computed(() => state.prompts.length);
const sourcePath = computed(() => ({ source: source.value, path: path.value }));
const availableSources = computed(() =>
  // Get all available sources from state.sources.info
  state.sources.info ? Object.keys(state.sources.info) : [state.req.source]
);
const sourceOptions = computed(() => availableSources.value.map((value) => ({ value, label: value })));
const showSourceSelector = computed(() => {
  if (props.hideDestinationSource) {
    return false;
  }
  return availableSources.value.length > 1 && !props.fileList && !getters.isShare() && !props.browseShare;
});
const usesSearchNavigation = computed(() => !!state.req && !!getters.searchNavigationItems(state.req));

const virtual = useListingVirtualization({
  container: list,
  groups: () => {
    const entries = visibleItems.value;
    return [{ key: "items", count: entries.length, at: (index) => ({ item: entries[index] as ListEntry, index }) }];
  },
  columnsHint: () => 1,
  // Lists in prompts scroll inside the prompt
  scroller: (el) => findScrollParents(el),
  minItems: 10,
});

watch(
  () => props.browseSource,
  (newSource) => {
    if (newSource && newSource !== source.value) {
      currentSource.value = newSource;
      resetToSource(newSource);
    }
  }
);
watch(
  () => props.browseShare,
  (newHash) => {
    if (newHash && newHash !== shareHash.value) {
      resetToShare(newHash);
    }
  }
);
watch(currentSource, (newSource) => {
  if (newSource && newSource !== source.value) {
    resetToSource(newSource);
  }
});
watch(path, () => {
  if (props.filterQuery) {
    emit("update:filterQuery", "");
  }
});
watch(
  () => props.filterQuery,
  () => {
    if (!selected.value) return;
    const stillVisible = visibleItems.value.some((item) => item.path === selected.value);
    if (!stillVisible) {
      clearSelection();
    }
  }
);
watch(currentPromptName, (now) => {
  if (now !== "ContextMenu") return;
  promptsBeforeMenu = promptCount.value - 1;
  ownsContextMenu.value = !root.value?.closest(".floating-window")?.classList.contains("prompt-behind");
});
watch(promptCount, (count) => {
  if (!ownsContextMenu.value || count > promptsBeforeMenu) return;
  ownsContextMenu.value = false;
  const previous = selectionBeforeMenu;
  selectionBeforeMenu = null;
  mutations.resetSelected();
  if (previous?.entries?.length) {
    mutations.setSelected([...previous.entries]);
  }
  if (previous?.multiple) {
    mutations.setMultiple(true);
  }
});
watch(loading, async (isLoading) => {
  if (!isLoading && props.fileList) {
    await nextTick();
    followCurrentItem();
  }
});
// Re-sort local items when the picker header changes the sort config
watch(pickerSort, async () => {
  if (props.sortable || props.fileList) {
    resortItems();
    if (props.fileList) {
      await nextTick();
      scrollToCurrentItem();
    }
  }
});

onMounted(async () => {
  if (props.fileList) {
    fillFromList();
  } else if (props.browseShare || getters.isShare()) {
    const hash = props.browseShare || getters.shareHash();
    const startPath = props.browseShare ? "/" : path.value;
    void withLoading(() => resourcesApi.fetchFilesPublic(startPath, hash).then(fillOptions));
  } else {
    // Normal browse mode: fetch files
    const sourceToUse = currentSource.value;
    const pathToUse = path.value; // Use the path initialized above which respects browsePath
    const initialReq = {
      ...state.req,
      source: sourceToUse,
      path: pathToUse,
    };
    // Fetch the initial data for the source
    if (currentSource.value !== state.req.source || props.browsePath) {
      void withLoading(() => resourcesApi.fetchFiles(sourceToUse, pathToUse).then(fillOptions));
    } else {
      const pending = fillOptions(initialReq);
      if (pending) void withLoading(() => pending);
    }
  }
  attachScrollListener();
  if (props.fileList) {
    await nextTick();
    followCurrentItem();
  }
});

onBeforeUnmount(() => {
  scrollContainer?.removeEventListener("scroll", handleScroll);
  stopFollowingCurrentItem?.();
});

async function withLoading(operation: () => Promise<unknown>) {
  const startTime = Date.now();
  loading.value = true;
  try {
    await operation();
  } catch (error) {
    console.error("FileList fetch error:", error);
    items.value = [];
    if (path.value !== "/" && props.showFolders) {
      items.value.push({
        name: "..",
        path: `${url.removeLastDir(path.value)}/`,
        source: source.value ?? undefined,
        type: "directory",
      });
    }
    // Emit the current (failed) path so parent knows about the state
    emit("update:selected", {
      path: path.value,
      source: source.value,
      type: "directory",
      isValid: !props.requireFileSelection,
      error: true, // Indicate there was an error
    });
  } finally {
    const elapsed = Date.now() - startTime;
    const remaining = Math.max(0, 200 - elapsed);
    await new Promise((resolve) => { setTimeout(resolve, remaining); });
    loading.value = false;
  }
}

// Check if file matches allowed file types
function isFileTypeAllowed(itemType?: string) {
  if (!props.allowedFileTypes || props.allowedFileTypes.length === 0) {
    return true; // No filter, allow all
  }
  if (!itemType || itemType === "directory") {
    return true;
  }
  return props.allowedFileTypes.some((allowedType) => {
    if (allowedType.endsWith("/")) {
      return itemType.startsWith(allowedType);
    }
    return itemType === allowedType;
  });
}

function resetToSource(newSource: string) {
  // Use current path if browsing the same source as current, otherwise start at root
  const newPath = (newSource === state.req.source ? state.req.path : "/") ?? "/";
  // Reset to the appropriate path for the new source
  path.value = newPath;
  source.value = newSource;
  shareHash.value = null;
  selected.value = null;
  selectedSource.value = null;
  selectedType.value = null;
  void withLoading(() => resourcesApi.fetchFiles(newSource, newPath).then(fillOptions));
}

function resetToShare(newHash: string) {
  // Reset to the share root
  path.value = "/";
  shareHash.value = newHash;
  source.value = null;
  selected.value = null;
  selectedSource.value = null;
  selectedType.value = null;
  void withLoading(() => resourcesApi.fetchFilesPublic("/", newHash).then(fillOptions));
}

function fillOptions(req: { source?: string; type?: string; items?: FileListItem[] }): Promise<unknown> | undefined {
  // if the path points to a file browse its parent and highlight the item
  if (req.type && req.type !== "directory" && path.value !== "/") {
    const filePath = path.value;
    const parent = `${url.removeLastDir(filePath)}/`;
    path.value = parent
    const showParent = (parentReq: Parameters<typeof fillOptions>[0]) => {
      const pending = fillOptions(parentReq);
      selectEntry(filePath);
      return pending;
    };
    if (!props.browseSource && (props.browseShare || getters.isShare())) {
      return resourcesApi.fetchFilesPublic(parent, props.browseShare || getters.shareHash()).then(showParent);
    }
    return resourcesApi.fetchFiles(source.value ?? currentSource.value, parent).then(showParent);
  }
  // Sets the current path and resets the current items.
  // Use path (the path we're browsing) instead of req.path (which may be relative)
  current.value = path.value;
  source.value = req.source || source.value; // Preserve the source we're browsing
  items.value = [];

  // Emit both path, source, and validity
  emit("update:selected", {
    path: current.value,
    source: source.value,
    type: "directory",
    isValid: !props.requireFileSelection, // Folder is only valid if file is not required
  });

  const entries: ListEntry[] = [];
  if (req.items && Array.isArray(req.items)) {
    for (const item of req.items) {
      if (!props.showFolders && item.type === "directory") continue;
      if (!props.showFiles && item.type !== "directory") continue;
      if (item.type !== "directory" && !isFileTypeAllowed(item.type)) continue;
      entries.push({
        name: item.name,
        path: item.path,
        source: item.source || req.source,
        type: item.type,
        pinned: !!item.pinned,
        size: item.size,
        modified: item.modified,
        created: item.created,
        metadata: item.metadata,
        originalItem: item as ItemInfo,
      });
    }
    items.value = sortEntries(entries);
  }

  if (path.value !== "/" && props.showFolders) {
    items.value.unshift({
      name: "..",
      path: `${url.removeLastDir(path.value)}/`,
      source: source.value ?? undefined,
      type: "directory",
    });
  }
  return undefined;
}

function selectEntry(itemPath: string) {
  const entry = items.value.find((item) => item.path === itemPath);
  if (!entry || entry.type === "directory") return;
  selected.value = entry.path;
  selectedSource.value = entry.source;
  selectedType.value = entry.type;
  emit("update:selected", { path: entry.path, source: entry.source, type: entry.type, isValid: true });
}

function sortEntries(entries: ListEntry[]) {
  const sorting = pickerSort.value;
  const dirs = entries.filter((item) => item.type === "directory");
  const files = entries.filter((item) => item.type !== "directory");
  return [...sortedItems(dirs, sorting.by, sorting.asc), ...sortedItems(files, sorting.by, sorting.asc)];
}

function resortItems() {
  const parentEntry = items.value.find((item) => item.name === "..");
  const rest = items.value.filter((item) => item.name !== "..");
  const sorted = sortEntries(rest);
  items.value = parentEntry ? [parentEntry, ...sorted] : sorted;
}

function next(itemPath: string) {
  // Fills the options with the content of the directory the user
  // just double clicked in.
  const clickedItem = items.value.find((item) => item.path === itemPath);
  const sourceToUse = clickedItem ? clickedItem.source : source.value;

  // If showFiles and showFolders is true, and clicked item is a file (not a directory), select it directly
  if (props.showFiles && clickedItem && clickedItem.type !== "directory") {
    selected.value = itemPath;
    selectedSource.value = sourceToUse;
    selectedType.value = clickedItem.type;
    emit("update:selected", {
      path: itemPath,
      source: sourceToUse,
      type: clickedItem.type,
      isValid: true, // File is always valid
    });
    return;
  }

  path.value = itemPath;
  // Reset selected when navigating to a directory
  selected.value = null;
  selectedSource.value = null;
  selectedType.value = null;

  // Priority: browseSource > browseShare > isShare
  if (props.browseSource) {
    // Explicitly browsing a source - use files API
    source.value = sourceToUse;
    void withLoading(() => resourcesApi.fetchFiles(sourceToUse, itemPath).then(fillOptions));
  } else if (props.browseShare || getters.isShare()) {
    // Browsing a share - use public API
    const hashToUse = props.browseShare || getters.shareHash();
    void withLoading(() => resourcesApi.fetchFilesPublic(itemPath, hashToUse).then(fillOptions));
  } else {
    source.value = sourceToUse;
    void withLoading(() => resourcesApi.fetchFiles(sourceToUse, itemPath).then(fillOptions));
  }
}

function handleItemClick(item: ListEntry, _index: number, event: MouseEvent) {
  event.preventDefault();
  event.stopPropagation();

  if (props.fileList) {
    if (isCurrentItem(item)) return;
    navigateToItem(item);
    return;
  }
  // Browse mode: single click selects only, emit selected, do not navigate
  select(item.path);
}

function handleItemDblClick(item: ListEntry, _index: number, event: MouseEvent) {
  event.preventDefault();
  event.stopPropagation();

  next(item.path);
}

// ListingItem overwrites state.selected when opening the menu (eg: from quick jump)
// this is to restore the previous selection that the previews use for the overflow menu, otherwise would remain undefined.
function snapshotSelection() {
  if (ownsContextMenu.value || currentPromptName.value === "ContextMenu") return;
  selectionBeforeMenu = {
    entries: Array.isArray(state.selected) ? [...state.selected] : [],
    multiple: state.multiple,
  };
}

function clearSelection() {
  selected.value = null;
  selectedSource.value = null;
  selectedType.value = null;
  emit("update:selected", {
    path: current.value,
    source: source.value,
    type: "directory",
    isValid: !props.requireFileSelection,
  });
}

function select(itemPath: string) {
  if (selected.value === itemPath) {
    clearSelection();
    return;
  }
  // Otherwise select the element.
  selected.value = itemPath;
  const clickedItem = items.value.find((item) => item.path === itemPath);
  selectedSource.value = clickedItem ? clickedItem.source : source.value;
  selectedType.value = clickedItem ? clickedItem.type : null;
  const isFile = clickedItem && clickedItem.type !== "directory";
  emit("update:selected", {
    path: selected.value,
    source: selectedSource.value,
    type: selectedType.value,
    isValid: !props.requireFileSelection || !!isFile,
  });
}

function jumpTo(jumpSource: string, jumpPath: string | null) {
  if (!jumpSource) {
    return;
  }
  let p = jumpPath === null || jumpPath === "" ? "/" : String(jumpPath);
  if (!p.startsWith("/")) {
    p = `/${p}`;
  }
  if (p !== "/" && !p.endsWith("/")) {
    p = `${p}/`;
  }
  currentSource.value = jumpSource;
  source.value = jumpSource;
  path.value = p;
  selected.value = null;
  selectedSource.value = null;
  selectedType.value = null;
  if (props.browseShare || getters.isShare()) {
    const hashToUse = props.browseShare || getters.shareHash();
    void withLoading(() => resourcesApi.fetchFilesPublic(p, hashToUse).then(fillOptions));
  } else {
    void withLoading(() => resourcesApi.fetchFiles(jumpSource, p).then(fillOptions));
  }
}

function fillFromList() {
  const allItems = props.fileList || [];
  const fileItems = allItems.filter((item) => !item.isDirectory && item.type !== "directory");
  items.value = sortEntries(fileItems);
}

function isCurrentItem(item: ListEntry) {
  if (!props.fileList || !state.req) return false;
  if (usesSearchNavigation.value) {
    return item.source === state.req.source && item.path === state.req.path;
  }
  return item.name === state.req.name;
}

// Items selected via right click or long press
function isContextItem(item: ListEntry) {
  return ownsContextMenu.value && !!contextItemPath.value && item.path === contextItemPath.value;
}

function attachScrollListener() {
  const prompt = root.value?.closest<HTMLElement>(".floating-window > .card-content");
  if (!prompt || !root.value) return;
  const el = findScrollParents(root.value).find((node) => node !== prompt && prompt.contains(node)) ?? prompt;
  el.addEventListener("scroll", handleScroll, { passive: true });
  scrollContainer = el;
}

function handleScroll() {
  if (!scrollContainer) return;
  const top = scrollContainer.scrollTop;
  const diff = top - lastScrollTop;
  if (top <= 10 || stopFollowingCurrentItem) {
    headerHidden.value = false;
  } else if (Math.abs(diff) >= 30) {
    headerHidden.value = diff > 0;
  } else {
    return;
  }
  lastScrollTop = top;
}

function scrollToCurrentItem() {
  const listEl = list.value;
  if (!listEl) return null;
  const currentEl = listEl.querySelector<HTMLElement>(".current-item");
  if (!currentEl) {
    if (!virtual.active.value) return null;
    const index = visibleItems.value.findIndex((item) => isCurrentItem(item));
    if (index === -1) return null;
    const container = findScroller(listEl);
    if (!container) return null;
    virtual.scrollToIndex(index, { block: "center", behavior: "instant" });
    return container;
  }
  const container = findScroller(currentEl);
  if (!container) return null;
  const itemTop =
    currentEl.getBoundingClientRect().top - container.getBoundingClientRect().top + container.scrollTop;
  container.scrollTo({
    top: itemTop - (container.clientHeight - currentEl.offsetHeight) / 2,
    behavior: "instant",
  });
  return container;
}

// Keeps the current item centered while the list is still resizing
function followCurrentItem() {
  stopFollowingCurrentItem?.();
  const container = scrollToCurrentItem();
  const listEl = list.value;
  if (!container || !listEl) return;
  const observer = new ResizeObserver(() => scrollToCurrentItem());
  observer.observe(container);
  observer.observe(listEl);
  const events = ["wheel", "touchstart", "pointerdown", "keydown"];
  const timer = setTimeout(() => stopFollowingCurrentItem?.(), 1000);
  const stop = () => {
    observer.disconnect();
    clearTimeout(timer);
    for (const name of events) {
      container.removeEventListener(name, stop);
    }
    stopFollowingCurrentItem = null;
  };
  stopFollowingCurrentItem = stop;
  for (const name of events) {
    container.addEventListener(name, stop, { passive: true });
  }
}

function navigateToItem(item: ListEntry) {
  mutations.closeTopPrompt();
  mutations.setNavigationTransitioning(true);
  const isShare = !!props.browseShare || getters.isShare();
  const itemSource = isShare ? state.shareInfo?.hash : item.source || state.req.source;
  url.goToItem(itemSource, item.path, undefined, false, isShare);
}

defineExpose({ items, path, source, fillOptions, jumpTo });
</script>

<style scoped>
.card-content {
  --listing-col-gap: 0.5rem;
  --listing-col-size: 5rem;
  --listing-col-modified: 8.5rem;
}

/* a bit of padding after the column inset */
.sticky-header :deep(.listing-item-header.desktop-view) {
  padding-right: calc(var(--listing-col-gap) + 0.5em);
}

.sticky-header :deep(.listing-item-header i) {
  font-size: 1.2em;
  margin-left: 0.1em;
}

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
  padding: 0.5em 0 0;
  margin-bottom: 0.5em;
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
