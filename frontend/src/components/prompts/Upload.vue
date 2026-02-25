<template>
  <SettingsItem :title="$t('fileLoading.uploadSettings')" :collapsable="true" :start-collapsed="true">
    <div class="settings-items upload-settings">
      <div class="settings-number-input item">
        <div class="no-padding">
          <label for="maxConcurrentUpload">{{ $t("fileLoading.maxConcurrentUpload") }}</label>
          <i class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('fileLoading.maxConcurrentUploadHelp'))" @mouseleave="hideTooltip">
            help
          </i>
        </div>
        <div>
          <input v-model.number="maxConcurrentUpload" type="range" min="1" max="10" @change="updateUploadSettings" />
          <span class="range-value">{{ maxConcurrentUpload }}</span>
        </div>
      </div>
      <div class="settings-number-input item">
        <div class="no-padding">
          <label for="uploadChunkSizeMb">{{ $t("fileLoading.uploadChunkSizeMb") }}</label>
          <i class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('fileLoading.uploadChunkSizeMbHelp'))" @mouseleave="hideTooltip">
            help
          </i>
        </div>
        <div class="no-padding">
          <input class="sizeInput input" v-model.number="uploadChunkSizeMb" type="number" min="0" @change="updateUploadSettings" />
        </div>
      </div>
      <ToggleSwitch class="item" v-model="clearAll" @change="updateUploadSettings"
        :name="$t('fileLoading.clearAll')"
        :description="$t('fileLoading.clearAllDescription')" />
    </div>
  </SettingsItem>
  <div class="upload-prompt" :class="{ dropping: isDragging }" @dragenter.prevent="onDragEnter"
    @dragover.prevent="onDragOver" @dragleave.prevent="onDragLeave" @drop.prevent="onDrop">
    <div class="upload-prompt-container">
      <i v-if="files.length === 0" class="material-icons">cloud_upload</i>
      <p v-if="files.length === 0">{{ $t("prompts.dragAndDrop") }}</p>
      <div class="button-group">
        <button @click="triggerFilePicker" class="button button--flat">
          {{ $t("general.file") }}
        </button>
        <button style="margin-left: 1em" @click="triggerFolderPicker" class="button button--flat">
          {{ $t("general.folder") }}
        </button>
      </div>
    </div>
  </div>
  <div v-show="files.length > 0" class="card-content" @drop.prevent="onDrop">
    <div v-if="showConflictPrompt" class="conflict-overlay">
      <div class="card">
        <div class="card-content">
          <p>{{ $t("prompts.conflictsDetected") }}</p>
        </div>
        <div class="card-actions">
          <button @click="resolveConflict(true)" class="button button--flat button--red">
            {{ $t("general.replace") }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="files.length > 0" class="upload-list">
      <div v-for="file in files" :key="file.id" class="upload-item">
        <i class="material-icons file-icon">{{ file.type === "directory" ? "folder" : "insert_drive_file" }}</i> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        <div class="file-info">
          <p class="file-name">{{ file.name }}</p>
          <progress-bar v-if="file.type !== 'directory'" :val="file.status === 'completed'
              ? $t('prompts.completed')
              : file.status === 'error'
                ? $t('prompts.error')
                : file.status === 'conflict'
                  ? $t('prompts.conflictsDetected')
                  : (file.progress / 100) * file.size
            " :unit="file.status === 'completed' || file.status === 'error' ? '' : 'bytes'" :max="file.size"
            :status="file.status" text-position="inside" size="20"
            :help-text="getHelpText(file)">
          </progress-bar>
          <div v-else class="status-label">
            <span>{{ getStatusText(file.status, file) }}</span>
          </div>
        </div>
        <div class="file-actions">
          <button v-if="file.status === 'uploading'" @click="uploadManager.pause(file.id)" class="action"
            :aria-label="$t('general.pause')" :title="$t('general.pause')">
            <i class="material-icons">pause</i>
          </button>
          <button v-if="file.status === 'paused'" @click="uploadManager.resume(file.id)" class="action"
            :aria-label="$t('general.resume')" :title="$t('general.resume')">
            <i class="material-icons">play_arrow</i>
          </button>
          <button v-if="file.status === 'error'" @click="uploadManager.retry(file.id)" class="action"
            :aria-label="$t('general.retry')" :title="$t('general.retry')">
            <i class="material-icons">replay</i>
          </button>
          <button v-if="file.status === 'conflict'" @click="handleConflictAction(file)" class="action"
            :aria-label="$t('general.replace')" :title="$t('general.replace')">
            <i class="material-icons">sync_problem</i>
          </button>
          <button @click="cancelUpload(file.id)" class="action" :aria-label="$t('general.cancel')"
            :title="$t('general.cancel')">
            <i class="material-icons">close</i>
          </button>
        </div>
      </div>
    </div>
  </div>

  <div class="card-actions">
    <button v-if="canPauseAll" @click="uploadManager.pauseAll" class="button button--flat"
      :aria-label="$t('buttons.pauseAll')" :title="$t('buttons.pauseAll')">
      {{ $t("buttons.pauseAll") }}
    </button>
    <button v-if="canResumeAll" @click="uploadManager.resumeAll" class="button button--flat"
      :aria-label="$t('buttons.resumeAll')" :title="$t('buttons.resumeAll')">
      {{ $t("buttons.resumeAll") }}
    </button>
    <button @click="clearCompleted" class="button button--flat" :disabled="!hasClearable"
      :aria-label="$t('buttons.clearCompleted')" :title="$t('buttons.clearCompleted')">
      {{ $t("buttons.clearCompleted") }}
    </button>
  </div>

  <input ref="fileInput" @change="onFilePicked" type="file" multiple style="display: none" />
  <input ref="folderInput" @change="onFolderPicked" type="file" webkitdirectory directory multiple
    style="display: none" />
</template>

<script>
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { uploadManager } from "@/utils/upload";
import { mutations, state } from "@/store";
import { notify } from "@/notify";
import { usersApi } from "@/api";
import ProgressBar from "@/components/ProgressBar.vue";
import SettingsItem from "@/components/settings/SettingsItem.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import i18n from "@/i18n";

export default {
  name: "UploadFiles",
  components: {
    ProgressBar,
    SettingsItem,
    ToggleSwitch,
  },
  props: {
    initialItems: {
      type: Object,
      default: null,
    },
    filesToReplace: {
      type: Array,
      default: () => [],
    },
    targetPath: {
      type: String,
      default: null,
    },
    targetSource: {
      type: String,
      default: null,
    },
  },
  computed: {
    shareInfo() {
      return state.shareInfo;
    },
    uploadSettingsDescription() {
      const maxConcurrentUpload = state.user.fileLoading?.maxConcurrentUpload || 3;
      let uploadChunkSizeMb = state.user.fileLoading?.uploadChunkSizeMb || 5;
      if (uploadChunkSizeMb === 0) {
        uploadChunkSizeMb = 5;
      }
      return this.$t("prompts.uploadSettingsChunked", {
        maxConcurrentUpload,
        uploadChunkSizeMb
      });
    }
  },
  setup(props) {
    const fileInput = ref(null);
    const folderInput = ref(null);
    const files = computed(() => uploadManager.queue);
    const isDragging = ref(false);
    const showConflictPrompt = ref(false);
    let conflictResolver = null;

    let wakeLock = null;

    // Upload settings
    const maxConcurrentUpload = ref(state.user.fileLoading?.maxConcurrentUpload || 3);
    const uploadChunkSizeMb = ref(state.user.fileLoading?.uploadChunkSizeMb || 5);
    if (uploadChunkSizeMb.value === 0) {
      uploadChunkSizeMb.value = 5;
    }
    const clearAll = ref(state.user.fileLoading?.clearAll || false);

    const showTooltip = (event, text) => {
      mutations.showTooltip({
        content: text,
        x: event.clientX,
        y: event.clientY,
      });
    };

    const hideTooltip = () => {
      mutations.hideTooltip();
    };

    const updateUploadSettings = async () => {
      try {
        const data = {
          ...state.user,
          fileLoading: {
            maxConcurrentUpload: maxConcurrentUpload.value,
            uploadChunkSizeMb: uploadChunkSizeMb.value,
            clearAll: clearAll.value,
          },
        };
        mutations.updateCurrentUser(data);
        await usersApi.update(data, ["fileLoading"]);
        notify.showSuccessToast("Upload settings updated");
      } catch (e) {
        console.error(e);
      }
    };

    const handleConflict = (resolver) => {
      conflictResolver = resolver;
      mutations.showHover({
        name: "replace-rename",
        pinnedHover: true,
        confirm: (event, option) => {
          if (option === "overwrite") {
            resolveConflict(true);
          } else if (option === "rename") {
            showRenamePrompt();
          } else {
            resolveConflict(false);
          }
        },
      });
    };

    const resolveConflict = (overwrite) => {
      if (conflictResolver) {
        conflictResolver(overwrite);
      }
      mutations.closeTopHover(); // Only close the conflict prompt, return to upload prompt
    };

    const showRenamePrompt = () => {
      mutations.closeTopHover(); // Only close the replace-rename prompt, keep upload prompt open
      // Get the conflicting folder name from the upload queue
      const conflictingFolder = uploadManager.getConflictingFolder();
      if (!conflictingFolder) {
        console.error("No conflicting folder found for rename");
        return;
      }

      mutations.showHover({
        name: "rename",
        confirm: (newName) => {
          renameUploadFolder(conflictingFolder, newName);
        },
        props: { folderName: conflictingFolder }
      });
    };

    const renameUploadFolder = async (oldName, newName) => {
      try {
        // Check if the new name already exists
        const existingItems = new Set(state.req.items.map(i => i.name));
        if (existingItems.has(newName)) {
          notify.showError(new Error(`A folder named "${newName}" already exists`));
          return;
        }

        // Update upload manager with the new folder name
        await uploadManager.renameFolder(oldName, newName);

        // Resolve the conflict and continue upload
        if (conflictResolver) {
          conflictResolver({ rename: newName });
        }
        mutations.closeTopHover(); // Only close the rename prompt, return to upload prompt
      } catch (error) {
        console.error(error);
      }
    };

    const acquireWakeLock = async () => {
      if (!("wakeLock" in navigator)) {
        return;
      }
      try {
        if (wakeLock !== null) return; // Already locked
        wakeLock = await navigator.wakeLock.request("screen");
        wakeLock.addEventListener("release", () => {
          wakeLock = null;
        });
      } catch (err) {
        console.error(`Wake Lock failed: ${err.name}, ${err.message}`);
      }
    };

    const releaseWakeLock = () => {
      if (wakeLock !== null) {
        wakeLock.release();
        wakeLock = null;
      }
    };

    const isUploading = computed(() => state.upload.isUploading);

    watch(isUploading, (active) => {
      if (active) {
        acquireWakeLock();
      } else {
        releaseWakeLock();
      }
    });

    const hasCompleted = computed(() =>
      files.value.some((file) => file.status === "completed")
    );

    const hasClearable = computed(() => {
      if (state.user.fileLoading?.clearAll) {
        // For "clear all" mode: check for completed, error, conflict, or paused uploads
        return files.value.some((file) => 
          file.status === "completed" || 
          file.status === "error" || 
          file.status === "conflict" || 
          file.status === "paused"
        );
      } else {
        // For "clear completed" mode: only check for completed uploads
        return files.value.some((file) => file.status === "completed");
      }
    });

    const canPauseAll = computed(() =>
      files.value.some((file) => file.status === "uploading")
    );

    const canResumeAll = computed(
      () =>
        !canPauseAll.value &&
        files.value.some((file) => file.status === "paused")
    );

    const close = () => {
      mutations.closeTopHover();
    };

    const clearCompleted = () => {
      uploadManager.clearCompleted();
    };

    const handleVisibilityChange = async () => {
      if (document.visibilityState === "visible" && isUploading.value) {
        acquireWakeLock();
      }
    };

    const handleBeforeUnload = (event) => {
      // Warn user if they try to leave/refresh the page while uploads are active
      if (isUploading.value) {
        event.preventDefault();
        // Chrome requires returnValue to be set
        event.returnValue = '';
        return '';
      }
    };

    // Helper to get the destination path (from prop or fallback to current request)
    const getDestinationPath = () => props.targetPath || state.req.path;

    const processItems = async (items) => {
      const destination = getDestinationPath();
      // When items are passed as a prop from ListingView, they can be either
      // an array of DataTransferItem (from drag and drop) or an array of File (from input).
      if (Array.isArray(items)) {
        if (items.length > 0 && items[0] instanceof File) {
          // This is an array of File objects from the input fallback in ListingView
          processFileList(items, destination);
        } else {
          // This is an array of DataTransferItem from drag and drop in ListingView
          await processDroppedItems(items, destination);
        }
      } else if (items) {
        // This case handles a FileList object from the upload prompt's own input fields.
        // It is not an array, so we convert it.
        await processDroppedItems(Array.from(items), destination);
      }
    };

    onMounted(async () => {
      document.addEventListener("visibilitychange", handleVisibilityChange);
      window.addEventListener("beforeunload", handleBeforeUnload);
      uploadManager.setOnConflict(handleConflict);
      if (props.initialItems) {
        await processItems(props.initialItems);
      }
    });

    onUnmounted(() => {
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("beforeunload", handleBeforeUnload);
      uploadManager.setOnConflict(() => {}); // cleanup
      releaseWakeLock();
    });

    const triggerFilePicker = () => {
      if (fileInput.value) fileInput.value.click();
    };

    const triggerFolderPicker = () => {
      if (folderInput.value) folderInput.value.click();
    };

    const onFilePicked = (event) => {
      const pickedFiles = event.target.files;
      if (pickedFiles.length > 0) {
        processFileList(pickedFiles, getDestinationPath());
      }
      if (event.target) event.target.value = null;
    };

    const onFolderPicked = (event) => {
      const pickedFiles = event.target.files;
      if (pickedFiles.length > 0) {
        processFileList(pickedFiles, getDestinationPath());
      }
      if (event.target) event.target.value = null;
    };

    const onDrop = async (event) => {
      isDragging.value = false;
      const destination = getDestinationPath();
      if (event.dataTransfer.items) {
        const items = Array.from(event.dataTransfer.items);
        await processDroppedItems(items, destination);
      } else {
        const droppedFiles = event.dataTransfer.files;
        console.log(
          "Upload.vue: Processing dropped files (fallback):",
          droppedFiles
        );
        processFileList(droppedFiles, destination);
      }
    };

    const onDragEnter = () => {
      isDragging.value = true;
    };

    const onDragOver = () => {
      isDragging.value = true;
    };

    const onDragLeave = () => {
      isDragging.value = false;
    };

    const getFilesFromDirectoryEntry = async (entry) => {
      if (entry.isFile) {
        return new Promise((resolve) => {
          entry.file((file) => {
            const relativePath = entry.fullPath.startsWith("/")
              ? entry.fullPath.substring(1)
              : entry.fullPath;
            resolve([{ file, relativePath }]);
          });
        });
      }
      if (entry.isDirectory) {
        const reader = entry.createReader();
        const entries = await new Promise((resolve) => {
          reader.readEntries((e) => resolve(e));
        });
        const allFiles = await Promise.all(
          entries.map((subEntry) => getFilesFromDirectoryEntry(subEntry))
        );
        return allFiles.flat();
      }
      return [];
    };

    const processDroppedItems = async (items, destination) => {
      const filesToUpload = [];
      const promises = items.map(item => {
        const entry = item.webkitGetAsEntry();
        if (entry) {
          return getFilesFromDirectoryEntry(entry);
        }
        return Promise.resolve([]);
      });

      const allFiles = await Promise.all(promises);
      allFiles.forEach(files => filesToUpload.push(...files));

      if (filesToUpload.length > 0) {
        uploadManager.add(destination, filesToUpload);
      }
    };

    const processFileList = (fileList, destination) => {
      const filesToAdd = Array.from(fileList).map((file) => ({
        file,
        relativePath: file.webkitRelativePath || file.name,
      }));
      if (filesToAdd.length > 0) {
        uploadManager.add(destination, filesToAdd);
      }
    };

    const handleConflictAction = (file) => {
      mutations.showHover({
        name: "replace",
        pinnedHover: true,
        confirm: () => {
          uploadManager.retry(file.id, true);
          mutations.closeTopHover();
        },
      });
    };

    const cancelUpload = (id) => {
      uploadManager.cancel(id);
    };

    const getStatusText = (status, file) => {
      // Show connection issue in status for paused uploads
      if (status === 'paused' && file?.connectionIssue) {
        return 'Paused (connection issue)';
      }
      
      switch (status) {
        case 'uploading':
          return i18n.global.t('general.uploading', { suffix: '...' });
        case 'completed':
          return i18n.global.t('prompts.completed');
        case 'error':
          return i18n.global.t('prompts.error');
        case 'paused':
          return i18n.global.t('general.paused');
        case 'conflict':
          return i18n.global.t('general.conflict');
        default:
          return status;
      }
    };

    const getHelpText = (file) => {
      if (file.status === 'error' && file.errorDetails) {
        return file.errorDetails;
      }
      if (file.status === 'paused' && file.connectionIssue) {
        return 'Connection stalled - upload paused. Click resume to retry.';
      }
      if (file.connectionIssue && file.status === 'error') {
        return file.errorDetails || 'Connection issue detected. Click retry to resume.';
      }
      return '';
    };

    return {
      triggerFilePicker,
      triggerFolderPicker,
      onFilePicked,
      onFolderPicked,
      fileInput,
      folderInput,
      files,
      isDragging,
      onDragEnter,
      onDragLeave,
      onDragOver,
      onDrop,
      cancelUpload,
      uploadManager,
      close,
      clearCompleted,
      hasCompleted,
      hasClearable,
      showConflictPrompt,
      resolveConflict,
      showRenamePrompt,
      renameUploadFolder,
      canPauseAll,
      canResumeAll,
      handleConflictAction,
      maxConcurrentUpload,
      uploadChunkSizeMb,
      clearAll,
      showTooltip,
      hideTooltip,
      getStatusText,
      getHelpText,
      updateUploadSettings,
    };
  },
};
</script>

<style scoped>
.upload-prompt {
  text-align: center;
  padding: 2em;
  border: 2px dashed #ccc;
  border-radius: 8px;
  margin: 1em;
}

.dropping {
  transform: scale(0.97);
  border-radius: 1em;
  box-shadow: var(--primaryColor) 0 0 1em;
}

.upload-prompt-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.upload-prompt i {
  font-size: 4em;
  color: #ccc;
}

.upload-list {
  overflow-y: auto;
  padding-right: 0.5em;
  /* To avoid scrollbar overlapping content */
  flex-grow: 1;
  display: flex;
  flex-direction: column-reverse;
  min-height: 0;
}

.upload-item {
  display: flex;
  align-items: center;
}

.file-icon {
  margin-right: 0.5em;
  color: #999;
}

.file-info {
  flex-grow: 1;
}

.file-name {
  margin: 0;
  font-size: 0.9em;
}

.status-label {
  color: #777;
  font-size: 0.8em;
  margin-top: 5px;
}

.file-actions .action {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0.2em;
}

.file-actions .action i {
  font-size: 1.2em;
}

.spacer {
  flex-grow: 1;
}

.card.floating {
  display: flex !important;
  flex-direction: column;
  max-height: 85vh;
}

.card-content {
  flex-grow: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  position: relative;
  padding: 1em;
}

.conflict-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.7);
  z-index: 999;
  display: flex;
  justify-content: center;
  align-items: center;
}

.conflict-overlay .card {
  background-color: var(--card-background-color);
  padding: 1em;
  border-radius: 8px;
}

/* Upload settings styles */
.upload-settings {
  margin: 0 1em;
}

.settings-number-input {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 0.5em 0;
}

.settings-number-input div {
  display: flex;
  padding: 0.25em;
  align-items: center;
}

.settings-number-input .no-padding {
  padding: 0;
}

.range-value {
  margin-left: 1em;
  min-width: 2ch;
  text-align: center;
  font-weight: bold;
}

.sizeInput {
  max-width: 100px;
}
</style>
