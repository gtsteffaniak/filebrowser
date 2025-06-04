<template>
  <a
    :href="getUrl()"
    :class="{
      item: true,
      'no-select': true,
      'listing-item': true,
      activebutton: isMaximized && isSelected,
      hiddenFile: isHiddenNotSelected && !this.isDraggedOver,
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
      <Icon :mimetype="type" :active="isSelected" :thumbnailUrl="thumbnailUrl" :filename="name" />
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
import { router } from "@/router";
import { url } from "@/utils";
import Icon from "@/components/files/Icon.vue";

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
    "url",
    "type",
    "size",
    "modified",
    "index",
    "readOnly",
    "path",
    "reducedOpacity",
  ],
  computed: {
    galleryView() {
      return state.user.viewMode === "gallery";
    },
    quickDownloadEnabled() {
      return state.user?.quickDownload;
    },
    isHiddenNotSelected() {
      return !this.isSelected && this.reducedOpacity;
    },
    getID() {
      return url.base64Encode(this.name);
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
        if (state.req.items[i].url === this.url) {
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
    isInView() {
      return enableThumbs;
    },
  },
  mounted() {
    // Prevent default navigation for left-clicks
    const observer = new IntersectionObserver(this.handleIntersect, {
      root: null,
      rootMargin: "0px",
      threshold: 0.5, // Adjust threshold as needed
    });

    // Get the thumbnail element and start observing
    const thumbnailElement = this.$refs.thumbnail; // Add ref="thumbnail" to the <img> tag
    if (thumbnailElement) {
      observer.observe(thumbnailElement);
    }
  },
  methods: {
    downloadFile(event) {
      event.preventDefault();
      event.stopPropagation();
      mutations.resetSelected();
      mutations.addSelected(this.index);
      downloadFiles();
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
      return this.url;
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
    dragStart() {
      if (getters.selectedCount() === 0) {
        mutations.addSelected(this.index);
        return;
      }

      if (!this.isSelected) {
        mutations.resetSelected();
        mutations.addSelected(this.index);
      }
    },
    dragOver(event) {
      if (!this.canDrop) return;
      event.preventDefault();
      this.isDraggedOver = true;
    },
    async drop(event) {
      if (!this.canDrop) return;
      event.preventDefault();

      if (getters.selectedCount() === 0) return;

      let el = event.target;
      for (let i = 0; i < 5; i++) {
        if (el !== null && !el.classList.contains("item")) {
          el = el.parentElement;
        }
      }

      let items = [];

      for (let i of this.selected) {
        items.push({
          from: state.req.items[i].url,
          to: this.url + encodeURIComponent(state.req.items[i].name),
          name: state.req.items[i].name,
        });
      }
      let response = await filesApi.fetchFiles(decodeURIComponent(el.__vue__.url));

      let action = async (overwrite, rename) => {
        await filesApi.moveCopy(items, "move", overwrite, rename);
        setTimeout(() => {
          mutations.setReload(true);
        }, 50);
      };
      let conflict = upload.checkConflict(items, response.items);

      let overwrite = false;
      let rename = false;

      if (conflict) {
        mutations.showHover({
          name: "replace-rename",
          confirm: (event, option) => {
            overwrite = option == "overwrite";
            rename = option == "rename";

            event.preventDefault();
            mutations.closeHovers();
            action(overwrite, rename);
          },
        });
        return;
      }

      action(overwrite, rename);
    },
    addSelected(event) {
      if (!state.isSafari) return;
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

      if (state.selected.indexOf(this.index) !== -1) {
        mutations.removeSelected(this.index);
        return;
      }
      if (event.shiftKey && this.selected.length > 0) {
        let fi = 0;
        let la = 0;

        if (this.index > this.selected[0]) {
          fi = this.selected[0] + 1;
          la = this.index;
        } else {
          fi = this.index;
          la = this.selected[0] - 1;
        }

        for (; fi <= la; fi++) {
          if (state.selected.indexOf(fi) == -1) {
            mutations.addSelected(fi);
          }
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
    },
    open() {
      location.hash = state.req.items[this.index].name;
      router.push({ path: this.url });
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
</style>
