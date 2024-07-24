<template>
  <header>
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
import { state, mutations,getters } from "@/store";
import { users, files as api } from "@/api";
import Action from "@/components/header/Action.vue";
import * as upload from "@/utils/upload";
import css from "@/utils/css";
import throttle from "@/utils/throttle";
import Search from "@/components/Search.vue";

export default {
  name: "listingView",
  components: {
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
      viewModes: ["list", "compact", "normal", "gallery"],
    };
  },
  computed: {
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
      const selectedCount = state.selected.length;
      return {
        select: selectedCount > 0,
        upload: state.user.perm.create && selectedCount > 0,
        download: state.user.perm.download && selectedCount > 0,
        delete: selectedCount > 0 && state.user.perm.delete,
        rename: selectedCount === 1 && state.user.perm.rename,
        share: selectedCount === 1 && state.user.perm.share,
        move: selectedCount > 0 && state.user.perm.rename,
        copy: selectedCount > 0 && state.user.perm.create,
      };
    },
  },
  watch: {
    req() {
      this.showLimit = 50;

      this.$nextTick(() => {
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
    if (!state.user || !state.user.perm.create) return;
    document.addEventListener("dragover", this.preventDefault);
    document.addEventListener("dragenter", this.dragEnter);
    document.addEventListener("dragleave", this.dragLeave);
    document.addEventListener("drop", this.drop);
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
    window.removeEventListener("scroll", this.scrollEvent);
    window.removeEventListener("resize", this.windowsResize);

    if (state.user && !state.user.perm.create) return;
    document.removeEventListener("dragover", this.preventDefault);
    document.removeEventListener("dragenter", this.dragEnter);
    document.removeEventListener("dragleave", this.dragLeave);
    document.removeEventListener("drop", this.drop);
  },
  methods: {
    action() {
      console.log("state.show",state.show)
      if (state.show) {
        // Assuming `showHover` is a method on a component
        this.$emit("action");
      }
    },
    toggleSidebar() {
      if (getters.currentPromptName() === "sidebar") {
        console.log("should close")
        mutations.closeHovers();
      } else {
        mutations.showHover("sidebar");
      }
    },
    base64(name) {
      return window.btoa(unescape(encodeURIComponent(name)));
    },
    keyEvent(event) {
      if (state.show !== null) {
        return;
      }

      if (event.keyCode === 27) {
        mutations.resetSelected();
      }

      if (event.keyCode === 46) {
        if (!state.user.perm.delete || state.selected.length === 0) return;
        mutations.showHover("delete");
      }

      if (event.keyCode === 113) {
        if (!state.user.perm.rename || state.selected.length !== 1) return;
        mutations.showHover("rename");
      }

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
            if (!state.selected.includes(file.index)) {
              mutations.setSelected([...state.selected, file.index]);
            }
          }
          for (let dir of this.items.dirs) {
            if (!state.selected.includes(dir.index)) {
              mutations.setSelected([...state.selected, dir.index]);
            }
          }
          break;
        case "s":
          event.preventDefault();
          document.getElementById("download-button").click();
          break;
      }
    },
    async switchView() {
      mutations.closeHovers();
      const currentIndex = this.viewModes.indexOf(state.user.viewMode);
      const nextIndex = (currentIndex + 1) % this.viewModes.length;
      const data = {
        id: state.user.id,
        viewMode: this.viewModes[nextIndex],
      };
      try {
        await users.update(data, ["viewMode"]);
        mutations.setUser(data);
      } catch (e) {
        this.$showError(e);
      }
    },
    preventDefault(event) {
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

      mutations.updateClipboard({
        key,
        items,
        path: this.$route.path,
      });
    },
    async paste(event) {
      if (event.target.tagName.toLowerCase() === "input") {
        return;
      }

      let items = state.clipboard.items.map((item) => ({
        from: item.from.endsWith("/") ? item.from.slice(0, -1) : item.from,
        to: this.$route.path + encodeURIComponent(item.name),
        name: item.name,
      }));

      if (items.length === 0) {
        return;
      }

      const action = (overwrite, rename) => {
        api
          .copy(items, overwrite, rename)
          .then(() => {
            mutations.setReload(true);
          })
          .catch(this.$showError);
      };

      if (state.clipboard.key === "x") {
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

      if (state.clipboard.path === this.$route.path) {
        action(false, true);
        return;
      }

      const conflict = upload.checkConflict(items, state.req.items);

      if (conflict) {
        mutations.showHover({
          name: "replace-rename",
          confirm: (event, option) => {
            const overwrite = option === "overwrite";
            const rename = option === "rename";

            event.preventDefault();
            mutations.closeHovers();;
            action(overwrite, rename);
          },
        });
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
    scrollEvent: throttle(function () {
      const totalItems = state.req.numDirs + state.req.numFiles;
      if (this.showLimit >= totalItems) return;

      const currentPos = window.innerHeight + window.scrollY;
      const triggerPos = document.body.offsetHeight - window.innerHeight * 0.25;

      if (currentPos > triggerPos) {
        const showQuantity = Math.ceil((window.innerHeight * 2) / this.itemWeight);
        this.showLimit += showQuantity;
      }
    }, 100),
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
      mutations.closeHovers();;

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
        await users.update({ id: state.user.id, sorting: { by, asc } }, ["sorting"]);
        mutations.setReload(true);
      } catch (e) {
        this.$showError(e);
      }
    },
    openSearch() {
      mutations.showHover("search");
    },
    toggleMultipleSelection() {
      mutations.toggleMultiple();
      mutations.closeHovers();
    },
    windowsResize: throttle(function () {
      this.colunmsResize();
      this.width = window.innerWidth;

      if (this.$refs.listingView == null) return;

      this.setItemWeight();
      this.fillWindow();
    }, 100),
    download() {
      if (state.selected.length === 1 && !state.req.items[state.selected[0]].isDir) {
        api.download(null, state.req.items[state.selected[0]].url);
        return;
      }

      mutations.showHover({
        name: "download",
        confirm: (format) => {
          mutations.closeHovers();;
          let files = [];
          if (state.selected.length > 0) {
            for (let i of state.selected) {
              files.push(state.req.items[i].url);
            }
          } else {
            files.push(this.$route.path);
          }

          api.download(format, ...files);
        },
      });
    },
    upload() {
      if (
        typeof window.DataTransferItem !== "undefined" &&
        typeof DataTransferItem.prototype.webkitGetAsEntry !== "undefined"
      ) {
        mutations.showHover("upload");
      } else {
        document.getElementById("upload-input").click();
      }
    },
    setItemWeight() {
      if (this.$refs.listingView == null) return;

      let itemQuantity = state.req.numDirs + state.req.numFiles;
      if (itemQuantity > this.showLimit) itemQuantity = this.showLimit;

      this.itemWeight = this.$refs.listingView.offsetHeight / itemQuantity;
    },
    fillWindow(fit = false) {
      const totalItems = state.req.numDirs + state.req.numFiles;
      if (this.showLimit >= totalItems && !fit) return;

      const windowHeight = window.innerHeight;
      const showQuantity = Math.ceil((windowHeight + windowHeight * 2) / this.itemWeight);

      if (this.showLimit > showQuantity && !fit) return;

      this.showLimit = showQuantity > totalItems ? totalItems : showQuantity;
    },
  },
};
</script>
