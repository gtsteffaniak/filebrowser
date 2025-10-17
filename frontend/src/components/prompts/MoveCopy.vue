<template>
  <div class="card-title">
    <h2>{{ $t(`prompts.${operation}`) }}</h2>
  </div>

  <div class="card-content">
    <!-- Loading spinner overlay -->
    <div v-if="isLoading" class="loading-content">
      <i class="material-icons spin">sync</i>
      <p class="loading-text">{{ $t("prompts.operationInProgress") }}</p>
    </div>
    <div v-else>
      <file-list  ref="fileList" @update:selected="updateDestination">
      </file-list>
    </div>
  </div>
  <div v-if="!isLoading" class="card-action" style="display: flex; align-items: center; justify-content: space-between">
    <template v-if="user.permissions.modify">
      <button class="button button--flat" @click="$refs.fileList.createDir()" :aria-label="$t('sidebar.newFolder')"
        :title="$t('sidebar.newFolder')" style="justify-self: left">
        <span>{{ $t("sidebar.newFolder") }}</span>
      </button>
    </template>
    <div>
      <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')">
        {{ $t("buttons.cancel") }}
      </button>
      <button :disabled="destContainsSrc" class="button button--flat" @click="performOperation"
        :aria-label="$t(`buttons.${operation}`)" :title="$t(`buttons.${operation}`)">
        {{ $t(`buttons.${operation}`) }}
      </button>
    </div>
  </div>
</template>

<script>
import { mutations, state, getters } from "@/store";
import FileList from "./FileList.vue";
import { filesApi, publicApi } from "@/api";
import buttons from "@/utils/buttons";
import * as upload from "@/utils/upload";
import { url } from "@/utils";
import { notify } from "@/notify";
import { goToItem } from "@/utils/url";
import { shareInfo } from "@/utils/constants";

export default {
  name: "move-copy",
  components: { FileList },
  props: {
    operation: {
      type: String,
      required: true,
      validator: (value) => ["move", "copy"].includes(value),
    },
    // When true, immediately shows loading state (for drag and drop operations)
    operationInProgress: {
      type: Boolean,
      default: false,
    },
  },
  data: function () {
    return {
      current: window.location.pathname,
      destPath: "/", // Start at root of selected source
      destSource: null, // Will be set by FileList component
      items: [],
      isLoading: false, // Track loading state for spinner
    };
  },
  computed: {
    destContainsSrc() {
      if (!this.destPath) {
        return false; // If dest is not set, we can't check containment
      }
      // Add null checks to prevent undefined errors
      if (!this.items || this.items.length === 0) {
        return false;
      }
      const itemPath = this.items[0]?.from;
      if (!itemPath) {
        return false; // If itemPath is undefined, we can't check containment
      }
      // For move, prevent moving to the same directory, but for copy allow it.
      if (this.operation === "move") {
        const parentDir = url.removeLastDir(itemPath) + "/";
        // Only disable if moving to the exact same directory
        if (this.destPath === parentDir) {
          return true;
        }
      }
      // Prevent move/copy into itself or subdirectories (into itself too)
      return this.destPath.startsWith(itemPath + "/") || this.destPath === itemPath;
    },
    user() {
      return state.user;
    },
    closeHovers() {
      return mutations.closeHovers();
    },
  },
  mounted() {
    // If operationInProgress is true, show loading immediately (for drag and drop)
    if (this.operationInProgress) {
      this.isLoading = true;
    }
    if (state.isSearchActive) {
      // Add null checks to prevent undefined values
      if (state.selected && state.selected[0] && state.selected[0].path) {
        this.items = [
          {
            from: state.selected[0].path,
            fromSource: state.selected[0].source,
            name: state.selected[0].name,
          },
        ];
      }
    } else {
      if (state.selected && state.req && state.req.items) {
        for (let item of state.selected) {
          const reqItem = state.req.items[item];
          if (reqItem && reqItem.path) {
            this.items.push({
              from: reqItem.path,
              fromSource: state.req.source,
              name: reqItem.name,
            });
          }
        }
      }
    }
  },
  methods: {
    updateDestination(pathOrData) {
      // Handle both old format (just path) and new format (object with path and source)
      if (typeof pathOrData === 'string') {
        this.destPath = pathOrData;
        // For backward compatibility, keep the current source
        // This will be updated when FileList is modified to emit both
      } else if (pathOrData && pathOrData.path) {
        this.destPath = pathOrData.path;
        // Update destSource from FileList's selection
        this.destSource = pathOrData.source;
      }
    },
    performOperation: async function (event) {
      event.preventDefault();
      this.isLoading = true; // Show loading spinner
      try {
        // Define the action function
        let action = async (overwrite, rename) => {
          for (let item of this.items) {
            // Ensure proper path construction without double slashes
            const destPath = this.destPath.endsWith('/') ? this.destPath : this.destPath + '/';
            item.to = destPath + item.name;
            // Always set toSource for cross-source operations
            item.toSource = this.destSource;
          }
          buttons.loading(this.operation);
          if (getters.isShare()) {
            await publicApi.moveCopy(this.items, this.operation, overwrite, rename);
          } else {
            await filesApi.moveCopy(this.items, this.operation, overwrite, rename);
          }
        };
        let conflict = false;
        let dstResp = null;
        if (getters.isShare()) {
          dstResp = await publicApi.fetchPub(this.destPath, shareInfo.hash);
        } else {
          dstResp = await filesApi.fetchFiles(this.destSource, this.destPath);
        }
        conflict = upload.checkConflict(this.items, dstResp.items);
        let overwrite = false;
        let rename = false;

        if (conflict) {
          await new Promise((resolve, reject) => {
            mutations.showHover({
              name: "replace-rename",
              confirm: async (event, option) => {
                overwrite = option == "overwrite";
                rename = option == "rename";
                event.preventDefault();
                try {
                  await action(overwrite, rename);
                  resolve(); // Resolve the promise if action succeeds
                } catch (e) {
                  reject(e); // Reject the promise if an error occurs
                }
              },
            });
          });
        } else {
          // Await the action call for non-conflicting cases
          await action(overwrite, rename);
        }
        mutations.closeHovers();
        mutations.setSearch(false);
        if (this.operation === "move") {
          notify.showSuccess(this.$t(`prompts.moveSuccess`));
        } else {
          notify.showSuccess(this.$t(`prompts.copySuccess`));
        }
        // Navigate to the destination folder after successful operation
        if (this.destSource && this.destPath) {
          goToItem(this.destSource, this.destPath, {});
        }
      } catch (error) {
        notify.showError(error);
      } finally {
        this.isLoading = false; // Hide loading spinner
      }
    },
  },
};
</script>

<style scoped>

.loading-content {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}

.loading-text {
  padding: 1em;
  margin: 0;
  font-size: 1em;
  font-weight: 500;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

/* Make card-content position relative for absolute positioning of overlay */
.card-content {
  position: relative;
}
</style>