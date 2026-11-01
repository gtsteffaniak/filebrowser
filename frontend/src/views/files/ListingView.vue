<template>
  <div v-if="shareInfo.shareType !== 'upload'" ref="rootEl" class="listing-view-root no-select" :style="containerStyles">
    <!-- Show loading spinner while loading OR if we haven't loaded any data yet -->
    <div v-if="loading">
      <h2 class="message delayed">
        <LoadingSpinner size="medium" />
        <span>{{ $t("general.loading", { suffix: "..." }) }}</span>
      </h2>
    </div>

    <!-- listing container -->
    <div
      v-else
      ref="listingView"
      :class="{
        'add-padding': isStickySidebar,
        [listingViewMode]: true,
        dropping: isDragging,
        'rectangle-selecting': isRectangleSelecting,
        'font-size-large': numDirs + numFiles + numPinned === 0
      }"
      :style="itemStyles"
      class="listing-items"
    >
      <!-- Rectangle selection overlay -->
      <div class="selection-rectangle" :style="rectangleStyle"></div>

      <!-- Drop indicator -->
      <div v-if="isDragging" class="drop-indicator">
        <div class="drop-indicator-content">
          <i class="material-symbols">cloud_upload</i>
          <p>{{ $t("prompts.dragAndDrop") }}</p>
        </div>
      </div>

      <!-- Empty state -->
      <template v-if="numDirs + numFiles + numPinned === 0">
        <h2 class="message font-size-large">
          <i class="material-symbols-outlined">sentiment_dissatisfied</i>
          <span>{{ $t("files.lonely") }}</span>
        </h2>
        <input
          style="display: none"
          type="file"
          id="upload-input"
          @change="uploadInput($event)"
          multiple
        />
        <input
          style="display: none"
          type="file"
          id="upload-folder-input"
          @change="uploadInput($event)"
          webkitdirectory
          multiple
        />
      </template>

      <template v-else>
        <!-- Pinned Items Section -->
        <div v-if="numPinned > 0">
          <h2>{{ pinnedHeaderText }}</h2>
        </div>
        <div
          v-if="numPinned > 0"
          class="pinned-items"
          role="group"
          aria-label="Pinned Items"
          :class="{ lastGroup: numDirs === 0 && numFiles === 0, firstGroup: true }"
        >
          <Item
            v-for="(item, idx) in pinnedItems"
            :key="`pinned-${item.path || item.name}`"
            :class="{ 'zebra-row': idx % 2 === 1 }"
            v-bind:index="item.index"
            v-bind:name="item.name"
            v-bind:isDir="item.type === 'directory'"
            v-bind:source="req.source"
            v-bind:modified="item.modified"
            v-bind:created="item.created"
            v-bind:type="item.type"
            v-bind:size="item.size"
            v-bind:path="item.path"
            v-bind:reducedOpacity="item.hidden"
            v-bind:hash="shareInfo.hash"
            v-bind:hasPreview="item.hasPreview"
            v-bind:metadata="item.metadata"
            v-bind:hasDuration="hasDuration"
            v-bind:isShared="item.isShared"
            v-bind:pinned="item.pinned"
            v-bind:viewToken="item.viewToken"
          />
        </div>

        <!-- Directories Section -->
        <div v-if="numDirs > 0">
          <h2>{{ $t("general.folders") }}</h2>
        </div>
        <div
          v-if="numDirs > 0"
          class="folder-items"
          role="group"
          aria-label="Folder Items"
          :class="{ lastGroup: numFiles === 0, firstGroup: numPinned === 0 }"
        >
          <Item
            v-for="(item, idx) in dirs"
            :key="item.path || item.name"
            :class="{ 'zebra-row': (numPinned + idx) % 2 === 1 }"
            v-bind:index="item.index"
            v-bind:name="item.name"
            v-bind:isDir="item.type === 'directory'"
            v-bind:source="req.source"
            v-bind:modified="item.modified"
            v-bind:created="item.created"
            v-bind:type="item.type"
            v-bind:size="item.size"
            v-bind:path="item.path"
            v-bind:reducedOpacity="item.hidden"
            v-bind:hash="shareInfo.hash"
            v-bind:hasPreview="item.hasPreview"
            v-bind:hasDuration="hasDuration"
            v-bind:isShared="item.isShared"
            v-bind:pinned="item.pinned"
            v-bind:viewToken="item.viewToken"
          />
        </div>

        <!-- Files Section -->
        <div v-if="numFiles > 0">
          <h2>{{ $t("general.files") }}</h2>
        </div>
        <div
          v-if="numFiles > 0"
          class="file-items"
          :class="{ lastGroup: numFiles > 0, firstGroup: numPinned === 0 && numDirs === 0 }"
          role="group"
          aria-label="File Items"
        >
          <Item
            v-for="(item, idx) in files"
            :key="item.path || item.name"
            :class="{ 'zebra-row': (numPinned + numDirs + idx) % 2 === 1 }"
            v-bind:index="item.index"
            v-bind:name="item.name"
            v-bind:isDir="item.type === 'directory'"
            v-bind:modified="item.modified"
            v-bind:created="item.created"
            v-bind:source="req.source"
            v-bind:type="item.type"
            v-bind:size="item.size"
            v-bind:path="item.path"
            v-bind:reducedOpacity="item.hidden"
            v-bind:hash="shareInfo.hash"
            v-bind:hasPreview="item.hasPreview"
            v-bind:metadata="item.metadata"
            v-bind:hasDuration="hasDuration"
            v-bind:isShared="item.isShared"
            v-bind:pinned="item.pinned"
            v-bind:viewToken="item.viewToken"
          />
        </div>

        <input
          style="display: none"
          type="file"
          id="upload-input"
          @change="uploadInput($event)"
          multiple
        />
        <input
          style="display: none"
          type="file"
          id="upload-folder-input"
          @change="uploadInput($event)"
          webkitdirectory
          multiple
        />
      </template>
    </div>
  </div>

  <!-- Upload Share Target -->
  <!-- Only show upload interface if password is validated (or no password required) -->
  <div v-else-if="!shareInfo.hasPassword || shareInfo.passwordValid" ref="rootEl" class="upload-share-embed">
    <Upload />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import type { fileListItemIndex } from "@/store/types";
import { useI18n } from "vue-i18n";
import downloadFiles from "@/utils/download";
import { resourcesApi } from "@/api";
import { router } from "@/router";
import { readAllDirectoryEntries, checkConflict } from "@/utils/upload";
import throttle from "@/utils/throttle";
import { state, mutations, getters } from "@/store";
import { eventBus } from "@/store/eventBus";
import { url } from "@/utils";
// biome-ignore lint/correctness/noUnusedImports: false positive, Item is used in template
import Item from "@/components/files/ListingItem.vue";
import Upload from "@/components/prompts/Upload.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import {
  isTypeAheadSessionActive,
  processTypeAheadKey,
  resetTypeAheadSession,
} from "@/utils/listingTypeAhead.js";
import { notifyMoveCopyFailure } from "@/utils/appNotifications";

defineOptions({ name: "listingView" });

interface Point {
  x: number;
  y: number;
}

interface ItemRect {
  index: number;
  left: number;
  top: number;
  right: number;
  bottom: number;
}

interface PointerInfo extends Point {
  additive: boolean;
}

interface ClipboardEntry {
  from: string;
  fromSource: string;
  name: string;
  isDir: boolean;
}

interface InternalClipboard {
  key?: string;
  items: ClipboardEntry[];
  path?: string;
}

interface CollectedFile {
  file: File;
  relativePath: string;
}

type PastedItem =
  | { type: "entry"; entry: FileSystemEntry }
  | { type: "file"; file: File };

const { t } = useI18n();

const rootEl = ref<HTMLElement | null>(null);
const listingView = ref<HTMLElement | null>(null);

const columnWidth = ref(250 + state.user.gallerySize * 50);
const dragTargets = reactive(new Set<Element>());
const width = ref(window.innerWidth);
const isRectangleSelecting = ref(false);
const rectangleStart = ref<Point>({ x: 0, y: 0 });
const rectangleEnd = ref<Point>({ x: 0, y: 0 });

// Non reactive on purpose: big DOM-derived caches and transient interaction state.
let itemNodes: HTMLCollectionOf<Element> | null = null; // .listing-item elements
let itemNodesHost: HTMLElement | null = null;
let rectangleItemRects: ItemRect[] | null = null;
let contentResizeObserver: ResizeObserver | null = null;
let observedListing: HTMLElement | null = null;
let lastSelected: typeof state.selected | null = null;
let clipboard: InternalClipboard = { items: [] };
let internalClipboardTimestamp = 0;
let contextTimeout: ReturnType<typeof setTimeout> | null = null;
let resizeTimeout: ReturnType<typeof setTimeout> | null = null;
let rafId: number | null = null;
let lastPointer: PointerInfo | null = null;
let selectionUpdatePending = false;
let isResizing = false;
let isSwipe = false;
let touchStartX = 0;
let touchStartY = 0;
let initialSelectionState: typeof state.selected = [];

const permissions = computed(() => ({ ...getters.globalPermissions(), ...getters.sourcePermissions() }));
const shareInfo = computed(() => state.shareInfo);
const isDragging = computed(() => {
  const el = rootEl.value;
  if (!el) return false;
  if (getters.isShare()) {
    return !!state.shareInfo.allowCreate && dragTargets.has(el);
  }
  return dragTargets.has(el);
});
const scrolling = computed(() => state.listing.listingScrollTop);
const isStickySidebar = computed(() => getters.isStickySidebar());
const items = computed(
  () =>
    getters.reqItems() as {
      pinned: fileListItemIndex[];
      dirs: fileListItemIndex[];
      files: fileListItemIndex[];
    },
);
const pinnedItems = computed(() => items.value.pinned || []);
const dirs = computed(() => items.value.dirs);
const files = computed(() => items.value.files);
const allItems = computed(() => pinnedItems.value.concat(dirs.value, files.value));
const numPinned = computed(() => pinnedItems.value.length);
const numFiles = computed(() => files.value.length);
const numDirs = computed(() => dirs.value.length);
const numColumns = computed(() => {
  void width.value;
  if (!getters.isCardView()) {
    return 1;
  }
  const elem = document.querySelector<HTMLElement>("#main");
  if (!elem) {
    return 1;
  }
  if (getters.viewMode() === "icons") {
    const containerSize = 70 + state.user.gallerySize * 15;
    let columns = Math.floor(elem.offsetWidth / containerSize);
    if (columns === 0) {
      columns = 1;
    }
    const minColumns = 3;
    const maxColumns = 12;
    return Math.max(minColumns, Math.min(columns, maxColumns));
  }
  let columns = Math.floor(elem.offsetWidth / columnWidth.value);
  if (columns === 0) {
    columns = 1;
  }
  return columns;
});
const gallerySize = computed(() => state.user.gallerySize);
const hasDuration = computed(() => {
  // Check if any pinned file or regular file has duration metadata
  const hasMetadataDuration = (file: fileListItemIndex) =>
    file.type !== "directory" && file.metadata?.duration;
  return pinnedItems.value.some(hasMetadataDuration) || files.value.some(hasMetadataDuration);
});
const pinnedHeaderText = computed(() => {
  const pinnedFolders = pinnedItems.value.filter((item) => item.type === "directory").length;
  const pinnedFiles = pinnedItems.value.filter((item) => item.type !== "directory").length;
  if (pinnedFolders > 0 && pinnedFiles === 0) {
    return t("general.pinnedFolders");
  }
  if (pinnedFiles > 0 && pinnedFolders === 0) {
    return t("general.pinnedFiles");
  }
  return t("general.pinned");
});
const listingViewMode = computed(() => getters.viewMode());
const selectedCount = computed(() => state.selected.length);
const req = computed(() => state.req);
const loading = computed(() => getters.isLoading());
const rectangleStyle = computed(() => {
  if (!isRectangleSelecting.value) return { display: "none" };
  const left = Math.min(rectangleStart.value.x, rectangleEnd.value.x);
  const top = Math.min(rectangleStart.value.y, rectangleEnd.value.y);
  const width = Math.abs(rectangleStart.value.x - rectangleEnd.value.x);
  const height = Math.abs(rectangleStart.value.y - rectangleEnd.value.y);
  return {
    left: `${left}px`,
    top: `${top}px`,
    width: `${width}px`,
    height: `${height}px`,
  };
});
const containerStyles = computed(() => {
  // Dynamic padding-top: applied to the entire container (loading spinner + listing items)
  const isEmpty = numDirs.value + numFiles.value + numPinned.value === 0;
  const isRootPath = state.req.path === "/" || !state.req.path;
  if (isEmpty) {
    return { "padding-top": "4.1em" }; // Empty - no files or folders
  } else if (isRootPath) {
    return { "padding-top": "4.29em" }; // Root - no breadcrumbs showing
  } else {
    return { "padding-top": "7.51em" }; // Non-root - breadcrumbs + listing header
  }
});
const itemStyles = computed(() => {
  const viewMode = getters.viewMode();
  const styles: Record<string, string> = {};
  const size = state.user.gallerySize;

  if (viewMode === "icons") {
    const baseSize = 20 + (size * 15); // 35px to 155px
    const cellSize = baseSize + 30;
    styles["--icon-size"] = `${baseSize}px`;
    styles["--icon-font-size"] = `${baseSize}px`;
    styles["--icons-view-cell-size"] = `${cellSize}px`;
  } else if (viewMode === "gallery") {
    const baseCalc = 80 + (size * 25);
    const extraScaling = Math.max(0, size - 5) * 15;
    const baseSize = baseCalc + extraScaling; // Size 5: 205px, Size 9: 345px
    const iconFontSize = (3 + (size * 0.5)).toFixed(2); // 3em to 7.5em
    styles["--icon-font-size"] = `${iconFontSize}em`;

    if (getters.isMobile()) {
      const minWidth = size <= 3 ? 120 : size <= 7 ? 160 : 280;
      const mobileHeight = 120 + (size * 20); // 120px to 300px
      styles["--gallery-mobile-min-width"] = `${minWidth}px`;
      styles["--item-width"] = `${minWidth}px`;
      styles["--item-height"] = `${mobileHeight}px`;
    } else {
      styles["--item-width"] = `${baseSize}px`;
      styles["--item-height"] = `${Math.round(baseSize * 1.2)}px`;
    }
  } else if (viewMode === "list" || viewMode === "compact") {
    const baseHeight = viewMode === "compact"
      ? 40 + (size * 2)  // 42px to 58px
      : 50 + (size * 3); // 53px to 77px
    const iconSize = (2 + (size * 0.12)).toFixed(2); // 2.12em to 3.08em
    const iconFontSize = (1.5 + (size * 0.12)).toFixed(2); // 1.62em to 2.58em

    styles["--item-width"] = `calc(${(100 / numColumns.value).toFixed(2)}% - 1em)`;
    styles["--item-height"] = `${baseHeight}px`;
    styles["--icon-size"] = `${iconSize}em`;
    styles["--icon-font-size"] = `${iconFontSize}em`;
  } else {
    // Normal view
    const iconSize = (3.2 + (size * 0.15)).toFixed(2); // 3.35em to 4.55em
    const iconFontSize = (2.2 + (size * 0.12)).toFixed(2); // 2.32em to 3.28em

    styles["--item-width"] = `calc(${(100 / numColumns.value)}% - 1em)`;
    styles["--item-height"] = "auto";
    styles["--icon-size"] = `${iconSize}em`;
    styles["--icon-font-size"] = `${iconFontSize}em`;
  }

  return styles;
});

watch(items, () => {
  rectangleItemRects = null;
});

watch(loading, () => {
  void nextTick(observeListingContent);
});

watch(gallerySize, () => {
  columnWidth.value = 250 + state.user.gallerySize * 50;
});

watch(scrolling, () => {
  const scrollContainer = listingView.value;
  const root = rootEl.value;
  if (!scrollContainer || !root) return;

  // Select all visible listing items
  const nodes = getListingItemNodes(scrollContainer);
  // Find the first item at/below the top of the viewport
  let lo = 0;
  let hi = nodes.length;
  while (lo < hi) {
    const mid = (lo + hi) >>> 1;
    if ((nodes[mid] as Element).getBoundingClientRect().top >= 0) {
      hi = mid;
    } else {
      lo = mid + 1;
    }
  }
  const topItem = nodes[lo] ?? null;
  // Decide category by checking which section is above
  let letter = "A";
  let category = "folders"; // Default category

  if (topItem) {
    letter = topItem.getAttribute("data-name")?.[0]?.toUpperCase() || "A";
  } else if (numPinned.value > 0) {
    const pinnedHeader = root.querySelector(".pinned-items h2");
    if (pinnedHeader && pinnedHeader.getBoundingClientRect().top >= 0) {
      category = "pinned";
      const firstPinned = pinnedItems.value[0];
      letter = firstPinned?.name?.[0]?.toUpperCase() as string;
    }
  }

  if (topItem?.closest(".pinned-items")) {
    category = "pinned";
  }
  else if (numFiles.value > 0) {
    const fileSection = root.querySelector(".file-items");
    const fileTop = fileSection?.getBoundingClientRect().top ?? 0;
    category = fileTop <= 0 ? "files" : "folders";
  }
  if (numDirs.value === 0 && category !== "pinned") {
    category = "files"; // If no directories, only files
  }

  mutations.updateListing({
    ...state.listing,
    category,
    letter,
  });
});

onMounted(() => {
  mutations.setSearch(false);
  lastSelected = state.selected;

  const el = rootEl.value;

  // Add the needed event listeners to the window and document.
  window.addEventListener("keydown", keyEvent);
  window.addEventListener("resize", windowsResize);
  contentResizeObserver = new ResizeObserver(() => eventBus.emit("scrollContentResized"));
  observeListingContent();
  window.addEventListener("click", clickClear);
  window.addEventListener("dragover", preventDefault);
  window.addEventListener("paste", handlePaste);
  document.addEventListener("mousemove", updateRectangleSelection, { passive: true });
  document.addEventListener("mouseup", endRectangleSelection);
  el?.addEventListener("mousedown", startRectangleSelection);
  el?.addEventListener("touchmove", handleTouchMove, { passive: true });
  el?.addEventListener("dblclick", handleDoubleClick);

  // Single dragend listener for all items (prevents N listeners for N items)
  document.addEventListener("dragend", handleGlobalDragEnd, { passive: true });

  el?.addEventListener("contextmenu", openContext);
  // Adjust contextmenu listener based on browser
  if (state.isSafari) {
    // For Safari, add touchstart or mousedown to open the context menu
    el?.addEventListener("touchstart", openContextForSafari, {
      passive: true,
    });
    el?.addEventListener("mousedown", openContextForSafari);

    // Also clear the timeout if the user clicks or taps quickly
    el?.addEventListener("touchend", cancelContext);
    el?.addEventListener("mouseup", cancelContext);
  }

  // if safari , make sure click and hold opens context menu, but not for any other browser
  // Always listen: dragEnter and drop check create on the current source on every event. Deciding
  // here, once, left no drop target after navigating from a source the user cannot write to.
  el?.addEventListener("dragenter", dragEnter);
  el?.addEventListener("dragleave", dragLeave);
  el?.addEventListener("drop", drop);
});

onBeforeUnmount(() => {
  resetTypeAheadSession();
  contentResizeObserver?.disconnect();

  if (resizeTimeout) {
    clearTimeout(resizeTimeout);
    resizeTimeout = null;
  }

  const el = rootEl.value;

  // Remove event listeners before destroying this page.
  window.removeEventListener("keydown", keyEvent);
  window.removeEventListener("resize", windowsResize);
  window.removeEventListener("click", clickClear);
  window.removeEventListener("dragover", preventDefault);
  window.removeEventListener("paste", handlePaste);
  document.removeEventListener("mousemove", updateRectangleSelection);
  stopRectangleScrollListener();
  if (rafId) {
    cancelAnimationFrame(rafId);
    rafId = null;
  }
  document.removeEventListener("mouseup", endRectangleSelection);
  document.removeEventListener("dragend", handleGlobalDragEnd);
  el?.removeEventListener("mousedown", startRectangleSelection);
  el?.removeEventListener("dblclick", handleDoubleClick);

  el?.removeEventListener("touchmove", handleTouchMove);
  el?.removeEventListener("contextmenu", openContext);

  // If Safari, remove touch/mouse listeners
  if (state.isSafari) {
    el?.removeEventListener("touchstart", openContextForSafari);
    el?.removeEventListener("mousedown", openContextForSafari);
    el?.removeEventListener("touchend", cancelContext);
    el?.removeEventListener("mouseup", cancelContext);
  }

  // Also clean up drag/drop listeners on the component's root element
  el?.removeEventListener("dragenter", dragEnter);
  el?.removeEventListener("dragleave", dragLeave);
  el?.removeEventListener("drop", drop);
});

function observeListingContent() {
  const el = listingView.value || null;
  if (!contentResizeObserver || el === observedListing) return;
  contentResizeObserver.disconnect();
  observedListing = el;
  if (el) contentResizeObserver.observe(el);
}

function handleGlobalDragEnd() {
  // Reset drag state for all items (replaces per-item dragend listeners)
  const listingItems = rootEl.value?.querySelectorAll(".listing-item.drag-hover, .listing-item.half-selected");
  if (listingItems) {
    for (const el of listingItems) {
      el.classList.remove("drag-hover", "half-selected");
    }
  }
  dragTargets.clear();
}

function cancelContext() {
  if (contextTimeout) {
    clearTimeout(contextTimeout);
    contextTimeout = null;
  }
}

function openContextForSafari(event: TouchEvent | MouseEvent) {
  cancelContext(); // Clear any previous timeouts
  isSwipe = false; // Reset swipe detection

  // mousedown is also registered here and has no touches
  const touch = (event as TouchEvent).touches?.[0];
  if (!touch) return;
  touchStartX = touch.clientX;
  touchStartY = touch.clientY;

  // Start the long press detection
  contextTimeout = setTimeout(() => {
    if (!isSwipe) {
      event.preventDefault(); // Suppress Safari's callout menu
      openContext(event); // Open the custom context menu
    }
  }, 500); // Long press delay (adjust as needed)
}

function handleTouchMove(event: TouchEvent) {
  const touch = event.touches[0];
  if (!touch) return;
  const deltaX = Math.abs(touch.clientX - touchStartX);
  const deltaY = Math.abs(touch.clientY - touchStartY);
  // Set a threshold for movement to detect a swipe
  const movementThreshold = 10; // Adjust as needed
  if (deltaX > movementThreshold || deltaY > movementThreshold) {
    isSwipe = true;
    cancelContext(); // Cancel long press if swipe is detected
  }
}

// stays in sync with the DOM and avoids rebuilding a NodeList on every call
function getListingItemNodes(container: HTMLElement) {
  if (itemNodesHost !== container || !itemNodes) {
    itemNodesHost = container;
    itemNodes = container.getElementsByClassName("listing-item");
  }
  return itemNodes;
}

// Item boxes relative to the listing element. Relative values don't change while scrolling
// so they can be measured once and reused every frame.
function measureItemRects(listingRect: DOMRect) {
  const rects: ItemRect[] = [];
  for (const element of rootEl.value?.querySelectorAll(".listing-item[data-index]") ?? []) {
    const index = parseInt(element.getAttribute("data-index") as string, 10);
    if (Number.isNaN(index)) continue;
    const r = element.getBoundingClientRect();
    rects.push({
      index,
      left: r.left - listingRect.left,
      top: r.top - listingRect.top,
      right: r.right - listingRect.left,
      bottom: r.bottom - listingRect.top,
    });
  }
  return rects;
}

function showDeletePrompt() {
  const promptItems = [];
  for (const index of state.selected as number[]) {
    const item = state.req.items.at(index);
    if (!item) continue;
    const previewUrl = item.hasPreview
      ? resourcesApi.getPreviewURL(item.source || state.req.source, item.path, item.modified)
      : null;
    promptItems.push({
      source: item.source || state.req.source,
      path: item.path,
      type: item.type,
      size: item.size,
      modified: item.modified,
      created: item.created,
      hasPreview: item.hasPreview,
      previewUrl: previewUrl,
    });
  }
  mutations.showPrompt({
    name: "delete",
    props: { items: promptItems },
  });
}

// Helper method to select the first item if nothing is selected
function selectFirstItem() {
  mutations.resetSelected();
  const first = allItems.value[0];
  if (first) {
    mutations.addSelected(first.index);
  }
}

// Helper method to find the closest item in the given direction (up or down) from the current one.
function findClosestItem(selectedItem: Element, direction: "up" | "down") {
  const listItems = rootEl.value?.querySelectorAll(".listing-item") ?? [];
  const selectedIndex = Array.prototype.indexOf.call(listItems, selectedItem);
  if (selectedIndex === -1) return null;

  const selectedBounds = selectedItem.getBoundingClientRect();
  const selectedMidX = (selectedBounds.left + selectedBounds.right) / 2;
  const down = direction === "down";

  let closestItem: Element | null = null;
  let closestDistance = Infinity;

  const step = down ? 1 : -1;
  for (let i = selectedIndex + step; i >= 0 && i < listItems.length; i += step) {
    const item = listItems[i] as Element;
    const itemBounds = item.getBoundingClientRect();
    const verticalGap = down
      ? itemBounds.top - selectedBounds.bottom
      : selectedBounds.top - itemBounds.bottom;

    if (verticalGap <= 0) continue; // same row as the selected item
    if (verticalGap >= closestDistance) break;

    const itemMidX = (itemBounds.left + itemBounds.right) / 2;
    const distance = Math.hypot(Math.abs(itemMidX - selectedMidX), verticalGap);
    if (distance < closestDistance) {
      closestDistance = distance;
      closestItem = item;
    }
  }
  return closestItem;
}

function moveSelectionBy(step: number) {
  const all = allItems.value;
  const selectedIndex = state.selected.length > 0 ? state.selected[0] : null;
  if (selectedIndex === null) {
    selectFirstItem();
    return false;
  }
  const currentItemIndex = all.findIndex((item) => item.index === selectedIndex);
  if (currentItemIndex === -1) {
    selectFirstItem();
    return false;
  }
  const nextItemIndex = currentItemIndex + step;
  const targetItem = all.at(nextItemIndex);
  if (nextItemIndex >= 0 && targetItem) {
    mutations.resetSelected();
    mutations.addSelected(targetItem.index);
    scrollSelectedIntoView();
    return true;
  }
  return false;
}

// Helper method to handle selection based on arrow keys
function navigateKeyboardArrows(arrowKey: string) {
  const isCardView = getters.isCardView(); // gallery, normal, icons
  const selectedIndex = state.selected.length > 0 ? state.selected[0] : null;

  if (selectedIndex === null) {
    // If nothing is selected, select the first item
    selectFirstItem();
    return;
  }

  // Left/Right arrows will always use the visual order that we see
  if (arrowKey === "ArrowLeft" || arrowKey === "ArrowRight") {
    const step = arrowKey === "ArrowLeft" ? -1 : 1;
    moveSelectionBy(step);
    return;
  }

  // On list/compact views we use a simple linear navigation since all the items are aligned
  if (!isCardView) {
    const step = arrowKey === "ArrowUp" ? -1 : 1;
    moveSelectionBy(step);
    return;
  }

  // But for gallery, normal, icons views, we need to find the closest item
  // because the rows aren't always consistent (some have 1 item, others 5, etc)
  // which caused "random jumps"
  const selectedItem = rootEl.value?.querySelector(`.listing-item[data-index="${selectedIndex}"]`);
  if (!selectedItem) return;

  let nextItem: Element | null = null;

  switch (arrowKey) {
    case "ArrowDown":
      nextItem = findClosestItem(selectedItem, "down");
      break;
    case "ArrowUp":
      nextItem = findClosestItem(selectedItem, "up");
      break;
  }

  const itemIndex = parseInt((nextItem as HTMLElement | null)?.dataset.index as string, 10);
  if (!Number.isNaN(itemIndex)) {
    mutations.resetSelected();
    mutations.addSelected(itemIndex);
    scrollSelectedIntoView();
  }
}

function scrollSelectedIntoView() {
  setTimeout(() => {
    const element = document.querySelector(
      '.listing-item[aria-selected="true"]'
    );
    if (element) {
      element.scrollIntoView({
        behavior: "smooth",
        block: "nearest",
        inline: "nearest",
      });
    }
  }, 50);
}

function keyEvent(event: KeyboardEvent) {
  const { key, ctrlKey, metaKey, altKey, which } = event;
  const isArrowKey = key === "ArrowUp" ||
                     key === "ArrowDown" ||
                     key === "ArrowLeft" ||
                     key === "ArrowRight";
  if (state.isSearchActive || getters.currentView() !== "listingView" ||
    getters.currentPromptName() || (event.repeat && (!isArrowKey || altKey))) return;

  if (
    (key === " " || key === "/") &&
    isTypeAheadSessionActive() &&
    !state.isSearchActive &&
    !getters.currentPromptName()
  ) {
    event.preventDefault();
    return;
  }

  const isAlphanumeric = /^[a-z0-9]$/i.test(key);
  const modifierKeys = ctrlKey || metaKey;
  if (isAlphanumeric && !modifierKeys && state.selected.length <= 1) {
    const target = event.target as HTMLElement | null;
    const tag = target?.tagName ? target.tagName.toLowerCase() : "";
    if (
      tag !== "input" &&
      tag !== "textarea" &&
      tag !== "select" &&
      !target?.isContentEditable
    ) {
      event.preventDefault();
      alphanumericKeyPress(key);
      return;
    }
  }

  if (modifierKeys) {
    const charKey = String.fromCharCode(which).toLowerCase();

    switch (charKey) {
      case "c":
      case "x":
        copyCut(event, charKey);
        return;
      case "a":
        event.preventDefault();
        mutations.selectAllItems({ multiple: false });
        return;
      case "d":
        event.preventDefault();
        downloadFiles(state.selected);
        return;
    }
    // Don't return here - allow other modifier key combinations to propagate
  }

  // Handle key events using a switch statement
  let shortcut = key;
  if (altKey) shortcut = `Alt+${key}`;

  switch (shortcut) {
    case "Alt+ArrowUp":
    case "Backspace": {
      event.preventDefault();
      // get current path and its parent
      const currentPath = state.req.path || "/";
      const parentPath = url.removeLastDir(currentPath);
      if (parentPath === currentPath) {
        return;
      }
      const source = getters.isShare() ? state.shareInfo.hash : state.req.source;
      const newPath = url.buildItemUrl(source, parentPath);
      void router.push({ path: newPath });
      break;
    }

    case "Alt+ArrowDown":
    case "Enter": {
      event.preventDefault();
      if (selectedCount.value === 1) {
        const selected = getters.getFirstSelected();
        if (!selected) return;
        const selectedUrl = url.buildItemUrl(selected.source, selected.path);
        if (selectedUrl === state.route.path) return;
        void router.push({ path: selectedUrl });
      }
      break;
    }

    case "Escape":
      if (dragTargets.size > 0) {
        dragTargets.clear();
        event.preventDefault();
        return;
      }
      mutations.resetSelected();
      break;

    case "Delete":
      if (!permissions.value?.modify || state.selected.length === 0) return;
      showDeletePrompt();
      break;

    case "ArrowUp":
    case "ArrowDown":
    case "ArrowLeft":
    case "ArrowRight":
      event.preventDefault();
      navigateKeyboardArrows(key);
      break;
  }
}

function alphanumericKeyPress(key: string) {
  const selectedIndex = state.selected.length === 1 ? state.selected[0] : null;
  const { matches, nextPos } = processTypeAheadKey(key, allItems.value, selectedIndex as number | null);
  if (matches.length === 0) {
    return;
  }

  const target = matches.at(nextPos);
  if (!target) return;
  mutations.resetSelected();
  mutations.addSelected(target.index);
  scrollSelectedIntoView();
}

function preventDefault(event: Event) {
  // Wrapper around prevent default.
  event.preventDefault();
}

function copyCut(event: KeyboardEvent, key: string) {
  if ((event.target as HTMLElement).tagName.toLowerCase() === "input") {
    return;
  }

  const clipboardItems = (state.selected as number[])
    .map((i): ClipboardEntry | null => {
      const item = state.req.items.at(i);
      if (!item) return null;
      return {
        from: item.path,
        fromSource: state.req.source as string,
        name: item.name,
        isDir: item.type === "directory",
      };
    })
    .filter((entry): entry is ClipboardEntry => entry !== null);

  if (clipboardItems.length === 0) {
    return;
  }

  clipboard = {
    key: key,
    items: clipboardItems,
    path: state.route.path,
  };
  internalClipboardTimestamp = Date.now();
}

async function collectFilesFromEntry(entry: FileSystemEntry, relativePath = ""): Promise<CollectedFile[]> {
  const collected: CollectedFile[] = [];
  const entryPath = relativePath ? `${relativePath}/${entry.name}` : entry.name;

  if (entry.isFile) {
    // If it's a file we get the File object and add it with its relative path
    const file = await new Promise<File>((resolve, reject) => {
      (entry as FileSystemFileEntry).file(resolve, reject);
    });
    collected.push({
      file,
      relativePath: entryPath,
    });
  } else if (entry.isDirectory) {
    // But if it's a directory, read the contents (readEntries may require
    // multiple calls — browsers often return at most ~100 entries per batch)
    const reader = (entry as FileSystemDirectoryEntry).createReader();
    const entries: FileSystemEntry[] = await readAllDirectoryEntries(reader, entryPath);
    // and then for each child recursively collect files
    for (const childEntry of entries) {
      const childFiles = await collectFilesFromEntry(
        childEntry,
        entryPath
      );
      collected.push(...childFiles);
    }
  }
  return collected;
}

async function getClipboardItems(pastedItems: PastedItem[]) {
  const collected: CollectedFile[] = [];
  for (const item of pastedItems) {
    if (item.type === "entry") {
      collected.push(...await collectFilesFromEntry(item.entry));
    } else if (item.type === "file") {
      collected.push({ file: item.file, relativePath: item.file.name });
    }
  }
  return collected;
}

async function handlePaste(event: ClipboardEvent) {
  // We can paste multiple files from clipboard and upload all of them at once, but
  // Firefox has an issue https://bugzilla.mozilla.org/show_bug.cgi?id=864052  (12 years old lol)
  // the clipboard entry is always getting 1 file (the first file/folder in selection),
  // while chromium sees all and has no issues with it. Not sure about safari.
  if (getters.currentView() !== "listingView" || getters.currentPromptName()) return;

  // Check if internal clipboard is recent (20 seconds)
  const internalRecent = clipboard.items.length > 0 && (Date.now() - internalClipboardTimestamp) < 20000;

  // If internal is recent (<20s), use it immediately, in case someone have both: internal and external clipboard with a file entry.
  // After those 20s, if the OS clipboard has a file as most recent entry, will use that.
  if (internalRecent) {
    await handleInternalPaste();
    event.preventDefault();
    event.stopPropagation();
    return;
  }

  if (event.clipboardData?.items) {
    // Collect all items from clipboard
    const collectedItems: PastedItem[] = [];
    // And loop through all items
    for (const item of Array.from(event.clipboardData.items)) {
      if (item.kind !== "file") continue;

      const entry = item.webkitGetAsEntry();
      if (entry) {
        collectedItems.push({ type: "entry", entry });
      } else {
        const file = item.getAsFile();
        if (file) {
          collectedItems.push({ type: "file", file });
        }
      }
    }

    // Then process the collected items (files and folders) asynchronously
    const itemsFromClipboard = await getClipboardItems(collectedItems);
    const pastedFiles = itemsFromClipboard.map((item) => item.file);

    // If found external files, upload them
    if (pastedFiles.length > 0) {
      event.preventDefault();
      event.stopPropagation();

      const canUpload = getters.sourcePermissions()?.modify;
      if (canUpload) {
        // Pass the full array of {file, relativePath} to preserve directory structure
        mutations.showPrompt({
          name: "upload",
          props: {
            initialItems: itemsFromClipboard,
            targetPath: state.req.path,
          },
        });
      }
      return;
    }
  }
  // If internal clipboard exists but is not recent (>20s), and we don't have any file in clipboard, use the internal one
  if (clipboard.items.length > 0) {
    console.log("No external files, using internal clipboard.");
    await handleInternalPaste();
    event.preventDefault();
    event.stopPropagation();
    return;
  }
}

async function handleInternalPaste() {
  if (!clipboard?.items || clipboard.items.length === 0) {
    return;
  }

  const operation = clipboard.key === "x" ? "move" : "copy";
  // Pasting writes into the current source; a cut also removes the items from where they were cut,
  // and a share refuses any change unless it allows modify.
  const canPaste =
    getters.sourcePermissions().create &&
    clipboard.items.every((item) => {
      const origin = getters.sourcePermissions(item.fromSource);
      return operation === "move" || getters.isShare() ? origin.modify : origin.download;
    });
  if (!canPaste) {
    return;
  }

  // Construct destination path properly (without URL prefix)
  const reqPath = state.req.path as string;
  const destPath = reqPath.endsWith("/") ? reqPath : `${reqPath}/`;

  const items = clipboard.items.map((item) => ({
    from: item.from,
    fromSource: item.fromSource,
    to: destPath + item.name,
    toSource: state.req.source,
    name: item.name,
    isDir: item.isDir,
  }));

  // Show confirmation prompt first
  mutations.showPrompt({
    name: "CopyPasteConfirm",
    props: {
      operation: operation,
      items: items,
      onConfirm: () => {
        return new Promise<void>((resolve, reject) => {

          const action = async (overwrite: boolean, rename: boolean) => {
            try {
              if (getters.isShare()) {
                await resourcesApi.moveCopyPublic(state.shareInfo.hash, items, operation, overwrite, rename);
              } else {
                await resourcesApi.moveCopy(items, operation, overwrite, rename);
              }
              if (operation === "move") {
                clipboard = { items: [] };
                internalClipboardTimestamp = 0;
              }
              mutations.setReload(true);
              resolve();
            } catch (error) {
              notifyMoveCopyFailure(error);
              reject(error);
            }
          };

          if (clipboard.path === state.route.path) {
            void action(false, true);
            return;
          }

          const conflict = checkConflict(items, state.req.items);

          if (conflict) {
            mutations.showPrompt({
              name: "replace-rename",
              pinned: true,
              confirm: (event: Event, option: string) => {
                const overwrite = option === "overwrite";
                const rename = option === "rename";
                event.preventDefault();
                mutations.closeTopPrompt();
                void action(overwrite, rename);
              },
            });
            return;
          }
          void action(false, false);
        });
      },
    },
  });
}

function dragEnter(event: DragEvent) {
  // If in upload share mode, let the embedded Upload component handle it
  if (state.shareInfo?.shareType === "upload") {
    return;
  }
  const isInternal = Array.from(event.dataTransfer?.types ?? []).includes(
    "application/x-filebrowser-internal-drag"
  );
  if (isInternal) return;
  if (!getters.sourcePermissions().create) return;

  const el = rootEl.value;
  if (el && !el.contains(event.relatedTarget as Node | null)) {
    dragTargets.add(el);
  }
}

function dragLeave(event: DragEvent) {
  // If in upload share mode, let the embedded Upload component handle it
  if (state.shareInfo?.shareType === "upload") {
    return;
  }
  const isInternal = Array.from(event.dataTransfer?.types ?? []).includes(
    "application/x-filebrowser-internal-drag"
  );
  if (isInternal) return;

  const el = rootEl.value;
  if (el && !el.contains(event.relatedTarget as Node | null)) {
    dragTargets.delete(el);
  }
}

async function drop(event: DragEvent) {
  event.preventDefault();
  // The drop listeners are always attached, so this is the gate: the source may be one the user
  // cannot write to, or may have turned out read-only since dragEnter when its info arrived.
  // Covers shares too (create = allowCreate). Clear the drop indicator dragEnter may have shown.
  if (!getters.sourcePermissions().create) {
    dragTargets.clear();
    return;
  }
  const isInternal = Array.from(event.dataTransfer?.types ?? []).includes(
    "application/x-filebrowser-internal-drag"
  );

  if (isInternal) {
    return;
  }
  await handleDrop(event);
}

async function uploadInput(event: Event) {
  await handleDrop(event);
}

const windowsResize = throttle(() => {
  if (!isResizing) {
    isResizing = true;
    listingView.value?.classList.add("resizing");
  }
  if (resizeTimeout) {
    clearTimeout(resizeTimeout);
  }
  width.value = window.innerWidth;
  rectangleItemRects = null;
  resizeTimeout = setTimeout(() => {
    isResizing = false;
    listingView.value?.classList.remove("resizing");
  }, 150);
}, 100);

function openContext(event: Event) {
  event.preventDefault();
  event.stopPropagation();

  // Prevent opening if already open
  if (getters.currentPromptName() === "ContextMenu") {
    return;
  }

  mutations.showPrompt({
    name: "ContextMenu",
    props: {
      showCentered: getters.isMobile(),
      posX: (event as MouseEvent).clientX,
      posY: (event as MouseEvent).clientY,
      createOnly: selectedCount.value === 0,
    },
  });
}

function clickClear(event: MouseEvent) {
  // Only process clicks if we're on the listing view
  if (getters.currentView() !== "listingView") {
    return;
  }

  const targetClasses: unknown = (event.target as Element).className;

  if (typeof targetClasses === "string" && targetClasses.includes("listing-item")) {
    return;
  }

  // if control, meta, or shift is pressed, do not clear the selection
  if (event.ctrlKey || event.metaKey || event.shiftKey) {
    return;
  }

  const sameAsBefore = state.selected === lastSelected;
  if (sameAsBefore && !state.multiple && getters.currentPromptName() === "") {
    mutations.resetSelected();
  }
  lastSelected = state.selected;
}

async function handleDrop(event: Event) {
  event.preventDefault();

  // If we're already in the embedded upload view, don't open a new prompt
  // The embedded Upload component will handle its own drops
  if (state.shareInfo?.shareType === "upload") {
    return;
  }

  if (event.type === "drop") {
    mutations.showPrompt({
      name: "upload",
      props: {
        initialItems: Array.from((event as DragEvent).dataTransfer?.items ?? []),
      },
    });
  } else {
    // This is for the <input type="file"> fallback
    const inputFiles = (event.target as HTMLInputElement).files;
    if (!inputFiles || inputFiles.length === 0) {
      return;
    }

    mutations.showPrompt({
      name: "upload",
      props: {
        // we send it as an array-like object so that it can be processed like a FileList by the Upload component
        initialItems: Array.from(inputFiles),
      },
    });
  }
  dragTargets.clear();
}

function measureSelectionBounds() {
  const listingRect = (listingView.value as HTMLElement).getBoundingClientRect();
  const statusBar = document.getElementById("status-bar");
  const statusBarVisible = statusBar && getComputedStyle(statusBar).display !== "none";
  const maxY = statusBarVisible
    ? Math.max(0, statusBar.getBoundingClientRect().top - listingRect.top)
    : listingRect.height;
  return { left: listingRect.left, top: listingRect.top, maxX: listingRect.width, maxY };
}

function startRectangleSelection(event: MouseEvent) {
  // Start rectangle selection when clicking on empty space - don't start if the click was in the status bar, an item or the header
  const target = event.target as Element;
  if (target.closest(".listing-item") || target.closest(".header") || target.closest("#status-bar")) {
    return;
  }

  // Don't start if it's a right click, this for avoid some issues with the context menu.
  if (event.button !== 0) return;

  isRectangleSelecting.value = true;
  rectangleItemRects = null;

  // Get the position to the listing view container
  const { left, top, maxX, maxY } = measureSelectionBounds();
  let startX = event.clientX - left;
  let startY = event.clientY - top;

  // Clamp to container bounds
  startX = Math.max(0, Math.min(startX, maxX));
  startY = Math.max(0, Math.min(startY, maxY));

  rectangleStart.value = {
    x: startX,
    y: startY
  };
  rectangleEnd.value = {
    x: startX,
    y: startY
  };
  lastPointer = {
    x: event.clientX,
    y: event.clientY,
    additive: event.ctrlKey || event.metaKey,
  };
  document.addEventListener("scroll", handleRectangleScroll, { capture: true, passive: true });

  // Store the current selection state when starting rectangle
  initialSelectionState = [...state.selected];

  // Only clear selection when CTRL is not holded
  const hasModifier = event.ctrlKey || event.metaKey;
  if (!hasModifier) {
    mutations.resetSelected();
  }

  event.preventDefault();
}

function updateRectangleSelection(event: MouseEvent) {
  if (!isRectangleSelecting.value) return;
  lastPointer = {
    x: event.clientX,
    y: event.clientY,
    additive: event.ctrlKey || event.metaKey,
  };
  scheduleRectangleUpdate();
}

function handleRectangleScroll() {
  rectangleItemRects = null;
  scheduleRectangleUpdate();
}

// mousemove + scroll into one update
function scheduleRectangleUpdate() {
  if (!isRectangleSelecting.value || selectionUpdatePending) return;
  selectionUpdatePending = true;
  rafId = requestAnimationFrame(() => {
    selectionUpdatePending = false;
    rafId = null;
    if (!isRectangleSelecting.value || !lastPointer) return;

    const { left, top, maxX, maxY } = measureSelectionBounds();
    const endX = Math.max(0, Math.min(lastPointer.x - left, maxX));
    const endY = Math.max(0, Math.min(lastPointer.y - top, maxY));
    rectangleEnd.value = { x: endX, y: endY };
    updateSelectedItemsInRectangle(lastPointer.additive);
  });
}

function stopRectangleScrollListener() {
  document.removeEventListener("scroll", handleRectangleScroll, { capture: true });
}

function endRectangleSelection(event: MouseEvent) {
  if (!isRectangleSelecting.value) return;

  // Cancel any pending animation frame
  if (rafId) {
    cancelAnimationFrame(rafId);
    rafId = null;
  }
  stopRectangleScrollListener();

  const { left, top, maxX, maxY } = measureSelectionBounds();
  rectangleEnd.value = {
    x: Math.max(0, Math.min(event.clientX - left, maxX)),
    y: Math.max(0, Math.min(event.clientY - top, maxY)),
  };
  updateSelectedItemsInRectangle(event.ctrlKey || event.metaKey);

  isRectangleSelecting.value = false;
  rectangleItemRects = null;
  selectionUpdatePending = false;
  lastPointer = null;

  // Clear rectangle after a short delay
  setTimeout(() => {
    rectangleStart.value = { x: 0, y: 0 };
    rectangleEnd.value = { x: 0, y: 0 };
    initialSelectionState = [];
  }, 100);
}

function updateSelectedItemsInRectangle(isAdditive: boolean) {
  if (!isRectangleSelecting.value) return;

  const listingRect = (listingView.value as HTMLElement).getBoundingClientRect();
  const rect = {
    left: Math.min(rectangleStart.value.x, rectangleEnd.value.x),
    top: Math.min(rectangleStart.value.y, rectangleEnd.value.y),
    right: Math.max(rectangleStart.value.x, rectangleEnd.value.x),
    bottom: Math.max(rectangleStart.value.y, rectangleEnd.value.y)
  };

  const rectangleSelectedIndexes: number[] = [];

  // Measure the items once per layout
  if (!rectangleItemRects) {
    rectangleItemRects = measureItemRects(listingRect);
  }
  for (const box of rectangleItemRects) {
    // Check if the item intersects with the rectangle
    if (
      box.left < rect.right &&
      box.right > rect.left &&
      box.top < rect.bottom &&
      box.bottom > rect.top
    ) {
      rectangleSelectedIndexes.push(box.index);
    }
  }
  // Compute the selection, and only touch the store when it actually changed
  let nextSelection: typeof state.selected;
  if (isAdditive) {
    // add more items to the current selection without reset selection
    const base = initialSelectionState || [];
    const known = new Set(base);
    nextSelection = [...base];
    for (const index of rectangleSelectedIndexes) {
      if (!known.has(index)) {
        known.add(index);
        nextSelection.push(index);
      }
    }
  } else {
    // Select only the items in the rectangle and reset initial selection
    // PS: If you don't want that just hold ctrl, the selection will not be reset, allowing multi select.
    nextSelection = rectangleSelectedIndexes;
  }
  const current = state.selected;
  const unchanged =
    current.length === nextSelection.length &&
    nextSelection.every((value, i) => current[i] === value);
  if (unchanged) return;
  mutations.setSelected(nextSelection);
}

function handleDoubleClick(event: MouseEvent) {
  if ((event.target as Element).closest(".listing-item")) return;
  if (getters.currentView() !== "listingView" || getters.currentPromptName()) return;
  if (event.ctrlKey || event.metaKey || event.shiftKey) return; // Don't interfere when pressing any mod key
  mutations.selectAllItems({ multiple: getters.isMobile() });
}
</script>

<style scoped>
.listing-view-root {
  height: 100%;
  display: flex;
  flex-direction: column;
  padding-right: 0.5em;
}

.add-padding {
  padding-left: 0.5em;
}

.font-size-large h2 {
  font-size: 2em !important;
}

.rectangle-selecting .listing-item {
  pointer-events: none;
}

.listing-items.dropping .listing-item:not(.activebutton, .drag-hover) {
  opacity: 0.5;
}

.listing-items.dropping {
  margin: 0.5em;
  overflow: hidden;
  min-height: 0;
  flex: 1 1 0;
}

.listing-items {
  position: relative;
  flex: 1;
}

.listing-item .pinned-indicator {
  font-size: 1rem;
}

.listing-items:not(.list, .compact) .folder-items a {
  border-width: var(--borderWidth);
  border-style: solid;
}

/* Upload Share Styles */
.upload-share-embed {
  padding: 2em;
  max-width: 768px;
  margin: 0 auto;
}

.selection-rectangle {
  position: absolute;
  border: 2px solid var(--primaryColor);
  background-color: color-mix(in srgb, var(--primaryColor) 25%, transparent);
  border-radius: var(--borderRadius);
  pointer-events: none;
  z-index: 10;
  box-shadow: 0 2px 8px rgb(0 0 0 / 15%);
}

.rectangle-selecting {
  cursor: crosshair;
  user-select: none;
}

.drop-indicator {
  position: absolute;
  inset: 0;
  bottom: 2.5em;
  z-index: 50;
  border: 0.2em dashed var(--primaryColor);
  background: rgb(0 0 0 / 30%);
  backdrop-filter: blur(2px);
  border-radius: var(--borderRadius);
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
  box-shadow: var(--primaryColor) 0 0 1em;
}

.drop-indicator-content {
  text-align: center;
  color: var(--textPrimary);
  font-size: 1.5em;
}

.drop-indicator-content i {
  font-size: 4em;
}
</style>
