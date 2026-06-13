<template>
  <a
    v-if="isClickable"
    :href="getUrl()"
    class="listing-item clickable no-select"
    :class="{
      activebutton: isSelected,
      'pinned-item': isPinned,
      hiddenFile: isHiddenNotSelected && this && !this.isDraggedOver,
      'half-selected': isDraggedOver,
      'drag-hover': isDraggedOver,
      'out-of-view': !isInView && !isSelected,
    }"
    :id="getID"
    role="button"
    tabindex="0"
    :draggable="isDraggable"
    @dragstart="dragStart"
    @dragleave="dragLeave"
    @dragover="dragOver"
    @drop="drop"
    :data-dir="isDir"
    :data-type="type"
    :data-name="name"
    :data-index="index"
    :aria-label="name"
    :aria-selected="isSelected"
    @contextmenu="onRightClick"
    @click="click"
    @touchstart="addSelected"
    @touchmove="handleTouchMove"
    @touchend="cancelContext"
    @mouseup="cancelContext"
  >
    <div :class="{ 'gallery-div': galleryView }">
      <Icon
        :mimetype="type"
        :active="isSelected"
        :thumbnailUrl="isThumbnailInView ? thumbnailUrl : ''"
        :filename="name"
        :hasPreview="hasPreview"
        :modified="modified"
        :path="path"
        :source="source"
        :size="size"
        :isShared="isShared"
      />
    </div>

    <div class="text">
      <!-- For list/compact pin inside .name (inline), or if inlinePin is true (like on FileList) -->
      <p v-if="isListMode || inlinePin" class="name">
        <span>{{ displayName }}</span>
        <i v-if="isPinned" class="material-symbols pinned-indicator">push_pin</i>
      </p>
      <!-- For other views pin is separate -->
      <p v-else class="name">
        <span>{{ displayName }}</span>
      </p>
      <p class="size" :data-order="humanSize">{{ humanSize }}</p>
      <p class="modified"><time :datetime="modified">{{ formattedTime }}</time></p>
      <p v-if="hasDuration" class="duration">{{ formattedDuration }}</p>
    </div>
    <div v-if="isPinned && !isListMode && !inlinePin" class="pin-icon-wrapper">
      <i class="material-symbols pinned-indicator">push_pin</i>
    </div>

    <Icon
      @click.stop="downloadFile"
      v-if="quickDownloadEnabled"
      :filename="name"
      :hasPreview="hasPreview"
      mimetype="file_download"
      class="download-icon"
      role="button"
      aria-label="Download"
      tabindex="0"
      :clickable="true"
      :isShared="isShared"
    />
  </a>
  <div
    v-else
    class="listing-item no-select clickable"
    :class="{
      activebutton: isSelected,
      'pinned-item': isPinned,
      hiddenFile: isHiddenNotSelected && this && !this.isDraggedOver,
      'half-selected': isDraggedOver,
      'drag-hover': isDraggedOver,
      'out-of-view': !isInView && !isSelected,
    }"
    :id="getID"
    role="button"
    tabindex="0"
    :data-dir="isDir"
    :data-type="type"
    :data-name="name"
    :data-index="index"
    :aria-label="name"
    :aria-selected="isSelected"
    @contextmenu="onRightClick"
    @click="click"
    @touchstart="addSelected"
    @touchmove="handleTouchMove"
    @touchend="cancelContext"
    @mouseup="cancelContext"
  >
    <div :class="{ 'gallery-div': galleryView }">
      <Icon
        :mimetype="type"
        :thumbnailUrl="isThumbnailInView ? thumbnailUrl : ''"
        :filename="name"
        :hasPreview="hasPreview"
        :modified="modified"
        :path="path"
        :source="source"
        :size="size"
        :isShared="isShared"
      />
    </div>

    <div class="text">
      <p v-if="isListMode || inlinePin" class="name">
        <span>{{ displayName }}</span>
        <i v-if="isPinned" class="material-symbols pinned-indicator">push_pin</i>
      </p>
      <p v-else class="name">
        <span>{{ displayName }}</span>
      </p>
      <p class="size" :data-order="humanSize">{{ humanSize }}</p>
      <p class="modified"><time :datetime="modified">{{ formattedTime }}</time></p>
      <p v-if="hasDuration" class="duration">{{ formattedDuration }}</p>
    </div>
    <div v-if="isPinned && !isListMode && !inlinePin" class="pin-icon-wrapper">
      <i class="material-symbols pinned-indicator">push_pin</i>
    </div>
  </div>
</template>

<script>
import { globalVars } from "@/utils/constants";
import downloadFiles from "@/utils/download";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { getObjectProperty } from '@/utils/object.js';
import { resourcesApi } from "@/api";
import * as upload from "@/utils/upload";
import { state, getters, mutations } from "@/store"; // Import your custom store
import { url } from "@/utils";
import { notify } from "@/notify";
import { goToItemNotificationButton } from "@/utils/notificationActions";
import Icon from "@/components/files/Icon.vue";

export default {
  name: "item",
  components: {
    Icon,
  },
  data() {
    return {
      isThumbnailInView: false,
      isInView: false,
      touches: 0,
      touchStartX: 0,
      touchStartY: 0,
      isLongPress: false,
      isSwipe: false,
      isDraggedOver: false,
      contextTimeout: null,
      observer: null,
      localSelected: false,
    };
  },
  props: {
    name: String,
    isDir: Boolean,
    source: String,
    type: String,
    size: Number,
    modified: String,
    index: [Number, String],
    readOnly: Boolean,
    path: String,
    reducedOpacity: Boolean,
    hash: String,
    hasPreview: Boolean,
    metadata: Object,
    hasDuration: Boolean,
    isShared: {
      type: Boolean,
      default: false,
    },
    displayFullPath: Boolean,
    updateGlobalState: {
      type: Boolean,
      default: true,
    },
    isSelectedProp: {
      type: Boolean,
      default: null, // null means use internal state
    },
    clickable: {
      type: Boolean,
      default: true,
    },
    forceFilesApi: {
      type: Boolean,
      default: false,
    },
    showLimitedOptions: {
      type: Boolean,
      default: false,
    },
    disableContextMenu: {
      type: Boolean,
      default: false,
    },
    inlinePin: {
      type: Boolean,
      default: false,
    },
    pinned: {
      type: Boolean,
      default: false,
    },
  },
  computed: {
    displayName() {
      // If displayFullPath is true, show the full path, otherwise just the name
      return this.displayFullPath ? this.path : this.name;
    },
    galleryView() {
      return getters.viewMode() === "gallery";
    },
    quickDownloadEnabled() {
      if (getters.isShare()) {
        return state.shareInfo?.quickDownload && !this.galleryView;
      }
      return state.user?.quickDownload && !this.galleryView;
    },
    quickDownloadPlaceholder() {
      if (getters.isShare()) {
        return state.shareInfo?.quickDownload && !this.galleryView && this.isDir;
      }
      return state.user?.quickDownload && !this.galleryView && getters.viewMode() !== "icons" && getters.viewMode() !== "normal" && this.isDir;
    },
    isHiddenNotSelected() {
      return !this.isSelected && this.reducedOpacity;
    },
    getID() {
      return url.base64Encode(encodeURIComponent(this.name));
    },
    quickNav() {
      return state.user.singleClick && !state.multiple;
    },
    user() {
      return state.user;
    },
    selected() {
      return state.selected;
    },
    isClicked() {
      if (state.user.singleClick || !this.allowedView) {
        return false;
      }
      return this.isSelected;
    },
    isSelected() {
      if (!this.updateGlobalState) {
        // If parent provides isSelectedProp, use it; otherwise use local state
        return this.isSelectedProp !== null ? this.isSelectedProp : this.localSelected;
      }
      return state.selected.indexOf(this.index) !== -1;
    },
    isDraggable() {
      return this.readOnly === undefined && state.user.permissions?.modify || state.shareInfo.allowCreate;
    },
    canDrop() {
      if (!this.isDir) return false;
      if (this.readOnly === true) return false;

      for (const i of this.selected) {
        const item = getObjectProperty(state.req.items, i);
        if (!item) continue;
        // Also check if we're trying to drop an item onto itself
        if (item.path === this.path && state.req.source === this.source) {
          return false;
        }
        if (item.index === this.index) {
          return false;
        }
      }
      return true;
    },
    thumbnailUrl() {
      if (!globalVars.enableThumbs) {
        return "";
      }

      // Use the path prop if available (e.g., in duplicate finder),
      // otherwise construct from state.req.path + name (normal file listing)
      let previewPath;
      if (this.path) {
        previewPath = this.path;
      } else if (state.req?.path && this.name) {
        previewPath = url.joinPath(state.req.path, this.name);
      } else {
        return "";
      }

      // If forceFilesApi is true, always use authenticated files API
      if (this.forceFilesApi) {
        return resourcesApi.getPreviewURL(this.source || state.req?.source, previewPath, this.modified);
      }

      if (getters.isShare()) {
        return resourcesApi.getPreviewURLPublic(previewPath);
      }
      return resourcesApi.getPreviewURL(this.source || state.req?.source, previewPath, this.modified);
    },
    isThumbsEnabled() {
      return globalVars.enableThumbs;
    },
    isClickable() {
      return this.clickable;
    },
    isPinned() {
      return this.pinned;
    },
    // Computed properties for display values - Vue caches these automatically!
    humanSize() {
      return this.type === "invalid_link"
        ? "invalid link"
        : getHumanReadableFilesize(this.size);
    },
    formattedTime() {
      return getters.getTime(this.modified);
    },
    formattedDuration() {
      if (!this.metadata?.duration) {
        return "";
      }
      const seconds = this.metadata.duration;
      const hours = Math.floor(seconds / 3600);
      const minutes = Math.floor((seconds % 3600) / 60);
      const secs = Math.floor(seconds % 60);
      if (hours > 0) {
        return `${hours}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
      }
      return `${minutes}:${secs.toString().padStart(2, '0')}`;
    },
    isListMode() {
      const mode = getters.viewMode();
      return mode === 'list' || mode === 'compact';
    },
  },
  mounted() {
    // Set up IntersectionObserver for lazy-loading thumbnails
    this.observer = new IntersectionObserver(this.handleIntersect, {
      root: null,
      rootMargin: "500px", // Reduced from 1500px for better performance
      threshold: 0,
    });

    // Use $nextTick to ensure $el is available and is an Element
    this.$nextTick(() => {
      if (this.$el && this.$el instanceof Element) {
        this.observer.observe(this.$el);
        const rect = this.$el.getBoundingClientRect();
        const isInViewport = rect.top < window.innerHeight + 500 && rect.bottom > -500;
        if (isInViewport && this.hasPreview) {
          this.isThumbnailInView = true;
          this.isInView = true;
        }
      }
    });
    // Note: dragend listener moved to parent ListingView for better performance
  },
  beforeUnmount() {
    // Clean up observer
    if (this.observer) {
      this.observer.disconnect();
      this.observer = null;
    }
    // Note: dragend listener removed - handled by parent ListingView
  },
  methods: {
    /** @param {MouseEvent} event */
    downloadFile(event) {
      event.preventDefault();
      event.stopPropagation();
      if (this.updateGlobalState) {
        mutations.resetSelected();
        mutations.addSelected(this.index);
        downloadFiles(state.selected);
      } else {
        // Emit selection event for local handling
        this.$emit('select', { index: this.index, selected: true });
      }
    },
    /** @param {TouchEvent} event */
    handleTouchMove(event) {
      if (!state.isSafari) return;
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
      if (!state.isSafari) return;
      this.cancelContext(); // Clear timeout
      this.isSwipe = false; // Reset swipe state
    },
    cancelContext() {
      if (this.contextTimeout) {
        clearTimeout(this.contextTimeout);
        this.contextTimeout = null;
      }
      this.isLongPress = false;
    },
    /** @param {string} path */
    updateHashAndNavigate(path) {
      // Update hash in the browser without full page reload
      window.location.hash = path;
    },
    getUrl() {
      if (this.hash) {
        return `${globalVars.baseURL}public/share/${this.hash}/${url.encodedPath(this.path)}`;
      }
      return `${globalVars.baseURL}files/${encodeURIComponent(this.source)}${url.encodedPath(this.path)}`;
    },
    /** @param {MouseEvent} event */
    onRightClick(event) {
      event.preventDefault(); // Prevent default context menu
      if (this.updateGlobalState) {
        // If one or fewer items are selected, reset the selection
        if (!state.multiple && getters.selectedCount() < 2) {
          mutations.resetSelected();
          mutations.addSelected(this.index);
        }
      } else {
        // Build full item object similar to Search.vue
        const selectedItem = {
          name: this.name,
          isDir: this.isDir,
          source: this.source,
          type: this.type,
          size: this.size,
          modified: this.modified,
          path: this.path,
          url: this.path,
          index: this.index,
        };
        mutations.resetSelected();
        mutations.addSelected(selectedItem);
      }
      
      if (this.disableContextMenu) {
        return;
      }
      mutations.showPrompt({
        name: "ContextMenu",
        props: {
          posX: event.clientX,
          posY: event.clientY,
          showLimitedOptions: this.showLimitedOptions,
        },
      });
    },
    /**
     * @param {IntersectionObserverEntry[]} entries
     * @param {IntersectionObserver} observer
     */
    handleIntersect(entries) {
      entries.forEach((entry) => {
        // Update both view state and thumbnail state
        this.isInView = entry.isIntersecting;
        if (entry.isIntersecting) {
          this.isThumbnailInView = true;
        }
      });
    },
    /** @param {DragEvent} event */
    dragLeave(event) {
      // Only reset visual state for internal drags
      const isInternal = Array.from(event.dataTransfer.types).includes(
        "application/x-filebrowser-internal-drag"
      );
      if (isInternal) {
        this.isDraggedOver = false;
      }
    },
    /** @param {DragEvent} event */
    dragStart(event) {
      if (this.updateGlobalState) {
        if (state.selected.indexOf(this.index) === -1) {
          mutations.resetSelected();
          mutations.addSelected(this.index);
        }
      } else {
        // Emit selection event for local handling
        if (!this.localSelected) {
          this.$emit('select', { index: this.index, selected: true });
        }
      }
      if (event.dataTransfer) {
        event.dataTransfer.setData(
          "application/x-filebrowser-internal-drag",
          "true"
        );
      }
    },
    /** @param {DragEvent} event */
    dragOver(event) {
      if (!this.canDrop) return;

      // Only allow internal drags (from filebrowser items), not external files from desktop
      const isInternal = Array.from(event.dataTransfer.types).includes(
        "application/x-filebrowser-internal-drag"
      );

      if (!isInternal) return;

      event.preventDefault();
      this.isDraggedOver = true;
    },
    dragEnd() {
      this.isDraggedOver = false;
    },
    /** @param {DragEvent} event */
    async drop(event) {
      this.isDraggedOver = false;
      if (!this.canDrop) return;

      // Only allow internal drags (from filebrowser items), not external files from desktop
      const isInternal = Array.from(event.dataTransfer.types).includes(
        "application/x-filebrowser-internal-drag"
      );

      if (!isInternal) {
        // Don't handle external drags - let the parent ListingView handle them
        return;
      }

      // Only stop propagation if we're actually going to handle this drop (moving files into a folder)
      event.preventDefault();
      event.stopPropagation();

      let items = [];
      for (const i of state.selected) {
        const item = getObjectProperty(state.req.items, i);
        if (!item) continue;
        items.push({
          from: item.path,
          fromSource: item.source,
          to: url.joinPath(this.path, item.name),
          toSource: this.source,
          name: item.name,
        });
      }

      // Filter out items being dropped onto themselves or into their own subdirectories
      items = items.filter(item => {
        // Skip if source and destination are the same
        if (item.from === item.to) {
          return false;
        }

        // Skip if trying to move a directory into itself
        // Check if the destination path would be within the source path
        const fromPath = item.from;
        const destinationDir = this.path;

        // If destination dir is the same as or contains the source path, skip
        if (fromPath === destinationDir || fromPath.startsWith(`${destinationDir}/`)) {
          return false;
        }

        return true;
      });

      // If all items were filtered out, silently skip the operation
      if (items.length === 0) {
        return;
      }

      const checkAction = async () => {
        if (getters.isShare()) {
          return await resourcesApi.fetchFilesPublic(this.path, state.shareInfo.hash);
        } else {
          return await resourcesApi.fetchFiles(this.source, this.path);
        }
      }
      const response = await checkAction();
      const conflict = upload.checkConflict(items, response?.items || [] );

      /**
       * @param {boolean} overwrite
       * @param {boolean} rename
       */
      const action = async (overwrite, rename) => {
        // Show move prompt with operation in progress
        mutations.showPrompt({
          name: "move",
          props: {
            operationInProgress: true,
          },
        });

        try {
          if (getters.isShare()) {
            await resourcesApi.moveCopyPublic(state.shareInfo.hash, items, "move", overwrite, rename);
          } else {
            await resourcesApi.moveCopy(items, "move", overwrite, rename);
          }
          // Notification to move into the folder
          const buttonProps = {
            icon: "folder",
            buttons: [
              goToItemNotificationButton(
                this.$t("buttons.goToItem"),
                this.source,
                this.path,
                getters.isShare()
              ),
            ],
          };
          notify.showSuccess(this.$t("prompts.moveSuccess"), buttonProps);
          // Close the prompt after successful operation and reload items for reflect the changes
          mutations.closeTopPrompt();
          mutations.setReload(true);
        } catch (error) {
          // Close the prompt and let error handling continue
          mutations.closeTopPrompt();
          throw error;
        }
      };

      if (conflict) {
        mutations.showPrompt({
          name: "replace-rename",
          pinned: true,
          /**
           * @param {Event} event
           * @param {string} option
           */
          confirm: async (event, option) => {
            const overwrite = option === "overwrite";
            const rename = option === "rename";

            event.preventDefault();
            mutations.closeTopPrompt();
            await action(overwrite, rename);
          },
        });
        return;
      }

      await action(false, false);
    },
    /** @param {TouchEvent} event */
    addSelected(event) {
      if (!state.isSafari) {
        return;
      }
      if (event.type !== "touchstart") {
        return;
      }
      const touch = event.touches[0];
      this.touchStartX = touch.clientX;
      this.touchStartY = touch.clientY;
      this.isLongPress = false; // Reset state
      this.isSwipe = false; // Reset swipe detection
      if (state.multiple) {
        return;
      }
      this.contextTimeout = setTimeout(() => {
        if (!this.isSwipe) {
          // Only reset selection if this item is not already selected
          // This prevents resetting selection when trying to open context menu on selected item
          if (!this.isSelected) {
            if (this.updateGlobalState) {
              mutations.resetSelected();
              mutations.addSelected(this.index);
            } else {
              this.$emit('select', { index: this.index, selected: true });
            }
          }
        }
      }, 500);
    },
    /** @param {MouseEvent} event */
    click(event) {
      if (event.button === 0) {
        // Left-click
        event.preventDefault();
        if (this.quickNav) {
          this.open();
        }
      }

      if (
        !state.user?.singleClick &&
        getters.selectedCount() !== 0 &&
        event.button === 0
      ) {
        event.preventDefault();
      }
      setTimeout(() => {
        this.touches = 0;
      }, 500);
      this.touches++;
      if (this.touches > 1) {
        this.open();
      }

      if (this.updateGlobalState) {
        if (event.shiftKey && this.selected.length > 0) {
          let fi = 0;
          let la = 0;

          if (state.lastSelectedIndex !== null) {
            if (this.index > state.lastSelectedIndex) {
              fi = state.lastSelectedIndex;
              la = this.index;
            } else {
              fi = this.index;
              la = state.lastSelectedIndex;
            }
          }

          mutations.resetSelected();

          for (; fi <= la; fi++) {
            if (this.selected.indexOf(fi) === -1) {
              mutations.addSelected(fi);
            }
          }
          return;
        }

        if (this.selected.indexOf(this.index) !== -1) {
          if (event.ctrlKey || event.metaKey) {
            mutations.removeSelected(this.index);
            mutations.setLastSelectedIndex(this.index);
            return;
          }

          // In multiple selection mode, clicking an already selected item should deselect it
          if (state.multiple) {
            mutations.removeSelected(this.index);
            mutations.setLastSelectedIndex(this.index);
            return;
          }

          if (this.selected.length > 1) {
            mutations.resetSelected();
            mutations.addSelected(this.index);
            mutations.setLastSelectedIndex(this.index);
          }
          return;
        }

        if (
          !state.user?.singleClick &&
          !event.ctrlKey &&
          !event.metaKey &&
          !event.shiftKey &&
          !state.multiple
        ) {
          mutations.resetSelected();
        }
        mutations.addSelected(this.index);
        mutations.setLastSelectedIndex(this.index);
      } else {
        // Local selection handling - emit events instead of updating global state
        if (event.shiftKey) {
          // Shift-click: select range (emit event for parent to handle)
          this.$emit('selectRange', {
            startIndex: state.lastSelectedIndex !== null ? state.lastSelectedIndex : this.index,
            endIndex: this.index
          });
          return;
        }

        // Always toggle - parent component will handle the toggle logic
        // Just emit the index, parent will check current state and toggle
        this.$emit('select', { index: this.index });
      }
    },
    open() {
      // Don't navigate if updateGlobalState is false (component is being used as a picker/selector)
      if (!this.updateGlobalState) {
        return;
      }

      // Check if state.req.items exists and has the item at this index
      // This prevents errors when ListingItem is used outside of the main file listing (e.g., duplicate finder)
      let previousHistoryItem = null;
      const currentItem = getObjectProperty(state.req.items, this.index);
      if (currentItem) {
        previousHistoryItem = {
          name: currentItem.name,
          source: state.req.source,
          path: state.req.path,
        };
      }
      url.goToItem(this.source, this.path, previousHistoryItem, false, getters.isShare());
    },
  },
};
</script>

<style>
.download-icon {
  cursor: pointer;
  color: var(--secondaryColor);
}

.icon-download {
  font-size: 0.5em;
}

.listing-item {
  -webkit-touch-callout: none; /* Disable the default long press preview */
}

.listing-item.pinned-item {
  border-color: color-mix(in srgb, var(--primaryColor) 35%, transparent);
}

/* Disable transitions and hide content for out-of-view items for better performance */
.listing-item.out-of-view {
  transition: none !important;
}

.listing-item.out-of-view * {
  transition: none !important;
  opacity: 0 !important;
  pointer-events: none !important;
}

/* Ensure items maintain their height even when content is hidden */
.listing-item > div {
  min-height: 1em; /* Forces layout calculation even with hidden content */
}

.hiddenFile {
  opacity: 0.5;
}

.activebutton {
  height: 10em;
}

.iconActive {
  font-size: 6em !important;
}

.activetitle {
  width: 9em !important;
  margin-right: 1em !important;
}

.half-selected {
  border-color: var(--primaryColor) !important;
  border-style: solid !important;
}
</style>
