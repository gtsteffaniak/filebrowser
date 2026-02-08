<template>
  <div v-if="shareInfo.shareType != 'upload'" class="no-select" :style="containerStyles">
    <!-- Show loading spinner while loading OR if we haven't loaded any data yet -->
    <div v-if="loading">
      <h2 class="message delayed">
        <LoadingSpinner size="medium" />
        <span>{{ $t("general.loading", { suffix: "..." }) }}</span>
      </h2>
    </div>
    <!-- Show empty state only when NOT loading AND data has been loaded AND there are no items -->
    <div v-else-if="numDirs + numFiles == 0 && req.name">
      <div
        ref="listingView"
        class="listing-items font-size-large"
        :class="{
          'add-padding': isStickySidebar,
          [listingViewMode]: true,
          dropping: isDragging,
          'rectangle-selecting': isRectangleSelecting
        }"
      >
        <h2 class="message">
          <i class="material-icons">sentiment_dissatisfied</i>
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
      </div>
    </div>
    <div v-else>
      <div
        ref="listingView"
        :class="{
          'add-padding': isStickySidebar,
          [listingViewMode]: true,
          dropping: isDragging,
          'rectangle-selecting': isRectangleSelecting
        }"
        :style="itemStyles"
        class="listing-items file-icons"
      >
        <!-- Rectangle selection overlay -->
        <div class="selection-rectangle"
          :style="rectangleStyle"
        ></div>

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
            v-bind:isDir="item.type == 'directory'"
            v-bind:source="req.source"
            v-bind:modified="item.modified"
            v-bind:type="item.type"
            v-bind:size="item.size"
            v-bind:path="item.path"
            v-bind:reducedOpacity="item.hidden || isDragging"
            v-bind:hash="shareInfo.hash"
            v-bind:hasPreview="item.hasPreview"
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
            v-bind:isDir="item.type == 'directory'"
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
      </div>
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
import { filesApi } from "@/api";
import { router } from "@/router";
import * as upload from "@/utils/upload";
import throttle from "@/utils/throttle";
import { state, mutations, getters } from "@/store";
import { url } from "@/utils";

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
      dragCounter: 0,
      width: window.innerWidth,
      lastSelected: {},
      contextTimeout: null,
      ctrKeyPressed: false,
      clipboard: { items: [] },
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

      if (!topItem) return;

      const letter = topItem.getAttribute("data-name")?.[0]?.toUpperCase() || "A";
      let category = "folders"; // Default category
      if (this.numFiles > 0) {
        // Decide category by checking which section is above
        const fileSection = this.$el.querySelector(".file-items");
        const fileTop = fileSection?.getBoundingClientRect().top ?? 0;
        category = fileTop <= 0 ? "files" : "folders";
      }
      if (this.numDirs == 0) {
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
        return state.shareInfo.allowCreate && this.dragCounter > 0;
      }
      return this.dragCounter > 0;
    },
    scrolling() {
      return state.listing.scrollRatio;
    },
    isStickySidebar() {
      return getters.isStickySidebar();
    },
    lastFolderIndex() {
      const allItems = [...this.items.dirs, ...this.items.files];
      for (let i = 0; i < allItems.length; i++) {
        if (allItems[i].type != "directory") {
          return i - 1;
        }
      }
      if (allItems.length > 0) {
        return allItems.length;
      }

      return null; // Return null if there are no files
    },
    numColumns() {
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
    getMultiple() {
      return state.multiple;
    },
    ascOrdered() {
      return getters.sorting().asc;
    },
    hasDuration() {
      // Check if any file has duration metadata
      return this.files.some(file => file.metadata && file.metadata.duration);
    },
    items() {
      return getters.reqItems();
    },
    numFiles() {
      const count = getters.reqNumFiles();
      return count;
    },
    numDirs() {
      const count = getters.reqNumDirs();
      return count;
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
        left: left + 'px',
        top: top + 'px',
        width: width + 'px',
        height: height + 'px',
      };
    },
    containerStyles() {
      // Dynamic padding-top: applied to the entire container (loading spinner + listing items)
      const isRootPath = state.req.path === '/' || !state.req.path;
      if (isRootPath) {
        return { 'padding-top': '4.25em' }; // Root - no breadcrumbs showing
      } else {
        return { 'padding-top': '7.25em' }; // Non-root - breadcrumbs + listing header
      }
    },
    itemStyles() {
      const viewMode = getters.viewMode();
      const styles = {};

      if (viewMode === 'icons') {
        const baseSize = 60 + (state.user.gallerySize * 15); // 60px to 135px - increased scaling
        const cellSize = baseSize + 30;
        styles['--icons-view-icon-size'] = `${baseSize}px`;
        styles['--icons-view-cell-size'] = `${cellSize}px`;
      } else if (viewMode === 'gallery') {
        // Use column width and percentage-based sizing for smooth animations
        // Keep size 5 at 205px, then scale more aggressively above that
        const baseCalc = 80 + (state.user.gallerySize * 25);
        const extraScaling = Math.max(0, state.user.gallerySize - 5) * 15; // Additional 15px per level above 5
        const baseSize = baseCalc + extraScaling; // Size 5: 205px, Size 9: 345px
        if (state.isMobile) {
          let columns;
          if (state.user.gallerySize <= 7) columns = 2;
          else if (state.user.gallerySize <= 9) columns = 1;
          else columns = 1;
          styles['--gallery-mobile-columns'] = columns.toString();
          // On mobile, scale height with gallery size for smooth animations
          const mobileHeight = 120 + (state.user.gallerySize * 20); // 120px to 300px range
          styles['--item-width'] = '150px'; // Minimum size for mobile grid
          styles['--item-height'] = `${mobileHeight}px`;
        } else {
          // Use pixel size for grid's minmax - items will stretch and animate smoothly
          // Make height 20% larger than width for better proportions
          styles['--item-width'] = `${baseSize}px`;
          styles['--item-height'] = `${Math.round(baseSize * 1.2)}px`;
        }
      } else if (viewMode === 'list' || viewMode === 'compact') {
        const baseHeight = viewMode === 'compact'
          ? 40 + (state.user.gallerySize * 2)  // 40px to 56px - compact
          : 50 + (state.user.gallerySize * 3); // 50px to 74px - list
        // Scale icons with gallery size - icon fonts: 1.6em to 2.4em, images: 1.2em to 1.8em
        const iconFontSize = (1.6 + (state.user.gallerySize * 0.1)).toFixed(2); // 1.7em to 2.5em
        const iconImageSize = (1.2 + (state.user.gallerySize * 0.075)).toFixed(3); // 1.275em to 1.875em

        styles['--item-width'] = `calc(${(100 / this.numColumns).toFixed(2)}% - 1em)`;
        styles['--item-height'] = `${baseHeight}px`;
        styles['--list-icon-font-size'] = `${iconFontSize}em`;
        styles['--list-icon-image-size'] = `${iconImageSize}em`;
      } else {
        // Normal view
        styles['--item-width'] = `calc(${(100 / this.numColumns)}% - 1em)`;
        styles['--item-height'] = 'auto';
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
    document.addEventListener('mousemove', this.updateRectangleSelection, { passive: true });
    document.addEventListener('mouseup', this.endRectangleSelection);
    this.$el.addEventListener('mousedown', this.startRectangleSelection);
    this.$el.addEventListener("touchmove", this.handleTouchMove, { passive: true });
    
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
    document.removeEventListener('mousemove', this.updateRectangleSelection);
    document.removeEventListener('mouseup', this.endRectangleSelection);
    document.removeEventListener('dragend', this.handleGlobalDragEnd);
    this.$el.removeEventListener('mousedown', this.startRectangleSelection);

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
      for (let index of state.selected) {
        const item = state.req.items[index];
        const previewUrl = item.hasPreview
          ? filesApi.getPreviewURL(item.source || state.req.source, item.path, item.modified)
          : null;
        items.push({
          source: item.source || state.req.source,
          path: item.path,
          type: item.type,
          size: item.size,
          modified: item.modified,
          previewUrl: previewUrl,
        });
      }
      mutations.showHover({
        name: "delete",
        props: {
          items: items,
        },
      });
    },
    // Helper method to select the first item if nothing is selected
    selectFirstItem() {
      mutations.resetSelected();
      const allItems = [...this.items.dirs, ...this.items.files];
      if (allItems.length > 0) {
        mutations.addSelected(allItems[0].index);
      }
    },
    // Helper method to select an item by index
    selectItem(index) {
      mutations.resetSelected();
      mutations.addSelected(index);
    },
    // Helper method to handle selection based on arrow keys
    navigateKeboardArrows(arrowKey) {
      let selectedIndex = state.selected.length > 0 ? state.selected[0] : null;

      if (selectedIndex === null) {
        // If nothing is selected, select the first item
        this.selectFirstItem();
        return;
      }

      const allItems = [...this.items.dirs, ...this.items.files]; // Combine files and directories

      // Find the current index of the selected item
      let currentIndex = allItems.findIndex((item) => item.index === selectedIndex);

      // If no item is selected, select the first item
      if (currentIndex === -1) {
        // Check if there are any items to select
        if (allItems.length > 0) {
          currentIndex = 0;
          this.selectItem(allItems[currentIndex].index);
        }
        return;
      }
      let newSelected = null;
      const fileSelected = currentIndex > this.lastFolderIndex;
      const nextIsDir = currentIndex - this.numColumns <= this.lastFolderIndex;
      const folderSelected = currentIndex <= this.lastFolderIndex;
      const nextIsFile = currentIndex + this.numColumns > this.lastFolderIndex;
      const nextHopExists = currentIndex + this.numColumns < allItems.length;
      const thisColumnNum =
        ((currentIndex - this.lastFolderIndex - 1) % this.numColumns) + 1;
      const lastFolderColumn = (this.lastFolderIndex % this.numColumns) + 1;
      const thisColumnNum2 = (currentIndex + 1) % this.numColumns;
      let firstRowColumnPos = this.lastFolderIndex + thisColumnNum2;
      let newPos = currentIndex - lastFolderColumn;
      switch (arrowKey) {
        case "ArrowUp":
          if (currentIndex - this.numColumns < 0) {
            // do nothing
            break;
          }
          if (!getters.isCardView) {
            newSelected = allItems[currentIndex - 1].index;
            break;
          }
          // do normal move
          if (!(fileSelected && nextIsDir)) {
            newSelected = allItems[currentIndex - this.numColumns].index;
            break;
          }

          // complex logic to move from files to folders
          if (lastFolderColumn < thisColumnNum) {
            newPos -= this.numColumns;
          }
          newSelected = allItems[newPos].index;

          break;

        case "ArrowDown":
          if (currentIndex >= allItems.length) {
            // do nothing - last item
            break;
          }
          if (!getters.isCardView) {
            newSelected = allItems[currentIndex + 1].index;
            break;
          }
          if (!nextHopExists) {
            // do nothing - next item is out of bounds
            break;
          }

          if (!(folderSelected && nextIsFile)) {
            newSelected = allItems[currentIndex + this.numColumns].index;
            break;
          }
          // complex logic for moving from folders to files
          if (firstRowColumnPos <= this.lastFolderIndex) {
            firstRowColumnPos += this.numColumns;
          }
          newSelected = allItems[firstRowColumnPos].index;
          break;

        case "ArrowLeft":
          if (currentIndex > 0) {
            newSelected = allItems[currentIndex - 1].index;
          }
          break;

        case "ArrowRight":
          if (currentIndex < allItems.length - 1) {
            newSelected = allItems[currentIndex + 1].index;
          }
          break;
      }
      if (newSelected != null) {
        this.selectItem(newSelected);
        setTimeout(() => {
          // Find the element with class "item" and aria-selected="true"
          const element = document.querySelector('.listing-item[aria-selected="true"]');
          // Scroll the element into view if it exists
          if (element) {
            element.scrollIntoView({
              behavior: "smooth",
              block: "end",
              inline: "nearest",
            });
          }
        }, 50);
      }
    },
    clearCtrKey(event) {
      const { ctrlKey, metaKey } = event;
      const modifierKeys = ctrlKey || metaKey;
      if (!modifierKeys) {
        this.ctrKeyPressed = false;
      }
    },
    keyEvent(event) {
      if (state.isSearchActive || getters.currentView() != "listingView" || getters.currentPromptName()) {
        return;
      }
      const { key, ctrlKey, metaKey, altKey, which } = event;
      if (altKey) {
        return;
      }
      // Check if the key is alphanumeric
      const isAlphanumeric = /^[a-z0-9]$/i.test(key);
      const modifierKeys = ctrlKey || metaKey;
      if (isAlphanumeric && !modifierKeys && getters.currentPromptName()) {
        this.alphanumericKeyPress(key); // Call the alphanumeric key press function
        return;
      }
      if (!modifierKeys && getters.currentPromptName()) {
        return;
      }
      // Handle the space bar key
      if (key === " " && !modifierKeys) {
        event.preventDefault();
        if (state.isSearchActive) {
          mutations.setSearch(false);
          mutations.closeHovers();
        } else {
          mutations.setSearch(true);
        }
      }
      if (getters.currentPromptName()) {
        return;
      }
      let currentPath = url.removeTrailingSlash(state.route.path);
      let newPath = currentPath.substring(0, currentPath.lastIndexOf("/"));

      if (modifierKeys) {
        this.ctrKeyPressed = true;
        const charKey = String.fromCharCode(which).toLowerCase();

        switch (charKey) {
          case "c":
          case "x":
            this.copyCut(event, charKey);
            return;
          case "v":
            this.paste(event);
            return;
          case "a":
            event.preventDefault();
            this.selectAll();
            return;
          case "d":
            event.preventDefault();
            downloadFiles(state.selected);
            return;
        }
        // Don't return here - allow other modifier key combinations to propagate
      }

      // Handle key events using a switch statement
      switch (key) {
        case "Enter":
          if (this.selectedCount === 1) {
            const selected = getters.getFirstSelected();
            const selectedUrl = url.buildItemUrl(selected.source, selected.path);
            router.push({ path: selectedUrl });
          }
          break;

        case "Backspace":
          if (getters.currentPromptName()) {
            return;
          }
          // go back
          router.push({ path: newPath });
          break;

        case "Escape":
          mutations.resetSelected();
          break;

        case "Delete":
          if (!this.permissions?.modify || state.selected.length === 0) return;
          this.showDeletePrompt();
          break;

        case "F2":
          if (!this.permissions?.modify || state.selected.length !== 1)  return;
          mutations.showHover({
            name: "rename",
            props: {
              item: getters.getFirstSelected(),
            },
          });
          break;

        case "ArrowUp":
        case "ArrowDown":
        case "ArrowLeft":
        case "ArrowRight":
          // Allow native browser navigation when Alt is held
          if (event.altKey) {
            return;
          }
          event.preventDefault();
          this.navigateKeboardArrows(key);
          break;
      }
    },
    selectAll() {
      for (let file of this.items.files) {
        if (state.selected.indexOf(file.index) === -1) {
          mutations.addSelected(file.index);
        }
      }
      for (let dir of this.items.dirs) {
        if (state.selected.indexOf(dir.index) === -1) {
          mutations.addSelected(dir.index);
        }
      }
    },
    alphanumericKeyPress(key) {
      // Convert the key to uppercase to match the case-insensitive search
      const searchLetter = key.toLowerCase();
      const currentSelected = getters.getFirstSelected();
      let currentName = null;
      let findNextWithName = false;

      if (currentSelected != undefined) {
        currentName = currentSelected.name.toLowerCase();
        if (currentName.startsWith(searchLetter)) {
          findNextWithName = true;
        }
      }
      // Combine directories and files (assuming they are stored in this.items.dirs and this.items.files)
      const allItems = [...this.items.dirs, ...this.items.files];
      let foundPrevious = false;
      let firstFound = null;
      // Iterate over all items to find the first one where the name starts with the searchLetter
      for (let i = 0; i < allItems.length; i++) {
        const itemName = allItems[i].name.toLowerCase();
        if (!itemName.startsWith(searchLetter)) {
          continue;
        }
        if (firstFound == null) {
          firstFound = allItems[i].index;
        }
        if (!findNextWithName) {
          // return first you find
          this.selectItem(allItems[i].index);
          return;
        }
        if (itemName == currentName) {
          foundPrevious = true;
          continue;
        }
        if (foundPrevious) {
          this.selectItem(allItems[i].index);
          return;
        }
      }
      // select the first item again
      if (firstFound != null) {
        this.selectItem(firstFound);
      }
    },
    preventDefault(event) {
      // Wrapper around prevent default.
      event.preventDefault();
    },
    copyCut(event, key) {
      if (event.target.tagName.toLowerCase() === "input") {
        return;
      }

      let items = state.selected.map((i) => ({
        from: state.req.items[i].path,
        fromSource: state.req.source,
        name: state.req.items[i].name,
      }));

      if (items.length === 0) {
        return;
      }

      this.clipboard = {
        key: key,
        items: items,
        path: state.route.path,
      };
    },
    async paste(event) {
      if (event.target.tagName.toLowerCase() === "input") {
        return;
      }

      if (!this.clipboard || !this.clipboard.items || this.clipboard.items.length === 0) {
        return;
      }

      // Construct destination path properly (without URL prefix)
      const destPath = state.req.path.endsWith('/') ? state.req.path : state.req.path + '/';

      let items = this.clipboard.items.map((item) => ({
        from: item.from,
        fromSource: item.fromSource,
        to: destPath + item.name,
        toSource: state.req.source
      }));

      const operation = this.clipboard.key === "x" ? "move" : "copy";

      // Show confirmation prompt first
      mutations.showHover({
        name: "CopyPasteConfirm",
        props: {
          operation: operation,
          items: items,
          onConfirm: async () => {
            mutations.setLoading("listing", true);

            let action = async (overwrite, rename) => {
              try {
              if (getters.isShare()) {
                await publicApi.moveCopy(state.shareInfo.hash, items, operation, overwrite, rename);
                } else {
                  await filesApi.moveCopy(items, operation, overwrite, rename);
                }
                if (operation === "move") {
                  this.clipboard = { items: [] };
                }
                mutations.setLoading("listing", false);
              } catch (error) {
                console.error("Error moving/copying items:", error);
              } finally {
                mutations.setLoading("listing", false);
                mutations.setReload(true);
              }
            };

            if (this.clipboard.path === state.route.path) {
              action(false, true);
              return;
            }

            const conflict = upload.checkConflict(items, state.req.items);

            if (conflict) {
              mutations.showHover({
                name: "replace-rename",
                confirm: (event, option) => {
                  const overwrite = option === "overwrite";
                  const rename = option === "rename";
                  event.preventDefault();
                  mutations.closeHovers();
                  action(overwrite, rename);
                },
              });
              return;
            }

            action(false, false);
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
      if (isInternal) {
        return;
      }
      this.dragCounter++;
    },
    dragLeave(event) {
      // If in upload share mode, let the embedded Upload component handle it
      if (state.shareInfo?.shareType === 'upload') {
        return;
      }
      const isInternal = Array.from(event.dataTransfer.types).includes(
        "application/x-filebrowser-internal-drag"
      );
      if (isInternal) {
        return;
      }
      if (this.dragCounter == 0) {
        return;
      }
      this.dragCounter--;
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
      this.handleDrop(event);
    },
    async uploadInput(event) {
      this.handleDrop(event);
    },
    setMultiple(val) {
      mutations.setMultiple(val == true);
      showMultipleSelection();
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
      if (this.$refs.listingView == null) return;
    }, 100),
    openContext(event) {
      event.preventDefault();
      event.stopPropagation();
      
      // Prevent opening if already open
      if (getters.currentPromptName() === "ContextMenu") {
        console.log("[ListingView] openContext: ContextMenu already open, skipping");
        return;
      }
      
      console.log("[ListingView] openContext: opening ContextMenu");
      mutations.showHover({
        name: "ContextMenu",
        props: {
          showCentered: getters.isMobile(),
          posX: event.clientX,
          posY: event.clientY,
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

      const sameAsBefore = state.selected == this.lastSelected;
      if (sameAsBefore && !state.multiple && getters.currentPromptName() == "") {
        mutations.resetSelected();
      }
      this.lastSelected = state.selected;
    },
    async handleDrop(event) {
      event.preventDefault();
      this.dragCounter = 0;

      // If we're already in the embedded upload view, don't open a new prompt
      // The embedded Upload component will handle its own drops
      if (state.shareInfo?.shareType === 'upload') {
        return;
      }

      if (event.type === "drop") {
        mutations.showHover({
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

        mutations.showHover({
          name: "upload",
          props: {
            // we send it as an array-like object so that it can be processed like a FileList by the Upload component
            initialItems: Array.from(files),
          },
        });
      }
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
      this.rectangleStart = {
        x: event.clientX - listingRect.left,
        y: event.clientY - listingRect.top
      };
      this.rectangleEnd = {
        x: event.clientX - listingRect.left,
        y: event.clientY - listingRect.top
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
      this.rectangleEnd = {
        x: event.clientX - listingRect.left,
        y: event.clientY - listingRect.top
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
          const index = parseInt(element.getAttribute('data-index'));
          if (!isNaN(index)) {
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
        newSelection.forEach(index => mutations.addSelected(index));
      } else {
        // Select only the items in the rectangle and reset initial selection
        // PS: If you don't want that just hold ctrl, the selection will not be reset, allowing multi select.
        mutations.resetSelected();
        rectangleSelectedIndexes.forEach(index => mutations.addSelected(index));
      }
    },
  },
};
</script>

<style scoped>

.add-padding {
  padding-left: 0.5em;
}
.font-size-large h2 {
  font-size: 2em !important;
}

.listing-items.dropping {
  border-radius: 1em;
  max-height: 70vh;
  width: 97%;
  overflow: hidden;
  margin: 1em;
  box-shadow: var(--primaryColor) 0 0 1em;
}

.listing-items {
  min-height: 75vh !important;
  position: relative;
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

</style>
