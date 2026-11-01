<template>
  <div v-if="active" id="search" :class="{ active, ongoing: isRunning, 'prompt-open': promptOpen, halloween: eventTheme === 'halloween' }" @click="clearContext">
    <!-- Search input section -->
    <div class="search-input-container">
      <!-- Close button visible when search is active -->
      <button v-if="active" type="button" class="action" @click="close" :aria-label="$t('general.close')"
        :title="$t('general.close')">
        <i class="material-symbols">close</i>
      </button>
      <!-- Search icon when search is not active -->
      <i v-else class="material-symbols">search</i>
      <!-- Input field for search -->
      <input id="search-input" type="text"
        @keyup.exact="keyup" @input="onInput" ref="input" :autofocus="active" v-model.trim="value"
        aria-label="search input" :placeholder="$t('general.search', { suffix: '...' })" />
      <div v-if="eventTheme === 'halloween'" class="halloween-eyes" aria-hidden="true">
        <div class="eye"><div class="pupil"></div></div>
        <div class="eye right"><div class="pupil"></div></div>
      </div>
    </div>
    <div v-show="active" id="results" ref="result">
      <div class="inputWrapper">
        <ExpandDropdown
          v-if="multipleSources"
          v-model="selectedSource"
          class="searchContext"
          transparent
          :options="searchSourceOptions"
          all-value="__all__"
          :all-selected-label="$t('general.all')"
          aria-label="search sources dropdown"
          @update:model-value="updateSource"
        />

        <!-- Formatted display of selected value -->
        <div class="searchContext">{{ $t("search.searchContext", { context: getContext }) }}</div>
      </div>

      <div id="result-list" ref="resultListEl">
        <div v-if="!disableSearchOptions">
          <div v-if="active">
            <SearchFilters
              v-model:expanded="advancedOptionsExpanded"
              class="search-options-settings-item"
              :filters="filters"
            >
              <template #extra-settings>
                <ToggleSwitch
                  class="item"
                  v-model="showPreviewImages"
                  :name="$t('search.showPreviewImages')"
                  :description="$t('search.showPreviewImagesDescription')"
                />
              </template>
            </SearchFilters>
          </div>
        </div>
        <!-- Loading icon when search is ongoing -->
          <LoadingSpinner v-if="isRunning" size="medium" />
        <!-- Message when no results are found -->
        <div class="searchPrompt" v-show="isEmpty && !isRunning">
          <p>{{ noneMessage }}</p>
          <HelpTooltipIcon :text="searchHelpText" />
        </div>
        <!-- List of search results -->
        <ul v-show="results.length > 0">
          <li
            v-for="(s, k) in results"
            :key="`${s.source}::${s.path}`"
            class="search-entry clickable"
            :class="{
              'zebra-row': k % 2 === 1,
              active: activeStates[k],
              'current-item': itemState[k] || k === highlightedIndex,
              'large-icons': showPreviewImages,
              'small-icons': !showPreviewImages
            }"
            :aria-label="baseName(s.path)"
            :data-result-index="k"
          >
            <a
              :href="getItemUrl(s)"
              @click="openResult($event, k)"
            >
              <Icon
                :mimetype="s.type"
                :filename="baseName(s.path)"
                :path="s.path"
                :has-preview="showPreviewImages && (s.hasPreview || false)"
                :thumbnail-url="showPreviewImages ? getThumbnailUrl(s) : ''"
              />
              <span class="text-container">
                <!-- eslint-disable-next-line @intlify/vue-i18n/no-raw-text -->
                {{ basePath(s.path, s.type === "directory") }}{{ baseName(s.path) }}
              </span>
              <div class="filesize">{{ humanSize(s.size) }}</div>
              <div v-if="isSearchingMultipleSources && s.source" class="source-badge">
                {{ s.source }}
              </div>
            </a>
          </li>
        </ul>
        <button
          type="button"
          class="button open-advanced-search-button"
          @click.stop.prevent="openInAdvancedSearch"
        >
          {{ $t("search.openInAdvancedSearch") }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, toRefs, watch, type Ref } from "vue";
import { useI18n } from "vue-i18n";
import { useListingContextMenu } from "@/composables/useListingContextMenu";
import { useSearchFilters } from "@/composables/useSearchFilters";
import { useSearchRequest } from "@/composables/useSearchRequest";
import { resourcesApi, toolsApi } from "@/api";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";
import Icon from "@/components/files/Icon.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import SearchFilters from "@/components/tools/SearchFilters.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { router } from "@/router";
import { getters, mutations, state } from "@/store";
import type { FileListItem } from "@/store/types";
import { url } from "@/utils/";
import { globalVars } from "@/utils/constants";
import { debounce } from "@/utils/debounce.js";
import { sortedItems } from "@/utils/sort.ts";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { isTypeAheadSessionActive } from "@/utils/listingTypeAhead.js";
import { shouldIgnoreOutsideTap } from "@/utils/tooltipHelp.js";

defineOptions({ name: "search" });

interface SearchResult {
  source?: string;
  path: string;
  type: string;
  size?: number;
  modified?: string;
  created?: string;
  hasPreview?: boolean;
}

const { t } = useI18n();

const resultListEl = ref<HTMLElement | null>(null);
const input = ref<HTMLInputElement | null>(null);
const noneMessage = ref(t("search.typeToSearch", { minSearchLength: globalVars.minSearchLength }));
const showPreviewImages = ref(false);
const advancedOptionsExpanded = ref(false);
const value = ref("");
const results = ref<SearchResult[]>([]);
const selectedSource = ref("");
const highlightedIndex = ref(-1);
let restoring = false;

// Coalesce keystrokes into one request 250ms after the last input.
const debouncedSubmit = debounce(() => submit(), 250);
const { filters, buildRequest, toRouteQuery } = useSearchFilters({ onChange: () => void submitNow() });
const { isRunning, run: runSearchRequest, abort: abortSearch } = useSearchRequest();

const formRefs: Record<string, Ref<unknown>> = {
  value, results, noneMessage, selectedSource, ...toRefs(filters), showPreviewImages, advancedOptionsExpanded,
};

const eventTheme = computed(() => getters.eventTheme());
const disableSearchOptions = computed(() => state.user?.disableSearchOptions);
const active = computed(() => state.isSearchActive);
const isEmpty = computed(() => results.value.length === 0);
const searchHelpText = computed(() =>
  [
    t("search.helpText1", { minSearchLength: globalVars.minSearchLength }),
    t("search.helpText2"),
  ].join("\n\n"),
);
const activeStates = computed(() => {
  const selectedItems = state.selected ? ([] as (number | FileListItem)[]).concat(state.selected) : [];
  if (selectedItems.length === 0) {
    // Return an array of all false if nothing is selected
    return new Array<boolean>(results.value.length).fill(false);
  }

  const selectedPaths = new Set(selectedItems.map((item) => (item as FileListItem).path));
  return results.value.map((result) => {
    return selectedPaths.has(resultToItem(result).path);
  });
});
const hasPrompts = computed(() => state.prompts.length > 0);
const promptOpen = computed(() =>
  state.prompts.some((prompt) => prompt.name !== "ContextMenu" && prompt.name !== "OverflowMenu"),
);
const itemState = computed(() => (hasPrompts.value ? activeStates.value : results.value.map(() => false)));
const sourceInfo = computed(() => state.sources.info);
const multipleSources = computed(() => Object.keys(state.sources.info).length > 1);
const searchSourceOptions = computed(() => {
  const sources = Object.keys(sourceInfo.value).map((name) => ({
    value: name,
    label: name,
  }));
  return [{ value: "__all__", label: t("general.all") }, ...sources];
});
const isSearchingMultipleSources = computed(
  () => selectedSource.value === "__all__" || (selectedSource.value === "" && multipleSources.value),
);
const getContext = computed(() => {
  // If searching all sources (explicitly or by default), always use root context
  if (selectedSource.value === "__all__" || (selectedSource.value === "" && multipleSources.value)) {
    return "/";
  }
  const result = url.extractSourceFromPath(decodeURIComponent(state.route.path ?? ""), Object.keys(state.sources.info));
  if (selectedSource.value === "" || result.source === selectedSource.value) {
    return result.path;
  } else {
    return "/"; // if searching on non-current source, search the whole thing
  }
});

useListingContextMenu(resultListEl, {
  getProps: () => ({ showLimitedOptions: true }),
  beforeOpen: selectResultAt,
});

watch(
  () => state.sources.current,
  () => {
    selectedSource.value = state.sources.current;
  },
);
watch(value, () => {
  if (results.value.length) {
    results.value = [];
  }
});
watch(active, (isNowActive) => {
  if (isNowActive) {
    if (state.searchResults?.focus) {
      void restoreSearchResults();
    } else {
      resetSearchOnOpen();
    }
  }
});
watch(hasPrompts, (now, before) => {
  if (before && !now && state.isSearchActive) {
    mutations.resetSelected();
  }
});

function handleKeydown(event: KeyboardEvent) {
  if (
    (event.key === "/" || event.key === " ") &&
    isTypeAheadSessionActive() &&
    getters.currentView() === "listingView"
  ) {
    event.preventDefault();
    return;
  }
  if ((event.key === "/" || event.key === " ") && !state.isSearchActive && !getters.currentPrompt()) {
    event.preventDefault();
    void open();
  }
}

onMounted(() => {
  selectedSource.value = state.sources.current;
  if (state.searchResults?.focus && state.previousHistoryItem?.returnTo === state.route.fullPath) {
    mutations.setPreviousHistoryItem(null);
    void nextTick().then(() => mutations.setSearch(true));
  }
  document.addEventListener("keydown", handleKeydown);
});

onBeforeUnmount(() => {
  document.removeEventListener("keydown", handleKeydown);
  debouncedSubmit.cancel();
});

function updateSource(newSource: string) {
  selectedSource.value = newSource;
  void submitNow();
}

function getItemUrl(s: SearchResult) {
  const { source, path } = resultToItem(s);
  return url.buildItemUrl(source, path, true);
}

function humanSize(size?: number) {
  return getHumanReadableFilesize(size);
}

function basePath(str: string, isDir: boolean) {
  let result = url.removeLastDir(str);
  if (!isDir) {
    result = url.removeLeadingSlash(result); // fix weird rtl thing
  }
  return `${result}/`;
}

function baseName(str: string) {
  const parts = url.removeTrailingSlash(str).split("/");
  return parts.pop() ?? "";
}

/** Clear query state whenever the overlay opens (open paths do not always call close()). */
function resetSearchOnOpen() {
  value.value = "";
  results.value = [];
  highlightedIndex.value = -1;
  debouncedSubmit.cancel();
  abortSearch();
  noneMessage.value = t("search.typeToSearch", {
    minSearchLength: globalVars.minSearchLength,
  });
}

function close(event: Event) {
  value.value = "";
  debouncedSubmit.cancel();
  abortSearch();
  event.stopPropagation();
  mutations.setSearch(false);
}

function keyup(event: KeyboardEvent) {
  if (event.keyCode === 27) {
    close(event);
    return;
  }
}

/** Debounced entry point for the search input (fires 250ms after the last keystroke). */
function onInput() {
  abortSearch();
  debouncedSubmit();
}

/** Immediate search, used by filter/source changes; cancels any pending debounced call. */
function submitNow(event?: Event) {
  debouncedSubmit.cancel();
  return submit(event);
}

async function submit(event?: Event) {
  if (restoring) return;
  results.value = [];
  highlightedIndex.value = -1;
  if (event !== undefined) {
    event.preventDefault();
  }
  if (value.value === "" || value.value.length < globalVars.minSearchLength) {
    noneMessage.value = t("search.notEnoughCharacters", { minSearchLength: globalVars.minSearchLength });
    return;
  }
  const { queryPrefix, dateParams } = buildRequest();
  const searchQuery = queryPrefix + value.value;

  // Determine which sources to search
  let sourcesToSearch: string[];
  if (selectedSource.value === "__all__" || selectedSource.value === "") {
    // Search all sources
    sourcesToSearch = Object.keys(sourceInfo.value);
  } else {
    // Search single source
    sourcesToSearch = [selectedSource.value || state.sources.current];
  }

  // Only pass scope if searching a single source
  const scope = sourcesToSearch.length === 1 ? getContext.value : null;

  const succeeded = await runSearchRequest(
    (signal) =>
      toolsApi.search(scope, sourcesToSearch, searchQuery, false, { ...dateParams, signal }) as Promise<SearchResult[]>,
    (found) => {
      results.value = sortResults(found);
      setSearchResults();
    },
  );
  if (succeeded && results.value.length === 0 && !isRunning.value) {
    noneMessage.value = t("search.noResults");
  }
}

function resultToItem(s: SearchResult): FileListItem {
  const context = url.removeTrailingSlash(getContext.value);
  return {
    name: baseName(s.path),
    path: `${context}/${url.removeLeadingSlash(url.removeTrailingSlash(s.path))}`,
    type: s.type,
    size: s.size,
    modified: s.modified,
    created: s.created,
    hasPreview: s.hasPreview,
    source: s.source || selectedSource.value || state.sources.current,
  };
}

function sortResults(found: SearchResult[]) {
  const term = value.value.trim().toLowerCase();
  const rank = ({ name }: { name: string }) => {
    const lower = name.toLowerCase();
    return lower === term ? 0 : lower.startsWith(term) ? 1 : 2;
  };
  const entries = found.map((s) => ({ s, name: baseName(s.path) }));
  return sortedItems(entries, "name").sort((a, b) => rank(a) - rank(b)).map((e) => e.s);
}

function setSearchResults() {
  state.searchResults = {
    kind: "quick",
    sorting: { by: "none", asc: true },
    items: results.value.map((s) => resultToItem(s)),
    form: getSearchForm(),
  };
}

function getSearchForm() {
  return Object.fromEntries(Object.entries(formRefs).map(([key, field]) => [key, field.value]));
}

async function restoreSearchResults() {
  const search = state.searchResults;
  if (!search?.form) return;
  const { results: savedResults, ...fields } = search.form as { results: SearchResult[] };
  const focus = search.focus;
  search.focus = undefined;
  restoring = true;
  for (const [key, field] of Object.entries(fields)) {
    const target = formRefs[key];
    if (target) target.value = field;
  }
  await nextTick();
  restoring = false;
  results.value = savedResults.filter((s) => search.items.some((i) => url.isSameItem(i, resultToItem(s))));
  highlightedIndex.value = results.value.findIndex((s) => url.isSameItem(resultToItem(s), focus));
  await nextTick();
  const list = document.getElementById("result-list");
  list?.classList.add("active");
  list?.querySelectorAll(".search-entry")[highlightedIndex.value]?.scrollIntoView({ block: "center" });
}

function openResult(event: MouseEvent, index: number) {
  if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) {
    return;
  }
  const result = results.value[index];
  if (!result) return;
  event.preventDefault();
  const item = resultToItem(result);
  if (state.searchResults) {
    state.searchResults.form = getSearchForm();
    state.searchResults.focus = { source: item.source, path: item.path };
  }
  mutations.setPreviousHistoryItem({
    name: item.name,
    source: item.source as string,
    path: item.path,
    returnTo: state.route.fullPath,
  });
  url.goToItem(item.source, item.path, null, false, getters.isShare());
}

function clearContext(event?: Event) {
  const target = event?.target;
  if (target instanceof Element && target.closest(".tooltip-info-icon, .floating-tooltip")) {
    return;
  }
  if (shouldIgnoreOutsideTap()) {
    return;
  }
  mutations.closeHovers();
}

function getThumbnailUrl(s: SearchResult) {
  if (!s.hasPreview) {
    return "";
  }
  try {
    const { source, path } = resultToItem(s);
    return resourcesApi.getPreviewURL(source, path, s.modified || "");
  } catch (_err) {
    return "";
  }
}

async function open() {
  if (state.isSearchActive) return;
  if (getters.currentPromptName()) return;

  mutations.closeHovers();
  mutations.closeSidebar();
  mutations.resetSelected();
  mutations.setSearch(true);

  await nextTick();
  if (input.value) {
    input.value.focus();
  }
  const resultList = document.getElementById("result-list");
  if (resultList) {
    resultList.classList.add("active");
  }
}

function openInAdvancedSearch() {
  let sourcesToSearch: string[];
  if (selectedSource.value === "__all__" || selectedSource.value === "") {
    sourcesToSearch = Object.keys(sourceInfo.value || {});
  } else {
    sourcesToSearch = [selectedSource.value || state.sources.current];
  }

  const picked = [...sourcesToSearch].sort();

  const query: Record<string, string | string[]> = {};
  query.term = value.value.trim();
  Object.assign(query, toRouteQuery());

  const scopeClauses: string[] = [];
  for (const sourceName of picked) {
    let scopedFolderPath = "/";
    if (picked.length === 1) {
      const raw = String(getContext.value || "").trim();
      if (raw !== "" && raw !== "/") {
        const withSlash = raw.startsWith("/") ? raw : `/${raw}`;
        scopedFolderPath = url.removeTrailingSlash(withSlash);
      }
    }
    if (scopedFolderPath === "") {
      scopedFolderPath = "/";
    }
    if (!scopedFolderPath.startsWith("/")) {
      scopedFolderPath = `/${scopedFolderPath}`;
    }
    scopeClauses.push(`${sourceName}:${scopedFolderPath}`);
  }
  if (scopeClauses.length === 1) {
    query.scope = scopeClauses[0] as string;
  } else {
    query.scope = scopeClauses;
  }

  mutations.setSearch(false);
  router
    .push({ path: "/tools/advancedSearch", query })
    .catch(() => {});
}

function selectResultAt(target: EventTarget | null): boolean {
  const entry = (target as Element | null)?.closest<HTMLElement>("[data-result-index]");
  const result = entry ? results.value[Number(entry.dataset.resultIndex)] : undefined;
  if (!result) return false;
  mutations.resetSelected();
  mutations.addSelected(resultToItem(result));
  return true;
}
</script>

<style scoped>
.inputWrapper {
  display: flex;
  align-items: stretch;
  min-height: 2.5em;
}

.searchContext {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
  min-height: 2.5em;
  margin-top: 0;
  padding: 0.5em 1em;
  background: var(--primaryColor);
  color: white;
  overflow-wrap: break-word;
  margin-bottom: 0 !important;
  box-sizing: border-box;
}

.searchContext.expand-dropdown {
  flex: 0 0 auto;
  width: 25%;
  min-width: 7em;
  max-width: 15em;
  padding: 0.5em 0.75em;
  align-self: stretch;
  min-height: 0;
}

.searchContext.expand-dropdown :deep(.expand-dropdown-anchor) {
  display: flex;
  align-items: center;
  width: 100%;
  height: 100%;
  min-height: 0;
  padding: 0;
  border: none;
  box-shadow: none;
  background: transparent;
}

.searchContext.expand-dropdown.expand-dropdown--open :deep(.expand-dropdown-anchor) {
  border: none !important;
  box-shadow: none !important;
  background-color: transparent !important;
  padding: 0 !important;
  border-radius: 0 !important;
  border-bottom-color: transparent !important;
}

.searchContext.expand-dropdown :deep(.expand-dropdown-trigger) {
  align-items: center;
  line-height: 1.2;
  height: 100%;
  min-height: 0;
  padding: 0;
  box-sizing: border-box;
}

.searchContext.expand-dropdown :deep(.expand-dropdown-trigger:hover:not(:disabled)) {
  width: 100% !important;
  margin-left: 0 !important;
  padding-left: 0 !important;
}

.searchContext.expand-dropdown :deep(.expand-dropdown-trigger i),
.searchContext.expand-dropdown :deep(.expand-dropdown-chevron) {
  padding: 0;
  line-height: 1;
}

:global(.searchContext.expand-dropdown.expand-dropdown-overlay--anchored) {
  flex: 0 0 auto;
  width: 25%;
  min-width: 7em;
  max-width: 15em;
  padding: 0.5em 0.75em;
  align-self: stretch;
  min-height: 0;
  background: var(--primaryColor);
  color: white;
  box-sizing: border-box;
}

:global(.searchContext.expand-dropdown.expand-dropdown-overlay--anchored) .expand-dropdown-anchor {
  display: flex;
  align-items: center;
  width: 100%;
  height: 100%;
  min-height: 0;
  padding: 0;
  border: none;
  box-shadow: none;
  background: transparent;
}

:global(.searchContext.expand-dropdown.expand-dropdown-overlay--anchored) .expand-dropdown-trigger {
  align-items: center;
  line-height: 1.2;
  height: 100%;
  min-height: 0;
  padding: 0;
  box-sizing: border-box;
  color: inherit;
}

:global(.searchContext.expand-dropdown.expand-dropdown-overlay--anchored) .expand-dropdown-trigger:hover:not(:disabled) {
  width: 100% !important;
  margin-left: 0 !important;
  padding-left: 0 !important;
  background-color: transparent;
}

:global(.searchContext.expand-dropdown.expand-dropdown-overlay--anchored) .expand-dropdown-trigger-label,
:global(.searchContext.expand-dropdown.expand-dropdown-overlay--anchored) .expand-dropdown-chevron {
  color: inherit;
}

:global(.searchContext.expand-dropdown.expand-dropdown-overlay--anchored) .expand-dropdown-trigger i,
:global(.searchContext.expand-dropdown.expand-dropdown-overlay--anchored) .expand-dropdown-chevron {
  padding: 0;
  line-height: 1;
}

.searchContext.input {
  background-color: var(--primaryColor) !important;
  border-radius: 0 !important;
  color: white;
  border: unset;
  width: 25%;
  min-width: 7em;
  max-width: 15em;
  height: auto;
}

.searchContext.input option {
  background: grey;
  color: white;
}

.searchContext.input option:hover {
  background: var(--primaryColor);
  color: white;
}

.open-advanced-search-row {
  display: flex;
  justify-content: flex-end;
  padding: 0.35em 0.75em 0;
}

.open-advanced-search-button {
  font-size: 0.875rem;
  margin: auto
}

#result-list {
  scrollbar-width: none;
  max-width: 95vw;
  background-color: var(--background);
  color: var(--textPrimary);
}

#results>#result-list {
  max-height: 80vh;
  width: 35em;
  overflow: scroll;
  padding-bottom: 1em;
  transition: width 0.3s ease 0s;
}

#results {
  animation: slide-down 0.5s forwards;
  border-radius: var(--borderRadius);
  border-top: none;
  border-top-left-radius: 0;
  border-top-right-radius: 0;
  border: var(--borderWidth) solid var(--surfaceSecondary);
  box-shadow: 0 2em 50px 10px rgb(0 0 0 / 30%);
  background-color: var(--surfacePrimary);
  max-height: 80vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.search-entry > a {
  padding-left: 0.5em;
}

#search.active #results ul li a {
  display: flex;
  align-items: center;
}

#search #result-list.active {
  width: 1000px;
}

/* Animations */
@keyframes slide-down {
  0% {
    transform: translateY(-3em);
    opacity: 0;
  }
  100% {
    transform: translateY(0);
    opacity: 1;
  }
}

/* Search */
#search {
  background-color: unset !important;
  z-index: 5;
  position: fixed;
  top: 0.5em;
  min-width: 35em;
  left: 50%;
  transform: translateX(-50%);
}

.search-input-container {
  background-color: rgb(100 100 100 / 20%);
  display: flex;
  padding: 0.5em 0.75em;
  border-radius: var(--borderRadius);
  border-style: unset;
  align-items: center;
  height: 3em;
  gap: 0.5em;
  width: 100%;
}

.search-input-container .material-symbols {
  font-size: 1.25em;
  color: rgb(255 255 255 / 70%);
}

#search.active .search-input-container .material-symbols {
  color: inherit;
}

#search .search-input-container input {
  border: 0;
  background-color: transparent;
  padding: 0;
  color: var(--divider);
  font-size: 0.95em;
}

#search-input {
  width: 100%;
  padding-left: 0.5em;
}

#search.active .search-input-container input {
  color: var(--textPrimary);
}

#search .search-input-container input::placeholder {
  color: color-mix(in srgb, var(--divider) 50%, transparent);
}

#search.active .search-input-container input::placeholder {
  color: var(--textSecondary);
}

#search.active .search-input-container {
  background-color: var(--background);
  border-color: var(--surfaceSecondary);
  border-style: solid;
  border-bottom-style: none;
  border-bottom-right-radius: 0 !important;
  border-bottom-left-radius: 0 !important;
  border-width: var(--borderWidth);
}

#result-list p {
  margin: 1em;
}

/* Hiding scrollbar for Chrome, Safari and Opera */
#result-list::-webkit-scrollbar {
  display: none;
}

.search-entry {
  position: relative;
  border-left: var(--borderWidth) solid var(--divider);
  border-right: var(--borderWidth) solid var(--divider);
}

.search-entry:first-child {
  border-top: var(--borderWidth) solid var(--divider);
  border-top-left-radius: var(--borderRadius);
  border-top-right-radius: var(--borderRadius);
}

.search-entry:last-child {
  border-bottom: var(--borderWidth) solid var(--divider);
  border-bottom-left-radius: var(--borderRadius);
  border-bottom-right-radius: var(--borderRadius);
}


.search-entry:hover {
  background-color: var(--hoverOverlay);
  transform: scale(1.01);
  box-shadow: none !important;
}

.search-entry.active {
  background-color: var(--surfacePrimary);
}

#search.prompt-open::after {
  content: "";
  position: absolute;
  inset: 0;
  z-index: 10;
  background-color: var(--overlay);
  border-radius: var(--borderRadius);
}

.search-entry.current-item {
  background-color: color-mix(in srgb, var(--primaryColor) 25%, transparent) !important;
}

.text-container {
  margin-left: 0.25em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  width: 100%;
  text-align: left;
  direction: rtl;
}

#search #result {
  padding-top: 1em;
  overflow: hidden;
  background: var(--background);
  color: var(--textPrimary);
  display: flex;
  top: -4em;
  flex-direction: column;
  align-items: center;
  text-align: left;
  height: 0;
  transition: height 2s ease, padding 2s ease;
  z-index: 3;
}

body.rtl #search #result {
  direction: rtl;
  text-align: right;
}

#search #result>div>*:first-child {
  margin-top: 0;
}

/* Search Results */
body.rtl #search #result ul>* {
  direction: ltr;
  text-align: left;
}

#search ul {
  margin-top: 1em;
  padding: 0;
  list-style: none;
}

#search li {
  margin: 0 0.5em;
  padding: 0.25em 0;
}

#search #renew {
  width: 100%;
  text-align: center;
  display: none;
  margin: 1em;
  max-width: none;
}

#search.ongoing #renew {
  display: block;
}

.search-options-settings-item :deep(.constraints) {
  display: flex;
  flex-flow: row wrap;
  place-content: center center;
  margin: 1em;
}

.searchPrompt {
  display: flex;
  flex-direction: row;
  place-content: center center;
  align-items: center;
  gap: 0.5em;
}

.searchPrompt .tooltip-info-icon {
  font-size: 1.5rem;
  cursor: pointer;
  color: var(--primaryColor);
}

.filesize {
  background: var(--surfaceSecondary);
  border-radius: var(--borderRadius);
  padding: 0.25em;
  padding-left: 0.5em;
  padding-right: 0.5em;
  margin-right: 0.5em;
  min-width: fit-content;
  opacity: 0.75;
  font-weight: 500;
}

.source-badge {
  background: var(--primaryColor);
  color: white;
  border-radius: var(--borderRadius);
  padding: 0.25em 0.5em;
  font-size: 0.85em;
  font-weight: 500;
  min-width: fit-content;
}

.search-options-settings-item {
  padding: 0.5em;
}

.search-options-settings-item :deep(.search-filters-inner) {
  overflow: hidden;
}

@media (width <= 768px) {
  #search {
    min-width: unset;
    max-width: 60%;
  }
  #search.active {
    display: block;
    position: fixed;
    top: 0;
    left: 50%;
    width: 100%;
    max-width: 100%;
  }
  .search-input-container {
    transition: height 1s ease, background-color 1s ease, box-shadow 1s ease;
  }
  #search.active .search-input-container {
    box-shadow: 0 0 10px rgb(0 0 0 / 10%);
    height: 4em;
    background: var(--surfacePrimary)
  }
  #search.active>div {
    border-radius: 0 !important;
  }
  #search.active #result {
    height: 100vh;
    padding-top: 0;
  }
  .search-input-container>.action,
  .search-input-container>i {
    margin-right: 0.3em;
    user-select: none;
  }
  #search.active #result>p>i {
    text-align: center;
    margin: 0 auto;
    display: table;
  }
  #search.active #result ul li a {
    display: flex;
    align-items: center;
    padding: .3em 0;
    margin-right: .3em;
  }
  #result-list {
    width: 100vw !important;
    max-width: 100vw !important;
    left: 0;
    top: 4em;
    overflow: scroll;
    display: flex;
    flex-direction: column;
  }

}
</style>
