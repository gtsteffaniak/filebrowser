<template>
  <div ref="rootEl" class="advanced-search-root">
    <div class="card advanced-search-toolbar padding-normal">
      <form class="card-content advanced-search-card-form" @submit.prevent="runSearch()">
        <div class="advanced-search-config-row">
          <div class="config-primary">
            <div class="as-section-label">{{ $t("tools.advancedSearch.searchTerms") }}</div>
            <template v-for="(term, idx) in termInputs" :key="`term-${idx}`">
              <div
                v-if="idx > 0"
                class="term-join-divider"
                role="separator"
              >
                <span class="term-join-line" aria-hidden="true" />
                <span class="term-join-text">{{ termJoinDividerLabel }}</span>
                <span class="term-join-line" aria-hidden="true" />
              </div>
              <div class="search-term-row">
                <input
                  v-model.trim="termInputs[idx]"
                  class="input flex-grow-input"
                  :class="{ 'flat-right': termInputs.length > 1 }"
                  type="search"
                  autocomplete="off"
                  :placeholder="$t('general.search', { suffix: '...' })"
                />
                <button
                  v-if="termInputs.length > 1"
                  type="button"
                  class="button flat-left no-height"
                  :aria-label="$t('general.delete')"
                  @click="removeTermAt(idx)"
                >
                  <i class="material-symbols material-size">delete</i>
                </button>
              </div>
            </template>
            <button
              type="button"
              class="button no-height add-term-button"
              @click="addTermField"
            >
              <i class="material-symbols material-size">add</i>
            </button>
            <ToggleSwitch
              v-if="termInputs.length > 1"
              v-model="termsJoinAnd"
              class="advanced-search-switch"
              :name="$t('tools.advancedSearch.matchAllTerms')"
              :description="$t('tools.advancedSearch.matchAllTermsDescription')"
            />
          </div>

          <div class="config-secondary">
            <div class="as-sources-block">
              <div class="as-section-label">{{ $t("general.sources") }}</div>
              <ExpandDropdown
                v-model="selectedSourceNames"
                class="source-dropdown-select"
                :options="sourceOptions"
                allow-multiple
                :all-selected-label="$t('general.allSources')"
                :aria-label="$t('general.sources', { suffix: '' })"
              />
              <template
                v-for="srcName in activeSources"
                  :key="`scope-${srcName}`"
              >
                <PathPickerButton
                  class="scope-picker"
                  v-model:path="sourceScopedPaths[srcName]"
                  :source="srcName"
                  :show-files="false"
                  :show-folders="true"
                  :aria-label="$t('tools.advancedSearch.scopeAria')"
                  :placeholder="$t('sidebar.chooseSource')"
                />
              </template>
            </div>

            <div v-if="!disableSearchOptions" class="advanced-search-options-section">
              <SearchFilters :filters="filters">
                <template #extra-settings>
                  <div class="settings-number-input item">
                    <label for="advanced-search-limit">{{ $t('tools.advancedSearch.resultLimit') }}</label>
                    <div>
                      <input
                        id="advanced-search-limit"
                        v-model.number="resultLimit"
                        type="range"
                        min="1"
                        :max="maxSearchResults"
                        :disabled="loading"
                      />
                      <output for="advanced-search-limit" class="range-value">{{ resultLimit }}</output>
                    </div>
                  </div>
                </template>
              </SearchFilters>
            </div>

            <div class="advanced-search-actions-row">
              <button type="submit" class="button search-submit" :disabled="loading" :aria-busy="loading">
                <i v-if="loading" class="material-symbols spin">autorenew</i>
                <span v-else>{{ $t("general.search") }}</span>
              </button>
            </div>
          </div>
        </div>
      </form>
    </div>

    <div v-if="searchExecuted" class="advanced-search-results-plane">
      <div v-if="loading" class="results-loading">
        <LoadingSpinner size="medium" />
        <span>{{ $t("general.loading", { suffix: "..." }) }}</span>
      </div>
      <template v-else>
        <div v-if="error" class="error-message padded-message">
          {{ error }}
        </div>
        <div v-else-if="resultsEmpty" class="empty-state-message padded-message">
          <p>{{ $t("tools.advancedSearch.noResults") }}</p>
        </div>
        <div v-else ref="results" class="advanced-search-listing-results">
          <div class="advanced-search-sticky-header" :class="{ 'header-hidden': headerHidden }">
            <ListingHeader class="advanced-search-inline-header" />
          </div>
          <div class="advanced-search-inner no-select">
            <div
              ref="listingView"
              :class="{
                'add-padding': isStickySidebar,
                'rectangle-selecting': isRectangleSelecting,
                [listingViewMode]: true,
              }"
              :style="listingItemStyles"
              class="listing-items file-icons"
            >
              <div v-if="isRectangleSelecting" class="selection-rectangle" :style="rectangleStyle"></div>
              <template v-if="reqListing.dirs.length > 0">
                <div>
                  <h2>{{ $t("general.folders") }}</h2>
                </div>
                <div
                  class="folder-items"
                  aria-label="Folder Items"
                  :class="{ lastGroup: reqListing.files.length === 0 }"
                >
                  <Item
                    v-for="entry in reqListing.dirs"
                    :key="listingItemKey(entry)"
                    :index="entry.index"
                    :name="entry.name"
                    :is-dir="entry.type === 'directory'"
                    :source="entry.source"
                    :modified="entry.modified"
                    :created="entry.created"
                    :type="entry.type"
                    :size="entry.size"
                    :path="entry.path"
                    :reduced-opacity="false"
                    :hash="shareInfo.hash"
                    :has-preview="entry.hasPreview"
                    :has-duration="false"
                    :is-shared="entry.isShared || false"
                    :read-only="true"
                    :force-files-api="true"
                  />
                </div>
              </template>
              <template v-if="reqListing.files.length > 0">
                <div>
                  <h2>{{ $t("general.files") }}</h2>
                </div>
                <div class="file-items" :class="{ lastGroup: true }" aria-label="File Items">
                  <Item
                    v-for="entry in reqListing.files"
                    :key="listingItemKey(entry)"
                    :index="entry.index"
                    :name="entry.name"
                    :is-dir="entry.type === 'directory'"
                    :modified="entry.modified"
                    :created="entry.created"
                    :source="entry.source"
                    :type="entry.type"
                    :size="entry.size"
                    :path="entry.path"
                    :reduced-opacity="false"
                    :hash="shareInfo.hash"
                    :has-preview="entry.hasPreview"
                    :metadata="entry.metadata"
                    :has-duration="false"
                    :is-shared="entry.isShared || false"
                    :read-only="true"
                    :force-files-api="true"
                  />
                </div>
              </template>
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import type { FileListItem, fileListItemIndex, ReqObject } from "@/store/types";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { toolsApi } from "@/api";
import { router } from "@/router";
import { state, getters, mutations } from "@/store";
import { eventBus } from "@/store/eventBus";
import { globalVars } from "@/utils/constants";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import PathPickerButton from "@/components/files/PathPickerButton.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import Item from "@/components/files/ListingItem.vue";
import { removeTrailingSlash, removeLeadingSlash, isSameItem, goToItem } from "@/utils/url.js";
import { selectAndScrollToIndex } from "@/utils/selectAndScroll.ts";
import ListingHeader from "@/components/files/ListingHeader.vue";
import SearchFilters from "@/components/tools/SearchFilters.vue";
import { useSearchFilters } from "@/composables/useSearchFilters";
import { useSearchRequest } from "@/composables/useSearchRequest";
import { useListingKeyboard } from "@/composables/useListingKeyboard";
import { useListingLayout } from "@/composables/useListingLayout";
import { useMouseSelection } from "@/composables/useRectangleSelection";
import { useListingContextMenu } from "@/composables/useListingContextMenu";
import { getObjectProperty, setObjectProperty } from '@/utils/object.js';

type QueryRecord = Record<string, unknown>;
type NormalizedQuery = Record<string, string | string[]>;

interface SearchResultRow {
  source?: string;
  path?: string;
  type?: string;
  size?: number;
  modified?: string;
  created?: string;
  hasPreview?: boolean;
}

interface ItemsChangedEvent {
  succeeded?: unknown[];
}

/**
 * Builds a canonical browse path within the selected scope (mirror of Search.vue getItemUrl path join).
 */
function browsePath(scopePath: string, apiPath: string, isDirectory: boolean) {
  const context =
    scopePath && scopePath !== "/" ? removeTrailingSlash(scopePath) : "";
  const trimmedApi = String(apiPath || "").trim();

  if (trimmedApi === "/" || trimmedApi === "") {
    if (context) {
      return context.endsWith("/") ? context : `${context}/`;
    }
    return "/";
  }

  const relativeSegmentsPath = removeLeadingSlash(removeTrailingSlash(trimmedApi));
  let resolvedBrowsePath =
    context === "" ? `/${relativeSegmentsPath}` : `${context}/${relativeSegmentsPath}`;

  if (isDirectory && !resolvedBrowsePath.endsWith("/")) {
    resolvedBrowsePath += "/";
  }
  if (!isDirectory && resolvedBrowsePath.endsWith("/") && resolvedBrowsePath.length > 1) {
    resolvedBrowsePath = removeTrailingSlash(resolvedBrowsePath);
  }

  return resolvedBrowsePath;
}

function displayName(fullPath: string, isDirectory: boolean) {
  const withoutTrailing =
    fullPath.endsWith("/") && fullPath.length > 1
      ? fullPath.slice(0, -1)
      : fullPath;
  const parts = withoutTrailing.split("/").filter(Boolean);
  const name = parts[parts.length - 1] ?? "";

  return name !== "" ? name : (isDirectory ? "/" : "?");
}

function parseTermsFromRouteQuery(routeQuery: QueryRecord) {
  const q = routeQuery || {};
  const out: string[] = [];

  const termRaw = q.term;
  if (Array.isArray(termRaw)) {
    for (const termEntry of termRaw) {
      const trimmedTerm = String(termEntry ?? "").trim();
      if (trimmedTerm !== "") {
        out.push(trimmedTerm);
      }
    }
  } else if (
    termRaw !== undefined &&
    termRaw !== null &&
    String(termRaw) !== ""
  ) {
    const trimmedTerm = String(termRaw).trim();
    if (trimmedTerm !== "") {
      out.push(trimmedTerm);
    }
  }

  if (out.length === 0 && q.q !== undefined && q.q !== null) {
    const trimmedAlias = String(q.q).trim();
    if (trimmedAlias !== "") {
      out.push(trimmedAlias);
    }
  }

  return out;
}

function hasQueryValue(q: QueryRecord, queryKey: string) {
  const raw = getObjectProperty(q, queryKey);
  return raw !== undefined && raw !== null && String(raw).trim() !== "";
}

function hasFilterOrTermSignalsForImplicitSources(q: QueryRecord) {
  return parseTermsFromRouteQuery(q).length > 0 || hasQueryValue(q, "limit") || hasRouteFilters(q);
}

function hasAnyAdvancedSearchRouteParams(q: QueryRecord) {
  return (
    hasFilterOrTermSignalsForImplicitSources(q) ||
    hasQueryValue(q, "sources") ||
    hasQueryValue(q, "scope") ||
    String(q.termJoin || "").toLowerCase() === "and"
  );
}

/** Canonicalize vue-router query for comparisons. */
function normalizeRouteQueryLoose(routeQuery: QueryRecord): NormalizedQuery {
  const q = routeQuery || {};
  const entries = Object.entries(q).sort((a, b) => a[0].localeCompare(b[0]));
  const nextEntries: [string, string | string[]][] = [];
  for (const [queryKey, rawValue] of entries) {
    if (rawValue === undefined || rawValue === null) continue;
    if (Array.isArray(rawValue)) {
      const sortedStrings = rawValue
        .map((entry) => String(entry))
        .filter((entry) => entry !== "")
        .sort();
      if (sortedStrings.length === 1) {
        nextEntries.push([queryKey, sortedStrings[0] as string]);
      } else if (sortedStrings.length > 1) {
        nextEntries.push([queryKey, sortedStrings]);
      }
    } else {
      const stringValue = String(rawValue);
      if (stringValue !== "") {
        nextEntries.push([queryKey, stringValue]);
      }
    }
  }
  return Object.fromEntries(nextEntries);
}

function stableQueryStringFromObject(obj: QueryRecord) {
  let pairs: [string, string][] = [];
  const keys = Object.keys(obj).sort();
  for (const paramKey of keys) {
    const paramValue = getObjectProperty(obj, paramKey);
    if (paramValue === undefined || paramValue === null) {
      continue;
    }
    if (Array.isArray(paramValue)) {
      const sortedValues = paramValue.map((entry) => String(entry)).sort();
      for (const arrayValue of sortedValues) {
        pairs.push([paramKey, arrayValue]);
      }
    } else {
      const stringValue = String(paramValue);
      if (stringValue !== "") {
        pairs.push([paramKey, stringValue]);
      }
    }
  }

  pairs = pairs.sort((a, b) => {
    if (a[0] !== b[0]) {
      return a[0] < b[0] ? -1 : 1;
    }
    return a[1] < b[1] ? -1 : a[1] > b[1] ? 1 : 0;
  });

  return new URLSearchParams(pairs).toString();
}

/** Raw `scope` query values (repeated or single). */
function collectScopeRawFromQuery(q: QueryRecord) {
  const raw = q.scope;
  if (raw === undefined || raw === null) {
    return [];
  }
  if (Array.isArray(raw)) {
    return raw.map((scopeFragment) => String(scopeFragment));
  }
  return [String(raw)];
}

const { t } = useI18n();
const route = useRoute();

const rootEl = ref<HTMLElement | null>(null);
const results = ref<HTMLElement | null>(null);
const listingView = ref<HTMLElement | null>(null);

const resultLimit = ref(Math.min(500, globalVars.searchResultsLimit || 1000));
const termInputs = ref<string[]>([""]);
const termsJoinAnd = ref(false);
const sourceEnabledFlags = ref<QueryRecord>({});
const sourceScopedPaths = ref<Record<string, string>>({});
const loading = ref(false);
const error = ref<string | null>(null);
const searchExecuted = ref(false);
const resultsEmpty = ref(false);
const headerHidden = ref(false);

// Updates the URL whenever a filter changes.
const { filters, buildRequest, toRouteQuery, applyRouteQuery, hasRouteFilters } = useSearchFilters({ onChange: scheduleAdvancedSearchUrlUpdate });
// Aborts a superseded search and keeps a late response from overwriting newer results.
const { isRunning, run: runSearchRequest, abort: abortSearch } = useSearchRequest();

let didApplySearchListing = false;
let hadInitialReq = false;
let initialReqFrozen = "";
let catalogRouteBootstrapDone = false;
let isInitializing = true;
let suppressRouteQueryNavigation = false;
let emptyCatalogHydrateTimer: number | null = null;
let refreshQueued = false;
let lastScrollTop = 0;
let scrollContainer: HTMLElement | null = null;

const maxSearchResults = computed(() => globalVars.searchResultsLimit || 1000);
const isAdvancedSearchRoute = computed(() => (route.path || "") === "/tools/advancedSearch");
const shareInfo = computed(() => state.shareInfo);
/** Keys from `state.sources.info`, sorted (sidebar catalogue). */
const sourceNameList = computed(() => Object.keys(state.sources.info || {}).sort());
const disableSearchOptions = computed(() => state.user.disableSearchOptions);
const termJoinDividerLabel = computed(() => {
  const raw = termsJoinAnd.value
    ? t("general.and")
    : t("general.or");
  return String(raw ?? "").toUpperCase();
});
const activeSources = computed(() => {
  const info = state.sources.info || {};
  return Object.keys(info)
    .filter((name) => getObjectProperty(sourceEnabledFlags.value, name) === true)
    .sort();
});
const sourceOptions = computed(() =>
  sourceNameList.value.map((name) => ({
    value: name,
    label: name,
  })),
);
const selectedSourceNames = computed({
  get() {
    return activeSources.value;
  },
  set(names: unknown) {
    const selected = Array.isArray(names) ? (names as string[]) : [];
    let nextFlags = { ...sourceEnabledFlags.value };
    for (const name of sourceNameList.value) {
      nextFlags = setObjectProperty(nextFlags, name, selected.includes(name));
    }
    sourceEnabledFlags.value = nextFlags;

    let nextPaths = { ...sourceScopedPaths.value };
    for (const name of selected) {
      const currentPath = getObjectProperty(nextPaths, name);
      if (currentPath === undefined || currentPath === null || currentPath === "") {
        nextPaths = setObjectProperty(nextPaths, name, "/") as Record<string, string>;
      }
    }
    sourceScopedPaths.value = nextPaths;

    if (selected.length === 0) {
      void nextTick().then(() => applyDefaultCurrentSourceIfNone());
    }
  },
});
const isStickySidebar = computed(() => getters.isStickySidebar());
const listingViewMode = computed(() => getters.viewMode());
const reqListing = computed(() => {
  const grouped = getters.reqItems();
  if (!grouped.dirs || !grouped.files) {
    return { dirs: [], files: [] };
  }
  return grouped;
});

const allItems = computed(() => [...reqListing.value.dirs, ...reqListing.value.files]);

const { itemStyles: listingItemStyles } = useListingLayout({ container: listingView });

const { isRectangleSelecting, rectangleStyle } = useMouseSelection({
  container: results,
  bounds: listingView,
  items: () => allItems.value,
  isActive: () => isAdvancedSearchRoute.value,
});

useListingKeyboard({
  container: results,
  items: () => allItems.value,
  isActive: () => isAdvancedSearchRoute.value,
  // Results come from several sources and each with its own permissions
  canModify: () =>
    (state.selected as number[]).every(
      (index) => !!getters.sourcePermissions(state.req.items.at(index)?.source).modify,
    ),
  ignoreEditableTargets: true,
  parentNavigation: false,
  openItem: (item) => {
    state.pendingHighlight = { path: item.path, source: item.source };
    if (state.searchResults) {
      state.searchResults.sorting = { ...getters.sorting() };
    }
    mutations.setPreviousHistoryItem({
      name: item.name,
      source: item.source as string,
      path: item.path,
      returnTo: route.fullPath,
    });
    goToItem(item.source, item.path, null, false, getters.isShare());
  },
});

useListingContextMenu(listingView, {
  getProps: () => ({ createOnly: state.selected.length === 0 }),
  beforeOpen: () =>
    state.selected.length > 0 ||
    getters.isAdmin() ||
    !!(state.user as { showSelectMultiple?: boolean } | undefined)?.showSelectMultiple ||
    getters.isMobile(),
});

async function highlightPending() {
  const p = state.pendingHighlight;
  if (!p) return;
  await nextTick();
  state.pendingHighlight = null;
  const items = state.req?.items ?? [];
  const idx = items.findIndex((i) => isSameItem(i, p));
  if (idx === -1) return;
  selectAndScrollToIndex(idx, true);
}

/** The sort header sticks under the app bar; hide it scrolling down and show it scrolling up (like FileList). */
function handleScroll() {
  const container = scrollContainer;
  if (!container) return;
  const top = container.scrollTop;
  const stickyTop = container.getBoundingClientRect().top + parseFloat(getComputedStyle(container).paddingTop);
  const resultsTop = results.value?.getBoundingClientRect().top;
  if (resultsTop === undefined || resultsTop > stickyTop) {
    headerHidden.value = false;
  } else if (Math.abs(top - lastScrollTop) >= 30) {
    headerHidden.value = top > lastScrollTop;
  } else {
    return;
  }
  lastScrollTop = top;
}

function handleSearchResultsChanged(data?: ItemsChangedEvent) {
  if (!isAdvancedSearchRoute.value || !searchExecuted.value) {
    return;
  }
  if (data?.succeeded?.length === 0) {
    return;
  }
  scheduleSearchRefresh();
}

function scheduleSearchRefresh() {
  if (refreshQueued) {
    return;
  }
  refreshQueued = true;
  void nextTick().then(() => {
    refreshQueued = false;
    void runSearch({ silent: true });
  });
}

/** When no source is toggled on, enable `state.sources.current` if known in the catalogue. */
function applyDefaultCurrentSourceIfNone() {
  if (!isAdvancedSearchRoute.value) {
    return;
  }
  const catalogue = [...sourceNameList.value];
  if (catalogue.length === 0) {
    return;
  }

  const enabledNames = catalogue.filter(
    (sourceName) => getObjectProperty(sourceEnabledFlags.value, sourceName) === true,
  );

  if (enabledNames.length > 0) {
    return;
  }

  const cur = state.sources.current;
  if (typeof cur === "string" && cur !== "" && catalogue.includes(cur)) {
    sourceEnabledFlags.value = setObjectProperty(sourceEnabledFlags.value, cur, true);
    let pathsNext = sourceScopedPaths.value;
    const currentPath = getObjectProperty(pathsNext, cur);
    if (currentPath === undefined || currentPath === null || currentPath === "") {
      pathsNext = setObjectProperty(pathsNext, cur, "/") as Record<string, string>;
    }
    sourceScopedPaths.value = pathsNext;
  }
}

function normalizedScopedPathForSource(sourceName: string) {
  let scopedPath = String(getObjectProperty(sourceScopedPaths.value, sourceName) ?? "/").trim();
  if (scopedPath === "") {
    scopedPath = "/";
  }
  if (!scopedPath.startsWith("/")) {
    scopedPath = `/${scopedPath}`;
  }
  return scopedPath;
}

function maybeFinishCatalogRouteBootstrapFromSources() {
  if (!isAdvancedSearchRoute.value) {
    return;
  }
  if (catalogRouteBootstrapDone) {
    return;
  }
  if (Object.keys(state.sources.info || {}).length === 0) {
    return;
  }
  if (typeof window !== "undefined" && emptyCatalogHydrateTimer !== null) {
    window.clearTimeout(emptyCatalogHydrateTimer);
    emptyCatalogHydrateTimer = null;
  }
  finishCatalogBootstrapFromRoute();
}

function finishCatalogBootstrapFromRoute() {
  if (!isAdvancedSearchRoute.value) {
    return;
  }
  if (catalogRouteBootstrapDone) {
    return;
  }
  catalogRouteBootstrapDone = true;

  applyQueryFromRoute();
  if (hasAnyAdvancedSearchRouteParams(route.query)) {
    void nextTick().then(() => {
      if (!restoreSearchResults()) {
        void runSearch();
      }
    });
  }
  isInitializing = false;
  void nextTick().then(() => {
    updateAdvancedSearchUrl();
  });
}

function searchKey() {
  return stableQueryStringFromObject(normalizeRouteQueryLoose(buildRouteQueryFromState()));
}

/** Reuse the last results when returning from a preview with the same query */
function restoreSearchResults() {
  const search = state.searchResults;
  if (search?.kind !== "advanced" || search.key !== searchKey()) {
    return false;
  }
  searchExecuted.value = true;
  showResults(search.items);
  return true;
}

function showResults(listingItems: FileListItem[]) {
  headerHidden.value = false;
  mutations.replaceRequest({
    type: "directory",
    path: "/",
    name: t("tools.advancedSearch.resultsTitle"),
    source: "",
    items: listingItems,
  } as ReqObject);
  didApplySearchListing = true;
  resultsEmpty.value = listingItems.length === 0;
  void highlightPending();
}

function buildRouteQueryFromState() {
  const query: NormalizedQuery = {};

  const terms = termInputs.value
    .map((termCell) => String(termCell || "").trim())
    .filter((trimmedCell) => trimmedCell !== "");
  if (terms.length === 1) {
    query.term = terms[0] as string;
  } else if (terms.length > 1) {
    query.term = terms;
  }

  if (termsJoinAnd.value) {
    query.termJoin = "and";
  }

  const catalogue = [...sourceNameList.value];
  const enabled = catalogue
    .filter((sourceName) => getObjectProperty(sourceEnabledFlags.value, sourceName) === true)
    .sort();

  if (enabled.length > 0) {
    const scopeClauses: string[] = [];
    for (const sourceName of enabled) {
      scopeClauses.push(`${sourceName}:${normalizedScopedPathForSource(sourceName)}`);
    }
    if (scopeClauses.length === 1) {
      query.scope = scopeClauses[0] as string;
    } else {
      query.scope = scopeClauses;
    }
  }
  Object.assign(query, toRouteQuery());
  if (resultLimit.value !== Math.min(500, maxSearchResults.value)) {
    query.limit = String(resultLimit.value);
  }
  return query;
}

function applyQueryFromRoute() {
  const q: QueryRecord = route.query || {};
  const catalogue = [...sourceNameList.value];

  const terms = parseTermsFromRouteQuery(q);
  termInputs.value = terms.length > 0 ? terms.map((termCell) => String(termCell)) : [""];

  const termJoinRaw = q.termJoin;
  termsJoinAnd.value = String(termJoinRaw ?? "").trim().toLowerCase() === "and";

  const sourcesStr = typeof q.sources === "string" ? q.sources.trim() : "";
  const rawSources = sourcesStr === ""
    ? []
    : sourcesStr.split(",").map((segment) => segment.trim()).filter((segment) => segment !== "");

  const validSel: string[] = [];
  for (const rawSourceName of rawSources) {
    if (catalogue.includes(rawSourceName)) {
      validSel.push(rawSourceName);
    }
  }

  const implicitAllSources = validSel.length === 0 && hasFilterOrTermSignalsForImplicitSources(q);

  const scopeRaws = collectScopeRawFromQuery(q);

  const colonPairs: { source: string; path: string }[] = [];

  for (const scopeRaw of scopeRaws) {
    const raw = String(scopeRaw || "").trim();
    if (raw === "") continue;
    const colonIndex = raw.indexOf(":");
    if (colonIndex > 0) {
      const sourceNameFromClause = raw.slice(0, colonIndex).trim();
      let relativePath = raw.slice(colonIndex + 1).trim();
      if (sourceNameFromClause === "") continue;
      if (relativePath === "") relativePath = "/";
      if (!relativePath.startsWith("/")) relativePath = `/${relativePath}`;
      if (catalogue.includes(sourceNameFromClause)) {
        colonPairs.push({ source: sourceNameFromClause, path: relativePath });
      }
    }
  }

  if (colonPairs.length > 0) {
    let nextFlags: QueryRecord = Object.fromEntries(catalogue.map(name => [name, false]));
    const scopedPathBySource = Object.fromEntries(
      colonPairs.map(pair => [pair.source, pair.path])
    );
    for (const sourcePathPair of colonPairs) {
      nextFlags = setObjectProperty(nextFlags, sourcePathPair.source, true);
    }
    sourceEnabledFlags.value = nextFlags;
    sourceScopedPaths.value = scopedPathBySource;
  } else {
    const nextFlagsEntries = catalogue.map((catalogueSourceName): [string, boolean] => {
      let sourceEnabled = false;
      if (validSel.length > 0) {
        sourceEnabled = validSel.includes(catalogueSourceName);
      } else if (implicitAllSources) {
        sourceEnabled = true;
      }
      return [catalogueSourceName, sourceEnabled];
    });
    const nextFlags = Object.fromEntries(nextFlagsEntries);
    sourceEnabledFlags.value = nextFlags;

    const enabledNames = catalogue.filter(
      (sourceName) => getObjectProperty(nextFlags, sourceName) === true
    );
    const scopedPathEntries = enabledNames.map((enabledSourceName): [string, string] => [
      enabledSourceName,
      "/",
    ]);
    sourceScopedPaths.value = Object.fromEntries(scopedPathEntries);
  }
  applyRouteQuery(q);

  const requestedLimit = Number(q.limit);
  resultLimit.value = Number.isSafeInteger(requestedLimit) && requestedLimit > 0
    ? Math.min(requestedLimit, maxSearchResults.value)
    : Math.min(500, maxSearchResults.value);

  applyDefaultCurrentSourceIfNone();
}

function updateAdvancedSearchUrl() {
  if (!isAdvancedSearchRoute.value || isInitializing) {
    return;
  }

  const nextPlain = normalizeRouteQueryLoose(buildRouteQueryFromState());

  const curQ = Object.fromEntries(Object.entries(route.query || {}));
  const curPlain = normalizeRouteQueryLoose(curQ);

  if (
    stableQueryStringFromObject(nextPlain) ===
    stableQueryStringFromObject(curPlain)
  ) {
    return;
  }

  suppressRouteQueryNavigation = true;
  router
    .replace({
      path: route.path,
      query: Object.keys(nextPlain).length > 0 ? nextPlain : {},
    })
    .catch(() => {})
    .finally(() => {
      void nextTick().then(() => {
        suppressRouteQueryNavigation = false;
      });
    });
}

function scheduleAdvancedSearchUrlUpdate() {
  if (!isAdvancedSearchRoute.value || isInitializing) {
    return;
  }
  void nextTick().then(() => {
    updateAdvancedSearchUrl();
  });
}

function addTermField() {
  termInputs.value.push("");
}

function removeTermAt(index: number) {
  if (termInputs.value.length <= 1) {
    return;
  }
  termInputs.value.splice(index, 1);
}

function listingItemKey(item: FileListItem) {
  return `${item.source}::${item.path}`;
}

function selectedItemKeys() {
  return (state.selected as number[])
    .map((index) => state.req.items.at(index))
    .filter((item): item is FileListItem => !!item)
    .map(listingItemKey);
}

// Indexes change when the results do so the selection follows the items
function restoreSelection(keys: string[]) {
  const known = new Set(keys);
  mutations.setSelected(
    (state.req.items as fileListItemIndex[])
      .filter((item) => known.has(listingItemKey(item)))
      .map((item) => item.index),
  );
}

async function runSearch(options: { silent?: boolean } = {}) {
  error.value = null;

  if (activeSources.value.length === 0) {
    abortSearch();
    error.value = t("tools.advancedSearch.invalidSources");
    searchExecuted.value = true;
    resultsEmpty.value = false;
    loading.value = false;
    return;
  }

  const terms: string[] = [];
  for (const rawTerm of termInputs.value) {
    const trimmedTerm = String(rawTerm || "").trim();
    if (trimmedTerm !== "") {
      terms.push(trimmedTerm);
    }
  }

  if (terms.length === 0) {
    abortSearch();
    error.value = t("tools.advancedSearch.noTerms");
    searchExecuted.value = true;
    loading.value = false;
    return;
  }

  for (const term of terms) {
    if (term.length < globalVars.minSearchLength) {
      abortSearch();
      error.value = t("tools.advancedSearch.termTooShort", {
        minSearchLength: globalVars.minSearchLength,
      });
      searchExecuted.value = true;
      loading.value = false;
      return;
    }
  }

  searchExecuted.value = true;
  if (!options.silent) loading.value = true;
  error.value = null;

  const { queryPrefix, dateParams } = buildRequest();
  const perSourceScopes: { source: string; path: string }[] = [];
  for (const activeSourceName of activeSources.value) {
    perSourceScopes.push({
      source: activeSourceName,
      path: normalizedScopedPathForSource(activeSourceName),
    });
  }
  const searchRequestKey = searchKey();
  await runSearchRequest(
    async (signal) => {
      const rows: SearchResultRow[] = await toolsApi.search(
        null,
        activeSources.value,
        queryPrefix,
        false,
        {
          ...dateParams,
          limit: resultLimit.value,
          terms,
          termJoin: termsJoinAnd.value ? "and" : undefined,
          perSourceScopes,
          signal,
        }
      );
      if (!Array.isArray(rows)) {
        throw new Error("Invalid search response");
      }
      return rows;
    },
    (rows) => {
      const listingItems = rows.map((searchResult): FileListItem => {
        const resultSource = String(searchResult.source || "");
        const isDir = String(searchResult.type) === "directory";
        const fullPath = browsePath(
          normalizedScopedPathForSource(resultSource),
          searchResult.path || "",
          isDir
        );
        return {
          name: displayName(fullPath, isDir),
          path: fullPath,
          type: isDir ? "directory" : String(searchResult.type || "application/octet-stream"),
          size: typeof searchResult.size === "number" ? searchResult.size : 0,
          modified: typeof searchResult.modified === "string" ? searchResult.modified : "",
          created: typeof searchResult.created === "string" ? searchResult.created : "",
          hasPreview: !!searchResult.hasPreview,
          source: resultSource,
          isShared: false,
          hidden: false,
        };
      });
      state.searchResults = { kind: "advanced", key: searchRequestKey, items: listingItems };
      const selectedKeys = options.silent ? selectedItemKeys() : [];
      showResults(listingItems);
      if (options.silent) restoreSelection(selectedKeys);
    },
    (err) => {
      const message = (err as { message?: unknown } | null)?.message;
      error.value = typeof message === "string" ? message : t("errors.internal");
    },
  );
  if (!isRunning.value) {
    loading.value = false;
  }
}

watch(() => state.reload, (shouldReload) => {
  if (!shouldReload || !isAdvancedSearchRoute.value || !searchExecuted.value) {
    return;
  }
  mutations.setReload(false);
  scheduleSearchRefresh();
});

watch(termInputs, () => {
  if (termInputs.value.length <= 1) {
    termsJoinAnd.value = false;
  }
  scheduleAdvancedSearchUrlUpdate();
}, { deep: true });

watch(termsJoinAnd, scheduleAdvancedSearchUrlUpdate);

watch(sourceEnabledFlags, scheduleAdvancedSearchUrlUpdate, { deep: true });

watch(sourceScopedPaths, scheduleAdvancedSearchUrlUpdate, { deep: true });

watch(resultLimit, scheduleAdvancedSearchUrlUpdate);

watch(() => route.query, () => {
  if (!isAdvancedSearchRoute.value) {
    return;
  }
  if (suppressRouteQueryNavigation || isInitializing) {
    return;
  }
  if (!catalogRouteBootstrapDone) {
    return;
  }

  const qRoute = normalizeRouteQueryLoose(route.query);

  const qFormRebuild = normalizeRouteQueryLoose(buildRouteQueryFromState());

  if (
    stableQueryStringFromObject(qRoute) ===
    stableQueryStringFromObject(qFormRebuild)
  ) {
    return;
  }

  applyQueryFromRoute();
  if (hasAnyAdvancedSearchRouteParams(route.query)) {
    void nextTick().then(() => {
      void runSearch();
    });
  }
  scheduleAdvancedSearchUrlUpdate();
}, { deep: true });

watch(() => state.sources.info, () => {
  const info = state.sources.info || {};
  const keys = Object.keys(info);
  let next = sourceEnabledFlags.value;
  for (const catalogueSourceName of keys) {
    if (!(catalogueSourceName in next)) {
      next = setObjectProperty(next, catalogueSourceName, false);
    }
  }
  const nextEntries = Object.entries(next).filter(([key]) => key in info);
  next = Object.fromEntries(nextEntries);
  sourceEnabledFlags.value = next;
  let pathNext = sourceScopedPaths.value;
  for (const catalogueSourceName of keys) {
    const current = getObjectProperty(pathNext, catalogueSourceName);
    if (current === undefined || current === null || current === "") {
      pathNext = setObjectProperty(pathNext, catalogueSourceName, "/") as Record<string, string>;
    }
  }
  const pathEntries = Object.entries(pathNext).filter(([key]) => key in info);
  pathNext = Object.fromEntries(pathEntries);
  sourceScopedPaths.value = pathNext;
  applyDefaultCurrentSourceIfNone();
  maybeFinishCatalogRouteBootstrapFromSources();
}, { immediate: true, deep: true });

watch(activeSources, (names) => {
  const list = Array.isArray(names) ? names : [];
  let next = sourceScopedPaths.value;
  for (const activeSourceName of list) {
    const current = getObjectProperty(next, activeSourceName);
    if (current === undefined || current === null || current === "") {
      next = setObjectProperty(next, activeSourceName, "/") as Record<string, string>;
    }
  }
  sourceScopedPaths.value = next;
}, { deep: true });

onMounted(() => {
  document.title = `${globalVars.name} - ${t("tools.title")} - ${t("tools.advancedSearch.name")}`;
  mutations.setSearch(false);

  scrollContainer = rootEl.value?.closest<HTMLElement>(".scroll-wrapper") ?? null;
  scrollContainer?.addEventListener("scroll", handleScroll, { passive: true });

  eventBus.on("itemsDeleted", handleSearchResultsChanged);
  eventBus.on("itemsMoved", handleSearchResultsChanged);
  eventBus.on("itemsRenamed", handleSearchResultsChanged);

  if (state.req !== null && state.req !== undefined) {
    hadInitialReq = true;
    initialReqFrozen = JSON.stringify(state.req);
  } else {
    hadInitialReq = false;
    initialReqFrozen = "";
  }

  if (typeof window !== "undefined") {
    emptyCatalogHydrateTimer = window.setTimeout(() => {
      emptyCatalogHydrateTimer = null;
      if (!catalogRouteBootstrapDone && isAdvancedSearchRoute.value) {
        finishCatalogBootstrapFromRoute();
      }
    }, 520);
  }
});

onBeforeUnmount(() => {
  if (
    typeof window !== "undefined" &&
    emptyCatalogHydrateTimer !== null
  ) {
    window.clearTimeout(emptyCatalogHydrateTimer);
    emptyCatalogHydrateTimer = null;
  }

  scrollContainer?.removeEventListener("scroll", handleScroll);

  eventBus.off("itemsDeleted", handleSearchResultsChanged);
  eventBus.off("itemsMoved", handleSearchResultsChanged);
  eventBus.off("itemsRenamed", handleSearchResultsChanged);

  if (!didApplySearchListing) {
    return;
  }
  if (!hadInitialReq) {
    mutations.clearRequest();
    return;
  }
  mutations.replaceRequest(JSON.parse(initialReqFrozen));
});
</script>

<style scoped>
.settings-number-input,
.settings-number-input > div {
  display: flex;
  align-items: center;
  gap: 1em;
}

.settings-number-input {
  justify-content: space-between;
  flex-wrap: wrap;
}

.range-value {
  min-width: 4ch;
  text-align: center;
}

.advanced-search-root {
  display: flex;
  flex-direction: column;
  width: 100%;
  flex: 1;
  align-self: stretch;
  min-height: 0;
}

.advanced-search-toolbar {
  margin: 1em;
}

.advanced-search-card-form {
  display: flex;
  flex-direction: column;
  width: 100%;
  box-sizing: border-box;
}

.advanced-search-config-row {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 1.25rem;
  width: 100%;
  box-sizing: border-box;
}

.config-primary,
.config-secondary {
  flex: 1 1 min(500px, 100%);
  min-width: min(500px, 100%);
  max-width: 100%;
  box-sizing: border-box;
}

.config-secondary {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.advanced-search-options-section {
  width: 100%;
  box-sizing: border-box;
}

.advanced-search-options-section :deep(.settings-group) {
  width: 100%;
  max-width: 100%;
}

.as-section-label {
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--textSecondary);
  margin-bottom: 0.35rem;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.term-join-divider {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.2em;
  width: fit-content;
  max-width: 100%;
  margin: 0.25em auto;
  box-sizing: border-box;
}

.term-join-line {
  flex: 0 0 auto;
  width: 1.65em;
  height: 1px;
  align-self: center;
  background-color: var(--divider, #aaa);
}

.term-join-text {
  flex-shrink: 0;
  font-size: 0.8125rem;
  line-height: 1.2;
  font-weight: 600;
  color: var(--textSecondary);
  user-select: none;
}

.advanced-search-inline-header {
  width: 100%;
  flex-shrink: 0;
}

.advanced-search-sticky-header {
  position: sticky;
  top: 0;
  z-index: 999;
  padding: 0.5em 0;
  backdrop-filter: blur(8px);
  transition: transform 0.25s ease, opacity 0.25s ease;
}

.advanced-search-sticky-header.header-hidden {
  transform: translateY(-100%);
  opacity: 0;
  pointer-events: none;
}

.search-term-row {
  display: flex;
  align-items: stretch;
  gap: 0;
}

.flex-grow-input {
  flex: 1;
  min-width: 0;
}

.add-term-button {
  margin: 0.35rem 0 0.75rem;
}

.advanced-search-switch {
  margin-top: 0.25rem;
}

.source-dropdown-select {
  width: 100%;
}

.scope-picker {
  margin-top: 0.35rem;
}

.advanced-search-actions-row {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
  margin-top: auto;
  width: 100%;
}

.search-submit {
  margin-left: auto;
}

@media (width <= 559px) {
  .advanced-search-actions-row .search-submit {
    margin-left: 0;
    flex: 1 1 100%;
    width: 100%;
  }
}

.advanced-search-results-plane {
  flex: 1;
  width: 100%;
  display: flex;
  flex-direction: column;
  min-height: min(65vh, 800px);
}

.results-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.75em;
  padding: 2rem;
}

.padded-message {
  padding: 1.5rem;
}

.error-message {
  color: var(--error, #c00);
}

.empty-state-message {
  text-align: center;
  color: var(--textSecondary);
}

.spin {
  animation: rotate 1s linear infinite;
}

@keyframes rotate {
  100% {
    transform: rotate(360deg);
  }
}

.advanced-search-listing-results .listing-items {
  position: relative;
}

.advanced-search-listing-results {
  margin: 1em;
  margin-top: 0;
}

.advanced-search-inner {
  margin-top: 0.5em;
}

.advanced-search-options-section :deep(.constraints) {
  box-sizing: border-box;
  margin-left: 1em;
  margin-right: 1em;
  width: calc(100% - 2em);
}

</style>
