<template>
  <header>
    <action icon="close" :label="$t('buttons.close')" @action="close()" />
    <title class="topTitle">{{ req.name }}</title>
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
import Vue from "vue";
import Action from "@/components/header/Action.vue";
import { users, files as api } from "@/api";
import url from "@/utils/url";
import * as upload from "@/utils/upload";
import css from "@/utils/css";
import throttle from "@/utils/throttle";
import { state, getters, mutations } from "@/store"; // Import your custom store

export default {
  name: "listingView",
  components: {
    Action,
  },
  data() {
    return {
      showLimit: 50,
      columnWidth: 280,
      dragCounter: 0,
      width: window.innerWidth,
      itemWeight: 0,
      viewModes: ["list", "compact", "normal", "gallery"],
    };
  },

  computed: {
    isSettings() {
      return this.$route.path.includes("/settings/");
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
        select: getters.selectedCount > 0,
        upload: state.user.perm.create && getters.selectedCount > 0,
        download: state.user.perm.download && getters.selectedCount > 0,
        delete: getters.selectedCount > 0 && state.user.perm.delete,
        rename: getters.selectedCount === 1 && state.user.perm.rename,
        share: getters.selectedCount === 1 && state.user.perm.share,
        move: getters.selectedCount > 0 && state.user.perm.rename,
        copy: getters.selectedCount > 0 && state.user.perm.create,
      };
    },
  },
  watch: {
    req() {
      Vue.nextTick(() => {
        this.setItemWeight();
        this.fillWindow(true);
      });
    },
  },
  mounted() {
    this.colunmsResize();
    this.setItemWeight();
    this.fillWindow(true);

    window.addEventListener("keydown", this.keyEvent);
    window.addEventListener("scroll", this.scrollEvent);
    window.addEventListener("resize", this.windowsResize);

    if (!this.$route.path.startsWith("/share") && state.user.perm.create) {
      document.addEventListener("dragover", this.preventDefault);
      document.addEventListener("dragenter", this.dragEnter);
      document.addEventListener("dragleave", this.dragLeave);
      document.addEventListener("drop", this.drop);
    }
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
    window.removeEventListener("scroll", this.scrollEvent);
    window.removeEventListener("resize", this.windowsResize);

    if (state.user.perm.create) {
      document.removeEventListener("dragover", this.preventDefault);
      document.removeEventListener("dragenter", this.dragEnter);
      document.removeEventListener("dragleave", this.dragLeave);
      document.removeEventListener("drop", this.drop);
    }
  },
  methods: {
    action() {
      if (state.show) {
        mutations.showHover(state.show);
      }
      this.$emit("action");
    },
    toggleSidebar() {
      if (state.show === "sidebar") {
        mutations.closeHovers();
      } else {
        mutations.showHover("sidebar");
      }
    },
    base64(name) {
      return window.btoa(unescape(encodeURIComponent(name)));
    },
    keyEvent(event) {
      if (state.show !== null) return;

      if (event.keyCode === 27) {
        mutations.resetSelected();
      }

      if (event.keyCode === 46) {
        if (!state.user.perm.delete || getters.selectedCount === 0) return;
        mutations.showHover("delete");
      }

      if (event.keyCode === 113) {
        if (!state.user.perm.rename || getters.selectedCount !== 1) return;
        mutations.showHover("rename");
      }

      if (!event.ctrlKey && !event.metaKey) return;

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
          this.items.files.forEach((file) => {
            if (!state.selected.includes(file.index)) {
              mutations.addSelected(file.index);
            }
          });
          this.items.dirs.forEach((dir) => {
            if (!state.selected.includes(dir.index)) {
              mutations.addSelected(dir.index);
            }
          });
          break;
        case "s":
          event.preventDefault();
          document.getElementById("download-button").click();
          break;
      }
    },
    switchView() {
      mutations.closeHovers();
      const currentIndex = this.viewModes.indexOf(state.user.viewMode);
      const nextIndex = (currentIndex + 1) % this.viewModes.length;
      const data = {
        id: state.user.id,
        viewMode: this.viewModes[nextIndex],
      };
      users.update(data, ["viewMode"]).catch(this.$showError);
      mutations.updateUser(data);
    },
    preventDefault(event) {
      event.preventDefault();
    },
    copyCut(event, key) {
      if (event.target.tagName.toLowerCase() === "input") return;

      const items = getters.selected.map((index) => ({
        from: state.req.items[index].url,
        name: state.req.items[index].name,
      }));

      if (items.length === 0) return;

      mutations.updateClipboard({
        key: key,
        items: items,
        path: this.$route.path,
      });
    },
    paste(event) {
      if (event.target.tagName.toLowerCase() === "input") return;

      const items = mutations.getClipboardItems().map((item) => {
        const from = item.from.endsWith("/") ? item.from.slice(0, -1) : item.from;
        const to = this.$route.path + encodeURIComponent(item.name);
        return { from, to, name: item.name };
      });

      if (items.length === 0) return;

      const action = (overwrite, rename) => {
        api
          .copy(items, overwrite, rename)
          .then(() => mutations.setReload(true))
          .catch(this.$showError);
      };

      if (mutations.getClipboardKey() === "x") {
        action = (overwrite, rename) => {
          api
            .move(items, overwrite, rename)
            .then(() => {
              mutations.resetClipboard();
              mutations.setReload(true);
            })
            .catch(this.$showError);
        };
      }

      if (mutations.getClipboardPath() === this.$route.path) {
        action(false, true);
        return;
      }

      const conflict = upload.checkConflict(items, state.req.items);

      let overwrite = false;
      let rename = false;

      if (conflict) {
        mutations.showHover({
          prompt: "replace-rename",
          confirm: (event, option) => {
            overwrite = option === "overwrite";
            rename = option === "rename";

            event.preventDefault();
            mutations.closeHovers();
            action(overwrite, rename);
          },
        });
        return;
      }

      action(overwrite, rename);
    },
    colunmsResize() {
      const columns = Math.floor(
        document.querySelector("main").offsetWidth / this.columnWidth
      );
      mutations.updateColumns(columns);
    },
    scrollEvent() {
      if (
        document.documentElement.scrollTop + window.innerHeight >
        document.documentElement.scrollHeight - 100
      ) {
        mutations.loadMore();
      }
    },
    dragEnter(event) {
      event.preventDefault();
      event.stopPropagation();
      this.dragCounter++;
      if (this.dragCounter > 0) {
        document.querySelector("main").classList.add("dragging");
      }
    },
    dragLeave(event) {
      event.preventDefault();
      event.stopPropagation();
      this.dragCounter--;
      if (this.dragCounter === 0) {
        document.querySelector("main").classList.remove("dragging");
      }
    },
    drop(event) {
      event.preventDefault();
      event.stopPropagation();
      this.dragCounter = 0;
      document.querySelector("main").classList.remove("dragging");

      if (event.dataTransfer.files.length) {
        upload
          .upload(event.dataTransfer.files, this.$route.path)
          .then(() => mutations.setReload(true))
          .catch(this.$showError);
      }
    },
    uploadInput(event) {
      upload
        .upload(event.target.files, this.$route.path)
        .then(() => mutations.setReload(true))
        .catch(this.$showError);
    },
    resetOpacity() {
      Array.from(document.querySelectorAll(".item")).forEach((item) => {
        item.style.opacity = 1;
      });
    },
    sort(by) {
      mutations.updateSorting({
        by,
        asc: state.req.sorting.by === by ? !state.req.sorting.asc : true,
      });
    },
    openSearch() {
      mutations.showHover("search");
    },
    toggleMultipleSelection() {
      mutations.toggleMultipleSelection();
    },
    windowsResize() {
      this.width = window.innerWidth;
      this.colunmsResize();
      this.setItemWeight();
    },
    download() {
      const items = getters.selected.map((index) => state.req.items[index]);
      if (items.length === 0) return;

      items.forEach((item) => {
        const link = document.createElement("a");
        link.href = item.url;
        link.download = item.name;
        link.click();
      });
    },
    close() {
      if (this.$route.path.endsWith("/")) {
        const parent = this.$route.path.split("/").slice(0, -2).join("/");
        this.$router.push(parent || "/");
      } else {
        this.$router.push(this.$route.path + "/");
      }
    },
    upload() {
      if (this.$route.path.startsWith("/share")) {
        document.querySelector(".upload-container input[type=file]").click();
      } else {
        document.querySelector(".upload-container").classList.add("active");
      }
    },
    setItemWeight() {
      const itemCount = state.req.items.length;
      const totalHeight =
        window.innerHeight - document.querySelector(".header").offsetHeight - 40;
      this.itemWeight = totalHeight / itemCount;
    },
    fillWindow(fit = false) {
      const itemCount = state.req.items.length;
      const totalHeight =
        window.innerHeight - document.querySelector(".header").offsetHeight - 40;
      this.itemWeight = totalHeight / (itemCount || 1);

      if (fit) {
        this.showLimit = Math.floor(totalHeight / this.itemWeight);
      }
    },
  },
};
</script>
