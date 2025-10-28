<template>
  <div
    id="search"
    :class="{ active, ongoing, 'dark-mode': isDarkMode }"
    @click="clearContext"
  >
    <!-- Search input section -->
    <div id="search-input" @click="open" :class="{'halloween-eyes': eventTheme === 'halloween'}">
      <div id="halloween-eyes" v-if="eventTheme=== 'halloween' && active">
        <div class="eye left">
          <div class="pupil"></div>
        </div>
        <div class="eye right">
          <div class="pupil"></div>
        </div>
      </div>
      <!-- Close button visible when search is active -->
      <button
        v-if="active"
        class="action"
        @click="close"
        :aria-label="$t('buttons.close')"
        :title="$t('buttons.close')"
      >
        <i class="material-icons">close</i>
      </button>
      <!-- Search icon when search is not active -->
      <i v-else class="material-icons">search</i>
      <!-- Input field for search -->
      <input
        id="main-input"
        class="main-input"
        :class="{ 'halloween-theme': eventTheme === 'halloween' }"
        type="text"
        @keyup.exact="keyup"
        @input="submit"
        ref="input"
        :autofocus="active"
        v-model.trim="value"
        :aria-label="$t('search.search')"
        :placeholder="$t('search.search')"
      />
    </div>

    <!-- Search results for desktop -->
    <div v-show="active"  id="results" class="fb-shadow" ref="result">
      <div class="inputWrapper" style="display: flex">
        <select
          v-if="multipleSources"
          class="searchContext button input"
          aria-label="search-path"
          v-model="selectedSource"
          :value="selectedSource"
          @change="updateSource"
        >
          <option v-for="(info, name) in sourceInfo" :key="info" :value="name">
            {{ name }}
          </option>
        </select>

        <!-- Formatted display of selected value -->
        <div class="searchContext">{{ $t("search.searchContext", { context: getContext }) }}</div>
      </div>

      <div id="result-list">
        <div v-if="!disableSearchOptions">
          <div v-if="active">
            <div v-if="isMobile">
              <ButtonGroup
                :buttons="toggleOptionButton"
                @button-clicked="enableOptions"
                @remove-button-clicked="disableOptions"
              />
            </div>
            <div v-show="showOptions">
              <!-- Button groups for filtering search results -->
              <ButtonGroup
                :buttons="folderSelect"
                @button-clicked="addToTypes"
                @remove-button-clicked="removeFromTypes"
                @disableAll="folderSelectClicked()"
                @enableAll="resetButtonGroups()"
              />
              <ButtonGroup
                :buttons="typeSelect"
                @button-clicked="addToTypes"
                @remove-button-clicked="removeFromTypes"
                :isDisabled="isTypeSelectDisabled"
              />
              <!-- Inputs for filtering by file size -->
              <div class="sizeConstraints">
                <div class="sizeInputWrapper">
                  <p>{{ $t("search.smallerThan") }}</p>
                  <input
                    class="sizeInput"
                    v-model="smallerThan"
                    type="number"
                    min="0"
                    :placeholder="$t('search.number')"
                  /><p>MB</p> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
                </div>
                <div class="sizeInputWrapper">
                  <p>{{ $t("search.largerThan") }}</p>
                  <input
                    class="sizeInput"
                    v-model="largerThan"
                    type="number"
                    :placeholder="$t('search.number')"
                  /><p>MB</p> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
                </div>
              </div>
            </div>
          </div>
        </div>
        <!-- Loading icon when search is ongoing -->
        <p v-show="isEmpty && isRunning" id="renew">
          <i class="material-icons spin">autorenew</i>
        </p>
        <!-- Message when no results are found -->
        <div class="searchPrompt" v-show="isEmpty && !isRunning">
          <p>{{ noneMessage }}</p>
          <div class="helpButton" @click="toggleHelp()">{{ $t("sidebar.help") }}</div>
        </div>
        <!-- Help text section -->
        <div class="helpText" v-if="showHelp">
          <p>{{ $t("search.helpText1") }}</p>
          <p>{{ $t("search.helpText2") }}</p>
        </div>
        <!-- List of search results -->
        <ul v-show="results.length > 0">
          <li
            v-for="(s, k) in results"
            :key="k"
            class="search-entry clickable"
            :class="{ active: activeStates[k] }"
            :aria-label="baseName(s.path)"
          >
            <a :href="getRelative(s.path)" @contextmenu="addSelected(event, s)">
              <Icon :mimetype="s.type" :filename="s.name" />
              <span class="text-container">
                {{ basePath(s.path, s.type == "directory") }}{{ baseName(s.path) }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
              </span>
              <div class="filesize">{{ humanSize(s.size) }}</div>
            </a>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>

<script>
import ButtonGroup from "./ButtonGroup.vue";
import { search } from "@/api";
import { getters, mutations, state } from "@/store";
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { url } from "@/utils/";
import Icon from "@/components/files/Icon.vue";
import { globalVars, serverHasMultipleSources } from "@/utils/constants";

var boxes = {
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
    ButtonGroup,
    Icon,
  },
  name: "search",
  data: function () {
    return {
      largerThan: "",
      smallerThan: "",
      noneMessage: this.$t("search.typeToSearch", { minSearchLength: globalVars.minSearchLength }),
      searchTypes: "",
      isTypeSelectDisabled: false,
      showHelp: false,
      folderSelect: [
        { label: this.$t("search.onlyFolders"), value: "type:folder" },
        { label: this.$t("search.onlyFiles"), value: "type:file" },
      ],
      typeSelect: [
        { label: this.$t("search.photos"), value: "type:image" },
        { label: this.$t("search.audio"), value: "type:audio" },
        { label: this.$t("search.videos"), value: "type:video" },
        { label: this.$t("search.documents"), value: "type:doc" },
        { label: this.$t("search.archives"), value: "type:archive" },
      ],
      toggleOptionButton: [{ label: this.$t("search.showOptions") }],
      value: "",
      ongoing: false,
      results: [],
      reload: false,
      scrollable: null,
      hiddenOptions: true,
      selectedSource: "",
    };
  },
  watch: {
    largerThan() {
      this.submit();
    },
    smallerThan() {
      this.submit();
    },
    searchTypes() {
      this.submit();
    },
    active(active) {
      // this is hear to allow for animation
      const resultList = document.getElementById("result-list");
      if (!active) {
        resultList.classList.remove("active");
        this.value = "";
        event.stopPropagation();
        mutations.closeHovers();
        return;
      }
      if (state.serverHasMultipleSources) {
        this.selectedSource = state.sources.current;
      }
      setTimeout(() => {
        resultList.classList.add("active");
        document.getElementById("main-input").focus();
      }, 100);
    },
    value() {
      if (this.results.length) {
        this.ongoing = false;
        this.results = [];
      }
    },
  },
  mounted() {
    if (state.serverHasMultipleSources) {
      this.selectedSource = state.sources.current;
    }
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
      if (event.key === '/' && !state.isSearchActive) {
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
    eventTheme() {
      return getters.eventTheme();
    },
    disableSearchOptions() {
      return state.user.disableSearchOptions;
    },
    showOptions() {
      return !this.hiddenOptions || !this.isMobile;
    },
    isMobile() {
      return state.isMobile;
    },
    foldersOnly() {
      return this.isTypeSelectDisabled;
    },
    active() {
      return state.isSearchActive;
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    showBoxes() {
      return this.searchTypes == "";
    },
    boxes() {
      return boxes;
    },
    isEmpty() {
      return this.results.length === 0;
    },
    text() {
      if (this.ongoing) {
        return "";
      }
      return this.$t("search.typeToSearch", { minSearchLength: globalVars.minSearchLength })
    },
    isRunning() {
      return this.ongoing;
    },
    searchHelp() {
      return this.showHelp;
    },
    activeStates() {
      const selectedItems = state.selected ? [].concat(state.selected) : [];

      if (selectedItems.length === 0) {
        // Return an array of all false if nothing is selected
        return new Array(this.results.length).fill(false);
      }

      const selectedPaths = new Set(selectedItems.map((item) => item.path));
      return this.results.map((result) => selectedPaths.has(result.path));
    },
    sourceInfo() {
      return state.sources.info;
    },
    multipleSources() {
      return Object.keys(state.sources.info).length > 1;
    },
    getContext() {
      let result = url.extractSourceFromPath(decodeURIComponent(state.route.path));
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
      mutations.showHover({
        name: "ContextMenu",
        props: {
          posX: event.clientX,
          posY: event.clientY,
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
    updateSource(event) {
      this.selectedSource = event.target.value;
      this.submit();
    },
    getRelative(path) {
      // double encode # to fix issue with # in path
      // replace all # with %23
      path = path.replaceAll('#', "%23");
      if (path.startsWith("/")) {
        path = path.slice(1); // remove leading slash
      }
      const context = url.removeTrailingSlash(this.getContext)
      const encodedPath = encodeURIComponent(context + "/" + path).replaceAll("%2F", "/");
      let fullpath = encodedPath;
      if (serverHasMultipleSources) {
        fullpath = globalVars.baseURL+"files/" + this.selectedSource + encodedPath;
      } else {
        fullpath = globalVars.baseURL+"files" + encodedPath;
      }
      return fullpath;
    },
    getIcon(mimetype) {
      return getMaterialIconForType(mimetype);
    },
    enableOptions() {
      this.hiddenOptions = false;
      this.toggleOptionButton = [{ label: "Hide Options" }];
    },
    disableOptions() {
      this.hiddenOptions = true;
      this.toggleOptionButton = [{ label: "Show Options" }];
    },
    humanSize(size) {
      return getHumanReadableFilesize(size);
    },
    basePath(str, isDir) {
      let result = url.removeLastDir(str);
      if (!isDir) {
        result = url.removeLeadingSlash(result); // fix weird rtl thing
      }
      return result + "/";
    },
    baseName(str) {
      let parts = url.removeTrailingSlash(str).split("/");
      let part = parts.pop();
      return part;
    },
    open() {
      if (!state.isSearchActive) {
        mutations.closeHovers();
        mutations.closeSidebar();
        mutations.resetSelected();
        this.resetSearchFilters();
        mutations.setSearch(true);
      }
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
      this.results.length === 0;
    },
    addToTypes(string) {
      if (this.searchTypes.includes(string)) {
        return true;
      }
      if (string == null || string == "") {
        return false;
      }
      this.searchTypes = this.searchTypes + string + " ";
    },
    resetSearchFilters() {
      this.searchTypes = "";
      this.hiddenOptions = true;
    },
    removeFromTypes(string) {
      if (string == null || string == "") {
        return false;
      }
      this.searchTypes = this.searchTypes.replaceAll(string + " ", "");
      if (state.isMobile) {
        this.$refs.input.focus();
      }
    },
    folderSelectClicked() {
      this.isTypeSelectDisabled = true; // Disable the other ButtonGroup
    },
    resetButtonGroups() {
      this.isTypeSelectDisabled = false;
    },
    async submit(event) {
      this.results = [];

      this.showHelp = false;
      if (event != undefined) {
        event.preventDefault();
      }
      if (this.value === "" || this.value.length < globalVars.minSearchLength) {
        this.ongoing = false;        this.noneMessage = this.$t("search.notEnoughCharacters", { minSearchLength: globalVars.minSearchLength });
        return;
      }
      let searchTypesFull = this.searchTypes;
      if (this.largerThan != "") {
        searchTypesFull = searchTypesFull + "type:largerThan=" + this.largerThan + " ";
      }
      if (this.smallerThan != "") {
        searchTypesFull = searchTypesFull + "type:smallerThan=" + this.smallerThan + " ";
      }
      this.ongoing = true;
      let source = this.selectedSource;
      if (source == "") {
        this.selectedSource = state.sources.current;
      }
      this.results = await search(this.getContext, this.selectedSource, searchTypesFull + this.value);

      this.ongoing = false;
      if (this.results.length == 0) {
        this.noneMessage = this.$t("search.noResults");
      }
    },
    toggleHelp() {
      this.showHelp = !this.showHelp;
    },
    clearContext() {
      mutations.closeHovers();
    },
    addSelected(event, s) {
      const pathParts = url.removeTrailingSlash(s.path).split("/");
      let path = this.getContext + url.removeTrailingSlash(s.path);
      const modifiedItem = {
        name: pathParts.pop(),
        path: path,
        size: s.size,
        type: s.type,
        source: this.selectedSource || state.sources.current,
      };
      mutations.resetSelected();
      mutations.addSelected(modifiedItem);
    },
  },
};
</script>

<style scoped>
.sizeInputWrapper {
  border: 1px solid #ccc;
}
.main-input {
  width: 100%;
}

.searchContext {
  width: 100%;
  padding: 0.5em 1em;
  background: var(--primaryColor);
  color: white;
  border-left: 1px solid gray;
  border-right: 1px solid gray;
  word-wrap: break-word;
}

.searchContext.input {
  background-color: var(--primaryColor) !important;
  border-radius: 0em !important;
  color: white;
  border: unset;
  width: 25%;
  min-width: 7em;
  max-width: 15em;
}

.searchContext.input option {
  background: grey;
  color: white;
}

.searchContext.input option:hover {
  background: var(--primaryColor);
  color: white;
}

#results > #result-list {
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
  border-radius: 1m;
  border-width: 1px;
  border-style: solid;
  border-radius: 1em;
  max-height: 100%;
  border-top: none;
  border-top-width: initial;
  border-top-style: none;
  border-top-color: initial;
  border-top-left-radius: 0px;
  border-top-right-radius: 0px;
  -webkit-transform: translateX(-50%);
  transform: translateX(-50%);
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
  max-width: 95vw;
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

#search-input {
  background-color: rgba(100, 100, 100, 0.2);
  display: flex;
  height: 100%;
  padding: 0em 0.75em;
  border-style: solid;
  border-radius: 1em;
  border-style: unset;
  border-width: 1px;
  align-items: center;
  height: 3em;
}

#search input {
  border: 0;
  background-color: transparent;
  padding: 0;
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
  /* Firefox */
  -ms-overflow-style: none;
  /* IE and Edge */
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
  transition: 2s ease height, 2s ease padding;
  transition: 2s ease width, 2s ease padding;
  z-index: 3;
}

body.rtl #search #result {
  direction: ltr;
}

#search #result > div > *:first-child {
  margin-top: 0;
}

body.rtl #search #result {
  direction: rtl;
  text-align: right;
}

/* Search Results */
body.rtl #search #result ul > * {
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

#search.active #search-input {
  background-color: var(--background);
  border-color: black;
  border-style: solid;
  border-bottom-style: none;
  border-bottom-right-radius: 0 !important;
  border-bottom-left-radius: 0 !important;
}

/* Search Input Placeholder */
#search::-webkit-input-placeholder {
  color: rgba(255, 255, 255, 0.5);
}

#search:-moz-placeholder {
  opacity: 1;
  color: rgba(255, 255, 255, 0.5);
}

#search::-moz-placeholder {
  opacity: 1;
  color: rgba(255, 255, 255, 0.5);
}

#search:-ms-input-placeholder {
  color: rgba(255, 255, 255, 0.5);
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

.helpText {
  padding: 1em;
}

.sizeConstraints {
  display: flex;
  flex-wrap: wrap;
  flex-direction: row;
  align-content: center;
  margin: 1em;
  justify-content: center;
}

.helpButton {
  position: absolute;
  right: 10px;
  cursor: pointer;
  text-align: center;
  background: rgb(211, 211, 211);
  padding: 0.25em;
  border-radius: 0.25em;
}

.searchPrompt {
  display: flex;
  flex-direction: column;
  align-content: center;
  justify-content: center;
  align-items: center;
}

.filesize {
  background: var(--alt-background);
  border-radius: 1em;
  padding: 0.25em;
  padding-left: 0.5em;
  padding-right: 0.5em;
  min-width: fit-content;
}

@media (max-width: 800px) {
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

  #search-input {
    transition: 1s ease all;
  }

  #search.active #search-input {
    border-bottom: 3px solid rgba(0, 0, 0, 0.075);
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

  #search-input>.action,
  #search-input>i {
    margin-right: 0.3em;
    user-select: none;
  }

  #result-list {
    width:100vw !important;
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
