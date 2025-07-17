<template>
  <a
    :href="getUrl()"
    :class="{
      item: true,
      'no-select': true,
      'listing-item': true,
      activebutton: isMaximized && isSelected,
      hiddenFile: isHiddenNotSelected && !this.isDraggedOver,
      'half-selected': isDraggedOver,
      'drag-hover': isDraggedOver,
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
    :aria-label="name"
    :aria-selected="isSelected"
    @contextmenu="onRightClick($event)"
    @click="click($event)"
    @touchstart="addSelected($event)"
    @touchmove="handleTouchMove($event)"
    @touchend="cancelContext($event)"
    @mouseup="cancelContext($event)"
  >
    <div @click="toggleClick" :class="{ 'gallery-div': galleryView }">
      <Icon
        :mimetype="type"
        :active="isSelected"
        :thumbnailUrl="isThumbnailInView ? thumbnailUrl : ''"
        :filename="name"
      />
    </div>

    <div class="text" :class="{ activecontent: isMaximized && isSelected }">
      <p :class="{ adjustment: quickDownloadEnabled }" class="name">{{ name }}</p>
      <p
        class="size"
        :class="{ adjustment: quickDownloadEnabled }"
        :data-order="humanSize()"
      >
        {{ humanSize() }}
      </p>
      <p class="modified">
        <time :datetime="modified">{{ getTime() }}</time>
      </p>
    </div>
    <Icon
      @click="downloadFile"
      v-if="quickDownloadEnabled"
      :filename="name"
      mimetype="file_download"
      style="padding-right: 0.5em"
    />
  </a>
</template>

<script>
import { enableThumbs } from "@/utils/constants";
import downloadFiles from "@/utils/download";

import { getHumanReadableFilesize } from "@/utils/filesizes";
import { filesApi, shareApi } from "@/api";
import * as upload from "@/utils/upload";
import { state, getters, mutations } from "@/store"; // Import your custom store
import { url } from "@/utils";
import Icon from "@/components/files/Icon.vue";
import { baseURL, serverHasMultipleSources } from "@/utils/constants";

export default {
  name: "item",
  components: {
    Icon,
  },
  data() {
    return {
      isThumbnailInView: false,
      isMaximized: false,
      touches: 0,
      touchStartX: 0,
      touchStartY: 0,
      isLongPress: false,
      isSwipe: false,
      isDraggedOver: false,
    };
  },
  props: [
    "name",
    "isDir",
    "source",
    "type",
    "size",
    "modified",
    "index",
    "readOnly",
    "path",
    "reducedOpacity",
    "hash",
  ],
  computed: {
    galleryView() {
      return state.user.viewMode === "gallery";
    },
    quickDownloadEnabled() {
      return state.user?.quickDownload && !this.galleryView && !this.isDir;
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
      return !this.isMaximized;
    },
    isSelected() {
      return this.selected.indexOf(this.index) !== -1;
    },
    isDraggable() {
      return this.readOnly == undefined && state.user.permissions?.modify;
    },
    canDrop() {
      if (!this.isDir || this.readOnly !== undefined) return false;
      for (let i of this.selected) {
        if (state.req.items[i].path === this.path && state.req.source === this.source) {
          return false;
        }
      }
      return true;
    },
    thumbnailUrl() {
      if (!enableThumbs) {
        return "";
      }
      let path = url.removeTrailingSlash(state.req.path) + "/" + this.name;
      if (getters.currentView() == "share") {
        let urlPath = getters.routePath("share");
        // Step 1: Split the path by '/'
        const hash = urlPath.split("/")[1];
        return shareApi.getPreviewURL(hash, path, state.req.modified);
      }
      return filesApi.getPreviewURL(state.req.source, path, state.req.modified);
    },
    isThumbsEnabled() {
      return enableThumbs;
    },
  },
  mounted() {
    // Prevent default navigation for left-clicks
    const observer = new IntersectionObserver(this.handleIntersect, {
      root: null,
      rootMargin: "0px",
      threshold: 0.1, // Adjust threshold as needed
    });

    observer.observe(this.$el);
  },
  methods: {
    downloadFile(event) {
      event.preventDefault();
      event.stopPropagation();
      mutations.resetSelected();
      mutations.addSelected(this.index);
      downloadFiles(state.selected);
    },
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
    updateHashAndNavigate(path) {
      // Update hash in the browser without full page reload
      window.location.hash = path;

      // Optional: Trigger native navigation
      window.location.href = this.getRelative(path);
    },
    getUrl() {
      if (this.hash) {
        return baseURL + "share/" + this.hash + this.path;
      }
      if (serverHasMultipleSources) {
        return baseURL + "files/" + this.source + this.path;
      }
      return baseURL + "files" + this.path;
    },
    onRightClick(event) {
      event.preventDefault(); // Prevent default context menu
      // If one or fewer items are selected, reset the selection
      if (!state.multiple && getters.selectedCount() < 2) {
        mutations.resetSelected();
        mutations.addSelected(this.index);
      }
      mutations.showHover({
        name: "ContextMenu",
        props: {
          posX: event.clientX,
          posY: event.clientY,
        },
      });
    },
    handleIntersect(entries, observer) {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          this.isThumbnailInView = true;
          // Stop observing once thumbnail is in view
          observer.unobserve(entry.target);
        }
      });
    },
    toggleClick() {
      this.isMaximized = this.isClicked;
    },
    humanSize() {
      return this.type == "invalid_link"
        ? "invalid link"
        : getHumanReadableFilesize(this.size);
    },
    getTime() {
      return getters.getTime(this.modified);
    },
    dragLeave() {
      this.isDraggedOver = false;
    },
    dragStart(event) {
      if (this.selected.indexOf(this.index) === -1) {
        mutations.resetSelected();
        mutations.addSelected(this.index);
      }
      event.dataTransfer.setData(
        "application/x-filebrowser-internal-drag",
        "true"
      );
    },
    dragOver(event) {
      if (!this.canDrop) return;
      event.preventDefault();
      this.isDraggedOver = true;
    },
    async drop(event) {
      event.preventDefault();
      event.stopPropagation();
      this.isDraggedOver = false;

      if (!this.canDrop) {
        return;
      }

      let items = [];
      for (let i of state.selected) {
        items.push({
          from: state.req.items[i].path,
          fromSource: state.req.items[i].source,
          to: this.path + "/" + state.req.items[i].name,
          toSource: this.source,
        });
      }

      const conflict = upload.checkConflict(
        items,
        (await filesApi.fetchFiles(this.source, this.path)).items
      );

      let action = async (overwrite, rename) => {
        await filesApi.moveCopy(items, "move", overwrite, rename);
      };

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
    addSelected(event) {
      if (state.isSafari) {
        if (event.type === "touchstart") {
          const touch = event.touches[0];
          this.touchStartX = touch.clientX;
          this.touchStartY = touch.clientY;
          this.isLongPress = false; // Reset state
          this.isSwipe = false; // Reset swipe detection
          if (!state.multiple) {
            this.contextTimeout = setTimeout(() => {
              if (!this.isSwipe) {
                mutations.resetSelected();
                mutations.addSelected(this.index);
              }
            }, 500);
          }
        }
      }
    },
    click(event) {
      if (event.button === 0) {
        // Left-click
        event.preventDefault();
        if (this.quickNav) {
          this.open();
        }
      }

      if (
        !state.user.singleClick &&
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

      if (event.shiftKey && state.selected.length > 0) {
        let fi = 0;
        let la = 0;

        if (this.index > state.lastSelectedIndex) {
          fi = state.lastSelectedIndex;
          la = this.index;
        } else {
          fi = this.index;
          la = state.lastSelectedIndex;
        }

        mutations.resetSelected();

        for (; fi <= la; fi++) {
          if (state.selected.indexOf(fi) === -1) {
            mutations.addSelected(fi);
          }
        }
        return;
      }

      if (state.selected.indexOf(this.index) !== -1) {
        if (event.ctrlKey || event.metaKey) {
          mutations.removeSelected(this.index);
          mutations.setLastSelectedIndex(this.index);
          return;
        }

        if (state.selected.length > 1) {
          mutations.resetSelected();
          mutations.addSelected(this.index);
          mutations.setLastSelectedIndex(this.index);
        }
        return;
      }

      if (
        !state.user.singleClick &&
        !event.ctrlKey &&
        !event.metaKey &&
        !state.multiple
      ) {
        mutations.resetSelected();
      }
      mutations.addSelected(this.index);
      mutations.setLastSelectedIndex(this.index);
    },
    open() {
      if (this.hash) {
        const shareHash = this.hash;
        url.goToItem(this.source, this.path, "", shareHash);
        return;
      }
      const previousHash = state.req.items[this.index].name;
      url.goToItem(this.source, this.path, previousHash);
    },
  },
};
</script>

<style>
.icon-download {
  font-size: 0.5em;
}

.item {
  -webkit-touch-callout: none; /* Disable the default long press preview */
}

.hiddenFile {
  opacity: 0.5;
}
.activebutton {
  height: 10em;
}

.activecontent {
  height: 5em !important;
  display: grid !important;
}

.activeimg {
  width: 8em !important;
  height: 8em !important;
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
