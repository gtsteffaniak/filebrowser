<template>
  <div v-if="active" id="search" :class="{ active, ongoing, 'dark-mode': isDarkMode }" @click="clearContext">
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
        @keyup.exact="keyup" @input="submit" ref="input" :autofocus="active" v-model.trim="value"
        aria-label="search input" :placeholder="$t('general.search', { suffix: '...' })" />
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

      <div id="result-list">
        <div v-if="!disableSearchOptions">
          <div v-if="active">
            <SettingsItem
              class="search-options-settings-item"
              :title="advancedOptionsExpanded ? $t('buttons.showLess') : $t('buttons.showMore')"
              :collapsable="true"
              :start-collapsed="!advancedOptionsExpanded"
              @toggle="advancedOptionsExpanded = $event"
            >
              <div class="search-options-inner">
                <div class="search-filter-dropdowns">
                  <ExpandDropdown
                    v-model="entryTypeFilter"
                    :options="entryTypeOptions"
                    all-value="all"
                    :all-selected-label="$t('search.filesAndFolders')"
                    :aria-label="$t('search.filesAndFolders')"
                  />
                  <ExpandDropdown
                    v-model="selectedMediaTypes"
                    :options="mediaTypeOptions"
                    allow-multiple
                    empty-means-all
                    :all-selected-label="$t('search.allFileTypes')"
                    :disabled="foldersOnly"
                    :aria-label="$t('search.allFileTypes')"
                  />
                </div>
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
                    class="item"
                    v-model="showPreviewImages"
                    :name="$t('search.showPreviewImages')"
                    :description="$t('search.showPreviewImagesDescription')"
                  />
                  <ToggleSwitch
                    class="item"
                    v-model="useWildcardSearch"
                    :name="$t('search.useWildcardSearch')"
                    :description="$t('search.useWildcardSearchDescription')"
                  />
                  <ToggleSwitch
                    class="item"
                    v-model="caseExactSearch"
                    :name="$t('search.caseExact')"
                    :description="$t('search.caseExactDescription')"
                  />
                </div>
              </div>
            </SettingsItem>
          </div>
        </div>
        <!-- Loading icon when search is ongoing -->
          <LoadingSpinner v-if="isRunning" size="medium" />
        <!-- Message when no results are found -->
        <div class="searchPrompt" v-show="isEmpty && !isRunning">
          <p>{{ noneMessage }}</p>
          <i class="material-symbols-outlined tooltip-info-icon" @mouseenter="showHelpTooltip"
            @mouseleave="hideTooltip">
            help
          </i>
        </div>
        <!-- List of search results -->
        <ul v-show="results.length > 0">
          <li v-for="(s, k) in results" :key="k" class="search-entry clickable"
            :class="{ active: activeStates[k], 'large-icons': showPreviewImages, 'small-icons': !showPreviewImages }" :aria-label="baseName(s.path)">
            <a :href="getItemUrl(s)" @contextmenu="addSelected($event, s)">
              <Icon :mimetype="s.type" :filename="baseName(s.path)" :path="s.path"
                :hasPreview="showPreviewImages && (s.hasPreview || false)"
                :thumbnailUrl="showPreviewImages ? getThumbnailUrl(s) : ''" />
              <span class="text-container">
                {{ basePath(s.path, s.type === "directory") }}{{ baseName(s.path) }}
                <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
              </span>
              <div class="filesize">{{ humanSize(s.size) }}</div>
              <div v-if="isSearchingMultipleSources && s.source" class="source-badge">{{ s.source }}</div>
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

<script>
import { resourcesApi, toolsApi } from "@/api";
import Icon from "@/components/files/Icon.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import SettingsItem from "@/components/settings/SettingsItem.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import router from "@/router";
import { getters, mutations, state } from "@/store";
import { url } from "@/utils/";
import { globalVars } from "@/utils/constants";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { utcStartOfDaySecondsFromDateInput } from "@/utils/moment";

const boxes = {
  folder: { label: "folders", icon: "folder" },
  file: { label: "files", icon: "insert_drive_file" },
  archive: { label: "archives", icon: "archive" },
  image: { label: "images", icon: "photo" },
  audio: { label: "audio files", icon: "volume_up" },
  video: { label: "videos", icon: "movie" },
  doc: { label: "documents", icon: "picture_as_pdf" },
};

export default {
  components: {
    Icon,
    ToggleSwitch,
    SettingsItem,
    LoadingSpinner,
    ExpandDropdown,
  },
  name: "search",
  data: function () {
    return {
      largerThan: "",
      smallerThan: "",
      modifiedOlderThan: "",
      modifiedNewerThan: "",
      noneMessage: this.$t("search.typeToSearch", { minSearchLength: globalVars.minSearchLength }),
      entryTypeFilter: "all",
      selectedMediaTypes: [],
      showPreviewImages: false,
      useWildcardSearch: false,
      caseExactSearch: false,
      value: "",
      ongoing: 0,
      results: [],
      reload: false,
      scrollable: null,
      advancedOptionsExpanded: false,
      selectedSource: "",
    };
  },
  watch: {
    currentSourceState(){
      this.selectedSource = state.sources.current;
    },
    largerThan() {
      this.submit();
    },
    smallerThan() {
      this.submit();
    },
    modifiedOlderThan() {
      this.submit();
    },
    modifiedNewerThan() {
      this.submit();
    },
    useWildcardSearch() {
      this.submit();
    },
    caseExactSearch() {
      this.submit();
    },
    entryTypeFilter(newValue) {
      if (newValue === "type:folder" && this.selectedMediaTypes.length > 0) {
        this.selectedMediaTypes = [];
        return;
      }
      this.submit();
    },
    selectedMediaTypes: {
      deep: true,
      handler() {
        this.submit();
      },
    },
    value() {
      if (this.results.length) {
        this.ongoing = 0;
        this.results = [];
      }
    },
    active(isNowActive) {
      if (isNowActive) {
        this.resetSearchOnOpen();
      }
    },
  },
  mounted() {
    this.selectedSource = state.sources.current;
    // Adjust contextmenu listener based on browser
    if (state.isSafari) {
      // For Safari, add touchstart or mousedown to open the context menu
      this.$el.addEventListener("touchstart", this.openContextForSafari, {
        passive: true,
      });
      this.$el.addEventListener("mousedown", this.openContextForSafari);
      this.$el.addEventListener("touchmove", this.handleTouchMove);

      // Also clear the timeout if the user clicks or taps quickly
      this.$el.addEventListener("touchend", this.cancelContext);
      this.$el.addEventListener("mouseup", this.cancelContext);
    } else {
      // For other browsers, use regular contextmenu
      this.$el.addEventListener("contextmenu", this.openContext);
    }

    // Add keyboard event listener for "/" to activate search
    this.handleKeydown = (event) => {
      if (event.key === '/' || event.key === ' ' && !state.isSearchActive && getters.currentPrompt() === null) {
        event.preventDefault();
        this.open();
      }
    };

    document.addEventListener('keydown', this.handleKeydown);
  },
  beforeUnmount() {
    // If Safari, remove touchstart listener
    if (state.isSafari) {
      this.$el.removeEventListener("touchstart", this.openContextForSafari);
      this.$el.removeEventListener("mousedown", this.openContextForSafari);
      this.$el.removeEventListener("touchend", this.cancelContext);
      this.$el.removeEventListener("mouseup", this.cancelContext);
      this.$el.removeEventListener("touchmove", this.handleTouchMove);
    } else {
      this.$el.removeEventListener("contextmenu", this.openContext);
    }

    // Clean up keyboard event listener
    if (this.handleKeydown) {
      document.removeEventListener('keydown', this.handleKeydown);
    }
  },
  computed: {
    currentSourceState() {
      return state.sources.current;
    },
    eventTheme() {
      return getters.eventTheme();
    },
    disableSearchOptions() {
      return state.user?.disableSearchOptions;
    },
    foldersOnly() {
      return this.entryTypeFilter === "type:folder";
    },
    searchTypes() {
      const parts = [];
      if (this.entryTypeFilter !== "all") {
        parts.push(this.entryTypeFilter);
      }
      parts.push(...this.selectedMediaTypes);
      return parts.length > 0 ? `${parts.join(" ")} ` : "";
    },
    entryTypeOptions() {
      return [
        { value: "all", label: this.$t("search.filesAndFolders") },
        { value: "type:file", label: this.$t("search.onlyFiles") },
        { value: "type:folder", label: this.$t("search.onlyFolders") },
      ];
    },
    mediaTypeOptions() {
      return [
        { value: "type:image", label: this.$t("general.photos") },
        { value: "type:audio", label: this.$t("general.audio") },
        { value: "type:video", label: this.$t("general.videos") },
        { value: "type:doc", label: this.$t("general.documents") },
        { value: "type:archive", label: this.$t("general.archives") },
      ];
    },
    active() {
      return state.isSearchActive;
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    showBoxes() {
      return this.searchTypes === "";
    },
    boxes() {
      return boxes;
    },
    isEmpty() {
      return this.results.length === 0;
    },
    text() {
      if (this.ongoing > 0) {
        return "";
      }
      return this.$t("search.typeToSearch", { minSearchLength: globalVars.minSearchLength })
    },
    isRunning() {
      return this.ongoing > 0;
    },
    activeStates() {
      const selectedItems = state.selected ? [].concat(state.selected) : [];
      if (selectedItems.length === 0) {
        // Return an array of all false if nothing is selected
        return new Array(this.results.length).fill(false);
      }

      const selectedPaths = new Set(selectedItems.map((item) => item.path));
      return this.results.map((result) => {
        // Construct the same full path as addSelected does
        const context = url.removeTrailingSlash(this.getContext);
        const pathStr = url.removeLeadingSlash(url.removeTrailingSlash(result.path));
        const fullPath = `${context}/${pathStr}`;
        return selectedPaths.has(fullPath);
      });
    },
    sourceInfo() {
      return state.sources.info;
    },
    multipleSources() {
      return Object.keys(state.sources.info).length > 1;
    },
    searchSourceOptions() {
      const sources = Object.keys(this.sourceInfo).map((name) => ({
        value: name,
        label: name,
      }));
      return [
        { value: "__all__", label: this.$t("general.all") },
        ...sources,
      ];
    },
    isSearchingMultipleSources() {
      return this.selectedSource === "__all__" || (this.selectedSource === "" && this.multipleSources);
    },
    getContext() {
      // If searching all sources (explicitly or by default), always use root context
      if (this.selectedSource === "__all__" || (this.selectedSource === "" && this.multipleSources)) {
        return "/";
      }
      const result = url.extractSourceFromPath(decodeURIComponent(state.route.path));
      if (this.selectedSource === "" || result.source === this.selectedSource) {
        return result.path;
      } else {  
        return "/"; // if searching on non-current source, search the whole thing
      }
    },
  },
  methods: {
    openContext(event) {
      event.preventDefault();
      event.stopPropagation();
      mutations.showPrompt({
        name: "ContextMenu",
        props: {
          posX: event.clientX,
          posY: event.clientY,
          showLimitedOptions: true,
        },
      });
    },
    cancelContext() {
      if (this.contextTimeout) {
        clearTimeout(this.contextTimeout);
        this.contextTimeout = null;
      }
      this.isLongPress = false;
    },
    openContextForSafari(event) {
      this.cancelContext(); // Clear any previous timeouts
      this.isLongPress = false; // Reset state
      this.isSwipe = false; // Reset swipe detection

      const touch = event.touches[0];
      this.touchStartX = touch.clientX;
      this.touchStartY = touch.clientY;

      // Start the long press detection
      this.contextTimeout = setTimeout(() => {
        if (!this.isSwipe) {
          this.isLongPress = true;
          event.preventDefault(); // Suppress Safari's callout menu
          this.openContext(event); // Open the custom context menu
        }
      }, 500); // Long press delay (adjust as needed)
    },
    handleTouchMove(event) {
      const touch = event.touches[0];
      const deltaX = Math.abs(touch.clientX - this.touchStartX);
      const deltaY = Math.abs(touch.clientY - this.touchStartY);
      // Set a threshold for movement to detect a swipe
      const movementThreshold = 10; // Adjust as needed
      if (deltaX > movementThreshold || deltaY > movementThreshold) {
        this.isSwipe = true;
        this.cancelContext(); // Cancel long press if swipe is detected
      }
    },
    handleTouchEnd() {
      this.cancelContext(); // Clear timeout
      this.isSwipe = false; // Reset swipe state
    },
    updateSource(value) {
      this.selectedSource = value;
      void this.submit();
    },
    getItemUrl(s) {
      // Use source from result if available, otherwise fall back to selectedSource
      const source = s.source || this.selectedSource || state.sources.current;
      // Combine context (scope) with the result path - needed when searching within a folder
      // Ensure exactly one slash between context and path
      const context = url.removeTrailingSlash(this.getContext);
      const path = url.removeLeadingSlash(url.removeTrailingSlash(s.path));
      const fullPath = `${context}/${path}`;
      return url.buildItemUrl(source, fullPath, true);
    },
    humanSize(size) {
      return getHumanReadableFilesize(size);
    },
    basePath(str, isDir) {
      let result = url.removeLastDir(str);
      if (!isDir) {
        result = url.removeLeadingSlash(result); // fix weird rtl thing
      }
      return `${result}/`;
    },
    baseName(str) {
      const parts = url.removeTrailingSlash(str).split("/");
      const part = parts.pop();
      return part;
    },
    /** Clear query state whenever the overlay opens (open paths do not always call close()). */
    resetSearchOnOpen() {
      this.value = "";
      this.results = [];
      this.ongoing = 0;
      this.noneMessage = this.$t("search.typeToSearch", {
        minSearchLength: globalVars.minSearchLength,
      });
    },
    close(event) {
      this.value = "";
      event.stopPropagation();
      mutations.setSearch(false);
    },
    keyup(event) {
      if (event.keyCode === 27) {
        this.close(event);
        return;
      }
    },
    resetSearchFilters() {
      this.entryTypeFilter = "all";
      this.selectedMediaTypes = [];
      this.advancedOptionsExpanded = false;
      this.showPreviewImages = false;
    },
    async submit(event) {
      this.results = [];
      if (event !== undefined) {
        event.preventDefault();
      }
      if (this.value === "" || this.value.length < globalVars.minSearchLength) {
        this.noneMessage = this.$t("search.notEnoughCharacters", { minSearchLength: globalVars.minSearchLength });
        return;
      }
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
      let searchQuery = searchTypesFull + this.value;
      if (this.caseExactSearch) {
        searchQuery =
          searchTypesFull.trim() === ""
            ? `case:exact ${this.value}`
            : `case:exact ${searchTypesFull}${this.value}`;
      }
      this.ongoing++;
      
      // Determine which sources to search
      let sourcesToSearch;
      if (this.selectedSource === "__all__" || this.selectedSource === "") {
        // Search all sources
        sourcesToSearch = Object.keys(this.sourceInfo);
      } else {
        // Search single source
        sourcesToSearch = [this.selectedSource || state.sources.current];
      }
      
      // Only pass scope if searching a single source
      const scope = sourcesToSearch.length === 1 ? this.getContext : null;
      
      this.results = await toolsApi.search(
        scope,
        sourcesToSearch,
        searchQuery,
        false,
        dateParams
      );

      this.ongoing--;
      if (this.results.length === 0 && this.ongoing === 0) {
        this.noneMessage = this.$t("search.noResults");
      }
    },
    showHelpTooltip(event) {
      const helpText = [
        this.$t("search.helpText1", { minSearchLength: globalVars.minSearchLength }),
        this.$t("search.helpText2"),
      ].join("\n\n");
      mutations.showTooltip({
        content: helpText,
        x: event.clientX,
        y: event.clientY,
      });
    },
    hideTooltip() {
      mutations.hideTooltip();
    },
    clearContext() {
      mutations.closeHovers();
    },
    getThumbnailUrl(s) {
      if (!s.hasPreview) {
        return "";
      }
      try {
        // Use source from result if available, otherwise fall back to selectedSource
        const source = s.source || this.selectedSource || state.sources.current;
        // Combine context (scope) with the result path - needed when searching within a folder
        // Ensure exactly one slash between context and path
        const context = url.removeTrailingSlash(this.getContext);
        const path = url.removeLeadingSlash(url.removeTrailingSlash(s.path));
        const fullPath = `${context}/${path}`;
        const modified = s.modified || "";
        return resourcesApi.getPreviewURL(source, fullPath, modified);
      } catch (_err) {
        return "";
      }
    },
    open() {
      if (state.isSearchActive) return;
      if (getters.currentPromptName()) return;

      mutations.closeHovers();
      mutations.closeSidebar();
      mutations.resetSelected();
      mutations.setSearch(true);

      this.$nextTick(() => {
        const input = this.$refs.input;
        if (input) {
          input.focus();
        }
        const resultList = document.getElementById("result-list");
        if (resultList) {
          resultList.classList.add("active");
        }
      });
    },
    openInAdvancedSearch() {
      let sourcesToSearch;
      if (this.selectedSource === "__all__" || this.selectedSource === "") {
        sourcesToSearch = Object.keys(this.sourceInfo || {});
      } else {
        sourcesToSearch = [this.selectedSource || state.sources.current];
      }

      const picked = [...sourcesToSearch].sort();

      /** @type {Record<string, string | string[]>} */
      const query = {};
      query.term = this.value.trim();

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
      if (this.foldersOnly) {
        query.typeLock = "1";
      }

      /** @type {string[]} */
      const scopeClauses = [];
      for (const sourceName of picked) {
        let scopedFolderPath = "/";
        if (picked.length === 1) {
          const raw = String(this.getContext || "").trim();
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
        query.scope = scopeClauses[0];
      } else {
        query.scope = scopeClauses;
      }

      mutations.setSearch(false);
      router
        .push({ path: "/tools/advancedSearch", query })
        .catch(() => {});
    },
    addSelected(event, s) {
      event.preventDefault();
      const pathParts = url.removeTrailingSlash(s.path).split("/");
      // Use source from result if available, otherwise fall back to selectedSource
      const source = s.source || this.selectedSource || state.sources.current;
      // Combine context (scope) with the result path - ensure exactly one slash between
      const context = url.removeTrailingSlash(this.getContext);
      const pathStr = url.removeLeadingSlash(url.removeTrailingSlash(s.path));
      const path = `${context}/${pathStr}`;
      const modifiedItem = {
        name: pathParts.pop(),
        path: path,
        size: s.size,
        type: s.type,
        source: source,
      };
      mutations.resetSelected();
      mutations.addSelected(modifiedItem);
      this.openContext(event);
    },
  },
};
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
  word-wrap: break-word;
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
  border-radius: 0em !important;
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

#results>#result-list {
  max-height: 80vh;
  width: 35em;
  overflow: scroll;
  padding-bottom: 1em;
  -webkit-transition: width 0.3s ease 0s;
  transition: width 0.3s ease 0s;
  background-color: unset;
}

#results {
  -webkit-animation: SlideDown 0.5s forwards;
  animation: SlideDown 0.5s forwards;
  border-radius: 1em;
  border-top: none;
  border-top-left-radius: 0px;
  border-top-right-radius: 0px;
  border: 2px solid var(--surfaceSecondary);
  box-shadow: 0px 2em 50px 10px rgba(0, 0, 0, 0.3);
  background-color: lightgray;
  max-height: 80vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

#search.active #results ul li a {
  display: flex;
  align-items: center;
}

#search #result-list.active {
  width: 1000px;
}

/* Animations */
@keyframes SlideDown {
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
  -webkit-transform: translateX(-50%);
  transform: translateX(-50%);
}

.search-input-container {
  background-color: rgba(100, 100, 100, 0.2);
  display: flex;
  padding: 0.5em 0.75em;
  border-radius: 1em;
  border-style: unset;
  align-items: center;
  height: 3em;
  gap: 0.5em;
  width: 100%;
}

.search-input-container .material-symbols {
  font-size: 1.25em;
  color: rgba(255, 255, 255, 0.7);
}

#search.active .search-input-container .material-symbols {
  color: inherit;
}

#search .search-input-container input {
  border: 0;
  background-color: transparent;
  padding: 0;
  color: rgba(255, 255, 255, 0.9);
  font-size: 0.95em;
}

#search-input {
  width: 100%;
  padding-left: 0.5em;
}

#search.active .search-input-container input {
  color: inherit;
}

#search .search-input-container input::placeholder {
  color: rgba(255, 255, 255, 0.5);
}

#search.active .search-input-container input::placeholder {
  color: rgba(0, 0, 0, 0.5);
}

#search.dark-mode .search-input-container {
  background-color: rgba(255, 255, 255, 0.1);
}

#search.dark-mode .search-input-container input::placeholder {
  color: gray !important;
}

#search.dark-mode.active .search-input-container {
  background-color: var(--background);
}

#search.active .search-input-container {
  background-color: var(--background);
  border-color: var(--surfaceSecondary);
  border-style: solid;
  border-bottom-style: none;
  border-bottom-right-radius: 0 !important;
  border-bottom-left-radius: 0 !important;
  border-width: 2px;
}

#result-list p {
  margin: 1em;
}

/* Hiding scrollbar for Chrome, Safari and Opera */
#result-list::-webkit-scrollbar {
  display: none;
}

/* Hiding scrollbar for IE, Edge and Firefox */
#result-list {
  scrollbar-width: none;
  -ms-overflow-style: none;
  max-width: 95vw;
}

.search-entry:hover {
  background-color: var(--alt-background);
}

.search-entry.active {
  background-color: var(--surfacePrimary);
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
  background: white;
  display: flex;
  top: -4em;
  flex-direction: column;
  align-items: center;
  text-align: left;
  color: rgba(0, 0, 0, 0.6);
  height: 0;
  transition: 2s ease height, 2s ease padding, 2s ease width, 2s ease padding;
  z-index: 3;
}

body.rtl #search #result {
  direction: ltr;
}

#search #result>div>*:first-child {
  margin-top: 0;
}

body.rtl #search #result {
  direction: rtl;
  text-align: right;
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
  margin: 0.5em;
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

#search .search-input-container input::placeholder {
  color: rgba(255, 255, 255, 0.5);
}

#search.active .search-input-container input::placeholder {
  color: rgba(0, 0, 0, 0.5);
}

#search.dark-mode .search-input-container {
  background-color: rgba(255, 255, 255, 0.1);
}

#search.dark-mode.active .search-input-container {
  background-color: var(--background);
}

/* Search Boxes */
#search .boxes {
  margin: 1em;
  text-align: center;
}

#search .boxes h3 {
  margin: 0;
  font-weight: 500;
  font-size: 1em;
  color: #212121;
  padding: 0.5em;
}

body.rtl #search .boxes h3 {
  text-align: right;
}

#search .boxes p {
  margin: 1em 0 0;
}

#search .boxes i {
  color: #fff !important;
  font-size: 3.5em;
}

.mobile-boxes {
  cursor: pointer;
  overflow: hidden;
  margin-bottom: 1em;
  background: var(--primaryColor);
  color: white;
  padding: 1em;
  border-radius: 1em;
  text-align: center;
}

/* Hiding scrollbar for Chrome, Safari and Opera */
.mobile-boxes::-webkit-scrollbar {
  display: none;
}

/* Hiding scrollbar for IE, Edge and Firefox */
.mobile-boxes {
  scrollbar-width: none;
  /* Firefox */
  -ms-overflow-style: none;
  /* IE and Edge */
}

.constraints {
  display: flex;
  flex-wrap: wrap;
  flex-direction: row;
  align-content: center;
  margin: 1em;
  justify-content: center;
}

.searchPrompt {
  display: flex;
  flex-direction: row;
  align-content: center;
  justify-content: center;
  align-items: center;
  gap: 0.5em;
}

.searchPrompt .tooltip-info-icon {
  font-size: 1.5rem;
  cursor: pointer;
  color: var(--primaryColor);
}

.filesize {
  background: var(--alt-background);
  border-radius: 1em;
  padding: 0.25em;
  padding-left: 0.5em;
  padding-right: 0.5em;
  min-width: fit-content;
}

.source-badge {
  background: var(--primaryColor);
  color: white;
  border-radius: 1em;
  padding: 0.25em 0.5em;
  font-size: 0.85em;
  font-weight: 500;
  min-width: fit-content;
}

.search-options-settings-item {
  padding: 0.5em;
}

.search-options-inner {
  box-sizing: border-box;
  width: 100%;
  overflow: hidden;
}

.search-filter-dropdowns {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5em;
  align-items: flex-start;
  justify-content: center;
  margin-bottom: 0.5em;
}

.search-filter-dropdowns :deep(.expand-dropdown) {
  width: auto;
  flex: 0 1 auto;
  min-width: 15em;
  max-width: 15em;
}

.search-filter-dropdowns :deep(.expand-dropdown-anchor.menu-panel) {
  min-width: 0;
}

@media (max-width: 768px) {
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
    transition: 1s ease all;
  }

  #search.active .search-input-container {
    box-shadow: 0 0 10px rgba(0, 0, 0, 0.1);
    backdrop-filter: blur(6px);
    height: 4em;
  }

  #search.active>div {
    border-radius: 0 !important;
  }

  #search.active #result {
    height: 100vh;
    padding-top: 0;
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

  .search-input-container>.action,
  .search-input-container>i {
    margin-right: 0.3em;
    user-select: none;
  }

  #result-list {
    width: 100vw !important;
    max-width: 100vw !important;
    left: 0;
    top: 4em;
    -webkit-box-direction: normal;
    -ms-flex-direction: column;
    overflow: scroll;
    display: flex;
    flex-direction: column;
  }

}
</style>
