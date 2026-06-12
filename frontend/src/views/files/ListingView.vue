<template>
  <div v-if="shareInfo.shareType !== 'upload'" class="listing-view-root no-select" :style="containerStyles">
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
          <h2 :class="{'dark-mode': isDarkMode}">{{ pinnedHeaderText }}</h2>
        </div>
        <div
          v-if="numPinned > 0"
          class="pinned-items"
          aria-label="Pinned Items"
          :class="{ lastGroup: numDirs === 0 && numFiles === 0 }"
        >
          <item
            v-for="item in pinnedItems"
            :key="base64(`pinned-${item.path || item.name}`)"
            v-bind:index="item.index"
            v-bind:name="item.name"
            v-bind:isDir="item.type == 'directory'"
            v-bind:source="req.source"
            v-bind:modified="item.modified"
            v-bind:type="item.type"
            v-bind:size="item.size"
            v-bind:path="item.path"
            v-bind:reducedOpacity="item.hidden || isDragging"
            v-bind:hash="shareInfo.hash"
            v-bind:hasPreview="item.hasPreview"
            v-bind:metadata="item.metadata"
            v-bind:hasDuration="hasDuration"
            v-bind:isShared="item.isShared"
            v-bind:pinned="item.pinned"
          />
        </div>

        <!-- Directories Section -->
        <div v-if="numDirs > 0">
          <h2 :class="{'dark-mode': isDarkMode}">{{ $t("general.folders") }}</h2>
        </div>
        <div
          v-if="numDirs > 0"
          class="folder-items"
          aria-label="Folder Items"
          :class="{ lastGroup: numFiles === 0 }"
        >
          <item
            v-for="item in dirs"
            :key="base64(item.name)"
            v-bind:index="item.index"
            v-bind:name="item.name"
            v-bind:isDir="item.type === 'directory'"
            v-bind:source="req.source"
            v-bind:modified="item.modified"
            v-bind:type="item.type"
            v-bind:size="item.size"
            v-bind:path="item.path"
            v-bind:reducedOpacity="item.hidden || isDragging"
            v-bind:hash="shareInfo.hash"
            v-bind:hasPreview="item.hasPreview"
            v-bind:hasDuration="hasDuration"
            v-bind:isShared="item.isShared"
            v-bind:pinned="item.pinned"
          />
        </div>

        <!-- Files Section -->
        <div v-if="numFiles > 0">
          <h2 :class="{'dark-mode': isDarkMode}">{{ $t("general.files") }}</h2>
        </div>
        <div
          v-if="numFiles > 0"
          class="file-items"
          :class="{ lastGroup: numFiles > 0 }"
          aria-label="File Items"
        >
          <item
            v-for="item in files"
            :key="base64(item.name)"
            v-bind:index="item.index"
            v-bind:name="item.name"
            v-bind:isDir="item.type === 'directory'"
            v-bind:modified="item.modified"
            v-bind:source="req.source"
            v-bind:type="item.type"
            v-bind:size="item.size"
            v-bind:path="item.path"
            v-bind:reducedOpacity="item.hidden || isDragging"
            v-bind:hash="shareInfo.hash"
            v-bind:hasPreview="item.hasPreview"
            v-bind:metadata="item.metadata"
            v-bind:hasDuration="hasDuration"
            v-bind:isShared="item.isShared"
            v-bind:pinned="item.pinned"
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
  <div v-else-if="!shareInfo.hasPassword || state.share?.passwordValid" class="upload-share-embed">
    <Upload :initialItems="null" />
  </div>
</template>

<script>
import downloadFiles from "@/utils/download";
import { resourcesApi } from "@/api";
import { router } from "@/router";
import * as upload from "@/utils/upload";
import throttle from "@/utils/throttle";
import { state, mutations, getters } from "@/store";
import { url } from "@/utils";
import { readAllDirectoryEntries } from "@/utils/upload";

import Item from "@/components/files/ListingItem.vue";
import Upload from "@/components/prompts/Upload.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";

export default {
  name: "listingView",
  components: {
    Item,
    Upload,
    LoadingSpinner,
  },
  data() {
    return {
      columnWidth: 250 + state.user.gallerySize * 50,
      dragTargets: new Set(),
      width: window.innerWidth,
      lastSelected: {},
      contextTimeout: null,
      ctrKeyPressed: false,
      clipboard: { items: [] },
      internalClipboardTimestamp: 0,
      isRectangleSelecting: false,
      rectangleStart: { x: 0, y: 0 },
      rectangleEnd: { x: 0, y: 0 },
      rectangleSelection: [],
      cssVariables: {},
      rafId: null,
      selectionUpdatePending: false,
      isResizing: false,
      resizeTimeout: null,
    };
  },
  watch: {
    gallerySize() {
      this.columnWidth = 250 + state.user.gallerySize * 50;
      this.colunmsResize();
    },
    scrolling() {
      const scrollContainer = this.$refs.listingView;
      if (!scrollContainer) return;

      // Select all visible listing items
      const itemNodes = scrollContainer.querySelectorAll(".listing-item");

      // Find the first item near the top of the viewport
      let topItem = null;
      let minTop = Infinity;
      itemNodes.forEach((el) => {
        const rect = el.getBoundingClientRect();
        if (rect.top >= 0 && rect.top < minTop) {
          minTop = rect.top;
          topItem = el;
        }
      });

      // Decide category by checking which section is above
      let letter = "A";
      let category = "folders"; // Default category

      if (topItem) {
        letter = topItem.getAttribute("data-name")?.[0]?.toUpperCase() || "A";
      } else if (this.numPinned > 0) {
        const pinnedHeader = this.$el.querySelector(".pinned-items h2");
        if (pinnedHeader && pinnedHeader.getBoundingClientRect().top >= 0) {
          category = "pinned";
          const firstPinned = this.pinnedItems[0];
          letter = firstPinned?.name?.[0]?.toUpperCase();
        }
      }

      if (topItem?.closest('.pinned-items')) {
        category = "pinned";
        const firstPinned = this.pinnedItems[0];
        letter = firstPinned?.name?.[0]?.toUpperCase();
      } 
      else if (this.numFiles > 0) {
        const fileSection = this.$el.querySelector(".file-items");
        const fileTop = fileSection?.getBoundingClientRect().top ?? 0;
        category = fileTop <= 0 ? "files" : "folders";
      }
      if (this.numDirs === 0 && category !== "pinned") {
        category = "files"; // If no directories, only files
      }

      mutations.updateListing({
        ...state.listing,
        category,
        letter,
      });
    },
  },
  computed: {
    permissions() {
      return getters.permissions();
    },
    shareInfo() {
      return state.shareInfo;
    },
    state() {
      return state;
    },
    isDragging() {
      if (getters.isShare()) {
        return state.shareInfo.allowCreate && this.dragTargets.has(this.$el);
      }
      return this.dragTargets.has(this.$el);
    },
    scrolling() {
      return state.listing.scrollRatio;
    },
    isStickySidebar() {
      return getters.isStickySidebar();
    },
    allItems() {
      return [...this.pinnedItems, ...this.dirs, ...this.files];
    },
    numColumns() {
      void this.width;
      if (!getters.isCardView()) {
        return 1;
      }
      const elem = document.querySelector("#main");
      if (!elem) {
        return 1;
      }
      if (getters.viewMode() === 'icons') {
        const containerSize = 70 + (state.user.gallerySize * 15); // 85px to 190px range
        let columns = Math.floor(elem.offsetWidth / containerSize);
        if (columns === 0) columns = 1;

        const minColumns = 3;
        const maxColumns = 12;
        columns = Math.max(minColumns, Math.min(columns, maxColumns));
        return columns;
      }
      // Rest of views
      let columns = Math.floor(elem.offsetWidth / this.columnWidth);
      if (columns === 0) columns = 1;
      return columns;
    },
    // Create a computed property that references the Vuex state
    gallerySize() {
      return state.user.gallerySize;
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    ascOrdered() {
      return getters.sorting().asc;
    },
    hasDuration() {
      // Check if any pinned file or regular file has duration metadata
      return [...this.pinnedItems, ...this.files].some(
        file => file.type !== "directory" && file.metadata?.duration
      );
    },
    items() {
      return getters.reqItems();
    },
    numPinned() {
      return this.pinnedItems.length;
    },
    pinnedItems() {
      return this.items.pinned || [];
    },
    numFiles() {
      return this.files.length;
    },
    numDirs() {
      return this.dirs.length;
    },
    pinnedHeaderText() {
      const pinnedFolders = this.pinnedItems.filter(item => item.type === 'directory').length;
      const pinnedFiles = this.pinnedItems.filter(item => item.type !== 'directory').length;
      if (pinnedFolders > 0 && pinnedFiles === 0) {
        return `${this.$t("files.pinnedFolders")}`; // "Pinned folders"
      }
      if (pinnedFiles > 0 && pinnedFolders === 0) {
        return `${this.$t("files.pinnedFiles")}`;   // "Pinned files"
      }
      return this.$t("files.pinnedItems"); // "Pinned items" if we pin both types
    },
    dirs() {
      return this.items.dirs;
    },
    files() {
      return this.items.files;
    },
    viewIcon() {
      const icons = {
        list: "view_module",
        compact: "view_module",
        normal: "grid_view",
        gallery: "view_list",
      };
      return icons[getters.viewMode()];
    },
    listingViewMode() {
      this.colunmsResize();
      return getters.viewMode();
    },
    selectedCount() {
      return state.selected.length;
    },
    req() {
      return state.req;
    },
    loading() {
      const isLoading = getters.isLoading();
      return isLoading;
    },
    rectangleStyle() {
      if (!this.isRectangleSelecting) return { display: 'none' };
      const left = Math.min(this.rectangleStart.x, this.rectangleEnd.x);
      const top = Math.min(this.rectangleStart.y, this.rectangleEnd.y);
      const width = Math.abs(this.rectangleStart.x - this.rectangleEnd.x);
      const height = Math.abs(this.rectangleStart.y - this.rectangleEnd.y);
      return {
        left: `${left}px`,
        top: `${top}px`,
        width: `${width}px`,
        height: `${height}px`,
      };
    },
    containerStyles() {
      // Dynamic padding-top: applied to the entire container (loading spinner + listing items)
      const isEmpty = this.numDirs + this.numFiles + this.numPinned === 0;
      const isRootPath = state.req.path === '/' || !state.req.path;

      if (isEmpty) {
        return { 'padding-top': '4.1em' }; // Empty - no files or folders
      } else if (isRootPath) {
        return { 'padding-top': '4.25em' }; // Root - no breadcrumbs showing
      } else {
        return { 'padding-top': '7.28em' }; // Non-root - breadcrumbs + listing header
      }
    },
    itemStyles() {
      const viewMode = getters.viewMode();
      const styles = {};
      const size = state.user.gallerySize;

      if (viewMode === 'icons') {
        const baseSize = 20 + (size * 15); // 35px to 155px
        const cellSize = baseSize + 30;
        styles['--icon-size'] = `${baseSize}px`;
        styles['--icon-font-size'] = `${baseSize}px`;
        styles['--icons-view-cell-size'] = `${cellSize}px`;
      } else if (viewMode === 'gallery') {
        const baseCalc = 80 + (size * 25);
        const extraScaling = Math.max(0, size - 5) * 15;
        const baseSize = baseCalc + extraScaling; // Size 5: 205px, Size 9: 345px
        const iconFontSize = (3 + (size * 0.5)).toFixed(2); // 3em to 7.5em
        styles['--icon-font-size'] = `${iconFontSize}em`;

        if (state.isMobile) {
          const minWidth = size <= 3 ? 120 : size <= 7 ? 160 : 280;
          const mobileHeight = 120 + (size * 20); // 120px to 300px
          styles['--gallery-mobile-min-width'] = `${minWidth}px`;
          styles['--item-width'] = `${minWidth}px`;
          styles['--item-height'] = `${mobileHeight}px`;
        } else {
          styles['--item-width'] = `${baseSize}px`;
          styles['--item-height'] = `${Math.round(baseSize * 1.2)}px`;
        }
      } else if (viewMode === 'list' || viewMode === 'compact') {
        const baseHeight = viewMode === 'compact'
          ? 40 + (size * 2)  // 42px to 58px
          : 50 + (size * 3); // 53px to 77px
        const iconSize = (2 + (size * 0.12)).toFixed(2); // 2.12em to 3.08em
        const iconFontSize = (1.5 + (size * 0.12)).toFixed(2); // 1.62em to 2.58em

        styles['--item-width'] = `calc(${(100 / this.numColumns).toFixed(2)}% - 1em)`;
        styles['--item-height'] = `${baseHeight}px`;
        styles['--icon-size'] = `${iconSize}em`;
        styles['--icon-font-size'] = `${iconFontSize}em`;
      } else {
        // Normal view
        const iconSize = (3.2 + (size * 0.15)).toFixed(2); // 3.35em to 4.55em
        const iconFontSize = (2.2 + (size * 0.12)).toFixed(2); // 2.32em to 3.28em

        styles['--item-width'] = `calc(${(100 / this.numColumns)}% - 1em)`;
        styles['--item-height'] = 'auto';
        styles['--icon-size'] = `${iconSize}em`;
        styles['--icon-font-size'] = `${iconFontSize}em`;
      }

      return styles;
    },
  },
  mounted() {
    mutations.setSearch(false);
    this.lastSelected = state.selected;
    this.colunmsResize();

    // Add the needed event listeners to the window and document.
    window.addEventListener("keydown", this.keyEvent);
    window.addEventListener("resize", this.windowsResize);
    window.addEventListener("click", this.clickClear);
    window.addEventListener("keyup", this.clearCtrKey);
    window.addEventListener("dragover", this.preventDefault);
    window.addEventListener('paste', this.handlePaste);
    document.addEventListener('mousemove', this.updateRectangleSelection, { passive: true });
    document.addEventListener('mouseup', this.endRectangleSelection);
    this.$el.addEventListener('mousedown', this.startRectangleSelection);
    this.$el.addEventListener("touchmove", this.handleTouchMove, { passive: true });
    this.$el.addEventListener('dblclick', this.handleDoubleClick);

    // Single dragend listener for all items (prevents N listeners for N items)
    document.addEventListener('dragend', this.handleGlobalDragEnd, { passive: true });

    this.$el.addEventListener("contextmenu", this.openContext);
    // Adjust contextmenu listener based on browser
    if (state.isSafari) {
      // For Safari, add touchstart or mousedown to open the context menu
      this.$el.addEventListener("touchstart", this.openContextForSafari, {
        passive: true,
      });
      this.$el.addEventListener("mousedown", this.openContextForSafari);

      // Also clear the timeout if the user clicks or taps quickly
      this.$el.addEventListener("touchend", this.cancelContext);
      this.$el.addEventListener("mouseup", this.cancelContext);
    }

    // if safari , make sure click and hold opens context menu, but not for any other browser
    if (this.permissions?.modify || getters.isShare()) {
      this.$el.addEventListener("dragenter", this.dragEnter);
      this.$el.addEventListener("dragleave", this.dragLeave);
      this.$el.addEventListener("drop", this.drop);
    }
  },
  beforeUnmount() {
    if (this.resizeTimeout) {
      clearTimeout(this.resizeTimeout);
      this.resizeTimeout = null;
    }

    // Clean up resize observer
    if (this.resizeObserver) {
      this.resizeObserver.disconnect();
      this.resizeObserver = null;
    }

    // Remove event listeners before destroying this page.
    window.removeEventListener("keydown", this.keyEvent);
    window.removeEventListener("resize", this.windowsResize);
    window.removeEventListener("click", this.clickClear);
    window.removeEventListener("keyup", this.clearCtrKey);
    window.removeEventListener("dragover", this.preventDefault);
    window.removeEventListener('paste', this.handlePaste);
    document.removeEventListener('mousemove', this.updateRectangleSelection);
    document.removeEventListener('mouseup', this.endRectangleSelection);
    document.removeEventListener('dragend', this.handleGlobalDragEnd);
    this.$el.removeEventListener('mousedown', this.startRectangleSelection);
    this.$el.removeEventListener('dblclick', this.handleDoubleClick);

    this.$el.removeEventListener("touchmove", this.handleTouchMove);
    this.$el.removeEventListener("contextmenu", this.openContext);

    // If Safari, remove touch/mouse listeners
    if (state.isSafari) {
      this.$el.removeEventListener("touchstart", this.openContextForSafari);
      this.$el.removeEventListener("mousedown", this.openContextForSafari);
      this.$el.removeEventListener("touchend", this.cancelContext);
      this.$el.removeEventListener("mouseup", this.cancelContext);
    }

    // Also clean up drag/drop listeners on the component's root element
    if (state.user && this.permissions?.modify || getters.isShare()) {
      this.$el.removeEventListener("dragenter", this.dragEnter);
      this.$el.removeEventListener("dragleave", this.dragLeave);
      this.$el.removeEventListener("drop", this.drop);
    }
  },
  methods: {
    handleGlobalDragEnd() {
      // Reset drag state for all items (replaces per-item dragend listeners)
      const items = this.$el?.querySelectorAll('.listing-item.drag-hover, .listing-item.half-selected');
      if (items) {
        items.forEach(el => {
          el.classList.remove('drag-hover', 'half-selected');
        });
      }
      this.dragTargets.clear();
    },
    cancelContext() {
      if (this.contextTimeout) {
        clearTimeout(this.contextTimeout);
        this.contextTimeout = null;
      }
      this.isLongPress = false;
    },
    openContextForSafari(event) {
      this.cancelContext(); // Clear any previous timeouts
      this.isLongPress = false; // Reset state
      this.isSwipe = false; // Reset swipe detection

      const touch = event.touches[0];
      this.touchStartX = touch.clientX;
      this.touchStartY = touch.clientY;

      // Start the long press detection
      this.contextTimeout = setTimeout(() => {
        if (!this.isSwipe) {
          this.isLongPress = true;
          event.preventDefault(); // Suppress Safari's callout menu
          this.openContext(event); // Open the custom context menu
        }
      }, 500); // Long press delay (adjust as needed)
    },
    handleTouchMove(event) {
      const touch = event.touches[0];
      const deltaX = Math.abs(touch.clientX - this.touchStartX);
      const deltaY = Math.abs(touch.clientY - this.touchStartY);
      // Set a threshold for movement to detect a swipe
      const movementThreshold = 10; // Adjust as needed
      if (deltaX > movementThreshold || deltaY > movementThreshold) {
        this.isSwipe = true;
        this.cancelContext(); // Cancel long press if swipe is detected
      }
    },
    handleTouchEnd() {
      this.cancelContext(); // Clear timeout
      this.isSwipe = false; // Reset swipe state
    },
    base64(name) {
      return url.base64Encode(name);
    },
    showDeletePrompt() {
      const items = [];
      for (const index of state.selected) {
        const item = state.req.items.at(index);
        if (!item) continue;
        const previewUrl = item.hasPreview
          ? resourcesApi.getPreviewURL(item.source || state.req.source, item.path, item.modified)
          : null;
        items.push({
          source: item.source || state.req.source,
          path: item.path,
          type: item.type,
          size: item.size,
          modified: item.modified,
          hasPreview: item.hasPreview,
          previewUrl: previewUrl,
        });
      }
      mutations.showPrompt({
        name: "delete",
        props: { items },
      });
    },
    // Helper method to select the first item if nothing is selected
    selectFirstItem() {
      mutations.resetSelected();
      if (this.allItems.length > 0) {
        mutations.addSelected(this.allItems[0].index);
      }
    },
    // Helper method to find the closest item in the given direction (up or down) from the current one.
    findClosestItem(selectedItem, direction) {
      const listItems = Array.from(this.$el.querySelectorAll('.listing-item:not(.out-of-view)'));
      const selectedBounds = selectedItem.getBoundingClientRect();
      const selectedMidX = (selectedBounds.left + selectedBounds.right) / 2;

      let closestItem = null;
      let closestDistance = Infinity;

      for (const item of listItems) {
        if (item === selectedItem) continue;
        const itemBounds = item.getBoundingClientRect();
        const itemMidX = (itemBounds.left + itemBounds.right) / 2;
        const horizontalOffset = Math.abs(itemMidX - selectedMidX);

        const verticalGap = direction === 'down'
          ? itemBounds.top - selectedBounds.bottom
          : selectedBounds.top - itemBounds.bottom;

        if (verticalGap > 0) {
          const distance = Math.hypot(horizontalOffset, verticalGap);
          if (distance < closestDistance) {
            closestDistance = distance;
            closestItem = item;
          }
        }
      }
      return closestItem;
    },
    moveSelectionBy(step) {
      const allItems = this.allItems;
      const selectedIndex = state.selected.length > 0 ? state.selected[0] : null;
      if (selectedIndex === null) {
        this.selectFirstItem();
        return false;
      }
      const currentItemIndex = allItems.findIndex(item => item.index === selectedIndex);
      if (currentItemIndex === -1) {
        this.selectFirstItem();
        return false;
      }
      const nextItemIndex = currentItemIndex + step;
      if (nextItemIndex >= 0 && nextItemIndex < allItems.length) {
        const targetItem = allItems.at(nextItemIndex);
        mutations.resetSelected();
        mutations.addSelected(targetItem.index);
        this.scrollSelectedIntoView();
        return true;
      }
      return false;
    },
    // Helper method to handle selection based on arrow keys
    navigateKeyboardArrows(arrowKey) {
      const isCardView = getters.isCardView(); // gallery, normal, icons
      const selectedIndex = state.selected.length > 0 ? state.selected[0] : null;

      if (selectedIndex === null) {
        // If nothing is selected, select the first item
        this.selectFirstItem();
        return;
      }

      // Left/Right arrows will always use the visual order that we see
      if (arrowKey === 'ArrowLeft' || arrowKey === 'ArrowRight') {
        const step = arrowKey === 'ArrowLeft' ? -1 : 1;
        this.moveSelectionBy(step);
        return;
      }

      // On list/compact views we use a simple linear navigation since all the items are aligned
      if (!isCardView) {
        const step = arrowKey === 'ArrowUp' ? -1 : 1;
        this.moveSelectionBy(step);
        return;
      }

      // But for gallery, normal, icons views, we need to find the closest item
      // because the rows aren't always consistent (some have 1 item, others 5, etc) 
      // which caused "random jumps"
      const selectedItem = this.$el.querySelector(`.listing-item[data-index="${selectedIndex}"]`);
      if (!selectedItem) return;

      let nextItem = null;

      switch (arrowKey) {
        case 'ArrowDown':
          nextItem = this.findClosestItem(selectedItem, 'down');
          break;
        case 'ArrowUp':
          nextItem = this.findClosestItem(selectedItem, 'up');
          break;
      }

      const itemIndex = parseInt(nextItem?.dataset.index, 10);
      if (!Number.isNaN(itemIndex)) {
        mutations.resetSelected();
        mutations.addSelected(itemIndex);
        this.scrollSelectedIntoView();
      }
    },
    scrollSelectedIntoView() {
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
    },
    clearCtrKey(event) {
      const { ctrlKey, metaKey } = event;
      const modifierKeys = ctrlKey || metaKey;
      if (!modifierKeys) {
        this.ctrKeyPressed = false;
      }
    },
    keyEvent(event) {
      const { key, ctrlKey, metaKey, altKey, which } = event;
      const isArrowKey = key === 'ArrowUp' ||
                         key === 'ArrowDown' ||
                         key === 'ArrowLeft' ||
                         key === 'ArrowRight';
      if (state.isSearchActive || getters.currentView() !== "listingView" ||
        getters.currentPromptName() || (event.repeat && (!isArrowKey || altKey))) return;

      const isAlphanumeric = /^[a-z0-9]$/i.test(key);
      const modifierKeys = ctrlKey || metaKey;
      if (isAlphanumeric && !modifierKeys && state.selected.length <= 1) {
        const t = event.target;
        const tag = t?.tagName ? t.tagName.toLowerCase() : "";
        if (
          tag !== "input" &&
          tag !== "textarea" &&
          tag !== "select" &&
          !t.isContentEditable
        ) {
          event.preventDefault();
          this.alphanumericKeyPress(key);
          return;
        }
      }

      if (modifierKeys) {
        this.ctrKeyPressed = true;
        const charKey = String.fromCharCode(which).toLowerCase();

        switch (charKey) {
          case "c":
          case "x":
            this.copyCut(event, charKey);
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
          event.preventDefault();
          // fall through
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
          event.preventDefault();
          // fall through
        case "Enter": {
          event.preventDefault();
          if (this.selectedCount === 1) {
            const selected = getters.getFirstSelected();
            const selectedUrl = url.buildItemUrl(selected.source, selected.path);
            if (selectedUrl === state.route.path) return;
            void router.push({ path: selectedUrl });
          }
          break;
        }

        case "Escape":
          if (this.dragTargets.size > 0) {
            this.dragTargets.clear();
            event.preventDefault();
            return;
          }
          mutations.resetSelected();
          break;

        case "Delete":
          if (!this.permissions?.modify || state.selected.length === 0) return;
          this.showDeletePrompt();
          break;

        case "ArrowUp":
        case "ArrowDown":
        case "ArrowLeft":
        case "ArrowRight":
          event.preventDefault();
          this.navigateKeyboardArrows(key);
          break;
      }
    },
    alphanumericKeyPress(key) {
      const prefix = key.toLowerCase();
      const allItems = this.allItems;
      const matches = allItems.filter(item =>
        item.name.toLowerCase().startsWith(prefix)
      );
      if (matches.length === 0) return;

      let nextPos = 0;
      if (state.selected.length === 1) {
        const curIdx = state.selected[0];
        const curPos = matches.findIndex(m => m.index === curIdx);
        if (curPos !== -1) nextPos = (curPos + 1) % matches.length;
      }

      const target = matches.at(nextPos);
      if (!target) return;
      mutations.resetSelected();
      mutations.addSelected(target.index);
      this.scrollSelectedIntoView();
    },
    preventDefault(event) {
      // Wrapper around prevent default.
      event.preventDefault();
    },
    copyCut(event, key) {
      if (event.target.tagName.toLowerCase() === "input") {
        return;
      }

      const items = state.selected
        .map((i) => {
          const item = state.req.items.at(i);
          if (!item) return null;
          return {
            from: item.path,
            fromSource: state.req.source,
            name: item.name,
          };
        })
        .filter(Boolean);

      if (items.length === 0) {
        return;
      }

      this.clipboard = {
        key: key,
        items: items,
        path: state.route.path,
      };
      this.internalClipboardTimestamp = Date.now();
    },
    async collectFilesFromEntry(entry, relativePath = "") {
      const files = [];
      const entryPath = relativePath ? `${relativePath}/${entry.name}` : entry.name;

      if (entry.isFile) {
        // If it's a file we get the File object and add it with its relative path
        const file = await new Promise((resolve, reject) => {
          entry.file(resolve, reject);
        });
        files.push({
          file,
          relativePath: entryPath,
        });
      } else if (entry.isDirectory) {
        // But if it's a directory, read the contents (readEntries may require
        // multiple calls — browsers often return at most ~100 entries per batch)
        const reader = entry.createReader();
        const entries = await readAllDirectoryEntries(reader, entryPath);
        // and then for each child recursively collect files
        for (const childEntry of entries) {
          const childFiles = await this.collectFilesFromEntry(
            childEntry,
            entryPath
          );
          files.push(...childFiles);
        }
      }
      return files;
    },

    async getClipboardItems(items) {
      const files = [];
      for (const item of items) {
        if (item.type === 'entry') {
          const collected = await this.collectFilesFromEntry(item.entry);
          files.push(...collected);
        } else if (item.type === 'file') {
          files.push({ file: item.file, relativePath: item.file.name });
        }
      }
      return files;
    },

    async handlePaste(event) {
      // We can paste multiple files from clipboard and upload all of them at once, but
      // Firefox has an issue https://bugzilla.mozilla.org/show_bug.cgi?id=864052  (12 years old lol)
      // the clipboard entry is always getting 1 file (the first file/folder in selection),
      // while chromium sees all and has no issues with it. Not sure about safari.
      if (getters.currentView() !== "listingView" || getters.currentPromptName()) return;

      // Check if internal clipboard is recent (20 seconds)
      const internalRecent = this.clipboard.items.length > 0 && (Date.now() - this.internalClipboardTimestamp) < 20000;

      // If internal is recent (<20s), use it immediately, in case someone have both: internal and external clipboard with a file entry.
      // After those 20s, if the OS clipboard has a file as most recent entry, will use that.
      if (internalRecent) {
        await this.handleInternalPaste();
        event.preventDefault();
        event.stopPropagation();
        return;
      }

      if (event.clipboardData?.items) {
        // Collect all items from clipboard
        const collectedItems = [];
        // And loop through all items
        for (const item of Array.from(event.clipboardData.items)) {
          if (item.kind !== 'file') continue;

          const entry = item.webkitGetAsEntry();
          if (entry) {
            collectedItems.push({ type: 'entry', entry });
          } else {
            const file = item.getAsFile();
            if (file) {
              collectedItems.push({ type: 'file', file });
            }
          }
        }

        // Then process the collected items (files and folders) asynchronously
        const itemsFromClipboard = await this.getClipboardItems(collectedItems);
        const files = itemsFromClipboard.map(item => item.file);

        // If found external files, upload them
        if (files.length > 0) {
          event.preventDefault();
          event.stopPropagation();

          const canUpload = getters.permissions()?.modify;
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
      if (this.clipboard.items.length > 0) {
        console.log('No external files, using internal clipboard.');
        await this.handleInternalPaste();
        event.preventDefault();
        event.stopPropagation();
        return;
      }
    },

    async handleInternalPaste() {
      if (!this.clipboard?.items || this.clipboard.items.length === 0) {
        return;
      }

      // Construct destination path properly (without URL prefix)
      const destPath = state.req.path.endsWith('/') ? state.req.path : `${state.req.path}/`;

      const items = this.clipboard.items.map((item) => ({
        from: item.from,
        fromSource: item.fromSource,
        to: destPath + item.name,
        toSource: state.req.source,
        name: item.name,
      }));

      const operation = this.clipboard.key === "x" ? "move" : "copy";

      // Show confirmation prompt first
      mutations.showPrompt({
        name: "CopyPasteConfirm",
        props: {
          operation: operation,
          items: items,
          onConfirm: () => {
            return new Promise((resolve, reject) => {

              const action = async (overwrite, rename) => {
                try {
                  if (getters.isShare()) {
                    await resourcesApi.moveCopyPublic(state.shareInfo.hash, items, operation, overwrite, rename);
                  } else {
                    await resourcesApi.moveCopy(items, operation, overwrite, rename);
                  }
                  if (operation === "move") {
                    this.clipboard = { items: [] };
                    this.internalClipboardTimestamp = 0;
                  }
                  mutations.setReload(true);
                  resolve();
                } catch (error) {
                  console.error("Error moving/copying items:", error);
                  reject(error);
                }
              };

              if (this.clipboard.path === state.route.path) {
                void action(false, true);
                return;
              }

              const conflict = upload.checkConflict(items, state.req.items);

              if (conflict) {
                mutations.showPrompt({
                  name: "replace-rename",
                  pinned: true,
                  confirm: (event, option) => {
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
    },
    colunmsResize() {
      // No longer needed - CSS variables are now handled reactively via itemStyles computed property
      // Kept for backwards compatibility with any remaining callers
    },
    dragEnter(event) {
      // If in upload share mode, let the embedded Upload component handle it
      if (state.shareInfo?.shareType === 'upload') {
        return;
      }
      const isInternal = Array.from(event.dataTransfer.types).includes(
        "application/x-filebrowser-internal-drag"
      );
      if (isInternal) return;

      if (!this.$el.contains(event.relatedTarget)) {
        this.dragTargets.add(this.$el);
      }
    },
    dragLeave(event) {
      // If in upload share mode, let the embedded Upload component handle it
      if (state.shareInfo?.shareType === 'upload') {
        return;
      }
      const isInternal = Array.from(event.dataTransfer.types).includes(
        "application/x-filebrowser-internal-drag"
      );
      if (isInternal) return;

      if (!this.$el.contains(event.relatedTarget)) {
        this.dragTargets.delete(this.$el);
      }
    },
    async drop(event) {
      event.preventDefault();
      if (getters.isShare() && !state.shareInfo.allowCreate) {
        return
      }
      const isInternal = Array.from(event.dataTransfer.types).includes(
        "application/x-filebrowser-internal-drag"
      );

      if (isInternal) {
        return;
      }
      await this.handleDrop(event);
    },
    async uploadInput(event) {
      await this.handleDrop(event);
    },
    windowsResize: throttle(function () {
      // Mark as resizing to disable transitions
      if (!this.isResizing) {
        this.isResizing = true;
        if (this.$refs.listingView) {
          this.$refs.listingView.classList.add('resizing');
        }
      }

      // Clear existing timeout
      if (this.resizeTimeout) {
        clearTimeout(this.resizeTimeout);
      }

      // Do the resize work
      this.colunmsResize();
      this.width = window.innerWidth;
      mutations.setMobile();

      // Re-enable transitions after resize is complete
      this.resizeTimeout = setTimeout(() => {
        this.isResizing = false;
        if (this.$refs.listingView) {
          this.$refs.listingView.classList.remove('resizing');
        }
      }, 150); // Wait 150ms after last resize event

      // Listing element is not displayed
      if (this.$refs.listingView === null) return;
    }, 100),
    openContext(event) {
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
          posX: event.clientX,
          posY: event.clientY,
          createOnly: this.selectedCount === 0,
        },
      });
    },
    clickClear(event) {
      // Only process clicks if we're on the listing view
      if (getters.currentView() !== 'listingView') {
        return;
      }

      const targetClasses = event.target.className;

      if (typeof targetClasses === 'string' && targetClasses.includes('listing-item')) {
        return;
      }

      // if control or shift is pressed, do not clear the selection
      if (this.ctrKeyPressed || event.shiftKey) {
        return;
      }

      const sameAsBefore = state.selected === this.lastSelected;
      if (sameAsBefore && !state.multiple && getters.currentPromptName() === "") {
        mutations.resetSelected();
      }
      this.lastSelected = state.selected;
    },
    async handleDrop(event) {
      event.preventDefault();

      // If we're already in the embedded upload view, don't open a new prompt
      // The embedded Upload component will handle its own drops
      if (state.shareInfo?.shareType === 'upload') {
        return;
      }

      if (event.type === "drop") {
        mutations.showPrompt({
          name: "upload",
          props: {
            initialItems: Array.from(event.dataTransfer.items),
          },
        });
      } else {
        // This is for the <input type="file"> fallback
        const files = event.target.files;
        if (!files || files.length === 0) {
          return;
        }

        mutations.showPrompt({
          name: "upload",
          props: {
            // we send it as an array-like object so that it can be processed like a FileList by the Upload component
            initialItems: Array.from(files),
          },
        });
      }
      this.dragTargets.clear();
    },
    startRectangleSelection(event) {
      // Start rectangle selection when clicking on empty space - don't start if the click was in the status bar, an item or the header
      if (event.target.closest('.listing-item') || event.target.closest('.header') || event.target.closest('#status-bar')) {
        return;
      }

      // Don't start if it's a right click, this for avoid some issues with the context menu.
      if (event.button !== 0) return;

      this.isRectangleSelecting = true;

      // Get the position to the listing view container
      const listingRect = this.$refs.listingView.getBoundingClientRect();
      let startX = event.clientX - listingRect.left;
      let startY = event.clientY - listingRect.top;

      // Clamp to container bounds
      const statusBar = document.getElementById('status-bar');
      const statusBarVisible = statusBar && getComputedStyle(statusBar).display !== 'none';
      const maxX = listingRect.width;
      const maxY = statusBarVisible
        ? Math.max(0, statusBar.getBoundingClientRect().top - listingRect.top)
        : listingRect.height;

      startX = Math.max(0, Math.min(startX, maxX));
      startY = Math.max(0, Math.min(startY, maxY));

      this.rectangleStart = {
        x: startX,
        y: startY
      };
      this.rectangleEnd = {
        x: startX,
        y: startY
      };

      // Store the current selection state when starting rectangle
      this.initialSelectionState = [...state.selected];

      // Only clear selection when CTRL is not holded
      const hasModifier = event.ctrlKey || event.metaKey;
      if (!hasModifier) {
        mutations.resetSelected();
      }

      event.preventDefault();
    },

    updateRectangleSelection(event) {
      if (!this.isRectangleSelecting) return;

      // Get the position to the listing view container
      const listingRect = this.$refs.listingView.getBoundingClientRect();
      let endX = event.clientX - listingRect.left;
      let endY = event.clientY - listingRect.top;

      // Clamp to container bounds
      const statusBar = document.getElementById('status-bar');
      const statusBarVisible = statusBar && getComputedStyle(statusBar).display !== 'none';
      const maxX = listingRect.width;
      const maxY = statusBarVisible
        ? Math.max(0, statusBar.getBoundingClientRect().top - listingRect.top)
        : listingRect.height;

      endX = Math.max(0, Math.min(endX, maxX));
      endY = Math.max(0, Math.min(endY, maxY));

      this.rectangleEnd = {
        x: endX,
        y: endY
      };

      // Use requestAnimationFrame to batch updates
      if (!this.selectionUpdatePending) {
        this.selectionUpdatePending = true;
        this.rafId = requestAnimationFrame(() => {
          this.updateSelectedItemsInRectangle(event.ctrlKey || event.metaKey);
          this.selectionUpdatePending = false;
        });
      }
    },

    endRectangleSelection(event) {
      if (!this.isRectangleSelecting) return;

      // Cancel any pending animation frame
      if (this.rafId) {
        cancelAnimationFrame(this.rafId);
        this.rafId = null;
      }

      this.isRectangleSelecting = false;
      this.selectionUpdatePending = false;
      this.updateSelectedItemsInRectangle(event.ctrlKey || event.metaKey);

      // Clear rectangle after a short delay
      setTimeout(() => {
        this.rectangleStart = { x: 0, y: 0 };
        this.rectangleEnd = { x: 0, y: 0 };
        this.initialSelectionState = [];
      }, 100);
    },

    updateSelectedItemsInRectangle(isAdditive) {
      if (!this.isRectangleSelecting) return;

      const listingRect = this.$refs.listingView.getBoundingClientRect();
      const rect = {
        left: Math.min(this.rectangleStart.x, this.rectangleEnd.x),
        top: Math.min(this.rectangleStart.y, this.rectangleEnd.y),
        right: Math.max(this.rectangleStart.x, this.rectangleEnd.x),
        bottom: Math.max(this.rectangleStart.y, this.rectangleEnd.y)
      };

      const rectangleSelectedIndexes = [];

      // Get all item elements - use querySelectorAll with specific selector for better performance
      const itemElements = this.$el.querySelectorAll('.listing-item[data-index]');

      itemElements.forEach((element) => {
        const elementRect = element.getBoundingClientRect();

        // Convert element position to be relative to listing view, this allows selection while scrolling
        const elementRelativeRect = {
          left: elementRect.left - listingRect.left,
          top: elementRect.top - listingRect.top,
          right: elementRect.right - listingRect.left,
          bottom: elementRect.bottom - listingRect.top
        };

        // Check if the item intersects with the rectangle
        if (
          elementRelativeRect.left < rect.right &&
          elementRelativeRect.right > rect.left &&
          elementRelativeRect.top < rect.bottom &&
          elementRelativeRect.bottom > rect.top
        ) {
          const index = parseInt(element.getAttribute('data-index'), 10);
          if (!Number.isNaN(index)) {
            rectangleSelectedIndexes.push(index);
          }
        }
      });

      // Batch DOM updates to minimize reflows
      if (isAdditive) {
        // only add more items to the current selection without reset selection
        const newSelection = [...state.selected];
        rectangleSelectedIndexes.forEach(index => {
          if (!newSelection.includes(index)) {
            newSelection.push(index);
          }
        });

        mutations.resetSelected();
        newSelection.forEach(index => { mutations.addSelected(index); });
      } else {
        // Select only the items in the rectangle and reset initial selection
        // PS: If you don't want that just hold ctrl, the selection will not be reset, allowing multi select.
        mutations.resetSelected();
        rectangleSelectedIndexes.forEach(index => { mutations.addSelected(index); });
      }
    },
    handleDoubleClick(event) {
      if (event.target.closest('.listing-item')) return;
      if (getters.currentView() !== 'listingView' || getters.currentPromptName()) return;
      if (event.ctrlKey || event.metaKey || event.shiftKey) return; // Don't interfere when pressing any mod key
      mutations.selectAllItems({ multiple: getters.isMobile() });
    },
  },
};
</script>

<style scoped>
.listing-view-root {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.add-padding {
  padding-left: 0.5em;
}
.font-size-large h2 {
  font-size: 2em !important;
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

.folder-items a {
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
  border-radius: 8px;
  pointer-events: none;
  z-index: 10;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

.rectangle-selecting {
  cursor: crosshair;
  user-select: none;
}

.rectangle-selecting .listing-item {
  pointer-events: none;
}

.drop-indicator {
  position: absolute;
  inset: 0;
  bottom: 1.75em;
  z-index: 50;
  border: 0.2em dashed var(--primaryColor);
  background: rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(2px);
  border-radius: 1em;
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
