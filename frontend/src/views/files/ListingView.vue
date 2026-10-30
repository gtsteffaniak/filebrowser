<template>
  <div v-if="shareInfo.shareType !== 'upload'" ref="rootEl" class="listing-view-root no-select" :style="containerStyles">
    <!-- Show loading spinner while loading OR if we haven't loaded any data yet -->
    <div v-if="loading">
      <h2 class="message delayed">
        <LoadingSpinner size="medium" />
        <span>{{ $t("general.loading", { suffix: "..." }) }}</span>
      </h2>
    </div>

    <!-- listing container -->
    <div
      v-else
      ref="listingView"
      :class="{
        'add-padding': isStickySidebar,
        [listingViewMode]: true,
        dropping: isDragging,
        'rectangle-selecting': isRectangleSelecting,
        'font-size-large': numDirs + numFiles + numPinned === 0
      }"
      :style="itemStyles"
      class="listing-items"
    >
      <!-- Rectangle selection overlay -->
      <div class="selection-rectangle" :style="rectangleStyle"></div>

      <!-- Drop indicator -->
      <div v-if="isDragging" class="drop-indicator">
        <div class="drop-indicator-content">
          <i class="material-symbols">cloud_upload</i>
          <p>{{ $t("prompts.dragAndDrop") }}</p>
        </div>
      </div>

      <!-- Empty state -->
      <template v-if="numDirs + numFiles + numPinned === 0">
        <h2 class="message font-size-large">
          <i class="material-symbols-outlined">sentiment_dissatisfied</i>
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
      </template>

      <template v-else>
        <!-- Pinned Items Section -->
        <div v-if="numPinned > 0">
          <h2>{{ pinnedHeaderText }}</h2>
        </div>
        <div
          v-if="numPinned > 0"
          class="pinned-items"
          role="group"
          aria-label="Pinned Items"
          :class="{ lastGroup: numDirs === 0 && numFiles === 0, firstGroup: true }"
        >
          <Item
            v-for="(item, idx) in pinnedItems"
            :key="`pinned-${item.path || item.name}`"
            :class="{ 'zebra-row': idx % 2 === 1 }"
            v-bind:index="item.index"
            v-bind:name="item.name"
            v-bind:isDir="item.type === 'directory'"
            v-bind:source="req.source"
            v-bind:modified="item.modified"
            v-bind:created="item.created"
            v-bind:type="item.type"
            v-bind:size="item.size"
            v-bind:path="item.path"
            v-bind:reducedOpacity="item.hidden"
            v-bind:hash="shareInfo.hash"
            v-bind:hasPreview="item.hasPreview"
            v-bind:metadata="item.metadata"
            v-bind:hasDuration="hasDuration"
            v-bind:isShared="item.isShared"
            v-bind:pinned="item.pinned"
            v-bind:viewToken="item.viewToken"
          />
        </div>

        <!-- Directories Section -->
        <div v-if="numDirs > 0">
          <h2>{{ $t("general.folders") }}</h2>
        </div>
        <div
          v-if="numDirs > 0"
          class="folder-items"
          role="group"
          aria-label="Folder Items"
          :class="{ lastGroup: numFiles === 0, firstGroup: numPinned === 0 }"
        >
          <Item
            v-for="(item, idx) in dirs"
            :key="item.path || item.name"
            :class="{ 'zebra-row': (numPinned + idx) % 2 === 1 }"
            v-bind:index="item.index"
            v-bind:name="item.name"
            v-bind:isDir="item.type === 'directory'"
            v-bind:source="req.source"
            v-bind:modified="item.modified"
            v-bind:created="item.created"
            v-bind:type="item.type"
            v-bind:size="item.size"
            v-bind:path="item.path"
            v-bind:reducedOpacity="item.hidden"
            v-bind:hash="shareInfo.hash"
            v-bind:hasPreview="item.hasPreview"
            v-bind:hasDuration="hasDuration"
            v-bind:isShared="item.isShared"
            v-bind:pinned="item.pinned"
            v-bind:viewToken="item.viewToken"
          />
        </div>

        <!-- Files Section -->
        <div v-if="numFiles > 0">
          <h2>{{ $t("general.files") }}</h2>
        </div>
        <div
          v-if="numFiles > 0"
          class="file-items"
          :class="{ lastGroup: numFiles > 0, firstGroup: numPinned === 0 && numDirs === 0 }"
          role="group"
          aria-label="File Items"
        >
          <Item
            v-for="(item, idx) in files"
            :key="item.path || item.name"
            :class="{ 'zebra-row': (numPinned + numDirs + idx) % 2 === 1 }"
            v-bind:index="item.index"
            v-bind:name="item.name"
            v-bind:isDir="item.type === 'directory'"
            v-bind:modified="item.modified"
            v-bind:created="item.created"
            v-bind:source="req.source"
            v-bind:type="item.type"
            v-bind:size="item.size"
            v-bind:path="item.path"
            v-bind:reducedOpacity="item.hidden"
            v-bind:hash="shareInfo.hash"
            v-bind:hasPreview="item.hasPreview"
            v-bind:metadata="item.metadata"
            v-bind:hasDuration="hasDuration"
            v-bind:isShared="item.isShared"
            v-bind:pinned="item.pinned"
            v-bind:viewToken="item.viewToken"
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
      </template>
    </div>
  </div>

  <!-- Upload Share Target -->
  <!-- Only show upload interface if password is validated (or no password required) -->
  <div v-else-if="!shareInfo.hasPassword || shareInfo.passwordValid" ref="rootEl" class="upload-share-embed">
    <Upload />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import type { fileListItemIndex } from "@/store/types";
import { useI18n } from "vue-i18n";
import { readAllDirectoryEntries } from "@/utils/upload";
import { state, mutations, getters } from "@/store";
import { eventBus } from "@/store/eventBus";
import Item from "@/components/files/ListingItem.vue";
import Upload from "@/components/prompts/Upload.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { useListingClipboard } from "@/composables/useListingClipboard";
import { useListingKeyboard } from "@/composables/useListingKeyboard";
import { useListingLayout } from "@/composables/useListingLayout";
import { useMouseSelection } from "@/composables/useRectangleSelection";
import { useListingContextMenu } from "@/composables/useListingContextMenu";

defineOptions({ name: "listingView" });

interface CollectedFile {
  file: File;
  relativePath: string;
}

type PastedItem =
  | { type: "entry"; entry: FileSystemEntry }
  | { type: "file"; file: File };

const { t } = useI18n();

const rootEl = ref<HTMLElement | null>(null);
const listingView = ref<HTMLElement | null>(null);

const dragTargets = reactive(new Set<Element>());

// Non reactive on purpose: big DOM-derived caches and transient interaction state.
let itemNodes: HTMLCollectionOf<Element> | null = null; // .listing-item elements
let itemNodesHost: HTMLElement | null = null;
let contentResizeObserver: ResizeObserver | null = null;
let observedListing: HTMLElement | null = null;

const permissions = computed(() => ({ ...getters.globalPermissions(), ...getters.sourcePermissions() }));
const shareInfo = computed(() => state.shareInfo);
const isDragging = computed(() => {
  const el = rootEl.value;
  if (!el) return false;
  if (getters.isShare()) {
    return !!state.shareInfo.allowCreate && dragTargets.has(el);
  }
  return dragTargets.has(el);
});
const scrolling = computed(() => state.listing.listingScrollTop);
const isStickySidebar = computed(() => getters.isStickySidebar());
const items = computed(() => getters.reqItems());
const pinnedItems = computed(() => items.value.pinned || []);
const dirs = computed(() => items.value.dirs);
const files = computed(() => items.value.files);
const allItems = computed(() => pinnedItems.value.concat(dirs.value, files.value));
const numPinned = computed(() => pinnedItems.value.length);
const numFiles = computed(() => files.value.length);
const numDirs = computed(() => dirs.value.length);
const hasDuration = computed(() => {
  // Check if any pinned file or regular file has duration metadata
  const hasMetadataDuration = (file: fileListItemIndex) =>
    file.type !== "directory" && file.metadata?.duration;
  return pinnedItems.value.some(hasMetadataDuration) || files.value.some(hasMetadataDuration);
});
const pinnedHeaderText = computed(() => {
  const pinnedFolders = pinnedItems.value.filter((item) => item.type === "directory").length;
  const pinnedFiles = pinnedItems.value.filter((item) => item.type !== "directory").length;
  if (pinnedFolders > 0 && pinnedFiles === 0) {
    return t("general.pinnedFolders");
  }
  if (pinnedFiles > 0 && pinnedFolders === 0) {
    return t("general.pinnedFiles");
  }
  return t("general.pinned");
});
const listingViewMode = computed(() => getters.viewMode());
const selectedCount = computed(() => state.selected.length);
const req = computed(() => state.req);
const loading = computed(() => getters.isLoading());
const containerStyles = computed(() => {
  // Dynamic padding-top: applied to the entire container (loading spinner + listing items)
  const isEmpty = numDirs.value + numFiles.value + numPinned.value === 0;
  const isRootPath = state.req.path === "/" || !state.req.path;
  if (isEmpty) {
    return { "padding-top": "4.1em" }; // Empty - no files or folders
  } else if (isRootPath) {
    return { "padding-top": "4.29em" }; // Root - no breadcrumbs showing
  } else {
    return { "padding-top": "7.51em" }; // Non-root - breadcrumbs + listing header
  }
});
const isListingView = () => getters.currentView() === "listingView";

const { isRectangleSelecting, rectangleStyle } = useMouseSelection({
  container: rootEl,
  bounds: listingView,
  items: () => allItems.value,
  isActive: isListingView,
});

useListingKeyboard({
  container: rootEl,
  items: () => allItems.value,
  isActive: isListingView,
  canModify: () => !!permissions.value?.modify,
  onEscape: () => {
    if (dragTargets.size === 0) return false;
    dragTargets.clear();
    return true;
  },
});

useListingContextMenu(rootEl, { getProps: () => ({ createOnly: selectedCount.value === 0 }) });

const { itemStyles } = useListingLayout({ container: listingView });

const { handleInternalPaste, hasInternalItems, isInternalRecent } = useListingClipboard();

watch(loading, () => {
  void nextTick(observeListingContent);
});

watch(scrolling, () => {
  const scrollContainer = listingView.value;
  const root = rootEl.value;
  if (!scrollContainer || !root) return;

  // Select all visible listing items
  const nodes = getListingItemNodes(scrollContainer);
  // Find the first item at/below the top of the viewport
  let lo = 0;
  let hi = nodes.length;
  while (lo < hi) {
    const mid = (lo + hi) >>> 1;
    if ((nodes[mid] as Element).getBoundingClientRect().top >= 0) {
      hi = mid;
    } else {
      lo = mid + 1;
    }
  }
  const topItem = nodes[lo] ?? null;
  // Decide category by checking which section is above
  let letter = "A";
  let category = "folders"; // Default category

  if (topItem) {
    letter = topItem.getAttribute("data-name")?.[0]?.toUpperCase() || "A";
  } else if (numPinned.value > 0) {
    const pinnedHeader = root.querySelector(".pinned-items h2");
    if (pinnedHeader && pinnedHeader.getBoundingClientRect().top >= 0) {
      category = "pinned";
      const firstPinned = pinnedItems.value[0];
      letter = firstPinned?.name?.[0]?.toUpperCase() as string;
    }
  }

  if (topItem?.closest(".pinned-items")) {
    category = "pinned";
  }
  else if (numFiles.value > 0) {
    const fileSection = root.querySelector(".file-items");
    const fileTop = fileSection?.getBoundingClientRect().top ?? 0;
    category = fileTop <= 0 ? "files" : "folders";
  }
  if (numDirs.value === 0 && category !== "pinned") {
    category = "files"; // If no directories, only files
  }

  mutations.updateListing({
    ...state.listing,
    category,
    letter,
  });
});

onMounted(() => {
  mutations.setSearch(false);
  const el = rootEl.value;
  contentResizeObserver = new ResizeObserver(() => eventBus.emit("scrollContentResized"));
  observeListingContent();
  window.addEventListener("dragover", preventDefault);
  window.addEventListener("paste", handlePaste);

  // Single dragend listener for all items
  document.addEventListener("dragend", handleGlobalDragEnd, { passive: true });
  el?.addEventListener("dragenter", dragEnter);
  el?.addEventListener("dragleave", dragLeave);
  el?.addEventListener("drop", drop);
});

onBeforeUnmount(() => {
  contentResizeObserver?.disconnect();
  const el = rootEl.value;

  // Remove event listeners before destroying this page.
  window.removeEventListener("dragover", preventDefault);
  window.removeEventListener("paste", handlePaste);
  document.removeEventListener("dragend", handleGlobalDragEnd);

  // Also clean up drag/drop listeners on the component's root element
  el?.removeEventListener("dragenter", dragEnter);
  el?.removeEventListener("dragleave", dragLeave);
  el?.removeEventListener("drop", drop);
});

function observeListingContent() {
  const el = listingView.value || null;
  if (!contentResizeObserver || el === observedListing) return;
  contentResizeObserver.disconnect();
  observedListing = el;
  if (el) contentResizeObserver.observe(el);
}

function handleGlobalDragEnd() {
  // Reset drag state for all items (replaces per-item dragend listeners)
  const listingItems = rootEl.value?.querySelectorAll(".listing-item.drag-hover, .listing-item.half-selected");
  if (listingItems) {
    for (const el of listingItems) {
      el.classList.remove("drag-hover", "half-selected");
    }
  }
  dragTargets.clear();
}

// stays in sync with the DOM and avoids rebuilding a NodeList on every call
function getListingItemNodes(container: HTMLElement) {
  if (itemNodesHost !== container || !itemNodes) {
    itemNodesHost = container;
    itemNodes = container.getElementsByClassName("listing-item");
  }
  return itemNodes;
}

function preventDefault(event: Event) {
  event.preventDefault();
}

async function collectFilesFromEntry(entry: FileSystemEntry, relativePath = ""): Promise<CollectedFile[]> {
  const collected: CollectedFile[] = [];
  const entryPath = relativePath ? `${relativePath}/${entry.name}` : entry.name;

  if (entry.isFile) {
    // If it's a file we get the File object and add it with its relative path
    const file = await new Promise<File>((resolve, reject) => {
      (entry as FileSystemFileEntry).file(resolve, reject);
    });
    collected.push({
      file,
      relativePath: entryPath,
    });
  } else if (entry.isDirectory) {
    // But if it's a directory, read the contents (readEntries may require
    // multiple calls — browsers often return at most ~100 entries per batch)
    const reader = (entry as FileSystemDirectoryEntry).createReader();
    const entries: FileSystemEntry[] = await readAllDirectoryEntries(reader, entryPath);
    // and then for each child recursively collect files
    for (const childEntry of entries) {
      const childFiles = await collectFilesFromEntry(
        childEntry,
        entryPath
      );
      collected.push(...childFiles);
    }
  }
  return collected;
}

async function getClipboardItems(pastedItems: PastedItem[]) {
  const collected: CollectedFile[] = [];
  for (const item of pastedItems) {
    if (item.type === "entry") {
      collected.push(...await collectFilesFromEntry(item.entry));
    } else if (item.type === "file") {
      collected.push({ file: item.file, relativePath: item.file.name });
    }
  }
  return collected;
}

async function handlePaste(event: ClipboardEvent) {
  // We can paste multiple files from clipboard and upload all of them at once, but
  // Firefox has an issue https://bugzilla.mozilla.org/show_bug.cgi?id=864052  (12 years old lol)
  // the clipboard entry is always getting 1 file (the first file/folder in selection),
  // while chromium sees all and has no issues with it. Not sure about safari.
  if (getters.currentView() !== "listingView" || getters.currentPromptName()) return;

  const internalRecent = isInternalRecent();

  // If internal is recent (<20s), use it immediately, in case someone have both: internal and external clipboard with a file entry.
  // After those 20s, if the OS clipboard has a file as most recent entry, will use that.
  if (internalRecent) {
    await handleInternalPaste();
    event.preventDefault();
    event.stopPropagation();
    return;
  }

  if (event.clipboardData?.items) {
    // Collect all items from clipboard
    const collectedItems: PastedItem[] = [];
    // And loop through all items
    for (const item of Array.from(event.clipboardData.items)) {
      if (item.kind !== "file") continue;

      const entry = item.webkitGetAsEntry();
      if (entry) {
        collectedItems.push({ type: "entry", entry });
      } else {
        const file = item.getAsFile();
        if (file) {
          collectedItems.push({ type: "file", file });
        }
      }
    }

    // Then process the collected items (files and folders) asynchronously
    const itemsFromClipboard = await getClipboardItems(collectedItems);
    const pastedFiles = itemsFromClipboard.map((item) => item.file);

    // If found external files, upload them
    if (pastedFiles.length > 0) {
      event.preventDefault();
      event.stopPropagation();

      const canUpload = getters.sourcePermissions()?.modify;
      if (canUpload) {
        // Pass the full array of {file, relativePath} to preserve directory structure
        mutations.showPrompt({
          name: "upload",
          props: {
            initialItems: itemsFromClipboard,
            targetPath: state.req.path,
          },
        });
      }
      return;
    }
  }
  // If internal clipboard exists but is not recent (>20s), and we don't have any file in clipboard, use the internal one
  if (hasInternalItems()) {
    console.log("No external files, using internal clipboard.");
    await handleInternalPaste();
    event.preventDefault();
    event.stopPropagation();
    return;
  }
}

function dragEnter(event: DragEvent) {
  // If in upload share mode, let the embedded Upload component handle it
  if (state.shareInfo?.shareType === "upload") {
    return;
  }
  const isInternal = Array.from(event.dataTransfer?.types ?? []).includes(
    "application/x-filebrowser-internal-drag"
  );
  if (isInternal) return;
  if (!getters.sourcePermissions().create) return;

  const el = rootEl.value;
  if (el && !el.contains(event.relatedTarget as Node | null)) {
    dragTargets.add(el);
  }
}

function dragLeave(event: DragEvent) {
  // If in upload share mode, let the embedded Upload component handle it
  if (state.shareInfo?.shareType === "upload") {
    return;
  }
  const isInternal = Array.from(event.dataTransfer?.types ?? []).includes(
    "application/x-filebrowser-internal-drag"
  );
  if (isInternal) return;

  const el = rootEl.value;
  if (el && !el.contains(event.relatedTarget as Node | null)) {
    dragTargets.delete(el);
  }
}

async function drop(event: DragEvent) {
  event.preventDefault();
  // The drop listeners are always attached, so this is the gate: the source may be one the user
  // cannot write to, or may have turned out read-only since dragEnter when its info arrived.
  // Covers shares too (create = allowCreate). Clear the drop indicator dragEnter may have shown.
  if (!getters.sourcePermissions().create) {
    dragTargets.clear();
    return;
  }
  const isInternal = Array.from(event.dataTransfer?.types ?? []).includes(
    "application/x-filebrowser-internal-drag"
  );

  if (isInternal) {
    return;
  }
  await handleDrop(event);
}

async function uploadInput(event: Event) {
  await handleDrop(event);
}

async function handleDrop(event: Event) {
  event.preventDefault();
  if (state.shareInfo?.shareType === "upload") {
    return;
  }

  if (event.type === "drop") {
    mutations.showPrompt({
      name: "upload",
      props: {
        initialItems: Array.from((event as DragEvent).dataTransfer?.items ?? []),
      },
    });
  } else {
    const inputFiles = (event.target as HTMLInputElement).files;
    if (!inputFiles || inputFiles.length === 0) {
      return;
    }

    mutations.showPrompt({
      name: "upload",
      props: {
        initialItems: Array.from(inputFiles),
      },
    });
  }
  dragTargets.clear();
}

</script>

<style scoped>
.listing-view-root {
  height: 100%;
  display: flex;
  flex-direction: column;
  padding-right: 0.5em;
}

.add-padding {
  padding-left: 0.5em;
}

.font-size-large h2 {
  font-size: 2em !important;
}

.listing-items.dropping .listing-item:not(.activebutton, .drag-hover) {
  opacity: 0.5;
}

.listing-items.dropping {
  margin: 0.5em;
  overflow: hidden;
  min-height: 0;
  flex: 1 1 0;
}

.listing-items {
  position: relative;
  flex: 1;
}

.listing-item .pinned-indicator {
  font-size: 1rem;
}

.listing-items:not(.list, .compact) .folder-items a {
  border-width: var(--borderWidth);
  border-style: solid;
}

/* Upload Share Styles */
.upload-share-embed {
  padding: 2em;
  max-width: 768px;
  margin: 0 auto;
}

.drop-indicator {
  position: absolute;
  inset: 0;
  bottom: 2.5em;
  z-index: 50;
  border: 0.2em dashed var(--primaryColor);
  background: rgb(0 0 0 / 30%);
  backdrop-filter: blur(2px);
  border-radius: var(--borderRadius);
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
  box-shadow: var(--primaryColor) 0 0 1em;
}

.drop-indicator-content {
  text-align: center;
  color: var(--textPrimary);
  font-size: 1.5em;
}

.drop-indicator-content i {
  font-size: 4em;
}
</style>
