<template>
  <div class="advanced-search-root">
    <div class="card advanced-search-toolbar padding-normal">
      <form class="card-content advanced-search-card-form" @submit.prevent="runSearch">
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
              <div class="source-toggles-wrap">
                <ToggleSwitch
                  v-for="name in sourceNameList"
                  :key="name"
                  class="source-toggle"
                  :model-value="isSourceEnabled(name)"
                  @update:model-value="(v) => setSourceEnabled(name, v)"
                  :name="name"
                />
              </div>
              <template
                v-for="srcName in activeSources"
                :key="'scope-' + srcName"
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
              <SettingsItem
                :title="advancedOptionsExpanded ? $t('buttons.showLess') : $t('buttons.showMore')"
                :collapsable="true"
                :start-collapsed="!advancedOptionsExpanded"
                @toggle="advancedOptionsExpanded = $event"
              >
                <div class="advanced-search-options-pane">
                  <ButtonGroup
                    :buttons="folderSelectButtons"
                    @button-clicked="addToTypes"
                    @remove-button-clicked="removeFromTypes"
                    @disable-all="folderSelectClicked"
                    @enable-all="resetButtonGroups"
                  />
                  <ButtonGroup
                    :buttons="typeSelectButtons"
                    :is-disabled="isTypeSelectDisabled"
                    @button-clicked="addToTypes"
                    @remove-button-clicked="removeFromTypes"
                  />
                  <div class="constraints">
                    <div class="sizeInputWrapper">
                      <p>{{ $t("search.smallerThan") }}</p>
                      <input
                        class="sizeInput"
                        v-model="smallerThan"
                        type="number"
                        min="0"
                        placeholder="MB"
                      />
                      <p>{{ $t("search.largerThan") }}</p>
                      <input class="sizeInput" v-model="largerThan" type="number" placeholder="MB" />
                    </div>
                    <div class="sizeInputWrapper">
                      <p>{{ $t("search.olderThanDate") }}</p>
                      <input class="sizeInput" v-model="modifiedOlderThan" type="date" />
                      <p>{{ $t("search.newerThanDate") }}</p>
                      <input class="sizeInput" v-model="modifiedNewerThan" type="date" />
                    </div>
                  </div>
                  <div class="settings-items">
                    <ToggleSwitch
                      v-model="useWildcardSearch"
                      class="item"
                      :name="$t('search.useWildcardSearch')"
                      :description="$t('search.useWildcardSearchDescription')"
                    />
                    <ToggleSwitch
                      v-model="caseExactSearch"
                      class="item"
                      :name="$t('tools.advancedSearch.caseExact')"
                    />
                  </div>
                </div>
              </SettingsItem>
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
        <div v-else class="advanced-search-listing-results">
          <ListingHeader class="advanced-search-inline-header" />
          <div class="advanced-search-inner no-select">
            <div
              ref="listingView"
              :class="{
                'add-padding': isStickySidebar,
                [listingViewMode]: true,
              }"
              :style="listingItemStyles"
              class="listing-items file-icons"
              @contextmenu="openListingContext"
            >
              <template v-if="reqListing.dirs.length > 0">
                <div>
                  <h2 :class="{ 'dark-mode': isDarkMode }">{{ $t("general.folders") }}</h2>
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
                  <h2 :class="{ 'dark-mode': isDarkMode }">{{ $t("general.files") }}</h2>
                </div>
                <div class="file-items" :class="{ lastGroup: true }" aria-label="File Items">
                  <Item
                    v-for="entry in reqListing.files"
                    :key="listingItemKey(entry)"
                    :index="entry.index"
                    :name="entry.name"
                    :is-dir="entry.type === 'directory'"
                    :modified="entry.modified"
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

<script>
import { toolsApi } from "@/api";
import router from "@/router";
import { state, getters, mutations } from "@/store";
import { globalVars } from "@/utils/constants";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import SettingsItem from "@/components/settings/SettingsItem.vue";
import ButtonGroup from "@/components/ButtonGroup.vue";
import PathPickerButton from "@/components/files/PathPickerButton.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import Item from "@/components/files/ListingItem.vue";
import { url } from "@/utils";
import ListingHeader from "@/components/files/ListingHeader.vue";
import { utcStartOfDaySecondsFromDateInput } from "@/utils/moment";

/**
 * Builds a canonical browse path within the selected scope (mirror of Search.vue getItemUrl path join).
 */
function browsePath(scopePath, apiPath, isDirectory) {
  const context =
    scopePath && scopePath !== "/" ? url.removeTrailingSlash(scopePath) : "";
  const trimmedApi = String(apiPath || "").trim();

  if (trimmedApi === "/" || trimmedApi === "") {
    if (context) {
      return context.endsWith("/") ? context : `${context}/`;
    }
    return "/";
  }

  let relativeSegmentsPath = url.removeLeadingSlash(url.removeTrailingSlash(trimmedApi));
  let resolvedBrowsePath =
    context === "" ? `/${relativeSegmentsPath}` : `${context}/${relativeSegmentsPath}`;

  if (isDirectory && !resolvedBrowsePath.endsWith("/")) {
    resolvedBrowsePath += "/";
  }
  if (!isDirectory && resolvedBrowsePath.endsWith("/") && resolvedBrowsePath.length > 1) {
    resolvedBrowsePath = url.removeTrailingSlash(resolvedBrowsePath);
  }

  return resolvedBrowsePath;
}

function displayName(fullPath, isDirectory) {
  const withoutTrailing =
    fullPath.endsWith("/") && fullPath.length > 1
      ? fullPath.slice(0, -1)
      : fullPath;
  const parts = withoutTrailing.split("/").filter(Boolean);
  const name = parts.length ? parts[parts.length - 1] : "";

  return name !== "" ? name : (isDirectory ? "/" : "?");
}

function queryTruthy(v) {
  if (v === undefined || v === null || v === "") {
    return false;
  }
  if (v === true || v === 1) {
    return true;
  }
  const s = String(v).toLowerCase();
  return s === "1" || s === "true" || s === "yes";
}

/** @param {Record<string, unknown>} routeQuery */
function parseTermsFromRouteQuery(routeQuery) {
  const q = routeQuery || {};
  const out = [];

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

/** @param {Record<string, unknown>} q */
function hasFilterOrTermSignalsForImplicitSources(q) {
  if (parseTermsFromRouteQuery(q).length > 0) {
    return true;
  }

  /** @param {string} queryKey */
  const nonempty = (queryKey) => {
    const raw = q[queryKey];
    return raw !== undefined && raw !== null && String(raw).trim() !== "";
  };

  if (
    nonempty("types") ||
    nonempty("largerThan") ||
    nonempty("smallerThan") ||
    nonempty("dateOlder") ||
    nonempty("dateNewer")
  ) {
    return true;
  }

  if (queryTruthy(q.wildcard) || queryTruthy(q.caseExact) || queryTruthy(q.typeLock)) {
    return true;
  }

  return false;
}

/** @param {Record<string, unknown>} q */
function hasAnyAdvancedSearchRouteParams(q) {
  if (hasFilterOrTermSignalsForImplicitSources(q)) {
    return true;
  }

  /** @param {string} queryKey */
  const nonempty = (queryKey) => {
    const raw = q[queryKey];
    return raw !== undefined && raw !== null && String(raw).trim() !== "";
  };

  if (
    nonempty("sources") ||
    nonempty("scope") ||
    String(q.termJoin || "").toLowerCase() === "and"
  ) {
    return true;
  }

  return false;
}

/** Canonicalize vue-router query for comparisons. */
/** @param {Record<string, unknown>} routeQuery */
function normalizeRouteQueryLoose(routeQuery) {
  const q = routeQuery || {};
  const next = {};

  /** @type {string[]} */
  const keys = Object.keys(q).sort();
  for (const queryKey of keys) {
    const rawValue = q[queryKey];
    if (rawValue === undefined || rawValue === null) {
      continue;
    }
    if (Array.isArray(rawValue)) {
      const sortedStrings = rawValue
        .map((entry) => String(entry))
        .filter((entry) => entry !== "")
        .sort();
      if (sortedStrings.length === 1) {
        next[queryKey] = sortedStrings[0];
      } else if (sortedStrings.length > 1) {
        next[queryKey] = sortedStrings;
      }
    } else {
      const stringValue = String(rawValue);
      if (stringValue !== "") {
        next[queryKey] = stringValue;
      }
    }
  }
  return next;
}

/** @param {Record<string, string|string[]|unknown>} obj */
function stableQueryStringFromObject(obj) {
  /** @type {string[][]} */
  let pairs = [];
  const keys = Object.keys(obj).sort();
  for (const paramKey of keys) {
    const paramValue = obj[paramKey];
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
/** @param {Record<string, unknown>} q */
function collectScopeRawFromQuery(q) {
  const raw = q.scope;
  if (raw === undefined || raw === null) {
    return [];
  }
  if (Array.isArray(raw)) {
    return raw.map((scopeFragment) => String(scopeFragment));
  }
  return [String(raw)];
}

export default {
  name: "AdvancedSearch",
  components: {
    ToggleSwitch,
    SettingsItem,
    ButtonGroup,
    PathPickerButton,
    LoadingSpinner,
    Item,
    ListingHeader,
  },
  data() {
    return {
      termInputs: [""],
      termsJoinAnd: false,
      sourceEnabledFlags: {},
      sourceScopedPaths: {},
      columnWidth: 250 + state.user.gallerySize * 50,
      innerWidth: typeof window !== "undefined" ? window.innerWidth : 1024,
      resizeListener: null,
      largerThan: "",
      smallerThan: "",
      modifiedOlderThan: "",
      modifiedNewerThan: "",
      searchTypes: "",
      isTypeSelectDisabled: false,
      useWildcardSearch: false,
      caseExactSearch: false,
      loading: false,
      error: null,
      searchExecuted: false,
      resultsEmpty: false,
      didApplySearchListing: false,
      hadInitialReq: false,
      initialReqFrozen: "",
      catalogRouteBootstrapDone: false,
      isInitializing: true,
      suppressRouteQueryNavigation: false,
      emptyCatalogHydrateTimer: null,
      advancedOptionsExpanded: false,
    };
  },
  computed: {
    isAdvancedSearchRoute() {
      return (this.$route.path || "") === "/tools/advancedSearch";
    },
    shareInfo() {
      return state.shareInfo;
    },
    sourcesInfoReactive() {
      return state.sources.info;
    },
    gallerySizeBump() {
      return state.user.gallerySize;
    },
    /** Keys from `state.sources.info`, sorted (sidebar catalogue). */
    sourceNameList() {
      return Object.keys(state.sources.info || {}).sort();
    },
    disableSearchOptions() {
      return state.user.disableSearchOptions;
    },
    folderSelectButtons() {
      return [
        { label: this.$t("search.onlyFolders"), value: "type:folder" },
        { label: this.$t("search.onlyFiles"), value: "type:file" },
      ];
    },
    typeSelectButtons() {
      return [
        { label: this.$t("general.photos"), value: "type:image" },
        { label: this.$t("general.audio"), value: "type:audio" },
        { label: this.$t("general.videos"), value: "type:video" },
        { label: this.$t("general.documents"), value: "type:doc" },
        { label: this.$t("general.archives"), value: "type:archive" },
      ];
    },
    termJoinDividerLabel() {
      const raw = this.termsJoinAnd
        ? this.$t("general.and")
        : this.$t("general.or");
      return String(raw ?? "").toUpperCase();
    },
    activeSources() {
      const info = state.sources.info || {};
      return Object.keys(info)
        .filter((name) => this.sourceEnabledFlags[name] === true)
        .sort();
    },
    isStickySidebar() {
      return getters.isStickySidebar();
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    listingViewMode() {
      return getters.viewMode();
    },
    reqListing() {
      const grouped = getters.reqItems();
      if (!grouped.dirs || !grouped.files) {
        return { dirs: [], files: [] };
      }
      return grouped;
    },
    listingNumColumns() {
      void this.innerWidth;
      if (!getters.isCardView()) {
        return 1;
      }
      const elem = document.querySelector("#main");
      if (!elem) {
        return 1;
      }
      if (getters.viewMode() === "icons") {
        const containerSize = 70 + state.user.gallerySize * 15;
        let columns = Math.floor(elem.offsetWidth / containerSize);
        if (columns === 0) {
          columns = 1;
        }
        const minColumns = 3;
        const maxColumns = 12;
        return Math.max(minColumns, Math.min(columns, maxColumns));
      }
      let columns = Math.floor(elem.offsetWidth / this.columnWidth);
      if (columns === 0) {
        columns = 1;
      }
      return columns;
    },
    listingItemStyles() {
      const viewMode = getters.viewMode();
      const styles = {};
      const size = state.user.gallerySize;

      if (viewMode === "icons") {
        const baseSize = 20 + size * 15;
        const cellSize = baseSize + 30;
        styles["--icon-size"] = `${baseSize}px`;
        styles["--icon-font-size"] = `${baseSize}px`;
        styles["--icons-view-cell-size"] = `${cellSize}px`;
      } else if (viewMode === "gallery") {
        const baseCalc = 80 + size * 25;
        const extraScaling = Math.max(0, size - 5) * 15;
        const baseSize = baseCalc + extraScaling;
        const iconFontSize = (3 + size * 0.5).toFixed(2);
        styles["--icon-font-size"] = `${iconFontSize}em`;

        if (state.isMobile) {
          const minWidth = size <= 3 ? 120 : size <= 7 ? 160 : 280;
          const mobileHeight = 120 + size * 20;
          styles["--gallery-mobile-min-width"] = `${minWidth}px`;
          styles["--item-width"] = `${minWidth}px`;
          styles["--item-height"] = `${mobileHeight}px`;
        } else {
          styles["--item-width"] = `${baseSize}px`;
          styles["--item-height"] = `${Math.round(baseSize * 1.2)}px`;
        }
      } else if (viewMode === "list" || viewMode === "compact") {
        const baseHeight =
          viewMode === "compact" ? 40 + size * 2 : 50 + size * 3;
        const iconSize = (2 + size * 0.12).toFixed(2);
        const iconFontSize = (1.5 + size * 0.12).toFixed(2);

        styles["--item-width"] = `calc(${(100 / this.listingNumColumns).toFixed(2)}% - 1em)`;
        styles["--item-height"] = `${baseHeight}px`;
        styles["--icon-size"] = `${iconSize}em`;
        styles["--icon-font-size"] = `${iconFontSize}em`;
      } else {
        const iconSize = (3.2 + size * 0.15).toFixed(2);
        const iconFontSize = (2.2 + size * 0.12).toFixed(2);

        styles["--item-width"] = `calc(${(100 / this.listingNumColumns)}% - 1em)`;
        styles["--item-height"] = "auto";
        styles["--icon-size"] = `${iconSize}em`;
        styles["--icon-font-size"] = `${iconFontSize}em`;
      }

      return styles;
    },
  },
  watch: {
    termInputs: {
      deep: true,
      handler() {
        if (this.termInputs.length <= 1) {
          this.termsJoinAnd = false;
        }
        this.scheduleAdvancedSearchUrlUpdate();
      },
    },
    termsJoinAnd() {
      this.scheduleAdvancedSearchUrlUpdate();
    },
    sourceEnabledFlags: {
      deep: true,
      handler() {
        this.scheduleAdvancedSearchUrlUpdate();
      },
    },
    sourceScopedPaths: {
      deep: true,
      handler() {
        this.scheduleAdvancedSearchUrlUpdate();
      },
    },
    searchTypes() {
      this.scheduleAdvancedSearchUrlUpdate();
    },
    largerThan() {
      this.scheduleAdvancedSearchUrlUpdate();
    },
    smallerThan() {
      this.scheduleAdvancedSearchUrlUpdate();
    },
    modifiedOlderThan() {
      this.scheduleAdvancedSearchUrlUpdate();
    },
    modifiedNewerThan() {
      this.scheduleAdvancedSearchUrlUpdate();
    },
    useWildcardSearch() {
      this.scheduleAdvancedSearchUrlUpdate();
    },
    caseExactSearch() {
      this.scheduleAdvancedSearchUrlUpdate();
    },
    isTypeSelectDisabled() {
      this.scheduleAdvancedSearchUrlUpdate();
    },
    "$route.query": {
      deep: true,
      handler() {
        if (!this.isAdvancedSearchRoute) {
          return;
        }
        if (this.suppressRouteQueryNavigation || this.isInitializing) {
          return;
        }
        if (!this.catalogRouteBootstrapDone) {
          return;
        }

        const qRoute = normalizeRouteQueryLoose(
          /** @type {Record<string, unknown>} */
          this.$route.query,
        );

        /** @type {Record<string, unknown>} */
        const qFormRebuild = normalizeRouteQueryLoose(
          this.buildRouteQueryFromState(),
        );

        if (
          stableQueryStringFromObject(qRoute) ===
          stableQueryStringFromObject(qFormRebuild)
        ) {
          return;
        }

        this.applyQueryFromRoute();
        if (hasAnyAdvancedSearchRouteParams(this.$route.query)) {
          this.$nextTick(() => void this.runSearch());
        }
        this.scheduleAdvancedSearchUrlUpdate();
      },
    },
    sourcesInfoReactive: {
      immediate: true,
      deep: true,
      handler() {
        const info = state.sources.info || {};
        const keys = Object.keys(info);
        const next = { ...this.sourceEnabledFlags };
        for (const catalogueSourceName of keys) {
          if (!(catalogueSourceName in next)) {
            next[catalogueSourceName] = false;
          }
        }
        Object.keys(next).forEach((name) => {
          if (!(name in info)) {
            delete next[name];
          }
        });
        this.sourceEnabledFlags = next;

        const pathNext = { ...this.sourceScopedPaths };
        for (const catalogueSourceName of keys) {
          if (
            pathNext[catalogueSourceName] === undefined ||
            pathNext[catalogueSourceName] === null ||
            pathNext[catalogueSourceName] === ""
          ) {
            pathNext[catalogueSourceName] = "/";
          }
        }
        Object.keys(pathNext).forEach((orphanKey) => {
          if (!(orphanKey in info)) {
            delete pathNext[orphanKey];
          }
        });
        this.sourceScopedPaths = pathNext;

        this.applyDefaultCurrentSourceIfNone();
        this.maybeFinishCatalogRouteBootstrapFromSources();
      },
    },
    activeSources: {
      deep: true,
      handler(names) {
        const list = Array.isArray(names) ? names : [];
        const next = { ...this.sourceScopedPaths };
        for (const activeSourceName of list) {
          if (
            next[activeSourceName] === undefined ||
            next[activeSourceName] === null ||
            next[activeSourceName] === ""
          ) {
            next[activeSourceName] = "/";
          }
        }
        this.sourceScopedPaths = next;
      },
    },
    gallerySizeBump() {
      this.columnWidth = 250 + state.user.gallerySize * 50;
    },
  },
  mounted() {
    document.title =
      globalVars.name +
      " - " +
      this.$t("tools.title") +
      " - " +
      this.$t("tools.advancedSearch.name");
    mutations.setSearch(false);

    this.resizeListener = () => {
      this.innerWidth = window.innerWidth;
    };
    window.addEventListener("resize", this.resizeListener);

    if (state.req !== null && state.req !== undefined) {
      this.hadInitialReq = true;
      this.initialReqFrozen = JSON.stringify(state.req);
    } else {
      this.hadInitialReq = false;
      this.initialReqFrozen = "";
    }

    if (typeof window !== "undefined") {
      this.emptyCatalogHydrateTimer = window.setTimeout(() => {
        this.emptyCatalogHydrateTimer = null;
        if (!this.catalogRouteBootstrapDone && this.isAdvancedSearchRoute) {
          this.finishCatalogBootstrapFromRoute();
        }
      }, 520);
    }
  },
  beforeUnmount() {
    if (
      typeof window !== "undefined" &&
      this.emptyCatalogHydrateTimer != null
    ) {
      window.clearTimeout(this.emptyCatalogHydrateTimer);
      this.emptyCatalogHydrateTimer = null;
    }

    if (this.resizeListener) {
      window.removeEventListener("resize", this.resizeListener);
      this.resizeListener = null;
    }
    if (!this.didApplySearchListing) {
      return;
    }
    if (!this.hadInitialReq) {
      mutations.clearRequest();
      return;
    }
    mutations.replaceRequest(JSON.parse(this.initialReqFrozen));
  },
  methods: {
    isSourceEnabled(name) {
      return this.sourceEnabledFlags[name] === true;
    },
    setSourceEnabled(name, enabled) {
      const nextFlags = {
        ...this.sourceEnabledFlags,
        [name]: !!enabled,
      };
      let nextPaths = { ...this.sourceScopedPaths };
      if (enabled) {
        if (nextPaths[name] === undefined || nextPaths[name] === null || nextPaths[name] === "") {
          nextPaths[name] = "/";
        }
      }
      this.sourceEnabledFlags = nextFlags;
      this.sourceScopedPaths = nextPaths;
    },
    /** When no source is toggled on, enable `state.sources.current` if known in the catalogue. */
    applyDefaultCurrentSourceIfNone() {
      if (!this.isAdvancedSearchRoute) {
        return;
      }
      const catalogue = [...this.sourceNameList];
      if (catalogue.length === 0) {
        return;
      }

      /** @type {string[]} */
      const enabledNames = catalogue.filter(
        (sourceName) => this.sourceEnabledFlags[sourceName] === true,
      );

      if (enabledNames.length > 0) {
        return;
      }

      const cur = state.sources.current;
      if (typeof cur === "string" && cur !== "" && catalogue.includes(cur)) {
        this.sourceEnabledFlags = {
          ...this.sourceEnabledFlags,
          [cur]: true,
        };
        const pathsNext = { ...this.sourceScopedPaths };
        if (pathsNext[cur] === undefined || pathsNext[cur] === null || pathsNext[cur] === "") {
          pathsNext[cur] = "/";
        }
        this.sourceScopedPaths = pathsNext;
      }
    },
    normalizedScopedPathForSource(sourceName) {
      let scopedPath = String(this.sourceScopedPaths[sourceName] || "/").trim();
      if (scopedPath === "") {
        scopedPath = "/";
      }
      if (!scopedPath.startsWith("/")) {
        scopedPath = `/${scopedPath}`;
      }
      return scopedPath;
    },
    maybeFinishCatalogRouteBootstrapFromSources() {
      if (!this.isAdvancedSearchRoute) {
        return;
      }
      if (this.catalogRouteBootstrapDone) {
        return;
      }
      if (Object.keys(state.sources.info || {}).length === 0) {
        return;
      }
      if (
        typeof window !== "undefined" &&
        this.emptyCatalogHydrateTimer !== null &&
        this.emptyCatalogHydrateTimer !== undefined
      ) {
        window.clearTimeout(this.emptyCatalogHydrateTimer);
        this.emptyCatalogHydrateTimer = null;
      }
      this.finishCatalogBootstrapFromRoute();
    },
    finishCatalogBootstrapFromRoute() {
      if (!this.isAdvancedSearchRoute) {
        return;
      }
      if (this.catalogRouteBootstrapDone) {
        return;
      }
      this.catalogRouteBootstrapDone = true;

      this.applyQueryFromRoute();
      if (
        hasAnyAdvancedSearchRouteParams(
          /** @type {Record<string, unknown>} */
          this.$route.query,
        )
      ) {
        this.$nextTick(() => {
          void this.runSearch();
        });
      }
      this.isInitializing = false;
      this.$nextTick(() => {
        this.updateAdvancedSearchUrl();
      });
    },
    buildRouteQueryFromState() {
      /** @type {Record<string, string | string[]>} */
      const query = {};

      const terms = this.termInputs
        .map((termCell) => String(termCell || "").trim())
        .filter((trimmedCell) => trimmedCell !== "");
      if (terms.length === 1) {
        query.term = terms[0];
      } else if (terms.length > 1) {
        query.term = terms;
      }

      if (this.termsJoinAnd) {
        query.termJoin = "and";
      }

      const catalogue = [...this.sourceNameList];
      const enabled = catalogue
        .filter((sourceName) => this.sourceEnabledFlags[sourceName] === true)
        .sort();

      if (enabled.length > 0) {
        /** @type {string[]} */
        const scopeClauses = [];
        for (const sourceName of enabled) {
          let scopedPath = String(this.sourceScopedPaths[sourceName] || "/").trim();
          if (scopedPath === "") {
            scopedPath = "/";
          }
          if (!scopedPath.startsWith("/")) {
            scopedPath = `/${scopedPath}`;
          }
          scopeClauses.push(`${sourceName}:${scopedPath}`);
        }
        if (scopeClauses.length === 1) {
          query.scope = scopeClauses[0];
        } else {
          query.scope = scopeClauses;
        }
      }

      const trimmedTypes = String(this.searchTypes || "").trim();
      if (trimmedTypes !== "") {
        query.types = trimmedTypes;
      }
      if (this.largerThan !== "") {
        query.largerThan = String(this.largerThan);
      }
      if (this.smallerThan !== "") {
        query.smallerThan = String(this.smallerThan);
      }
      if (this.modifiedOlderThan !== "") {
        query.dateOlder = String(this.modifiedOlderThan);
      }
      if (this.modifiedNewerThan !== "") {
        query.dateNewer = String(this.modifiedNewerThan);
      }
      if (this.useWildcardSearch) {
        query.wildcard = "1";
      }
      if (this.caseExactSearch) {
        query.caseExact = "1";
      }
      if (this.isTypeSelectDisabled) {
        query.typeLock = "1";
      }

      return query;
    },
    applyQueryFromRoute() {
      /** @type {Record<string, unknown>} */
      const q = this.$route.query || {};
      const catalogue = [...this.sourceNameList];

      const terms = parseTermsFromRouteQuery(q);
      this.termInputs =
        terms.length > 0 ? terms.map((termCell) => String(termCell)) : [""];

      const termJoinRaw = q.termJoin;
      this.termsJoinAnd =
        String(termJoinRaw ?? "").trim().toLowerCase() === "and";

      const sourcesStr =
        typeof q.sources === "string" ? q.sources.trim() : "";
      const rawSources =
        sourcesStr === ""
          ? []
          : sourcesStr
            .split(",")
            .map((segment) => segment.trim())
            .filter((segment) => segment !== "");

      /** @type {string[]} */
      const validSel = [];
      for (const rawSourceName of rawSources) {
        if (catalogue.includes(rawSourceName)) {
          validSel.push(rawSourceName);
        }
      }

      const implicitAllSources =
        validSel.length === 0 &&
        hasFilterOrTermSignalsForImplicitSources(q);

      const scopeRaws = collectScopeRawFromQuery(q);

      /** @type {{ source: string, path: string }[]} */
      const colonPairs = [];
      /** @type {string[]} */
      const bareScopeLegacy = [];

      for (const scopeRaw of scopeRaws) {
        const raw = String(scopeRaw || "").trim();
        if (raw === "") {
          continue;
        }
        const colonIndex = raw.indexOf(":");
        if (colonIndex > 0) {
          const sourceNameFromClause = raw.slice(0, colonIndex).trim();
          let relativePath = raw.slice(colonIndex + 1).trim();
          if (sourceNameFromClause === "") {
            continue;
          }
          if (relativePath === "") {
            relativePath = "/";
          }
          if (!relativePath.startsWith("/")) {
            relativePath = `/${relativePath}`;
          }
          if (catalogue.includes(sourceNameFromClause)) {
            colonPairs.push({ source: sourceNameFromClause, path: relativePath });
          }
        } else {
          bareScopeLegacy.push(raw.startsWith("/") ? raw : `/${raw}`);
        }
      }

      if (colonPairs.length > 0) {
        const nextFlags = {};
        for (const catalogueSourceName of catalogue) {
          nextFlags[catalogueSourceName] = false;
        }
        /** @type {Record<string, string>} */
        const scopedPathBySource = {};
        for (const sourcePathPair of colonPairs) {
          nextFlags[sourcePathPair.source] = true;
          scopedPathBySource[sourcePathPair.source] = sourcePathPair.path;
        }
        this.sourceEnabledFlags = nextFlags;
        this.sourceScopedPaths = { ...scopedPathBySource };
      } else {
        const nextFlags = {};
        for (const catalogueSourceName of catalogue) {
          let sourceEnabled = false;
          if (validSel.length > 0) {
            sourceEnabled = validSel.includes(catalogueSourceName);
          } else if (implicitAllSources) {
            sourceEnabled = true;
          }
          nextFlags[catalogueSourceName] = sourceEnabled;
        }
        this.sourceEnabledFlags = nextFlags;

        const flatLegacy =
          bareScopeLegacy.length > 0 ? bareScopeLegacy[0] : "/";
        /** @type {string[]} */
        const enabledNames = catalogue.filter(
          (sourceName) => nextFlags[sourceName] === true,
        );
        /** @type {Record<string, string>} */
        const scopedPathBySource = {};
        for (const enabledSourceName of enabledNames) {
          if (enabledNames.length === 1 && flatLegacy !== "/") {
            scopedPathBySource[enabledSourceName] = flatLegacy;
          } else {
            scopedPathBySource[enabledSourceName] = "/";
          }
        }
        this.sourceScopedPaths = scopedPathBySource;
      }

      const typesIncoming = q.types;
      this.searchTypes =
        typeof typesIncoming === "string"
          ? typesIncoming.trim()
          : typesIncoming !== undefined && typesIncoming !== null
          ? String(typesIncoming).trim()
          : "";

      const largerThanRaw = q.largerThan;
      this.largerThan =
        largerThanRaw !== undefined && largerThanRaw !== null
          ? String(largerThanRaw).trim()
          : "";

      const smallerThanRaw = q.smallerThan;
      this.smallerThan =
        smallerThanRaw !== undefined && smallerThanRaw !== null
          ? String(smallerThanRaw).trim()
          : "";

      const dateOlderRaw = q.dateOlder;
      this.modifiedOlderThan =
        dateOlderRaw !== undefined && dateOlderRaw !== null
          ? String(dateOlderRaw).trim()
          : "";

      const dateNewerRaw = q.dateNewer;
      this.modifiedNewerThan =
        dateNewerRaw !== undefined && dateNewerRaw !== null
          ? String(dateNewerRaw).trim()
          : "";

      this.useWildcardSearch = queryTruthy(q.wildcard);
      this.caseExactSearch = queryTruthy(q.caseExact);
      this.isTypeSelectDisabled = queryTruthy(q.typeLock);

      this.applyDefaultCurrentSourceIfNone();
    },
    updateAdvancedSearchUrl() {
      if (!this.isAdvancedSearchRoute || this.isInitializing) {
        return;
      }

      const nextPlain = normalizeRouteQueryLoose(this.buildRouteQueryFromState());

      /** @type {Record<string, unknown>} */
      const curQ = {};
      /** @type {Record<string, string | string[]>} */
      const rq = /** @type {Record<string, string | string[]>} */ (
        this.$route.query
      );
      const routeQueryKeys = Object.keys(rq || {});
      for (const routeQueryKey of routeQueryKeys) {
        curQ[routeQueryKey] = rq[routeQueryKey];
      }
      const curPlain = normalizeRouteQueryLoose(curQ);

      if (
        stableQueryStringFromObject(nextPlain) ===
        stableQueryStringFromObject(curPlain)
      ) {
        return;
      }

      this.suppressRouteQueryNavigation = true;
      router
        .replace({
          path: this.$route.path,
          query: Object.keys(nextPlain).length > 0 ? nextPlain : {},
        })
        .catch(() => {})
        .finally(() => {
          this.$nextTick(() => {
            this.suppressRouteQueryNavigation = false;
          });
        });
    },
    scheduleAdvancedSearchUrlUpdate() {
      if (!this.isAdvancedSearchRoute || this.isInitializing) {
        return;
      }
      this.$nextTick(() => {
        this.updateAdvancedSearchUrl();
      });
    },
    addTermField() {
      this.termInputs.push("");
    },
    removeTermAt(index) {
      if (this.termInputs.length <= 1) {
        return;
      }
      this.termInputs.splice(index, 1);
    },
    addToTypes(string) {
      if (string === null || string === undefined || string === "") {
        return false;
      }
      if (this.searchTypes.includes(string)) {
        return true;
      }
      this.searchTypes = this.searchTypes + string + " ";
      return true;
    },
    removeFromTypes(string) {
      if (string === null || string === undefined || string === "") {
        return false;
      }
      this.searchTypes = this.searchTypes.replaceAll(`${string} `, "");
      return true;
    },
    folderSelectClicked() {
      this.isTypeSelectDisabled = true;
    },
    resetButtonGroups() {
      this.isTypeSelectDisabled = false;
    },
    listingItemKey(item) {
      return `${item.source}::${item.path}`;
    },
    openListingContext(event) {
      event.preventDefault();
      event.stopPropagation();
      if (getters.currentPromptName() === "ContextMenu") {
        return;
      }

      mutations.showPrompt({
        name: "ContextMenu",
        props: {
          showCentered: getters.isMobile(),
          posX: event.clientX,
          posY: event.clientY,
          createOnly: state.selected.length === 0,
        },
      });
    },
    async runSearch() {
      this.error = null;

      if (this.activeSources.length === 0) {
        this.error = this.$t("tools.advancedSearch.invalidSources");
        this.searchExecuted = true;
        this.resultsEmpty = false;
        this.loading = false;
        return;
      }

      const terms = [];
      for (const rawTerm of this.termInputs) {
        const trimmedTerm = String(rawTerm || "").trim();
        if (trimmedTerm !== "") {
          terms.push(trimmedTerm);
        }
      }

      if (terms.length === 0) {
        this.error = this.$t("tools.advancedSearch.noTerms");
        this.searchExecuted = true;
        this.loading = false;
        return;
      }

      for (const term of terms) {
        if (term.length < globalVars.minSearchLength) {
          this.error = this.$t("tools.advancedSearch.termTooShort", {
            minSearchLength: globalVars.minSearchLength,
          });
          this.searchExecuted = true;
          this.loading = false;
          return;
        }
      }

      this.searchExecuted = true;
      this.loading = true;
      this.error = null;

      let searchTypesFull = this.searchTypes;
      if (this.largerThan !== "") {
        searchTypesFull = `${searchTypesFull}type:largerThan=${this.largerThan} `;
      }
      if (this.smallerThan !== "") {
        searchTypesFull = `${searchTypesFull}type:smallerThan=${this.smallerThan} `;
      }

      const dateParams = {};
      const olderUnix = utcStartOfDaySecondsFromDateInput(this.modifiedOlderThan);
      if (olderUnix !== null) {
        dateParams.olderThan = olderUnix;
      }
      const newerUnix = utcStartOfDaySecondsFromDateInput(this.modifiedNewerThan);
      if (newerUnix !== null) {
        dateParams.newerThan = newerUnix;
      }
      if (this.useWildcardSearch) {
        dateParams.useWildcard = true;
      }

      let typedQueryPrefix = `${searchTypesFull}`.trim();
      if (this.caseExactSearch) {
        typedQueryPrefix =
          typedQueryPrefix === ""
            ? "case:exact "
            : `case:exact ${typedQueryPrefix} `;
      }

      const perSourceScopes = [];
      for (const activeSourceName of this.activeSources) {
        perSourceScopes.push({
          source: activeSourceName,
          path: this.normalizedScopedPathForSource(activeSourceName),
        });
      }

      try {
        const rows = await toolsApi.search(
          null,
          this.activeSources,
          typedQueryPrefix,
          false,
          {
            ...dateParams,
            terms,
            termJoin: this.termsJoinAnd ? "and" : undefined,
            perSourceScopes,
          }
        );

        if (!Array.isArray(rows)) {
          throw new Error("Invalid search response");
        }

        const listingItems = rows.map((searchResult) => {
          const resultSource = String(searchResult.source || "");
          const isDir = String(searchResult.type) === "directory";
          const fullPath = browsePath(
            this.normalizedScopedPathForSource(resultSource),
            searchResult.path || "",
            isDir
          );
          const nm = displayName(fullPath, isDir);
          return {
            name: nm,
            path: fullPath,
            type: isDir ? "directory" : String(searchResult.type || "application/octet-stream"),
            size: typeof searchResult.size === "number" ? searchResult.size : 0,
            modified: typeof searchResult.modified === "string" ? searchResult.modified : "",
            hasPreview: !!(searchResult.hasPreview || searchResult.HasPreview),
            source: resultSource,
            isShared: false,
            hidden: false,
          };
        });

        mutations.replaceRequest({
          type: "directory",
          path: "/",
          name: this.$t("tools.advancedSearch.resultsTitle"),
          source: "",
          items: listingItems,
        });
        this.didApplySearchListing = true;
        if (listingItems.length === 0) {
          this.resultsEmpty = true;
        } else {
          this.resultsEmpty = false;
        }
      } catch (err) {
        this.error =
          err && typeof err.message === "string"
            ? err.message
            : this.$t("errors.internal");
      } finally {
        this.loading = false;
      }
    },
  },
};
</script>

<style scoped>
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

.source-toggles-wrap .source-toggle + .source-toggle {
  margin-top: 0.35rem;
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

@media (max-width: 559px) {
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

.advanced-search-listing-results {
  margin: 1em;
  margin-top: 0;
}

.advanced-search-inner {
  margin-top: 0.5em;
}

.advanced-search-options-pane {
  box-sizing: border-box;
  width: 100%;
}

.advanced-search-options-pane .constraints {
  box-sizing: border-box;
  margin-left: 1em;
  margin-right: 1em;
  width: calc(100% - 2em);
}

</style>
