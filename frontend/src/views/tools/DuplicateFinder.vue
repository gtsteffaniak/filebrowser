<template>
  <div class="duplicate-finder">
    <div class="card duplicate-finder-config padding-normal">
      <div class="card-content">
        <PathPickerButton
          v-model:path="searchPath"
          v-model:source="selectedSource"
          aria-label="duplicate-finder-path"
          :show-files="false"
          :show-folders="true"
          :placeholder="$t('sidebar.chooseSource')"
        />
        <h3>{{ $t('duplicateFinder.minSize') }}</h3>
        <input aria-label="Minimum size input" v-model.number="minSizeValue" type="number" min="0" placeholder="1" class="input" />
        <p class="hint">{{ $t('duplicateFinder.minSizeHint') }}</p>
        <button
          type="button"
          aria-label="Find duplicates button"
          @click="fetchData"
          class="button"
          :disabled="loading"
        >
          <i v-if="loading" class="material-symbols spin">autorenew</i>
          <span v-else>{{ $t('duplicateFinder.findDuplicates') }}</span>
        </button>
      </div>
    </div>

    <div class="card duplicate-finder-results padding-normal">
      <div v-if="error" class="error-message">
        {{ error }}
      </div>

      <div v-if="duplicateGroups.length > 0 && resultsApplyToInputs">
        <div class="card-title">
          <h2>{{ $t('general.results') }}</h2>
        </div>
        <div class="card-content">
          <div class="stats">
            <span>{{ $t('duplicateFinder.groupsFound', { suffix: ': ' }) }}<strong>{{ duplicateGroups.length }}</strong></span>
            <span>{{ $t('duplicateFinder.totalWastedSpace', { suffix: ': ' }) }}<strong>{{ humanSize(totalWastedSpace) }}</strong></span>
          </div>

          <!-- Show timeout/limit warning first if applicable -->
          <div v-if="isIncomplete" class="warning-message">
            <i class="material-symbols">warning</i>
            <div>
              <strong>{{ $t('fileSizeAnalyzer.incompleteResults') }}</strong> {{ incompleteReason }}
            </div>
          </div>
          <!-- Show complete/maxGroups warning if no timeout -->
          <div v-else-if="duplicateGroups.length < maxGroups" class="success-message">
            <i class="material-symbols">check_circle</i>
            <div>
              <strong>{{ $t('fileSizeAnalyzer.completeResults') }}</strong>
            </div>
          </div>
          <div v-else class="warning-message">
            <i class="material-symbols">warning</i>
            <div>
              <strong>{{ $t('fileSizeAnalyzer.incompleteResults') }}</strong> {{ $t('messages.incompleteResultsDetails', { max: maxGroups }) }}
            </div>
          </div>
          <div ref="list" class="duplicate-groups">
            <template v-for="(group, index) in duplicateGroups" :key="index">
              <div class="group-header">
                <span class="group-title">{{ $t('general.group') }} {{ index + 1 }}</span>
                <span class="group-size">{{ getGroupSizeText(group) }}</span>
                <span class="wasted-space">{{ $t('duplicateFinder.wasted', { suffix: ': ' }) }} {{ humanSize(group.size * (group.count - 1)) }}</span>
              </div>
              <div class="group-files" :data-virtual-group="`g${index}`" :style="virtual.group(`g${index}`).style">
                <div
                  v-for="entry in virtual.group(`g${index}`).items"
                  :key="entry.index"
                  class="file-item-wrapper"
                  :class="{ 'deleted': isDeleted(entry.file), 'failed': isFailed(entry.file) }"
                >
                  <ListingItem
                    :name="getFileName(entry.file.path)"
                    :isDir="entry.file.type === 'directory'"
                    :source="entry.file.source"
                    :type="entry.file.type"
                    :size="entry.file.size"
                    :modified="entry.file.modified"
                    :created="entry.file.created"
                    :index="entry.index"
                    :path="getFullPath(entry.file.path)"
                    :hasPreview="entry.file.hasPreview || false"
                    :reducedOpacity="isDeleted(entry.file)"
                    :displayFullPath="true"
                    :updateGlobalState="false"
                    :isSelectedProp="selectedIndices.has(entry.index)"
                    :clickable="false"
                    :showLimitedOptions="true"
                    @select="handleItemSelect"
                    @clearSelection="clearSelection"
                    @selectRange="handleSelectRange"
                  />
                </div>
              </div>
            </template>
          </div>
        </div>
      </div>
      <div v-else-if="!loading" class="empty-state">
        <template v-if="showNoDuplicateResults">
          <i v-if="isIncomplete" class="material-symbols-outlined">warning</i>
          <i v-else class="material-symbols-outlined">check_circle</i>
          <template v-if="isIncomplete">
            <p><strong>{{ $t('fileSizeAnalyzer.incompleteResults') }}</strong></p>
            <p v-if="incompleteReason">{{ incompleteReason }}</p>
          </template>
          <p v-else>{{ $t('duplicateFinder.noResults') }}</p>
        </template>
        <template v-else>
          <i class="material-symbols-outlined">content_copy</i>
          <p>{{ $t('duplicateFinder.emptyState') }}</p>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { toolsApi } from "@/api";
import { router } from "@/router";
import { state, mutations } from "@/store";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { eventBus } from "@/store/eventBus";
import ListingItem from "@/components/files/ListingItem.vue";
import PathPickerButton from "@/components/files/PathPickerButton.vue";
import { useListingVirtualization } from "@/composables/useListingVirtualization.ts";
import { globalVars } from "@/utils/constants";

defineOptions({ name: "DuplicateFinder" });

interface DuplicateFile {
  path: string;
  type?: string;
  size?: number;
  modified?: string;
  created?: string;
  source?: string;
  hasPreview?: boolean;
}

interface DuplicateGroup {
  size: number;
  count: number;
  files: DuplicateFile[];
}

interface BulkDeleteEntry {
  source: string;
  path: string;
  message?: string;
}

interface BulkDeleteResult {
  succeeded?: BulkDeleteEntry[];
  failed?: BulkDeleteEntry[];
}

const GROUP_STRIDE = 1_000_000;

const { t } = useI18n();
const route = useRoute();

const list = ref<HTMLElement | null>(null);
const searchPath = ref("/");
const selectedSource = ref("");
const resultPath = ref("/");
const resultSource = ref("");
const minSizeValue = ref(10);
const loading = ref(false);
const hasSearched = ref(false);
const searchedCriteria = ref<{ path: string; source: string; minSize: number } | null>(null);
const error = ref<string | null>(null);
const duplicateGroups = shallowRef<DuplicateGroup[]>([]);
const isIncomplete = ref(false);
const incompleteReason = ref("");
let lastRequestTime = 0;
const maxGroups = 500;
const selectedIndices = ref(new Set<number>());
const deletedFiles = ref(new Set<string>());
const failedFiles = ref(new Map<string, string>());
let isInitializing = true;

const resultsApplyToInputs = computed(() => {
  if (!searchedCriteria.value) {
    return false;
  }
  const s = searchedCriteria.value;
  return s.path === searchPath.value &&
    s.source === selectedSource.value &&
    s.minSize === minSizeValue.value;
});

const showNoDuplicateResults = computed(() =>
  hasSearched.value && resultsApplyToInputs.value && duplicateGroups.value.length === 0,
);

const totalWastedSpace = computed(() => {
  return duplicateGroups.value.reduce((sum, group) => {
    // Wasted space = size × (count - 1)
    // We keep one copy, so the rest is wasted
    return sum + (group.size * (group.count - 1));
  }, 0);
});

// one virtual group per duplicate group, only the rows around the screen are rendered
const virtual = useListingVirtualization({
  container: list,
  groups: () =>
    duplicateGroups.value.map((group, groupIndex) => ({
      key: `g${groupIndex}`,
      count: group.files.length,
      at: (fileIndex: number) => ({
        file: group.files.at(fileIndex) as DuplicateFile,
        index: getUniqueIndex(groupIndex, fileIndex),
      }),
    })),
  columnsHint: () => 1,
  itemClass: "file-item-wrapper",
});

watch([searchPath, selectedSource, minSizeValue], () => {
  if (!isInitializing) updateUrl();
});

onMounted(() => {
  document.title = `${globalVars.name} - ${t('tools.title')} - ${t('tools.duplicateFinder.name')}`;

  // Initialize from URL query params or use defaults
  const query = route.query;

  searchPath.value = (typeof query.path === 'string' ? query.path : null) || "/";
  selectedSource.value = (typeof query.source === 'string' ? query.source : null) || state.sources.current || Object.keys(state.sources.info || {})[0] || "";

  if (query.minSize) {
    const parsed = parseInt(String(query.minSize), 10);
    if (!Number.isNaN(parsed)) {
      minSizeValue.value = parsed;
    }
  }
  isInitializing = false;
  eventBus.on('itemsDeleted', handleItemsDeleted);
  eventBus.on('duplicateFinderDeleteRequested', showDeleteConfirm);
  eventBus.on('duplicateFinderClearRequested', clearSelection);
});

onBeforeUnmount(() => {
  // Clear local selection when leaving
  selectedIndices.value.clear();

  eventBus.off('itemsDeleted', handleItemsDeleted);
  eventBus.off('duplicateFinderDeleteRequested', showDeleteConfirm);
  eventBus.off('duplicateFinderClearRequested', clearSelection);

  // Notify Shelf.vue that selection is cleared
  eventBus.emit('duplicateFinderSelectionChanged', 0);
  eventBus.emit('duplicateFinderDeletingChanged', false);
});

function handleItemsDeleted(data?: BulkDeleteResult) {
  // Update local state when items are deleted from the delete prompt
  if (data?.succeeded) {
    for (const item of data.succeeded) {
      const key = `${item.source}::${item.path}`;
      deletedFiles.value.add(key);
      failedFiles.value.delete(key);
    }
  }
  if (data?.failed) {
    for (const item of data.failed) {
      const key = `${item.source}::${item.path}`;
      failedFiles.value.set(key, item.message || 'Unknown error');
    }
  }
  clearSelection();
}

async function fetchData() {
  // Prevent rapid-fire requests - require at least 1 second between requests
  const now = Date.now();
  if (loading.value) {
    return; // Already loading
  }
  if (now - lastRequestTime < 1000) {
    console.log("Duplicate search throttled - please wait before searching again");
    return;
  }

  loading.value = true;
  error.value = null;
  lastRequestTime = now;
  const requestPath = searchPath.value;
  const requestSource = selectedSource.value;
  const requestMinSize = minSizeValue.value;

  try {
    // API now expects minSizeMb directly (in megabytes)
    // Always use false for checksums due to performance issues
    const result = await toolsApi.duplicateFinder(
      requestPath,
      requestSource,
      minSizeValue.value,
      false // Checksum disabled
    );

    if (
      requestPath !== searchPath.value ||
      requestSource !== selectedSource.value ||
      requestMinSize !== minSizeValue.value
    ) {
      return;
    }

    resultPath.value = requestPath;
    resultSource.value = requestSource;
    duplicateGroups.value = result.groups || [];
    isIncomplete.value = result.incomplete || false;
    incompleteReason.value = result.reason || "";
    searchedCriteria.value = {
      path: requestPath,
      source: requestSource,
      minSize: requestMinSize,
    };

    hasSearched.value = true;
    clearSelection();
    deletedFiles.value.clear();
    failedFiles.value.clear();
  } catch (err) {
    error.value = (err as Error).message || "Failed to find duplicates";
    duplicateGroups.value = [];
    isIncomplete.value = false;
    incompleteReason.value = "";
    hasSearched.value = false;
    searchedCriteria.value = null;
    clearSelection();
    deletedFiles.value.clear();
    failedFiles.value.clear();
  } finally {
    loading.value = false;
  }
}

function updateUrl() {
  if (!route.path.startsWith('/tools/duplicateFinder')) return;
  const query: Record<string, string> = {};

  if (searchPath.value && searchPath.value !== "/") {
    query.path = searchPath.value;
  }

  if (selectedSource.value) {
    query.source = selectedSource.value;
  }

  if (minSizeValue.value !== 1) {
    query.minSize = String(minSizeValue.value);
  }

  router.replace({
    path: route.path,
    query: Object.keys(query).length > 0 ? query : undefined,
  }).catch(() => {});
}

function humanSize(size: number) {
  return getHumanReadableFilesize(size);
}

function getGroupSizeText(group: DuplicateGroup) {
  return `${humanSize(group.size)} × ${group.count} = ${humanSize(group.size * group.count)}`;
}

function getFileName(path: string) {
  const parts = path.split("/").filter(p => p);
  return parts[parts.length - 1] || path;
}

function ensureLeadingSlash(path: string) {
  return path.startsWith('/') ? path : `/${path}`;
}

function getFullPath(filePath: string) {
  const basePath = resultPath.value || '/';
  const normalizedSearchPath = basePath === '/' || basePath === ''
    ? ''
    : (basePath.endsWith('/') ? basePath.slice(0, -1) : basePath);

  if (filePath.startsWith(`${normalizedSearchPath}/`) ||
      filePath === normalizedSearchPath ||
      (normalizedSearchPath === '' && filePath.startsWith('/'))) {
    return ensureLeadingSlash(filePath);
  }

  if (normalizedSearchPath === '') {
    return ensureLeadingSlash(filePath);
  }

  // Remove leading slash from file path if present before combining
  const cleanFilePath = filePath.startsWith('/') ? filePath.substring(1) : filePath;
  return `${normalizedSearchPath}/${cleanFilePath}`;
}

function getUniqueIndex(groupIndex: number, fileIndex: number) {
  return groupIndex * GROUP_STRIDE + fileIndex;
}

function getFileKey(file: DuplicateFile) {
  // Create a unique key for the file based on source and path
  return `${resultSource.value}::${getFullPath(file.path)}`;
}

function handleItemSelect({ index }: { index: number }) {
  if (selectedIndices.value.has(index)) {
    selectedIndices.value.delete(index);
  } else {
    selectedIndices.value.add(index);
  }
  eventBus.emit('duplicateFinderSelectionChanged', selectedIndices.value.size);
}

function handleSelectRange({ startIndex, endIndex }: { startIndex: number; endIndex: number }) {
  const start = Math.min(startIndex, endIndex);
  const end = Math.max(startIndex, endIndex);
  for (let groupIndex = Math.floor(start / GROUP_STRIDE); groupIndex <= Math.floor(end / GROUP_STRIDE); groupIndex++) {
    const count = duplicateGroups.value.at(groupIndex)?.files.length ?? 0;
    for (let fileIndex = 0; fileIndex < count; fileIndex++) {
      const index = getUniqueIndex(groupIndex, fileIndex);
      if (index >= start && index <= end) selectedIndices.value.add(index);
    }
  }
  eventBus.emit('duplicateFinderSelectionChanged', selectedIndices.value.size);
}

function clearSelection() {
  selectedIndices.value.clear();
  eventBus.emit('duplicateFinderSelectionChanged', 0);
}

function isDeleted(file: DuplicateFile) {
  return deletedFiles.value.has(getFileKey(file));
}

function isFailed(file: DuplicateFile) {
  return failedFiles.value.has(getFileKey(file));
}

function showDeleteConfirm() {
  if (selectedIndices.value.size === 0) {
    return;
  }
  const items = [];
  for (const selectedIndex of selectedIndices.value) {
    const file = duplicateGroups.value.at(Math.floor(selectedIndex / GROUP_STRIDE))?.files.at(selectedIndex % GROUP_STRIDE);
    if (!file) continue;
    items.push({
      source: resultSource.value,
      path: getFullPath(file.path),
      type: file.type,
      size: file.size,
      modified: file.modified,
    });
  }

  mutations.showPrompt({
    name: "delete",
    props: {
      items: items,
    },
  });
}
</script>

<style scoped>
.duplicate-finder {
  max-width: 1200px;
  margin-left: auto;
  margin-right: auto;
  padding: 1em;
}

.duplicate-finder-results {
  margin-bottom: 2em;
}

.button {
  margin-top: 1rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.button .material-symbols {
  font-size: 1.2rem;
}

.error-message {
  background: #fee;
  color: var(--dark-red);
  padding: 1rem;
  border-radius: 4px;
  margin-bottom: 1rem;
  border: 1px solid color-mix(in srgb, var(--red), transparent 60%);
}

.hint {
  font-size: 0.85rem;
  color: var(--textSecondary);
  margin-top: 0.25rem;
  margin-bottom: 1rem;
}

.stats {
  display: flex;
  gap: 2rem;
  margin-bottom: 1.5rem;
  padding: 1rem;
  background: var(--surfaceSecondary);
  border-radius: 4px;
}

.duplicate-groups {
  --icon-size: 2.5em;
  --icon-font-size: 1.8em;

  display: flex;
  flex-direction: column;
}

.group-header {
  background: var(--surfaceSecondary);
  padding: 1rem;
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-top: 1.5rem;
  border: 1px solid var(--divider);
  border-bottom-color: rgb(0 0 0 / 10%);
  border-radius: var(--borderRadius) var(--borderRadius) 0 0;
}

.group-header:first-child {
  margin-top: 0;
}

.group-title {
  font-weight: 600;
  font-size: 1rem;
}

.group-size {
  color: var(--textSecondary);
  font-size: 0.9rem;
}

.wasted-space {
  margin-left: auto;
  color: var(--icon-red);
  font-weight: 600;
  font-size: 0.9rem;
}

.group-files {
  display: flex;
  flex-direction: column;
  padding-top: 0.3em;
  padding-bottom: 0.3em;
  border: 1px solid var(--divider);
  border-top: 0;
  border-radius: 0 0 var(--borderRadius) var(--borderRadius);
  overflow: hidden;
}

.file-item-wrapper {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  border: 1px solid rgb(0 0 0 / 10%);
  border-top: 0;
  padding: 0.2em 1.2%;
  border-radius: 0;
}

.file-item-wrapper.deleted {
  opacity: 0.5;
  background: var(--surfaceSecondary);
}

.file-item-wrapper.failed {
  border-left: 3px solid var(--icon-red);
}

.file-item-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}


.group-files .listing-item {
  width: 100%;
  margin: 0;
  cursor: pointer;
  content-visibility: visible;
}

/* Highlight selected items */
.group-files .listing-item.activebutton {
  background: var(--primaryColor) !important;
  color: #fff !important;
}

.path-text {
  font-size: 0.75rem !important;
  color: var(--textSecondary) !important;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.empty-state {
  text-align: center;
  padding: 4rem 2rem;
  color: var(--textSecondary);
}

.empty-state .material-symbols {
  font-size: 4rem;
  opacity: 0.3;
  margin-bottom: 1rem;
}

.empty-state p {
  font-size: 1.1rem;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

@media (width <= 768px) {
  .duplicate-finder {
    padding: 1rem;
  }
  .stats {
    flex-direction: column;
    gap: 0.5rem;
  }
  .group-header {
    flex-wrap: wrap;
  }
  .wasted-space {
    margin-left: 0;
    width: 100%;
  }
}
</style>
