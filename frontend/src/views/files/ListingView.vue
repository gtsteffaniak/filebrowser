<template>
  <div style="padding-bottom: 5em">
    <div v-if="selectedCount > 0" id="file-selection">
      <span>{{ selectedCount }} selected</span>
      <template>
        <action
          v-if="headerButtons.select"
          icon="info"
          :label="$t('buttons.info')"
          show="info"
        />
        <action
          v-if="headerButtons.select"
          icon="check_circle"
          :label="$t('buttons.selectMultiple')"
          @action="toggleMultipleSelection"
        />
        <action
          v-if="headerButtons.download"
          icon="file_download"
          :label="$t('buttons.download')"
          @action="download"
          :counter="selectedCount"
        />
        <action
          v-if="headerButtons.share"
          icon="share"
          :label="$t('buttons.share')"
          show="share"
        />
        <action
          v-if="headerButtons.rename"
          icon="mode_edit"
          :label="$t('buttons.rename')"
          show="rename"
        />
        <action
          v-if="headerButtons.copy"
          icon="content_copy"
          :label="$t('buttons.copyFile')"
          show="copy"
        />
        <action
          v-if="headerButtons.move"
          icon="forward"
          :label="$t('buttons.moveFile')"
          show="move"
        />
        <action
          v-if="headerButtons.delete"
          icon="delete"
          :label="$t('buttons.delete')"
          show="delete"
        />
      </template>
    </div>

    <div v-if="loading">
      <h2 class="message delayed">
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
        <span>{{ $t("files.loading") }}</span>
      </h2>
    </div>
    <template v-else>
      <div v-if="req.numDirs + req.numFiles == 0">
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
      <div
        v-else
        id="listingView"
        ref="listingView"
        :class="listingViewMode + ' file-icons'"
      >
        <div>
          <div class="item header">
            <div></div>
            <div>
              <p
                :class="{ active: nameSorted }"
                class="name"
                role="button"
                tabindex="0"
                @click="sort('name')"
                :title="$t('files.sortByName')"
                :aria-label="$t('files.sortByName')"
              >
                <span>{{ $t("files.name") }}</span>
                <i class="material-icons">{{ nameIcon }}</i>
              </p>

              <p
                :class="{ active: sizeSorted }"
                class="size"
                role="button"
                tabindex="0"
                @click="sort('size')"
                :title="$t('files.sortBySize')"
                :aria-label="$t('files.sortBySize')"
              >
                <span>{{ $t("files.size") }}</span>
                <i class="material-icons">{{ sizeIcon }}</i>
              </p>
              <p
                :class="{ active: modifiedSorted }"
                class="modified"
                role="button"
                tabindex="0"
                @click="sort('modified')"
                :title="$t('files.sortByLastModified')"
                :aria-label="$t('files.sortByLastModified')"
              >
                <span>{{ $t("files.lastModified") }}</span>
                <i class="material-icons">{{ modifiedIcon }}</i>
              </p>
            </div>
          </div>
        </div>
        <div v-if="req.numDirs > 0">
          <div class="header-items">
            <h2>{{ $t("files.folders") }}</h2>
          </div>
        </div>
        <div v-if="req.numDirs > 0">
          <item
            v-for="item in dirs"
            :key="base64(item.name)"
            v-bind:index="item.index"
            v-bind:name="item.name"
            v-bind:isDir="item.isDir"
            v-bind:url="item.url"
            v-bind:modified="item.modified"
            v-bind:type="item.type"
            v-bind:size="item.size"
            v-bind:path="item.path"
          >
          </item>
        </div>

        <div v-if="req.numFiles > 0">
          <div class="header-items">
            <h2>{{ $t("files.files") }}</h2>
          </div>
        </div>
        <div v-if="req.numFiles > 0">
          <item
            v-for="item in files"
            :key="base64(item.name)"
            v-bind:index="item.index"
            v-bind:name="item.name"
            v-bind:isDir="item.isDir"
            v-bind:url="item.url"
            v-bind:modified="item.modified"
            v-bind:type="item.type"
            v-bind:size="item.size"
            v-bind:path="item.path"
          >
          </item>
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

        <div :class="{ active: $store.state.multiple }" id="multiple-selection">
          <p>{{ $t("files.multipleSelectionEnabled") }}</p>
          <div
            @click="$store.commit('multiple', false)"
            tabindex="0"
            role="button"
            :title="$t('files.clear')"
            :aria-label="$t('files.clear')"
            class="action"
          >
            <i class="material-icons">clear</i>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style>
.header-items {
  width: 100% !important;
  max-width: 100% !important;
  justify-content: center;
}
</style>
<script>
import { files as api } from "@/api";
import * as upload from "@/utils/upload";
import css from "@/utils/css";
import throttle from "@/utils/throttle";

import Action from "@/components/header/Action";
import Item from "@/components/files/ListingItem.vue";

export default {
  name: "listingView",
  components: {
    Action,
    Item,
  },
  data() {
    return {
      sortField: "name",
      columnWidth: 280,
      dragCounter: 0,
      width: window.innerWidth,
      req: {}, // Replace with your actual initial state
      selected: [], // Replace with your actual initial state
      user: {}, // Replace with your actual initial state
      multiple: false,
      loading: false,
      clipboard: {},
      showLimit: 0,
      itemWeight: 0,
      currentPrompt: null,
    };
  },
  computed: {
    nameSorted() {
      return this.req.sorting.by === "name";
    },
    sizeSorted() {
      return this.req.sorting.by === "size";
    },
    modifiedSorted() {
      return this.req.sorting.by === "modified";
    },
    ascOrdered() {
      return this.req.sorting.asc;
    },
    items() {
      const dirs = [];
      const files = [];

      this.req.items.forEach((item) => {
        if (this.user.hideDotfiles && item.name.startsWith(".")) {
          return;
        }
        if (item.isDir) {
          dirs.push(item);
        } else {
          item.Path = this.req.Path;
          files.push(item);
        }
      });
      return { dirs, files };
    },
    dirs() {
      return this.items.dirs;
    },
    files() {
      return this.items.files;
    },
    nameIcon() {
      if (this.nameSorted && !this.ascOrdered) {
        return "arrow_upward";
      }

      return "arrow_downward";
    },
    sizeIcon() {
      if (this.sizeSorted && this.ascOrdered) {
        return "arrow_downward";
      }

      return "arrow_upward";
    },
    modifiedIcon() {
      if (this.modifiedSorted && this.ascOrdered) {
        return "arrow_downward";
      }

      return "arrow_upward";
    },
    viewIcon() {
      const icons = {
        list: "view_module",
        compact: "view_module",
        normal: "grid_view",
        gallery: "view_list",
      };
      return icons[this.user.viewMode];
    },
    listingViewMode() {
      return this.user.viewMode;
    },
    headerButtons() {
      return {
        select: this.selected.length > 0,
        upload: this.user.perm.create && this.selected.length > 0,
        download: this.user.perm.download && this.selected.length > 0,
        delete: this.selected.length > 0 && this.user.perm.delete,
        rename: this.selected.length === 1 && this.user.perm.rename,
        share: this.selected.length === 1 && this.user.perm.share,
        move: this.selected.length > 0 && this.user.perm.rename,
        copy: this.selected.length > 0 && this.user.perm.create,
      };
    },
  },
  watch: {
    req() {
      // Ensures that the listing is displayed
    },
  },
  mounted() {
    // Check the columns size for the first time.
    this.colunmsResize();

    // Add the needed event listeners to the window and document.
    window.addEventListener("keydown", this.keyEvent);
    window.addEventListener("scroll", this.scrollEvent);
    window.addEventListener("resize", this.windowsResize);

    if (!this.user.perm.create) return;
    document.addEventListener("dragover", this.preventDefault);
    document.addEventListener("dragenter", this.dragEnter);
    document.addEventListener("dragleave", this.dragLeave);
    document.addEventListener("drop", this.drop);
  },
  beforeUnmount() {
    // Remove event listeners before destroying this page.
    window.removeEventListener("keydown", this.keyEvent);
    window.removeEventListener("scroll", this.scrollEvent);
    window.removeEventListener("resize", this.windowsResize);

    if (this.user && !this.user.perm.create) return;
    document.removeEventListener("dragover", this.preventDefault);
    document.removeEventListener("dragenter", this.dragEnter);
    document.removeEventListener("dragleave", this.dragLeave);
    document.removeEventListener("drop", this.drop);
  },
  methods: {
    base64(name) {
      return window.btoa(unescape(encodeURIComponent(name)));
    },
    keyEvent(event) {
      // No prompts are shown
      if (this.currentPrompt !== null) {
        return;
      }

      // Esc!
      if (event.keyCode === 27) {
        // Reset files selection.
        this.selected = [];
      }

      // Del!
      if (event.keyCode === 46) {
        if (!this.user.perm.delete || this.selected.length === 0) return;

        // Show delete prompt.
        this.currentPrompt = "delete";
      }

      // F2!
      if (event.keyCode === 113) {
        if (!this.user.perm.rename || this.selected.length !== 1) return;

        // Show rename prompt.
        this.currentPrompt = "rename";
      }

      // Ctrl is pressed
      if (!event.ctrlKey && !event.metaKey) {
        return;
      }

      let key = String.fromCharCode(event.which).toLowerCase();

      switch (key) {
        case "f":
          event.preventDefault();
          this.currentPrompt = "search";
          break;
        case "c":
        case "x":
          this.copyCut(event, key);
          break;
        case "v":
          this.paste(event);
          break;
        case "a":
          event.preventDefault();
          this.selectAllItems();
          break;
        case "s":
          event.preventDefault();
          document.getElementById("download-button").click();
          break;
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

      let items = this.selected.map((i) => ({
        from: this.req.items[i].url,
        name: this.req.items[i].name,
      }));

      if (items.length === 0) {
        return;
      }

      this.clipboard = {
        key: key,
        items: items,
        path: this.$route.path,
      };
    },
    async paste(event) {
      if (event.target.tagName.toLowerCase() === "input") {
        return;
      }

      let items = this.clipboard.items.map((item) => ({
        from: item.from.endsWith("/") ? item.from.slice(0, -1) : item.from,
        to: this.$route.path + encodeURIComponent(item.name),
        name: item.name,
      }));

      if (items.length === 0) {
        return;
      }

      let action = (overwrite, rename) => {
        api
          .copy(items, overwrite, rename)
          .then(() => {
            this.loading = true;
          })
          .catch(this.$showError);
      };

      if (this.clipboard.key === "x") {
        action = (overwrite, rename) => {
          api
            .move(items, overwrite, rename)
            .then(() => {
              this.clipboard = {};
              this.loading = true;
            })
            .catch(this.$showError);
        };
      }

      if (this.clipboard.path === this.$route.path) {
        action(false, true);
        return;
      }

      const conflict = upload.checkConflict(items, this.req.items);

      if (conflict) {
        this.currentPrompt = {
          prompt: "replace-rename",
          confirm: (event, option) => {
            const overwrite = option === "overwrite";
            const rename = option === "rename";

            event.preventDefault();
            this.currentPrompt = null;
            action(overwrite, rename);
          },
        };
        return;
      }

      action(false, false);
    },
    colunmsResize() {
      let columns = Math.floor(
        document.querySelector("main").offsetWidth / this.columnWidth
      );
      let items = css(["#listingView .item", "#listingView .item"]);
      if (columns === 0) columns = 1;
      items.style.width = `calc(${100 / columns}% - 1em)`;
    },
    dragEnter() {
      this.dragCounter++;
      let items = document.getElementsByClassName("item");

      Array.from(items).forEach((file) => {
        file.style.opacity = 0.5;
      });
    },
    dragLeave() {
      this.dragCounter--;
      if (this.dragCounter === 0) {
        this.resetOpacity();
      }
    },
    async drop(event) {
      event.preventDefault();
      this.dragCounter = 0;
      this.resetOpacity();

      let dt = event.dataTransfer;
      let el = event.target;

      if (dt.files.length <= 0) return;

      for (let i = 0; i < 5; i++) {
        if (el !== null && !el.classList.contains("item")) {
          el = el.parentElement;
        }
      }

      let files = await upload.scanFiles(dt);
      let items = this.req.items;
      let path = this.$route.path.endsWith("/")
        ? this.$route.path
        : this.$route.path + "/";

      if (el !== null && el.classList.contains("item") && el.dataset.dir === "true") {
        path = el.__vue__.url;

        try {
          items = (await api.fetch(path)).items;
        } catch (error) {
          this.$showError(error);
        }
      }

      const conflict = upload.checkConflict(files, items);

      if (conflict) {
        this.currentPrompt = {
          prompt: "replace",
          confirm: (event) => {
            event.preventDefault();
            this.currentPrompt = null;
            upload.handleFiles(files, path, true);
          },
        };
        return;
      }

      upload.handleFiles(files, path);
    },
    uploadInput(event) {
      this.currentPrompt = null;

      let files = event.currentTarget.files;
      let folder_upload =
        files[0].webkitRelativePath !== undefined && files[0].webkitRelativePath !== "";

      if (folder_upload) {
        for (let i = 0; i < files.length; i++) {
          files[i].fullPath = files[i].webkitRelativePath;
        }
      }

      let path = this.$route.path.endsWith("/")
        ? this.$route.path
        : this.$route.path + "/";
      const conflict = upload.checkConflict(files, this.req.items);

      if (conflict) {
        this.currentPrompt = {
          prompt: "replace",
          confirm: (event) => {
            event.preventDefault();
            this.currentPrompt = null;
            upload.handleFiles(files, path, true);
          },
        };
        return;
      }

      upload.handleFiles(files, path);
    },
    resetOpacity() {
      let items = document.getElementsByClassName("item");
      Array.from(items).forEach((file) => {
        file.style.opacity = 1;
      });
    },
    sort(field) {
      let asc = false;
      if (
        (field === "name" && this.nameIcon === "arrow_upward") ||
        (field === "size" && this.sizeIcon === "arrow_upward") ||
        (field === "modified" && this.modifiedIcon === "arrow_upward")
      ) {
        asc = true;
      }

      // Directly update the sort configuration
      this.req.sorting.by = field;
      this.req.sorting.asc = asc;
      this.updateListingItems();
    },
    updateListingItems() {
      // Call your API or method to update listing items
    },
    selectAllItems() {
      this.selected = [
        ...this.items.files.map((file) => file.index),
        ...this.items.dirs.map((dir) => dir.index),
      ];
    },
    openSearch() {
      this.currentPrompt = "search";
    },
    toggleMultipleSelection() {
      this.multiple = !this.multiple;
      this.currentPrompt = null;
    },
    windowsResize: throttle(function () {
      this.colunmsResize();
      this.width = window.innerWidth;
      // Listing element is not displayed
      if (this.$refs.listingView == null) return;
    }, 100),
    download() {
      if (this.selected.length === 1 && !this.req.items[this.selected[0]].isDir) {
        api.download(null, this.req.items[this.selected[0]].url);
        return;
      }

      this.currentPrompt = {
        prompt: "download",
        confirm: (format) => {
          this.currentPrompt = null;
          let files = [];
          if (this.selected.length > 0) {
            for (let i of this.selected) {
              files.push(this.req.items[i].url);
            }
          } else {
            files.push(this.$route.path);
          }

          api.download(format, ...files);
        },
      };
    },
    upload() {
      if (
        typeof window.DataTransferItem !== "undefined" &&
        typeof DataTransferItem.prototype.webkitGetAsEntry !== "undefined"
      ) {
        this.currentPrompt = "upload";
      } else {
        document.getElementById("upload-input").click();
      }
    },
  },
};
</script>
