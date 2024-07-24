<template>
  <div
    :class="{ activebutton: this.isMaximized && this.isSelected }"
    class="item"
    role="button"
    tabindex="0"
    :draggable="isDraggable"
    @dragstart="dragStart"
    @dragover="dragOver"
    @drop="drop"
    @click="itemClick"
    :data-dir="isDir"
    :data-type="type"
    :aria-label="name"
    :aria-selected="isSelected"
  >
    <div
      @click="toggleClick"
      :class="{ activetitle: this.isMaximized && this.isSelected }"
    >
      <img
        v-if="readOnly === undefined && type === 'image' && isThumbsEnabled && isInView"
        v-lazy="thumbnailUrl"
        :class="{ activeimg: this.isMaximized && this.isSelected }"
        ref="thumbnail"
      />
      <i
        :class="{ iconActive: this.isMaximized && this.isSelected }"
        v-else
        class="material-icons"
      ></i>
    </div>

    <div :class="{ activecontent: this.isMaximized && this.isSelected }">
      <p class="name">{{ name }}</p>

      <p v-if="isDir" class="size" data-order="-1">&mdash;</p>
      <p v-else class="size" :data-order="humanSize()">{{ humanSize() }}</p>

      <p class="modified">
        <time :datetime="modified">{{ humanTime() }}</time>
      </p>
    </div>
  </div>
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
import moment from "moment";
import { files as api } from "@/api";
import * as upload from "@/utils/upload";
import { state, getters, mutations } from "@/store"; // Import your custom store

export default {
  name: "item",
  data: function () {
    return {
      isThumbnailInView: false,
      isMaximized: false,
      touches: 0,
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
    user() {
      return state.user;
    },
    selected() {
      return state.selected;
    },
    req() {
      return state.req;
    },
    jwt() {
      return state.jwt;
    },
    selectedCount() {
      return getters.selectedCount();
    },
    isClicked() {
      if (this.user.singleClick || !this.allowedView) {
        return false;
      }
      return !this.isMaximized;
    },
    allowedView() {
      return this.user.viewMode != "gallery" && this.user.viewMode != "normal";
    },
    singleClick() {
      return this.readOnly == undefined && this.user.singleClick;
    },
    isSelected() {
      return this.selected.indexOf(this.index) !== -1;
    },
    isDraggable() {
      return this.readOnly == undefined && this.user.perm?.rename;
    },
    canDrop() {
      if (!this.isDir || this.readOnly !== undefined) return false;

      for (let i of this.selected) {
        if (this.req.items[i].url === this.url) {
          return false;
        }
      }

      return true;
    },
    thumbnailUrl() {
      let path = this.req.path;
      if (this.req.path == "/") {
        path = "";
      }
      const file = {
        path: path + "/" + this.name,
        modified: this.modified,
      };

      return api.getPreviewURL(file, "thumb");
    },
    isThumbsEnabled() {
      return enableThumbs;
    },
    isInView() {
      return enableThumbs;
    },
  },
  mounted() {
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
    humanSize: function () {
      return this.type == "invalid_link"
        ? "invalid link"
        : getHumanReadableFilesize(this.size);
    },
    humanTime: function () {
      if (this.readOnly == undefined && this.user.dateFormat) {
        return moment(this.modified).format("L LT");
      }
      return moment(this.modified).fromNow();
    },
    dragStart: function () {
      if (this.selectedCount === 0) {
        mutations.addSelected(this.index);
        return;
      }

      if (!this.isSelected) {
        mutations.resetSelected();
        mutations.addSelected(this.index);
      }
    },
    dragOver: function (event) {
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
    drop: async function (event) {
      if (!this.canDrop) return;
      event.preventDefault();

      if (this.selectedCount === 0) return;

      let el = event.target;
      for (let i = 0; i < 5; i++) {
        if (el !== null && !el.classList.contains("item")) {
          el = el.parentElement;
        }
      }

      let items = [];

      for (let i of this.selected) {
        items.push({
          from: this.req.items[i].url,
          to: this.url + encodeURIComponent(this.req.items[i].name),
          name: this.req.items[i].name,
        });
      }

      // Get url from ListingItem instance
      let path = el.__vue__.url;
      let baseItems = (await api.fetch(path)).items;

      let action = (overwrite, rename) => {
        api
          .move(items, overwrite, rename)
          .then(() => {
            mutations.setReload(true);
          })
          .catch(this.$showError);
      };

      let conflict = upload.checkConflict(items, baseItems);

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
    itemClick: function (event) {
      if (this.singleClick && !state.multiple) this.open();
      else this.click(event);
    },
    click: function (event) {
      if (!this.singleClick && this.selectedCount !== 0) event.preventDefault();

      setTimeout(() => {
        this.touches = 0;
      }, 300);

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
      if (!this.singleClick && !event.ctrlKey && !event.metaKey && !state.multiple)
        mutations.resetSelected();
      mutations.addSelected(this.index);
    },
    open: function () {
      this.$router.push({ path: this.url });
    },
  },
};
</script>
