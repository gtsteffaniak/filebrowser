<template>
  <header-bar>
    <action
      class="menu-button"
      icon="menu"
      :label="$t('buttons.toggleSidebar')"
      @action="toggleSidebar()"
    />
    <search />
    <action
      class="menu-button"
      icon="grid_view"
      :label="$t('buttons.switchView')"
      @action="switchView"
    />
  </header-bar>
</template>

<style>
.flexbar {
  display:flex;
  flex-direction:block;
  justify-content: space-between;
}
</style>

<script>
import Vue from "vue";
import { mapState, mapGetters, mapMutations } from "vuex";
import { users, files as api } from "@/api";
import HeaderBar from "@/components/header/HeaderBar.vue";
import Action from "@/components/header/Action.vue";
import * as upload from "@/utils/upload";
import css from "@/utils/css";
import throttle from "lodash.throttle";
import Search from "@/components/Search.vue";


export default {
  name: "listing",
  components: {
    HeaderBar,
    Action,
    Search,
  },
  data: function () {
    return {
      showLimit: 50,
      columnWidth: 280,
      dragCounter: 0,
      width: window.innerWidth,
      itemWeight: 0,
      viewModes: ['list', 'compact', 'normal', 'gallery'],
    };
  },
  computed: {
    ...mapState(["req", "selected", "user", "show", "multiple", "selected", "loading"]),
    ...mapGetters(["selectedCount"]),
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
        if (item.isDir) {
          dirs.push(item);
        } else {
          files.push(item);
        }
      });

      return { dirs, files };
    },
    dirs() {
      return this.items.dirs.slice(0, this.showLimit);
    },
    files() {
      let showLimit = this.showLimit - this.items.dirs.length;

      if (showLimit < 0) showLimit = 0;

      return this.items.files.slice(0, showLimit);
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
    headerButtons() {
      return {
        select: this.selectedCount > 0,
        upload: this.user.perm.create && this.selectedCount > 0,
        download: this.user.perm.download && this.selectedCount > 0,
        delete: this.selectedCount > 0 && this.user.perm.delete,
        rename: this.selectedCount === 1 && this.user.perm.rename,
        share: this.selectedCount === 1 && this.user.perm.share,
        move: this.selectedCount > 0 && this.user.perm.rename,
        copy: this.selectedCount > 0 && this.user.perm.create,
      };
    },
  },
  watch: {
    req: function () {
      // Reset the show value
      this.showLimit = 50;

      // Ensures that the listing is displayed
      Vue.nextTick(() => {
        // How much every listing item affects the window height
        this.setItemWeight();

        // Fill and fit the window with listing items
        this.fillWindow(true);
      });
    },
  },
  mounted: function () {
    // Check the columns size for the first time.
    this.colunmsResize();

    // How much every listing item affects the window height
    this.setItemWeight();

    // Fill and fit the window with listing items
    this.fillWindow(true);

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
    action: function () {
      if (this.show) {
        this.$store.commit("showHover", this.show);
      }

      this.$emit("action");
    },
    toggleSidebar() {
      if (this.$store.state.show == "sidebar") {
        this.$store.commit("closeHovers");
      } else {
        this.$store.commit("showHover", "sidebar");
      }
    },
    ...mapMutations(["updateUser", "addSelected"]),
    base64: function (name) {
      return window.btoa(unescape(encodeURIComponent(name)));
    },
    keyEvent(event) {
      // No prompts are shown
      if (this.show !== null) {
        return;
      }

      // Esc!
      if (event.keyCode === 27) {
        // Reset files selection.
        this.$store.commit("resetSelected");
      }

      // Del!
      if (event.keyCode === 46) {
        if (!this.user.perm.delete || this.selectedCount == 0) return;

        // Show delete prompt.
        this.$store.commit("showHover", "delete");
      }

      // F2!
      if (event.keyCode === 113) {
        if (!this.user.perm.rename || this.selectedCount !== 1) return;

        // Show rename prompt.
        this.$store.commit("showHover", "rename");
      }

      // Ctrl is pressed
      if (!event.ctrlKey && !event.metaKey) {
        return;
      }

      let key = String.fromCharCode(event.which).toLowerCase();

      switch (key) {
        case "f":
          event.preventDefault();
          this.$store.commit("showHover", "search");
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
            if (this.$store.state.selected.indexOf(file.index) === -1) {
              this.addSelected(file.index);
            }
          }
          for (let dir of this.items.dirs) {
            if (this.$store.state.selected.indexOf(dir.index) === -1) {
              this.addSelected(dir.index);
            }
          }
          break;
        case "s":
          event.preventDefault();
          document.getElementById("download-button").click();
          break;
      }
    },
    switchView: async function () {
      console.log(this.user.viewMode)
      this.$store.commit("closeHovers");
      const currentIndex = this.viewModes.indexOf(this.user.viewMode);
      const nextIndex = (currentIndex + 1) % this.viewModes.length;
      const data = {
        id: this.user.id,
        viewMode: this.viewModes[nextIndex],
      };
      this.$store.commit("updateUser", data);
      console.log(data)
    },
    preventDefault(event) {
      // Wrapper around prevent default.
      event.preventDefault();
    },
    copyCut(event, key) {
      if (event.target.tagName.toLowerCase() === "input") {
        return;
      }

      let items = [];

      for (let i of this.selected) {
        items.push({
          from: this.req.items[i].url,
          name: this.req.items[i].name,
        });
      }

      if (items.length == 0) {
        return;
      }

      this.$store.commit("updateClipboard", {
        key: key,
        items: items,
        path: this.$route.path,
      });
    },
    paste(event) {
      if (event.target.tagName.toLowerCase() === "input") {
        return;
      }

      let items = [];

      for (let item of this.$store.state.clipboard.items) {
        const from = item.from.endsWith("/") ? item.from.slice(0, -1) : item.from;
        const to = this.$route.path + encodeURIComponent(item.name);
        items.push({ from, to, name: item.name });
      }

      if (items.length === 0) {
        return;
      }

      let action = (overwrite, rename) => {
        api
          .copy(items, overwrite, rename)
          .then(() => {
            this.$store.commit("setReload", true);
          })
          .catch(this.$showError);
      };

      if (this.$store.state.clipboard.key === "x") {
        action = (overwrite, rename) => {
          api
            .move(items, overwrite, rename)
            .then(() => {
              this.$store.commit("resetClipboard");
              this.$store.commit("setReload", true);
            })
            .catch(this.$showError);
        };
      }

      if (this.$store.state.clipboard.path == this.$route.path) {
        action(false, true);

        return;
      }

      let conflict = upload.checkConflict(items, this.req.items);

      let overwrite = false;
      let rename = false;

      if (conflict) {
        this.$store.commit("showHover", {
          prompt: "replace-rename",
          confirm: (event, option) => {
            overwrite = option == "overwrite";
            rename = option == "rename";

            event.preventDefault();
            this.$store.commit("closeHovers");
            action(overwrite, rename);
          },
        });

        return;
      }

      action(overwrite, rename);
    },
    colunmsResize() {
      // Update the columns size based on the window width.
      let columns = Math.floor(
        document.querySelector("main").offsetWidth / this.columnWidth
      );
      let items = css(["#listing .item", "#listing .item"]);
      if (columns === 0) columns = 1;
      items.style.width = `calc(${100 / columns}% - 1em)`;
    },
    scrollEvent: throttle(function () {
      const totalItems = this.req.numDirs + this.req.numFiles;

      // All items are displayed
      if (this.showLimit >= totalItems) return;

      const currentPos = window.innerHeight + window.scrollY;

      // Trigger at the 75% of the window height
      const triggerPos = document.body.offsetHeight - window.innerHeight * 0.25;

      if (currentPos > triggerPos) {
        // Quantity of items needed to fill 2x of the window height
        const showQuantity = Math.ceil((window.innerHeight * 2) / this.itemWeight);

        // Increase the number of displayed items
        this.showLimit += showQuantity;
      }
    }, 100),
    dragEnter() {
      this.dragCounter++;

      // When the user starts dragging an item, put every
      // file on the listing with 50% opacity.
      let items = document.getElementsByClassName("item");

      Array.from(items).forEach((file) => {
        file.style.opacity = 0.5;
      });
    },
    dragLeave() {
      this.dragCounter--;

      if (this.dragCounter == 0) {
        this.resetOpacity();
      }
    },
    drop: async function (event) {
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
        // Get url from ListingItem instance
        path = el.__vue__.url;

        try {
          items = (await api.fetch(path)).items;
        } catch (error) {
          this.$showError(error);
        }
      }

      let conflict = upload.checkConflict(files, items);

      if (conflict) {
        this.$store.commit("showHover", {
          prompt: "replace",
          confirm: (event) => {
            event.preventDefault();
            this.$store.commit("closeHovers");
            upload.handleFiles(files, path, true);
          },
        });

        return;
      }

      upload.handleFiles(files, path);
    },
    uploadInput(event) {
      this.$store.commit("closeHovers");

      let files = event.currentTarget.files;
      let folder_upload =
        files[0].webkitRelativePath !== undefined && files[0].webkitRelativePath !== "";

      if (folder_upload) {
        for (let i = 0; i < files.length; i++) {
          let file = files[i];
          files[i].fullPath = file.webkitRelativePath;
        }
      }

      let path = this.$route.path.endsWith("/")
        ? this.$route.path
        : this.$route.path + "/";
      let conflict = upload.checkConflict(files, this.req.items);

      if (conflict) {
        this.$store.commit("showHover", {
          prompt: "replace",
          confirm: (event) => {
            event.preventDefault();
            this.$store.commit("closeHovers");
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
    async sort(by) {
      let asc = false;

      if (by === "name") {
        if (this.nameIcon === "arrow_upward") {
          asc = true;
        }
      } else if (by === "size") {
        if (this.sizeIcon === "arrow_upward") {
          asc = true;
        }
      } else if (by === "modified") {
        if (this.modifiedIcon === "arrow_upward") {
          asc = true;
        }
      }

      try {
        await users.update({ id: this.user.id, sorting: { by, asc } }, ["sorting"]);
      } catch (e) {
        this.$showError(e);
      }

      this.$store.commit("setReload", true);
    },
    openSearch() {
      this.$store.commit("showHover", "search");
    },
    toggleMultipleSelection() {
      this.$store.commit("multiple", !this.multiple);
      this.$store.commit("closeHovers");
    },
    windowsResize: throttle(function () {
      this.colunmsResize();
      this.width = window.innerWidth;

      // Listing element is not displayed
      if (this.$refs.listing == null) return;

      // How much every listing item affects the window height
      this.setItemWeight();

      // Fill but not fit the window
      this.fillWindow();
    }, 100),
    download() {
      if (this.selectedCount === 1 && !this.req.items[this.selected[0]].isDir) {
        api.download(null, this.req.items[this.selected[0]].url);
        return;
      }

      this.$store.commit("showHover", {
        prompt: "download",
        confirm: (format) => {
          this.$store.commit("closeHovers");
          let files = [];
          if (this.selectedCount > 0) {
            for (let i of this.selected) {
              files.push(this.req.items[i].url);
            }
          } else {
            files.push(this.$route.path);
          }

          api.download(format, ...files);
        },
      });
    },

    upload: function () {
      if (
        typeof window.DataTransferItem !== "undefined" &&
        typeof DataTransferItem.prototype.webkitGetAsEntry !== "undefined"
      ) {
        this.$store.commit("showHover", "upload");
      } else {
        document.getElementById("upload-input").click();
      }
    },
    setItemWeight() {
      // Listing element is not displayed
      if (this.$refs.listing == null) return;

      let itemQuantity = this.req.numDirs + this.req.numFiles;
      if (itemQuantity > this.showLimit) itemQuantity = this.showLimit;

      // How much every listing item affects the window height
      this.itemWeight = this.$refs.listing.offsetHeight / itemQuantity;
    },
    fillWindow(fit = false) {
      const totalItems = this.req.numDirs + this.req.numFiles;

      // More items are displayed than the total
      if (this.showLimit >= totalItems && !fit) return;

      const windowHeight = window.innerHeight;

      // Quantity of items needed to fill 2x of the window height
      const showQuantity = Math.ceil((windowHeight + windowHeight * 2) / this.itemWeight);

      // Less items to display than current
      if (this.showLimit > showQuantity && !fit) return;

      // Set the number of displayed items
      this.showLimit = showQuantity > totalItems ? totalItems : showQuantity;
    },
  },
};
</script>
