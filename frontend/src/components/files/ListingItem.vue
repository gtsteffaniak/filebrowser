<template>
  <a
    :href="getUrl()"
    :class="{
      item: true,
      'no-select': true,
      activebutton: isMaximized && isSelected,
    }"
    :id="getID"
    role="button"
    tabindex="0"
    :draggable="isDraggable"
    @dragstart="dragStart"
    @dragover="dragOver"
    @drop="drop"
    :data-dir="isDir"
    :data-type="type"
    :aria-label="name"
    :aria-selected="isSelected"
    @contextmenu="onRightClick($event)"
    @click="click($event)"
    @touchstart="addSelected($event)"
    @touchmove="handleTouchMove($event)"
    @touchend="cancelContext($event)"
    @mouseup="cancelContext($event)"
  >
    <div @click="toggleClick" :class="{ activetitle: isMaximized && isSelected }">
      <img
        v-if="
          readOnly === undefined &&
          type.startsWith('image') &&
          isThumbsEnabled &&
          isInView
        "
        v-lazy="thumbnailUrl"
        :class="{ activeimg: isMaximized && isSelected }"
        ref="thumbnail"
      />
      <Icon v-else :mimetype="type" :active="isSelected" />
    </div>

    <div class="text" :class="{ activecontent: isMaximized && isSelected }">
      <p class="name">{{ name }}</p>
      <p class="size" :data-order="humanSize()">{{ humanSize() }}</p>
      <p class="modified">
        <time :datetime="modified">{{ humanTime() }}</time>
      </p>
    </div>
  </a>
</template>

<style>
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

<script>
import { enableThumbs } from "@/utils/constants";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { fromNow } from "@/utils/moment";
import { filesApi } from "@/api";
import * as upload from "@/utils/upload";
import { state, getters, mutations } from "@/store"; // Import your custom store
import { baseURL } from "@/utils/constants";
import { router } from "@/router";
import { url } from "@/utils";
import Icon from "@/components/Icon.vue";

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
  ],
  computed: {
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
      return this.readOnly == undefined && state.user.perm?.rename;
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
      let path = state.req.path;
      if (state.req.path == "/") {
        path = "";
      }
      return filesApi.getPreviewURL(path + "/" + this.name, "small", state.req.modified);
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
    handleTouchMove(event) {
      if (!state.isSafari) return
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
      if (!state.isSafari) return
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
      return baseURL.slice(0, -1) + this.url;
    },
    onRightClick(event) {
      event.preventDefault(); // Prevent default context menu
      // If no items are selected, select the right-clicked item
      if (!state.multiple) {
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
    humanTime() {
      if (this.readOnly == undefined && state.user.dateFormat) {
        return fromNow(this.modified, state.user.locale).format("L LT");
      }
      return fromNow(this.modified, state.user.locale);
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
      let el = event.target;

      for (let i = 0; i < 5; i++) {
        if (!el.classList.contains("item")) {
          el = el.parentElement;
        }
      }

      el.style.opacity = 1;
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
      let response = await filesApi.fetchFiles(el.__vue__.url);

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
      if (!state.isSafari) return
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

      if (!state.user.singleClick && getters.selectedCount() !== 0 && event.button === 0) {
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
      if (!state.user.singleClick && !event.ctrlKey && !event.metaKey && !state.multiple) {
        mutations.resetSelected();
      }
      mutations.addSelected(this.index);
    },
    open() {
      location.hash = state.req.items[this.index].name;
      const newurl = url.removePrefix(this.url);
      router.push({ path: newurl });
    },
  },
};
</script>

<style>
.item {
  -webkit-touch-callout: none; /* Disable the default long press preview */
}
</style>