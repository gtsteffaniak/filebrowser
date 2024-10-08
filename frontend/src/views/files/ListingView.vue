<template>
  <div style="padding-bottom: 35vh">
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
          />
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
</template>

<script>
import download from "@/utils/download";
import { files as api } from "@/api";
import { router } from "@/router";
import * as upload from "@/utils/upload";
import css from "@/utils/css";
import throttle from "@/utils/throttle";
import { state, mutations, getters } from "@/store";

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
      lastSelected: {}, // Add this to track the currently focused item
    };
  },
  watch: {
    gallerySize() {
      this.columnWidth = 250 + state.user.gallerySize * 50;
      this.colunmsResize();
    },
  },
  computed: {
    lastFolderIndex() {
      const allItems = [...this.items.dirs, ...this.items.files];
      for (let i = 0; i < allItems.length; i++) {
        if (!allItems[i].isDir) {
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
      let columns = Math.floor(
        document.querySelector("main").offsetWidth / this.columnWidth
      );
      if (columns === 0) columns = 1;
      return columns;
    },
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
      return state.user.sorting.by === "name";
    },
    sizeSorted() {
      return state.user.sorting.by === "size";
    },
    modifiedSorted() {
      return state.user.sorting.by === "modified";
    },
    ascOrdered() {
      return state.user.sorting.asc;
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
    this.lastSelected = state.selected;
    // Check the columns size for the first time.
    this.colunmsResize();
    // Add the needed event listeners to the window and document.
    window.addEventListener("keydown", this.keyEvent);
    window.addEventListener("scroll", this.scrollEvent);
    window.addEventListener("resize", this.windowsResize);
    this.$el.addEventListener("click", this.clickClear);
    this.$el.addEventListener("contextmenu", this.openContext);

    if (!state.user.perm?.create) return;
    this.$el.addEventListener("dragover", this.preventDefault);
    this.$el.addEventListener("dragenter", this.dragEnter);
    this.$el.addEventListener("dragleave", this.dragLeave);
    this.$el.addEventListener("drop", this.drop);
  },
  beforeUnmount() {
    // Remove event listeners before destroying this page.
    window.removeEventListener("keydown", this.keyEvent);
    window.removeEventListener("scroll", this.scrollEvent);
    window.removeEventListener("resize", this.windowsResize);
  },
  methods: {
    base64(name) {
      return window.btoa(unescape(encodeURIComponent(name)));
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
          const element = document.querySelector('.item[aria-selected="true"]');
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
    keyEvent(event) {
      const { key, ctrlKey, metaKey, which } = event;
      // Check if the key is alphanumeric
      const isAlphanumeric = /^[a-z0-9]$/i.test(key);
      const noModifierKeys = !ctrlKey && !metaKey;

      if (isAlphanumeric && noModifierKeys && getters.currentPromptName() == null) {
        this.alphanumericKeyPress(key); // Call the alphanumeric key press function
        return;
      }
      if (noModifierKeys && getters.currentPromptName() != null) {
        return;
      }
      // Handle the space bar key
      if (key === " ") {
        event.preventDefault();
        if (getters.currentPromptName() == "search") {
          mutations.closeHovers();
        } else {
          mutations.showHover("search");
        }
      }
      if (getters.currentPromptName() != null) {
        return;
      }
      let currentPath = state.route.path.replace(/\/+$/, ""); // Remove trailing slashes
      let newPath = currentPath.substring(0, currentPath.lastIndexOf("/"));
      // Handle key events using a switch statement
      switch (key) {
        case "Enter":
          if (this.selectedCount === 1) {
            router.push({ path: getters.getFirstSelected().url });
          }
          break;

        case "Backspace":
          // go back
          router.push({ path: newPath });
          break;

        case "Escape":
          mutations.resetSelected();
          break;

        case "Delete":
          if (!state.user.perm.delete || state.selected.length === 0) return;
          mutations.showHover("delete");
          break;

        case "F2":
          if (!state.user.perm.rename || state.selected.length !== 1) return;
          mutations.showHover("rename");
          break;

        case "ArrowUp":
        case "ArrowDown":
        case "ArrowLeft":
        case "ArrowRight":
          event.preventDefault();
          this.navigateKeboardArrows(key);
          break;

        default:
          // Handle keys with ctrl or meta keys
          if (!ctrlKey && !metaKey) return;
          break;
      }

      const charKey = String.fromCharCode(which).toLowerCase();

      switch (charKey) {
        case "c":
        case "x":
          this.copyCut(event, charKey);
          break;
        case "v":
          this.paste(event);
          break;
        case "a":
          event.preventDefault();
          this.selectAll();
          break;
        case "s":
          event.preventDefault();
          download();
          break;
      }
    },

    // Helper method to select all files and directories
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
        api.copy(items, overwrite, rename).then(() => {
          mutations.setLoading("listing", false);
        });
      };

      if (this.clipboard.key === "x") {
        action = (overwrite, rename) => {
          api.move(items, overwrite, rename).then(() => {
            this.clipboard = {};
            mutations.setLoading("listing", false);
          });
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
      let items = css(["#listingView .item", "#listingView .item"]);
      items.style.width = `calc(${100 / this.numColumns}% - 1em)`;
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
      const folderUpload = !!files[0].webkitRelativePath;

      const uploadFiles = [];
      for (let i = 0; i < files.length; i++) {
        const file = files[i];
        const fullPath = folderUpload ? file.webkitRelativePath : undefined;
        uploadFiles.push({
          file, // File object directly
          name: file.name,
          size: file.size,
          isDir: false,
          fullPath,
        });
      }
      let items = state.req.items;
      let path = getters.getRoutePath();

      if (el !== null && el.classList.contains("item") && el.dataset.dir === "true") {
        path = el.__vue__.url;

        items = (await api.fetch(path)).items;
      }

      const conflict = upload.checkConflict(uploadFiles, items);

      if (conflict) {
        mutations.showHover({
          name: "replace",
          confirm: async (event) => {
            event.preventDefault();
            mutations.closeHovers();
            await upload.handleFiles(uploadFiles, path, true);
          },
        });
      } else {
        await upload.handleFiles(uploadFiles, path);
      }
      mutations.setReload(true);
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
      showMultipleSelection();
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
    openContext(event) {
      event.preventDefault();
      mutations.showHover({
        name: "ContextMenu",
        props: {
          posX: event.clientX,
          posY: event.clientY,
        },
      });
    },
    clickClear() {
      const sameAsBefore = state.selected == this.lastSelected;
      if (sameAsBefore && !state.multiple) {
        mutations.resetSelected();
      }
      this.lastSelected = state.selected;
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
