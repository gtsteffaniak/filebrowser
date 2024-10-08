<template>
  <header>
    <action icon="close" :label="$t('buttons.close')" @action="close()" />
    <title v-if="isSettings" class="topTitle">Settings</title>
    <title v-else class="topTitle">{{ req.name }}</title>
  </header>
</template>

<style>
.flexbar {
  display: flex;
  flex-direction: block;
  justify-content: space-between;
}
</style>

<script>
import url from "@/utils/url";
import router from "@/router";
import { state, mutations, getters } from "@/store";
import { files as api } from "@/api";
import Action from "@/components/Action.vue";
import css from "@/utils/css";

export default {
  name: "listingView",
  components: {
    Action,
  },
  data() {
    return {
      columnWidth: 350,
      width: window.innerWidth,
      itemWeight: 0,
      viewModes: ["list", "compact", "normal", "gallery"],
    };
  },

  computed: {
    isSettings() {
      return getters.isSettings();
    },
    // Map state and getters
    req() {
      return state.req;
    },
    user() {
      return state.user;
    },
    selected() {
      return state.selected;
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
      return state.req.sorting.asc;
    },
    items() {
      const dirs = [];
      const files = [];

      state.req.items.forEach((item) => {
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
      return icons[state.user.viewMode];
    },
    headerButtons() {
      return {
        select: getters.selectedCount() > 0,
        upload: state.user.perm?.create && getters.selectedCount() > 0,
        download: state.user.perm?.download && getters.selectedCount() > 0,
        delete: getters.selectedCount() > 0 && state.user.perm.delete,
        rename: getters.selectedCount() === 1 && state.user.perm.rename,
        share: getters.selectedCount() === 1 && state.user.perm.share,
        move: getters.selectedCount() > 0 && state.user.perm.rename,
        copy: getters.selectedCount() > 0 && state.user.perm?.create,
      };
    },
  },

  mounted() {
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
    if (state.route.path.startsWith("/share")) {
      return;
    }
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
    fillWindow(fit = false) {
      const totalItems = state.req.numDirs + state.req.numFiles;

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
    setItemWeight() {
      // Listing element is not displayed
      if (this.$refs.listingView == null) return;

      let itemQuantity = state.req.numDirs + state.req.numFiles;
      if (itemQuantity > this.showLimit) itemQuantity = this.showLimit;

      // How much every listing item affects the window height
      this.itemWeight = this.$refs.listingView.offsetHeight / itemQuantity;
    },
    colunmsResize() {
      // Update the columns size based on the window width.
      let columns = Math.floor(
        document.querySelector("main").offsetWidth / this.columnWidth
      );
      let items = css(["#listingView .item", "#listingView .item"]);
      if (columns === 0) columns = 1;
      items.style.width = `calc(${100 / columns}%)`;
    },
    action() {
      if (this.show) {
        mutations.showHover(this.show);
      }

      this.$emit("action");
    },
    close() {
      if (getters.isSettings()) {
        // Use this.isSettings to access the computed property
        router.push({ path: "/files/", hash: "" });
        mutations.closeHovers();
        return;
      }
      mutations.replaceRequest({});
      let uri = url.removeLastDir(state.route.path) + "/";
      router.push({ path: uri });
      mutations.closeHovers();
    },
    base64(name) {
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
        mutations.resetSelected();
      }

      // Del!
      if (event.keyCode === 46) {
        if (!state.user.perm.delete || getters.selectedCount() == 0) return;

        // Show delete prompt.
        mutations.showHover("delete");
      }

      // F2!
      if (event.keyCode === 113) {
        if (!state.user.perm.rename || getters.selectedCount() !== 1) return;

        // Show rename prompt.
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
              this.addSelected(file.index);
            }
          }
          for (let dir of this.items.dirs) {
            if (state.selected.indexOf(dir.index) === -1) {
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
      mutations.closeHovers();
      const currentIndex = this.viewModes.indexOf(state.user.viewMode);
      const nextIndex = (currentIndex + 1) % this.viewModes.length;
      const newView = this.viewModes[nextIndex];
      mutations.updateCurrentUser({ viewMode: newView });
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

      for (let i of state.selected) {
        items.push({
          from: state.req.items[i].url,
          name: state.req.items[i].name,
        });
      }

      if (items.length == 0) {
        return;
      }
      mutations.updateClipboard({
        key: key,
        items: items,
        path: state.route.path,
      });
    },
    paste(event) {
      if (event.target.tagName.toLowerCase() === "input") {
        return;
      }

      let items = [];

      for (let item of state.clipboard.items) {
        const from = item.from.endsWith("/") ? item.from.slice(0, -1) : item.from;
        const to = state.route.path + encodeURIComponent(item.name);
        items.push({ from, to, name: item.name });
      }

      if (items.length === 0) {
        return;
      }

      let action = (overwrite, rename) => {
        const promises = [];

        items.forEach((item) => {
          promises.push(
            api.copy({
              from: item.from,
              to: item.to,
              name: item.name,
              overwrite: overwrite,
              rename: rename,
            })
          );
        });

        Promise.all(promises).then(() => {
          mutations.resetClipboard();
          mutations.resetSelected();
          this.$showMessage("success", "Copied successfully");
        });
      };

      this.$confirm(
        "Are you sure you want to copy these items?",
        "Copy",
        () => {
          action(false, false);
        },
        () => {
          action(true, false);
        },
        () => {
          action(true, true);
        }
      );
    },
  },
};
</script>
