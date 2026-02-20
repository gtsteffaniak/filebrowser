<template>
  <div class="card-content">
    <!-- Loading spinner overlay -->
    <!-- changed to v-show (for keep the loading spinner), otherwise the path showed in the prompt will be always "/" -->
    <div v-show="isLoading" class="loading-content">
      <LoadingSpinner size="small" mode="placeholder" />
      <p class="loading-text">{{ $t("prompts.operationInProgress") }}</p>
    </div>
    <div v-show="!isLoading">
      <file-list ref="fileList" @update:selected="updateDestination">
      </file-list>
    </div>
  </div>
  <div class="card-actions split-buttons" >
    <button v-if="canCreateFolder && showNewDirInput" class="button button--flat" @click="cancelNewDir" :aria-label="$t('general.cancel')" :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button
      v-if="canCreateFolder && !showNewDirInput"
      class="button button--flat"
      @click="createNewDir"
      :aria-label="$t('files.newFolder')"
      :title="$t('files.newFolder')"
    >
      <span>{{ $t("files.newFolder") }}</span>
    </button>
    <input v-if="showNewDirInput" ref="newDirInput" class="input new-dir-input" :class="{ 'form-invalid': !isDirNameValid }"
    v-model.trim="newDirName" :placeholder="$t('prompts.newDirMessage')" @keydown.enter="handleEnter" />
    <button v-else :disabled="destContainsSrc" class="button button--flat" @click="performOperation"
      :aria-label="operation === 'move' ? $t('general.move') : $t('general.copy')"
      :title="operation === 'move' ? $t('general.move') : $t('general.copy')">
      {{ operation === 'move' ? $t('general.move') : $t('general.copy') }}
    </button>
    <button v-if="showNewDirInput" class="button button--flat" @click="createDirectory" :disabled="!newDirName || !isDirNameValid">
      {{ $t("general.create") }}
    </button>
  </div>
</template>

<script>
import { mutations, state, getters } from "@/store";
import FileList from "../files/FileList.vue";
import { filesApi, publicApi } from "@/api";
import buttons from "@/utils/buttons";
import * as upload from "@/utils/upload";
import { url } from "@/utils";
import { notify } from "@/notify";
import { goToItem } from "@/utils/url";
import LoadingSpinner from "@/components/LoadingSpinner.vue";

export default {
  name: "move-copy",
  components: { FileList, LoadingSpinner },
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
      showNewDirInput: false, // When true will replace the new folder button with a input field
      newDirName: "",
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
      // For different sources, allow move to root path
      if (this.destSource !== this.items[0]?.fromSource) {
        return false;
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
    canCreateFolder() {
      const perms = getters.permissions();
      return !!perms?.create;
    },
    closeHovers() {
      return mutations.closeTopHover();
    },
    isDirNameValid() {
      return this.validateDirName(this.newDirName);
    }
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
    createNewDir() {
      this.showNewDirInput = true;
      this.newDirName = "";
      // Focus the new dir input automatically
      this.$nextTick(() => {
        this.$refs.newDirInput.focus();
      });
    },
    validateDirName(value) {
      // Check if a folder with the same name already exists in current directory
      if (this.$refs.fileList && this.$refs.fileList.items) {
        const currentItems = this.$refs.fileList.items.filter(item => item.name !== '..');
        return !currentItems.some(item => item.name.toLowerCase() === value.toLowerCase());
      }
      return true;
    },
    cancelNewDir() {
      // Clicking cancel will return the buttons to their normal state
      this.showNewDirInput = false;
      this.newDirName = "";
    },
    handleEnter(event) {
      // Trigger create if you press enter instead of move/copy
      event.stopPropagation();
      event.preventDefault();
      if (this.newDirName && this.isDirNameValid) {
        this.createDirectory();
      }
    },
    async createDirectory() {
      if (!this.newDirName || !this.isDirNameValid) return;
      try {
        this.isLoading = true;
        // Get current navigation from FileList
        const currentPath = this.$refs.fileList.path;
        const currentSource = this.$refs.fileList.source;
        const fullPath = currentPath.endsWith('/') ? currentPath + this.newDirName + '/' : currentPath + '/' + this.newDirName + '/';
        if (getters.isShare()) {
          await publicApi.post(state.shareInfo?.hash, fullPath, "", false, undefined, {}, true);
        } else {
          await filesApi.post(currentSource, fullPath, "", false, undefined, {}, true);
        }
        // Refresh the file list while keeping the current navigation that we did in the prompt
        if (getters.isShare()) {
          publicApi.fetchPub(currentPath, state.shareInfo?.hash)
            .then((req) => this.$refs.fileList.fillOptions(req, true));
        } else {
          filesApi.fetchFiles(currentSource, currentPath)
            .then((req) => this.$refs.fileList.fillOptions(req, true));
        }
        // Clicking create will also return the buttons to their normal state
        mutations.setReload(true);
        this.showNewDirInput = false;
        this.newDirName = "";
      } catch (error) {
        console.error('Error creating directory:', error);
      } finally {
        this.isLoading = false;
      }
    },
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
          let result;
          if (getters.isShare()) {
            result = await publicApi.moveCopy(state.shareInfo.hash, this.items, this.operation, overwrite, rename);
          } else {
            result = await filesApi.moveCopy(this.items, this.operation, overwrite, rename);
          }
          return result; // Return the result to check for failures
        };
        let conflict = false;
        let dstResp = null;
        if (getters.isShare()) {
          dstResp = await publicApi.fetchPub(this.destPath, state.shareInfo?.hash);
        } else {
          dstResp = await filesApi.fetchFiles(this.destSource, this.destPath);
        }
        conflict = upload.checkConflict(this.items, dstResp.items);
        let overwrite = false;
        let rename = false;
        let result = null;

        if (conflict) {
          this.isLoading = false;
          // Check if any item is being copied/moved to itself
          const isSameFile = this.items.some(item => {
            const destPath = this.destPath.endsWith('/') ? this.destPath : this.destPath + '/';
            const targetPath = destPath + item.name;
            return item.from === targetPath && item.fromSource === this.destSource;
          });

          await new Promise((resolve, reject) => {
            mutations.showHover({
              name: "replace-rename",
              props: {
                isSameFile: isSameFile,
                operation: this.operation
              },
              confirm: async (event, option) => {
                overwrite = option == "overwrite";
                rename = option == "rename";
                event.preventDefault();
                try {
                  this.isLoading = true;
                  result = await action(overwrite, rename);
                  resolve(); // Resolve the promise if action succeeds
                } catch (e) {
                  reject(e); // Reject the promise if an error occurs
                } finally {
                  this.isLoading = false;
                }
              },
            });
          });
        } else {
          // Await the action call for non-conflicting cases
          result = await action(overwrite, rename);
        }

        // Check if there were any failures in the result
        const hasFailures = result && result.failed && result.failed.length > 0;
        const hasSuccesses = result && result.succeeded && result.succeeded.length > 0;

        if (hasFailures && !hasSuccesses) {
          // All operations failed - show error but DON'T close prompt
          const errorMessage = result.failed[0]?.message || this.$t("prompts.operationFailed");
          notify.showError(errorMessage);
          return;
        } else if (hasFailures && hasSuccesses) {
          // Partial failure - show warning and continue
          const failedCount = result.failed.length;
          const succeededCount = result.succeeded.length;
          notify.showError(
            this.$t("prompts.partialSuccess", { succeeded: succeededCount, failed: failedCount })
          );
        }

        // Only close prompts and reload on success (or partial success)
        mutations.setReload(true);
        mutations.closeTopHover();
        mutations.setSearch(false);

        // Only show success notification if there were no failures (or partial success was already shown)
        if (!hasFailures || hasSuccesses) {
          // Store destination info for the button action
          const destSource = this.destSource;
          const destPath = this.destPath;

          // Show success notification with optional button to navigate to destination
          // For shares, destSource might be null, but goToItem handles shares via state.shareInfo.hash
          const buttonAction = () => {
            if (destPath) {
              // For shares, goToItem will use state.shareInfo.hash, so source can be null
              // For regular files, destSource should be set
              goToItem(destSource || null, destPath, {});
            }
          };
          const buttonProps = {
            icon: "folder",
            buttons: destPath ? [
              {
                label: this.$t("buttons.goToItem"),
                primary: true,
                action: buttonAction
              }
            ] : undefined
          };
          if (this.operation === "move") {
            notify.showSuccess(this.$t("prompts.moveSuccess"), buttonProps);
          } else {
            notify.showSuccess(this.$t("prompts.copySuccess"), buttonProps);
          }
        }
      } catch (error) {
        // Handle errors thrown by the API (e.g., 500 errors)
        // DON'T close the prompt on error - let user try again or cancel manually

        // Try to extract error message from the error response
        let errorMessage = null;

        // Check if error has a response body with failed items
        if (error && error.failed && error.failed.length > 0 && error.failed[0]?.message) {
          errorMessage = error.failed[0].message;
        } else if (error && error.message) {
          errorMessage = error.message;
        } else if (typeof error === 'string') {
          errorMessage = error;
        }

        // Only use fallback if we couldn't extract a message
        if (!errorMessage) {
          errorMessage = this.$t("prompts.operationFailed");
        }

        notify.showError(errorMessage);
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
  padding-top: 2em;
}

.loading-text {
  padding: 1em;
  margin: 0;
  font-size: 1em;
  font-weight: 500;
}

/* Make card-content position relative for absolute positioning of overlay */
.card-content {
  position: relative;
}

.new-dir-input {
  justify-self: left
}

.split-buttons {
  justify-content: space-between;
}
</style>