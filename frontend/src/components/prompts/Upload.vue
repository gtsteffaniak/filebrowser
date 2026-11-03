<template>
  <div class="card-content">
    <p style="text-align: center">{{ uploadSettingsDescription }}</p>
    <div>
      <SettingsItem :title="$t('fileLoading.uploadSettings')" :collapsable="true" :start-collapsed="true">
        <div class="settings-items upload-settings">
          <div class="settings-number-input item">
            <div class="no-padding">
              <label for="maxConcurrentUpload">{{ $t("fileLoading.maxConcurrentUpload") }}</label>
              <HelpTooltipIcon :text="$t('fileLoading.maxConcurrentUploadHelp')" />
            </div>
            <div>
              <input v-model.number="maxConcurrentUpload" type="range" min="1" max="10" @change="updateUploadSettings" />
              <span class="range-value">{{ maxConcurrentUpload }}</span>
            </div>
          </div>
          <div class="settings-number-input item">
            <div class="no-padding">
              <label for="uploadChunkSizeMb">{{ $t("fileLoading.uploadChunkSizeMb") }}</label>
              <HelpTooltipIcon :text="$t('fileLoading.uploadChunkSizeMbHelp')" />
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
      <div class="upload-prompt" :class="{ dropping: isDragging }"
        @dragenter.prevent="onDragEnter"
        @dragover.prevent="onDragOver" @dragleave.prevent="onDragLeave" @drop.prevent="onDrop">
        <div class="upload-prompt-container">
          <i v-if="files.length === 0" class="material-symbols">cloud_upload</i>
          <p v-if="files.length === 0">{{ $t("prompts.dragAndDrop") }}</p>
          <ButtonGroup
            class="upload-action-buttons"
            :buttons="uploadActionButtons"
            action-mode
            @button-clicked="onUploadAction"
          />
        </div>
      </div>
    </div>
    <div v-if="files.length > 0" style="position: relative;">
      <div v-if="showConflictPrompt" class="conflict-overlay">
        <div class="card">
          <div class="card-content">
            <p>{{ $t("prompts.conflictsDetected") }}</p>
          </div>
          <div class="card-actions">
            <button
              type="button"
              @click="resolveConflict(true)"
              class="button button--flat button--red"
            >
              {{ $t("general.replace") }}
            </button>
          </div>
        </div>
      </div>
      <div ref="list" class="upload-list">
        <div data-virtual-group="uploads" :style="virtual.group('uploads').style">
        <div v-for="{ file, index } in virtual.group('uploads').items" :key="file.id" class="upload-item" :data-index="index">
          <i class="file-icon" :class="fileTypeInfo(file).classes">{{ fileTypeInfo(file).materialSymbol }}</i>
          <div class="file-info">
            <p class="file-name">{{ file.name }}</p>
            <ProgressBar v-if="file.type !== 'directory'" :val="file.status === 'completed'
                ? $t('prompts.completed')
                : file.status === 'error'
                  ? $t('prompts.error')
                  : file.status === 'conflict'
                    ? $t('prompts.conflictsDetected')
                    : (file.progress / 100) * file.size
              " :unit="file.status === 'completed' || file.status === 'error' ? '' : 'bytes'" :max="file.size"
              :status="file.status" text-position="inside" size="20"
              :help-text="getHelpText(file)">
            </ProgressBar>
            <div v-else class="status-label">
              <span>{{ getStatusText(file.status, file) }}</span>
            </div>
          </div>
          <div class="file-actions">
            <button
              v-if="file.status === 'uploading'"
              type="button"
              @click="uploadManager.pause(file.id)"
              class="action"
              :aria-label="$t('general.pause')"
              :title="$t('general.pause')"
            >
              <i class="material-symbols">pause</i>
            </button>
            <button
              v-if="file.status === 'paused'"
              type="button"
              @click="uploadManager.resume(file.id)"
              class="action"
              :aria-label="$t('general.resume')"
              :title="$t('general.resume')"
            >
              <i class="material-symbols">play_arrow</i>
            </button>
            <button
              v-if="file.status === 'error'"
              type="button"
              @click="uploadManager.retry(file.id)"
              class="action"
              :aria-label="$t('general.retry')"
              :title="$t('general.retry')"
            >
              <i class="material-symbols">replay</i>
            </button>
            <button
              v-if="file.status === 'conflict'"
              type="button"
              @click="handleConflictAction(file)"
              class="action"
              :aria-label="$t('general.replace')"
              :title="$t('general.replace')"
            >
              <i class="material-symbols">sync_problem</i>
            </button>
            <button
              type="button"
              @click="uploadManager.cancel(file.id)"
              class="action"
              :aria-label="$t('general.cancel')"
              :title="$t('general.cancel')"
            >
              <i class="material-symbols">close</i>
            </button>
          </div>
        </div>
        </div>
      </div>
    </div>
  </div>

  <div class="card-actions">
    <button
      v-if="canPauseAll"
      type="button"
      @click="uploadManager.pauseAll()"
      class="button button--flat"
      :aria-label="$t('buttons.pauseAll')"
      :title="$t('buttons.pauseAll')"
    >
      {{ $t("buttons.pauseAll") }}
    </button>
    <button
      v-if="canResumeAll"
      type="button"
      @click="uploadManager.resumeAll()"
      class="button button--flat"
      :aria-label="$t('buttons.resumeAll')"
      :title="$t('buttons.resumeAll')"
    >
      {{ $t("buttons.resumeAll") }}
    </button>
    <button
      type="button"
      @click="uploadManager.clearCompleted()"
      class="button button--flat"
      :disabled="!hasClearable"
      :aria-label="$t('buttons.clearCompleted')"
      :title="$t('buttons.clearCompleted')"
    >
      {{ $t("buttons.clearCompleted") }}
    </button>
  </div>

  <input ref="fileInput" @change="onFilePicked" type="file" multiple style="display: none" />
  <input ref="folderInput" @change="onFolderPicked" type="file" webkitdirectory directory multiple
    style="display: none" />
  <input ref="cameraInput" @change="onFilePicked" type="file" accept="image/*" capture="environment"
    style="display: none" />
  <input ref="videoInput" @change="onFilePicked" type="file" accept="video/*" capture="environment"
    style="display: none" />
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { readAllDirectoryEntries, uploadManager } from "@/utils/upload";
import type { UploadConflictResolution, UploadEntry, UploadItem, UploadStatus } from "@/utils/upload.ts";
import { getTypeInfo, getTypeInfoFromExt } from "@/utils/mimetype";
import { getters, mutations, state } from "@/store";
import { notify } from "@/notify";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";
import ProgressBar from "@/components/ProgressBar.vue";
import SettingsItem from "@/components/settings/SettingsItem.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import ButtonGroup from "@/components/ButtonGroup.vue";
import { findScrollParents, useListingVirtualization } from "@/composables/useListingVirtualization.ts";

/** What `initialItems` can be: dropped items, picked files, or entries pasted from the OS clipboard */
type UploadInput = DataTransferItem[] | File[] | UploadEntry[] | FileList | null;

const props = withDefaults(
  defineProps<{
    promptId?: string | number | null;
    initialItems?: UploadInput;
    filesToReplace?: unknown[];
    targetPath?: string | null;
    targetSource?: string | null;
  }>(),
  {
    promptId: null,
    initialItems: null,
    filesToReplace: () => [],
    targetPath: null,
    targetSource: null,
  },
);

defineOptions({ name: "UploadFiles" });

const { t } = useI18n();

const fileInput = ref<HTMLInputElement | null>(null);
const folderInput = ref<HTMLInputElement | null>(null);
const cameraInput = ref<HTMLInputElement | null>(null);
const videoInput = ref<HTMLInputElement | null>(null);
const list = ref<HTMLElement | null>(null);
const files = computed(() => uploadManager.queue);
const isDragging = ref(false);
const showConflictPrompt = ref(false);
let conflictResolver: ((resolution: UploadConflictResolution) => void) | null = null;
let wakeLock: WakeLockSentinel | null = null;

const orderedItems = computed(() => {
  const active: UploadItem[] = [];
  const finished: UploadItem[] = [];
  for (const entry of files.value) {
    if (entry.status === "completed") finished.push(entry);
    else active.push(entry);
  }
  return [...active, ...finished];
});

const REORDER_INTERVAL_MS = 800;
const sortedItems = shallowRef(orderedItems.value);
let lastReorder = 0;
let reorderTimer: ReturnType<typeof setTimeout> | undefined;

function applyOrder() {
  clearTimeout(reorderTimer);
  reorderTimer = undefined;
  sortedItems.value = orderedItems.value;
  lastReorder = Date.now();
}

watch(orderedItems, (next) => {
  if (next.length !== sortedItems.value.length) {
    applyOrder();
    return;
  }
  if (reorderTimer !== undefined) return;
  reorderTimer = setTimeout(applyOrder, Math.max(0, REORDER_INTERVAL_MS - (Date.now() - lastReorder)));
});

const virtual = useListingVirtualization<{ file: UploadItem; index: number }>({
  container: list,
  groups: () => [
    {
      key: "uploads",
      count: sortedItems.value.length,
      at: (index) => ({ file: sortedItems.value[index] as UploadItem, index }),
    },
  ],
  columnsHint: () => 1,
  scroller: (container) => findScrollParents(container, true),
  itemClass: "upload-item",
  register: false,
});

const uploadSettingsDescription = computed(() => {
  const maxConcurrentUpload = state.user.fileLoading?.maxConcurrentUpload;
  const uploadChunkSizeMb = state.user.fileLoading?.uploadChunkSizeMb;
  if (uploadChunkSizeMb === 0) {
    return t("prompts.uploadSettingsNoChunk", { maxConcurrentUpload });
  }
  return t("prompts.uploadSettingsChunked", { maxConcurrentUpload, uploadChunkSizeMb });
});

// Upload settings
const maxConcurrentUpload = ref(state.user.fileLoading?.maxConcurrentUpload);
const uploadChunkSizeMb = ref(state.user.fileLoading?.uploadChunkSizeMb);
const clearAll = ref(state.user.fileLoading?.clearAll || false);

async function updateUploadSettings() {
  try {
    const data = {
      ...state.user,
      fileLoading: {
        maxConcurrentUpload: maxConcurrentUpload.value,
        uploadChunkSizeMb: uploadChunkSizeMb.value,
        clearAll: clearAll.value,
      },
    };
    void mutations.updateCurrentUser(data);
    notify.showSuccessToast("Upload settings updated");
  } catch (e) {
    console.error(e);
  }
}

function handleConflict(resolver: (resolution: UploadConflictResolution) => void, options: { allowRename?: boolean } = {}) {
  conflictResolver = resolver;
  mutations.showPrompt({
    name: "replace-rename",
    pinned: true,
    props: {
      allowSkip: !getters.isShare(),
      allowRename: options.allowRename !== false,
      ...(getters.isShare() ? {} : { title: t("prompts.uploadConflictTitle") }),
    },
    confirm: (_event: unknown, option: string) => {
      if (option === "overwrite") {
        resolveConflict(true);
      } else if (option === "skip") {
        resolveConflict({ skip: true });
      } else if (option === "rename") {
        showRenamePrompt();
      } else {
        resolveConflict(false);
      }
    },
  });
}

function resolveConflict(overwrite: UploadConflictResolution) {
  if (conflictResolver) {
    conflictResolver(overwrite);
  }
  mutations.closeTopPrompt(); // Only close the conflict prompt, return to upload prompt
}

function showRenamePrompt() {
  mutations.closeTopPrompt(); // Only close the replace-rename prompt, keep upload prompt open
  // Get the conflicting folder name from the upload queue
  const conflictingFolder = uploadManager.getConflictingFolder();
  if (!conflictingFolder) {
    console.error("No conflicting folder found for rename");
    return;
  }

  mutations.showPrompt({
    name: "rename",
    confirm: (newName: string) => {
      void renameUploadFolder(conflictingFolder, newName);
    },
    props: { folderName: conflictingFolder },
  });
}

async function renameUploadFolder(oldName: string, newName: string) {
  try {
    // Check if the new name already exists
    const existingItems = new Set(((state.req.items ?? []) as { name: string }[]).map((i) => i.name));
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
    mutations.closeTopPrompt(); // Only close the rename prompt, return to upload prompt
  } catch (error) {
    console.error(error);
  }
}

async function acquireWakeLock() {
  if (!("wakeLock" in navigator)) {
    return;
  }
  try {
    if (wakeLock !== null) return; // Already locked
    const sentinel = await navigator.wakeLock.request("screen");
    wakeLock = sentinel;
    sentinel.addEventListener("release", () => {
      wakeLock = null;
    });
  } catch (err) {
    const error = err as Error;
    console.error(`Wake Lock failed: ${error.name}, ${error.message}`);
  }
}

function releaseWakeLock() {
  if (wakeLock !== null) {
    void wakeLock.release();
    wakeLock = null;
  }
}

const isUploading = computed(() => state.upload.isUploading);

watch(isUploading, (active) => {
  if (active) {
    void acquireWakeLock();
  } else {
    releaseWakeLock();
  }
});

const hasClearable = computed(() => {
  if (state.user.fileLoading?.clearAll) {
    // For "clear all" mode: check for completed, error, conflict, or paused uploads
    return files.value.some(
      (file) =>
        file.status === "completed" ||
        file.status === "error" ||
        file.status === "conflict" ||
        file.status === "paused",
    );
  }
  // For "clear completed" mode: only check for completed uploads
  return files.value.some((file) => file.status === "completed");
});

const canPauseAll = computed(() => files.value.some((file) => file.status === "uploading"));

const canResumeAll = computed(() => !canPauseAll.value && files.value.some((file) => file.status === "paused"));

function handleVisibilityChange() {
  if (document.visibilityState === "visible" && isUploading.value) {
    void acquireWakeLock();
  }
}

// Helper to get the destination path (from prop or fallback to current request)
function getDestinationPath(): string {
  return props.targetPath || state.req.path || "";
}

const isFile = (item: unknown): item is File => item instanceof File;
const isDataTransferItem = (item: unknown): item is DataTransferItem =>
  typeof (item as DataTransferItem | undefined)?.getAsFile === "function";
const isUploadEntry = (item: unknown): item is UploadEntry =>
  (item as UploadEntry | undefined)?.file instanceof File &&
  typeof (item as UploadEntry).relativePath === "string";

async function processItems(items: UploadInput) {
  const destination = getDestinationPath();
  // When items are passed as a prop from ListingView, they can be either
  // an array of DataTransferItem (from drag and drop) or an array of File (from input).
  // or an array of objects (from clipboard paste of the OS).
  if (Array.isArray(items)) {
    if (items.length === 0) return;
    const first: unknown = items[0];
    if (isFile(first)) {
      // This is an array of File objects from the input fallback in ListingView
      processFileList(items as File[], destination);
    } else if (isDataTransferItem(first)) {
      // This is an array of DataTransferItem from drag and drop in ListingView
      await processDroppedItems(items as DataTransferItem[], destination);
    } else if (isUploadEntry(first)) {
      // This is an array of objects {file, relativePath} for paste from clipboard OS.
      void uploadManager.add(destination, items as UploadEntry[]);
    }
  } else if (items) {
    // This case handles a FileList object from the upload prompt's own input fields.
    // It is not an array, so we convert it.
    processFileList(items, destination);
  }
}

onMounted(async () => {
  document.addEventListener("visibilitychange", handleVisibilityChange);
  uploadManager.setOnConflict(handleConflict);
  if (props.initialItems) {
    await processItems(props.initialItems);
  }
});

onUnmounted(() => {
  clearTimeout(reorderTimer);
  document.removeEventListener("visibilitychange", handleVisibilityChange);
  uploadManager.setOnConflict(() => {}); // cleanup
  releaseWakeLock();
});

const uploadActionButtons = computed(() => [
  { label: t("general.file"), value: "file" },
  { label: t("general.folder"), value: "folder" },
  { label: t("general.camera"), value: "camera" },
  { label: t("general.video"), value: "video" },
]);

function onUploadAction(value: string) {
  switch (value) {
    case "file":
      fileInput.value?.click();
      break;
    case "folder":
      folderInput.value?.click();
      break;
    case "camera":
      cameraInput.value?.click();
      break;
    case "video":
      videoInput.value?.click();
      break;
    default:
      break;
  }
}

function onFilePicked(event: Event) {
  const input = event.target as HTMLInputElement | null;
  const pickedFiles = input?.files;
  if (pickedFiles && pickedFiles.length > 0) {
    processFileList(pickedFiles, getDestinationPath());
  }
  if (input) input.value = "";
}

// Files and folders are handled the same way, the folder structure comes from webkitRelativePath
const onFolderPicked = onFilePicked;

async function onDrop(event: DragEvent) {
  isDragging.value = false;
  const destination = getDestinationPath();
  const transfer = event.dataTransfer;
  if (!transfer) return;
  if (transfer.items) {
    await processDroppedItems(Array.from(transfer.items), destination);
  } else {
    console.log("Upload.vue: Processing dropped files (fallback):", transfer.files);
    processFileList(transfer.files, destination);
  }
}

function onDragEnter() {
  isDragging.value = true;
}

function onDragOver() {
  isDragging.value = true;
}

function onDragLeave() {
  isDragging.value = false;
}

async function getFilesFromDirectoryEntry(entry: FileSystemEntry): Promise<UploadEntry[]> {
  if (entry.isFile) {
    return new Promise((resolve) => {
      (entry as FileSystemFileEntry).file((file) => {
        const relativePath = entry.fullPath.startsWith("/") ? entry.fullPath.substring(1) : entry.fullPath;
        resolve([{ file, relativePath }]);
      });
    });
  }
  if (entry.isDirectory) {
    const reader = (entry as FileSystemDirectoryEntry).createReader();
    const pathLabel = entry.fullPath || entry.name || "";
    const entries: FileSystemEntry[] = await readAllDirectoryEntries(reader, pathLabel);
    const allFiles = await Promise.all(entries.map((subEntry) => getFilesFromDirectoryEntry(subEntry)));
    return allFiles.flat();
  }
  return [];
}

async function processDroppedItems(items: DataTransferItem[], destination: string) {
  const filesToUpload: UploadEntry[] = [];
  const promises = items.map((item) => {
    const entry = item.webkitGetAsEntry();
    if (entry) {
      return getFilesFromDirectoryEntry(entry);
    }
    return Promise.resolve([] as UploadEntry[]);
  });

  const allFiles = await Promise.all(promises);
  for (const group of allFiles) {
    filesToUpload.push(...group);
  }

  if (filesToUpload.length > 0) {
    void uploadManager.add(destination, filesToUpload);
  }
}

function processFileList(fileList: ArrayLike<File>, destination: string) {
  const filesToAdd: UploadEntry[] = Array.from(fileList).map((file) => ({
    file,
    relativePath: file.webkitRelativePath || file.name,
  }));
  if (filesToAdd.length > 0) {
    void uploadManager.add(destination, filesToAdd);
  }
}

function handleConflictAction(file: UploadItem) {
  mutations.showPrompt({
    name: "replace",
    pinned: true,
    confirm: () => {
      uploadManager.retry(file.id, true);
      mutations.closeTopPrompt();
    },
  });
}

function getStatusText(status: UploadStatus, file?: UploadItem): string {
  // Show connection issue in status for paused uploads
  if (status === "paused" && file?.connectionIssue) {
    return "Paused (connection issue)";
  }

  switch (status) {
    case "uploading":
      return t("general.uploading", { suffix: "..." });
    case "completed":
      return t("prompts.completed");
    case "error":
      return t("prompts.error");
    case "paused":
      return t("general.paused");
    case "conflict":
      return t("general.conflict");
    default:
      return status;
  }
}

function fileTypeInfo(file: UploadItem) {
  return file.type === "directory" ? getTypeInfo("directory") : getTypeInfoFromExt(file.name);
}

function getHelpText(file: UploadItem): string {
  if (file.status === "error" && file.errorDetails) {
    return file.errorDetails;
  }
  if (file.status === "paused" && file.connectionIssue) {
    return "Connection stalled - upload paused. Click resume to retry.";
  }
  if (file.connectionIssue && file.status === "error") {
    return file.errorDetails || "Connection issue detected. Click retry to resume.";
  }
  return "";
}
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
  border-radius: var(--borderRadius);
  box-shadow: var(--primaryColor) 0 0 1em;
}

.upload-prompt-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
}

.upload-action-buttons {
  margin: 0.5em 0 0;
}

.upload-prompt i {
  font-size: 4em;
  color: #ccc;
}

.upload-list {
  min-height: 0;
}

.upload-item {
  display: flex;
  align-items: center;
}

.file-icon {
  margin-right: 0.5em;
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
  height: 20px;
  line-height: 20px;
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
  inset: 0;
  background-color: rgb(0 0 0 / 70%);
  z-index: 999;
  display: flex;
  justify-content: center;
  align-items: center;
}

.conflict-overlay .card {
  padding: 1em;
  border-radius: 8px;
}

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

.card-actions {
  display: flex;
  justify-content: flex-end;
}
</style>
