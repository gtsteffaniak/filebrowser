<template>
  <div id="breadcrumbs" :class="{ 'add-padding': addPadding }">
    <ul v-if="items.length > 0">
      <li>
        <router-link :to="base" :aria-label="$t('general.home')" :title="$t('general.home')"
          :class="{ 'droppable-breadcrumb': isDroppable, 'drag-over': dragOverIndex === -1 }"
          @dragenter.prevent="dragEnter($event, -1)"
          @dragleave.prevent="dragLeave($event, -1)"
          @dragover.prevent="dragOver($event, -1)"
          @drop.prevent="drop($event, -1)">
          <i class="material-icons">home</i>
        </router-link>
      </li>
      <li class="item" v-for="(link, index) in items" :key="index">
        <router-link
          :to="link.url"
          :aria-label="'breadcrumb-link-' + link.name"
          :title="link.name"
          :key="index"
          :class="{ changeAvailable: hasUpdate,
            'droppable-breadcrumb': isDroppable && link.name !== '...',
            'drag-over': dragOverIndex === index && link.name !== '...' }"
          @dragenter="dragEnter($event, index)"
          @dragleave="dragLeave($event, index)"
          @dragover="dragOver($event, index)"
          @drop="drop($event, index)">
          {{ link.name }}
        </router-link>
      </li>
    </ul>
  </div>
</template>

<script>
import { state, getters, mutations } from "@/store";
import { url } from "@/utils";
import { filesApi, publicApi } from "@/api";
import { notify } from "@/notify";

export default {
  name: "breadcrumbs",
  data() {
    return {
      base: "/files/",
      dragOverIndex: -2, // Track which breadcrumb is being dragged over (-2 = none, -1 = home, 0+ = others breadcrumbs)
      breadcrumbPaths: [],
      currentDragElement: null,
    };
  },
  props: ["noLink"],
  mounted() {
    this.updatePaths();
  },
  watch: {
    $route() {
      this.updatePaths();
    },
    req() {
      this.updatePaths();
    },
  },
  computed: {
    req() {
      return state.req;
    },
    hasUpdate() {
      return state.req.hasUpdate;
    },
    addPadding() {
      return getters.isStickySidebar() || getters.isShare();
    },
    isDroppable() {
      if (getters.isShare()) {
        return state.shareInfo?.allowCreate || state.shareInfo?.allowModify;
      }
      return state.user?.permissions?.modify;
    },
    items() {
      const req = state.req;
      if (!req.items || !req.path) {
        return [];
      }
      let encodedPathString = url.encodedPath(state.req.path);
      let originalParts = state.req.path.split("/");
      let encodedParts = encodedPathString.split("/");
      // Remove empty strings from both arrays consistently
      if (originalParts[0] === "") {
        originalParts.shift();
        encodedParts.shift();
      }
      if (originalParts[originalParts.length - 1] === "") {
        originalParts.pop();
        encodedParts.pop();
      }
      let breadcrumbs = [];
      let buildRef = this.base;
      for (let i = 0; i < originalParts.length; i++) {
        const origPart = originalParts[i];
        const encodedElement = encodedParts[i];
        buildRef = buildRef + encodedElement + "/";
        breadcrumbs.push({
          name: origPart,
          url: buildRef,
        });
      }
      if (breadcrumbs.length > 3) {
        while (breadcrumbs.length !== 4) {
          breadcrumbs.shift();
        }
        breadcrumbs[0].name = "...";
      }
      return breadcrumbs;
    },
  },
  methods: {
    updatePaths() {
      if (getters.isShare()) {
        this.base = getters.sharePathBase();
      } else {
        this.base = `/files/${state.req.source}/`;
      }
    },

    getBreadcrumbPath(index) {
      if (index === -1) return "/"; // home = index -1

      const parts = state.req.path?.split("/").filter(p => p) || [];
      const paths = [];
      let accumulated = "";

      // Build paths for each breadcrumb level (e.g. "/a", "/a/b", "/a/b/c" and so)
      for (let i = 0; i < parts.length; i++) {
        accumulated = accumulated ? `${accumulated}/${parts[i]}` : parts[i];
        paths.push(`/${accumulated}`);
      }

      // Adjust for truncated breadcrumbs (like when the  "..." is present)
      if (this.items.length > 3) {
        if (index === 0) {
          // return path of first truncated item
          const truncatedCount = paths.length - 3;
          return paths[truncatedCount - 1];
        } else {
          const actualIndex = (paths.length - 3) + (index - 1);
          return paths[actualIndex];
        }
      } else {
        // Otherwise just map the path directly
        return paths[index];
      }
    },

    dragEnter(event, index) {
      if (!this.isDroppable || this.items[index]?.name === "...") return;
      if (!event.dataTransfer.types.includes("application/x-filebrowser-internal-drag")) return;
      event.preventDefault();
      this.dragOverIndex = index;
    },

    dragOver(event, index) {
      if (!this.isDroppable || this.items[index]?.name === "...") return;
      if (!event.dataTransfer.types.includes("application/x-filebrowser-internal-drag")) return;
      event.preventDefault();
    },

    dragLeave(event, index) {
      // Clear drag state when leaving the breadcrumb area when dragging a item
      const relatedTarget = event.relatedTarget;
      if (!relatedTarget || !this.currentDragElement?.contains(relatedTarget)) {
        if (this.dragOverIndex === index) {
          this.dragOverIndex = -2; // Reset drag state
        }
      }
    },

    async drop(event, index) {
      const link = index === -1 ? null : this.items[index];
      if (link && link.name === "...") return;
      if (!this.isDroppable) return;

      event.preventDefault();
      event.stopPropagation();
      this.dragOverIndex = -2;

      const isInternal = Array.from(event.dataTransfer.types).includes(
        "application/x-filebrowser-internal-drag"
      );

      if (!isInternal) return;

      if (state.selected.length === 0) return;

      const targetPath = this.getBreadcrumbPath(index);
      const currentPath = state.req.path;
      
      // debug log paths when dropping
      console.log("Drop target:", { 
        index,
        targetPath,
        currentPath,
        items: this.items,
        breadcrumbName: link?.name
      });

      // Normalize paths for comparison
      const normalizePath = (path) => {
        if (!path || path === "/") return "/";
        return path.replace(/\/$/, ''); // Remove trailing slash
      };

      const normalizedTarget = normalizePath(targetPath);
      const normalizedCurrent = normalizePath(currentPath);

      console.log("comparing paths:", {
        normalizedCurrent,
        normalizedTarget,
      });

      if (normalizedTarget === normalizedCurrent) {
        notify.showErrorToast("Cannot move to same folder");
        return;
      }

      // Build list of items to move from selected items
      let items = [];
      for (let i of state.selected) {
        if (i < 0 || i >= state.req.items.length) continue;

        const selectedItem = state.req.items[i];

        let fromPath = selectedItem.path;

        if (!fromPath) {
          fromPath = url.joinPath(state.req.path, selectedItem.name);
        }

        items.push({
          from: fromPath,
          fromSource: selectedItem.source || state.req.source,
          to: url.joinPath(targetPath, selectedItem.name),
          toSource: state.req.source,
          itemType: selectedItem.type
        });
      }

      // Filter out invalid moves
      items = items.filter(item => {
        if (item.from === item.to) return false;

        // Prevent moving a directory into itself -- likely never will happen but just in case
        if (item.itemType === 'directory') {
          const fromDir = normalizePath(item.from);
          const toDir = normalizePath(item.to);

          // Check if destination is inside the directory
          if (toDir.startsWith(fromDir + "/")) {
            return false;
          }
        }
        return true;
      });

      if (items.length === 0) {
        notify.showErrorToast("No valid items to move");
        return;
      }

      // Check for conflicts in target directory
      let targetDirItems = [];
      try {
        if (getters.isShare()) {
          const response = await publicApi.fetchPub(targetPath, state.shareInfo.hash);
          targetDirItems = response?.items;
        } else {
          const response = await filesApi.fetchFiles(state.req.source, targetPath);
          targetDirItems = response?.items;
        }
      } catch (error) {
        notify.showErrorToast("Cannot access target directory");
        return;
      }

      // if any item conflics will show replace-rename prompt later
      const conflict = items.some(item => {
        const itemName = item.to.split('/').pop(); // Extract filename from destination path
        return targetDirItems.some(targetItem => targetItem.name === itemName);
      });

      const moveAction = async (overwrite, rename) => {
        mutations.showHover({
          name: "move",
          props: { operationInProgress: true },
        });

        try {
          if (getters.isShare()) {
            await publicApi.moveCopy(state.shareInfo.hash, items, "move", overwrite, rename);
          } else {
            await filesApi.moveCopy(items, "move", overwrite, rename);
          }

          const navigateToTarget = () => {
            const targetUrl = getters.isShare()
              ? `/public/share/${state.shareInfo.hash}${url.encodedPath(targetPath)}`
              : `/files/${encodeURIComponent(state.req.source)}${url.encodedPath(targetPath)}`;
            this.$router.push(targetUrl);
          };

          notify.showSuccess(this.$t("prompts.moveSuccess"), {
            icon: "folder",
            buttons: [{
              label: this.$t("buttons.goToItem"),
              primary: true,
              action: navigateToTarget
            }]
          });
          mutations.closeHovers();
          mutations.setReload(true);
        } catch (error) {
          mutations.closeHovers();
          notify.showErrorToast("Move failed");
        }
      };

      if (conflict) {
        mutations.showHover({
          name: "replace-rename",
          confirm: (event, option) => {
            const overwrite = option === "overwrite";
            const rename = option === "rename";
            event.preventDefault();
            mutations.closeHovers();
            moveAction(overwrite, rename);
          },
        });
        return;
      }
      // If no conflicts, proceed with move
      await moveAction(false, false);
    },
  },
};
</script>

<style scoped>
#breadcrumbs {
  margin-top: 0.5em;
  overflow-y: hidden;
}
#breadcrumbs * {
  box-sizing: unset;
}

#breadcrumbs ul {
  display: flex;
  margin: 0;
  padding: 0;
  margin-bottom: 0.5em;
}

#breadcrumbs ul li {
  display: inline-block;
  margin: 0 10px 0 0;
}

#breadcrumbs ul li a {
  display: flex;
  height: 1em;
  background: var(--alt-background);
  text-align: center;
  padding: 1em;
  padding-left: 2em;
  position: relative;
  text-decoration: none;
  color: var(--textPrimary);
  border-radius: 0;
  align-content: center;
  align-items: center;
}

#breadcrumbs ul li a::after {
  content: "";
  border-top: 1.5em solid transparent;
  border-bottom: 1.5em solid transparent;
  border-left: 1.5em solid var(--alt-background);
  position: absolute;
  right: -1.46em; /* Don't modify this... was difficult to find the value */
  top: 0;
  z-index: 5;
}

#breadcrumbs ul li a::before {
  content: "";
  border-top: 1.5em solid transparent;
  border-bottom: 1.5em solid transparent;
  border-left: 1.5em solid var(--background);
  position: absolute;
  left: 0;
  top: 0;
  z-index: 1;
}

#breadcrumbs ul li:first-child a {
  border-top-left-radius: 1em;
  border-bottom-left-radius: 1em;
  padding-left: 1.5em;
  z-index: 3;
}

#breadcrumbs ul li:first-child a::before {
  display: none;
}

#breadcrumbs ul li:last-child a {
  padding-right: 1.5em;
  border-top-right-radius: 1em;
  border-bottom-right-radius: 1em;
}

#breadcrumbs ul li:last-child a::after {
  display: none;
}

#breadcrumbs ul li a:hover {
  background: var(--primaryColor);
  color: white;
}

#breadcrumbs ul li a:hover::after {
  border-left-color: var(--primaryColor);
}

#breadcrumbs ul li:last-child a.changeAvailable {
  filter: contrast(0.8) hue-rotate(200deg) saturate(1);
}

.drag-over {
  background: var(--primaryColor) !important; /* Needs to be !important, without it is not working */
  color: white !important;
  z-index: 2;
}

.drag-over::after {
  border-left-color: var(--primaryColor) !important;
}

@keyframes breadcrumbPulse {
  0% { transform: scale(1); }
  50% { transform: scale(1.05); }
  100% { transform: scale(1); }
}

.drag-over {
  animation: breadcrumbPulse 0.5s ease-in-out infinite;
}

</style>
