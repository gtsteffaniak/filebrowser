<template>
  <div style="padding-bottom: 5em">
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
    <div v-else>
      <div v-if="numDirs + numFiles == 0">
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
          <div class="header" :class="{ 'dark-mode-item-header': isDarkMode }">
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
        <div v-if="numDirs > 0">
          <div class="header-items">
            <h2>{{ $t("files.folders") }}</h2>
          </div>
        </div>
        <div
          v-if="numDirs > 0"
          class="folder-items"
          :class="{ lastGroup: numFiles === 0 }"
        >
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
        <div v-if="numFiles > 0">
          <div class="header-items">
            <h2>{{ $t("files.files") }}</h2>
          </div>
        </div>
        <div v-if="numFiles > 0" class="file-items" :class="{ lastGroup: numFiles > 0 }">
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
          getMultiple
        />
        <input
          style="display: none"
          type="file"
          id="upload-folder-input"
          @change="uploadInput($event)"
          webkitdirectory
          getMultiple
        />

        <div :class="{ active: getMultiple }" id="multiple-selection">
          <p>{{ $t("files.multipleSelectionEnabled") }}</p>
          <div
            @click="this.setMultiple(false)"
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
    </div>
  </div>
</template>

<script>
import { files as api } from "@/api";
import * as upload from "@/utils/upload";
import css from "@/utils/css";
import throttle from "@/utils/throttle";
import { state, mutations, getters } from "@/store";
import { showError } from "@/notify";

import Item from "@/components/files/ListingItem.vue";
export default {
  name: "listingView",
  components: {
    Item,
  },
  data() {
    return {
      sortField: "name",
      columnWidth: 250 + state.user.gallerySize * 50,
      dragCounter: 0,
      width: window.innerWidth,
    };
  },
  watch: {
    gallerySize() {
      this.columnWidth = 250 + state.user.gallerySize * 50; // Update columnWidth based on new gallery size\
      this.colunmsResize();
    },
  },
  computed: {
    // Create a computed property that references the Vuex state
    gallerySize() {
      return state.user.gallerySize;
    },
    isDarkMode() {
      return state.user?.darkMode;
    },
    getMultiple() {
      return state.multiple;
    },
    nameSorted() {
      return state.req.sorting.by === "name";
    },
    sizeSorted() {
      return state.req.sorting.by === "size";
    },
    modifiedSorted() {
      return state.req.sorting.by === "modified";
    },
    ascOrdered() {
      return state.req.sorting.asc;
    },
    items() {
      return getters.reqItems();
    },
    numDirs() {
      return getters.reqNumDirs();
    },
    numFiles() {
      return getters.reqNumFiles();
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
      return icons[state.user.viewMode];
    },
    listingViewMode() {
      this.colunmsResize();
      return state.user.viewMode;
    },

    selectedCount() {
      return state.selected.length;
    },
    req() {
      return state.req;
    },
    loading() {
      return getters.isLoading();
    },
  },
  mounted() {
    // Check the columns size for the first time.
    this.colunmsResize();
    // Add the needed event listeners to the window and document.
    window.addEventListener("keydown", this.keyEvent);
    window.addEventListener("scroll", this.scrollEvent);
    window.addEventListener("resize", this.windowsResize);

    if (!state.user.perm?.create) return;
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

    if (state.user && !state.user.perm?.create) return;
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
      // Esc!
      if (event.keyCode === 27) {
        mutations.resetSelected();
      }

      // Del!
      if (event.keyCode === 46) {
        if (!state.user.perm.delete || state.selected.length === 0) return;
        mutations.showHover("delete");
      }

      // F2!
      if (event.keyCode === 113) {
        if (!state.user.perm.rename || state.selected.length !== 1) return;
        mutations.showHover("rename");
      }

      // Ctrl is pressed
      if (!event.ctrlKey && !event.metaKey) {
        return;
      }

      let key = String.fromCharCode(event.which).toLowerCase();

      switch (key) {
        case "f":
          event.preventDefault();
          mutations.showHover("search");
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

      let items = state.selected.map((i) => ({
        from: state.req.items[i].url,
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

      let items = this.clipboard.items.map((item) => ({
        from: item.from.endsWith("/") ? item.from.slice(0, -1) : item.from,
        to: state.route.path + encodeURIComponent(item.name),
        name: item.name,
      }));

      if (items.length === 0) {
        return;
      }
      mutations.setLoading("listing", true);
      let action = (overwrite, rename) => {
        api
          .copy(items, overwrite, rename)
          .then(() => {
            mutations.setLoading("listing", false);
          })
          .catch(showError);
      };

      if (this.clipboard.key === "x") {
        action = (overwrite, rename) => {
          api
            .move(items, overwrite, rename)
            .then(() => {
              this.clipboard = {};
              mutations.setLoading("listing", false);
            })
            .catch(showError);
        };
      }

      if (this.clipboard.path === state.route.path) {
        action(false, true);
        return;
      }

      const conflict = upload.checkConflict(items, state.req.items);

      if (conflict) {
        this.currentPrompt = {
          name: "replace-rename",
          confirm: (event, option) => {
            const overwrite = option === "overwrite";
            const rename = option === "rename";

            event.preventDefault();
            mutations.closeHovers();
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
      if (state.user.viewMode == "gallery") {
        items.style.height = `${this.columnWidth / 20}em`;
      } else {
        items.style.height = `auto`;
      }
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
      let items = state.req.items;
      let path = getters.getRoutePath();

      if (el !== null && el.classList.contains("item") && el.dataset.dir === "true") {
        path = el.__vue__.url;

        try {
          items = (await api.fetch(path)).items;
        } catch (error) {
          showError(error);
        }
      }

      const conflict = upload.checkConflict(files, items);

      if (conflict) {
        mutations.showHover({
          name: "replace",
          confirm: (event) => {
            event.preventDefault();
            mutations.closeHovers();
            upload.handleFiles(files, path, true);
          },
        });
        return;
      }

      upload.handleFiles(files, path);
    },
    uploadInput(event) {
      mutations.closeHovers();

      let files = event.currentTarget.files;
      let folder_upload =
        files[0].webkitRelativePath !== undefined && files[0].webkitRelativePath !== "";

      if (folder_upload) {
        for (let i = 0; i < files.length; i++) {
          files[i].fullPath = files[i].webkitRelativePath;
        }
      }

      let path = getters.getRoutePath();
      const conflict = upload.checkConflict(files, state.req.items);

      if (conflict) {
        mutations.showHover({
          name: "replace",
          confirm: (event) => {
            event.preventDefault();
            mutations.closeHovers();
            upload.handleFiles(files, path, true);
          },
        });
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

      // Commit the updateSort mutation
      mutations.updateListingSortConfig({ field, asc });
      mutations.updateListingItems();
    },
    setMultiple(val) {
      mutations.setMultiple(val == true);
    },
    openSearch() {
      this.currentPrompt = "search";
    },
    windowsResize: throttle(function () {
      this.colunmsResize();
      this.width = window.innerWidth;
      // Listing element is not displayed
      if (this.$refs.listingView == null) return;
    }, 100),
    upload() {
      if (
        typeof window.DataTransferItem !== "undefined" &&
        typeof DataTransferItem.prototype.webkitGetAsEntry !== "undefined"
      ) {
        mutations.closeHovers();
      } else {
        document.getElementById("upload-input").click();
      }
    },
  },
};
</script>

<style>
.dark-mode-item-header {
  border-color: var(--divider) !important;
  background: var(--surfacePrimary) !important;
}
.header-items {
  width: 100% !important;
  max-width: 100% !important;
  justify-content: center;
}
</style>
