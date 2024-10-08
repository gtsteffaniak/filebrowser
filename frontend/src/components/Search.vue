<template>
  <div id="search" :class="{ active, ongoing, 'dark-mode': isDarkMode }">
    <!-- Search input section -->
    <div id="input" @click="open">
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

    <!-- Search results for mobile -->
    <div v-if="isMobile && active" id="result" :class="{ hidden: !active }" ref="result">
      <div id="result-list">
        <div class="button" style="width: 100%">Search Context: {{ getContext }}</div>
        <!-- List of search results -->
        <ul v-show="results.length > 0">
          <li v-for="(s, k) in results" :key="k" style="cursor: pointer">
            <router-link :to="s.url">
              <i v-if="s.dir" class="material-icons folder-icons"> folder </i>
              <i v-else-if="s.audio" class="material-icons audio-icons"> volume_up </i>
              <i v-else-if="s.image" class="material-icons image-icons"> photo </i>
              <i v-else-if="s.video" class="material-icons video-icons"> movie </i>
              <i v-else-if="s.archive" class="material-icons archive-icons"> archive </i>
              <i v-else class="material-icons file-icons"> insert_drive_file </i>
              <span class="text-container">
                {{ basePath(s.path, s.dir) }}<b>{{ baseName(s.path) }}</b>
              </span>
            </router-link>
          </li>
        </ul>
        <!-- Loading icon when search is ongoing -->
        <p v-show="isEmpty && isRunning" id="renew">
          <i class="material-icons spin">autorenew</i>
        </p>
        <!-- Message when no results are found -->
        <div v-show="isEmpty && !isRunning">
          <div class="searchPrompt" v-show="isEmpty && !isRunning">
            <p>{{ noneMessage }}</p>
          </div>
        </div>
        <div v-if="isEmpty">
          <!-- Reset filters button -->
          <button
            class="mobile-boxes"
            v-if="value.length === 0 && !showBoxes"
            @click="resetSearchFilters()"
          >
            Reset filters
          </button>
          <!-- Box types when no search input is present -->
          <div v-if="value.length === 0 && showBoxes">
            <div class="boxes">
              <h3>{{ $t("search.types") }}</h3>
              <div>
                <div
                  class="mobile-boxes"
                  tabindex="0"
                  v-for="(v, k) in boxes"
                  :key="k"
                  role="button"
                  @click="addToTypes('type:' + k)"
                  :aria-label="v.label"
                >
                  <i class="material-icons">{{ v.icon }}</i>
                  <p>{{ v.label }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Search results for desktop -->
    <div v-show="!isMobile && active" id="result-desktop" ref="result">
      <div class="searchContext">Search Context: {{ getContext }}</div>
      <div id="result-list">
        <div>
          <div>
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
            <div v-if="!foldersOnly" class="sizeConstraints">
              <div class="sizeInputWrapper">
                <p>Smaller Than:</p>
                <input
                  class="sizeInput"
                  v-model="smallerThan"
                  type="number"
                  min="0"
                  placeholder="number"
                />
                <p>MB</p>
              </div>
              <div class="sizeInputWrapper">
                <p>Larger Than:</p>
                <input
                  class="sizeInput"
                  v-model="largerThan"
                  type="number"
                  placeholder="number"
                />
                <p>MB</p>
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
          <div class="helpButton" @click="toggleHelp()">Help</div>
        </div>
        <!-- Help text section -->
        <div class="helpText" v-if="showHelp">
          <p>
            Search occurs on each character you type (3 character minimum for search
            terms).
          </p>
          <p>
            <b>The index:</b> Search utilizes the index which automatically gets updated
            on the configured interval (default: 5 minutes). Searching when the program
            has just started may result in incomplete results.
          </p>
          <p>
            <b>Filter by type:</b> You can have multiple type filters by adding
            <code>type:condition</code> followed by search terms.
          </p>
          <p>
            <b>Multiple Search terms:</b> Additional terms separated by <code>|</code>,
            for example <code>"test|not"</code> searches for both terms independently.
          </p>
        </div>
        <!-- List of search results -->
        <ul v-show="results.length > 0">
          <li v-for="(s, k) in results" :key="k" style="cursor: pointer">
            <router-link :to="s.url">
              <i v-if="s.dir" class="material-icons folder-icons"> folder </i>
              <i v-else-if="s.audio" class="material-icons audio-icons"> volume_up </i>
              <i v-else-if="s.image" class="material-icons image-icons"> photo </i>
              <i v-else-if="s.video" class="material-icons video-icons"> movie </i>
              <i v-else-if="s.archive" class="material-icons archive-icons"> archive </i>
              <i v-else class="material-icons file-icons"> insert_drive_file </i>
              <span class="text-container">
                {{ basePath(s.path, s.dir) }}<b>{{ baseName(s.path) }}</b>
              </span>
            </router-link>
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
  },
  name: "search",
  data: function () {
    return {
      largerThan: "",
      smallerThan: "",
      noneMessage: "Start typing 3 or more characters to begin searching.",
      searchTypes: "",
      isTypeSelectDisabled: false,
      showHelp: false,
      folderSelect: [
        { label: "Only Folders", value: "type:folder" },
        { label: "Only Files", value: "type:file" },
      ],
      typeSelect: [
        { label: "Photos", value: "type:image" },
        { label: "Audio", value: "type:audio" },
        { label: "Videos", value: "type:video" },
        { label: "Documents", value: "type:doc" },
        { label: "Archives", value: "type:archive" },
      ],
      value: "",
      ongoing: false,
      results: [],
      reload: false,
      scrollable: null,
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
  computed: {
    isMobile() {
      return state.isMobile;
    },
    foldersOnly() {
      return this.isTypeSelectDisabled;
    },
    active() {
      return getters.currentPromptName() == "search";
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

      return this.value === ""
        ? this.$t("search.typeToSearch")
        : this.$t("search.pressToSearch");
    },
    isRunning() {
      return this.ongoing;
    },
    searchHelp() {
      return this.showHelp;
    },
    getContext() {
      let path = state.route.path;
      path = path.slice(1);
      path = "./" + path.substring(path.indexOf("/") + 1);
      path = path.replace(/\/+$/, "") + "/";
      if (path == "./files/") {
        path = "./";
      }
      return path;
    },
  },
  methods: {
    basePath(str, isDir) {
      let parts = str.replace(/(\/$|^\/)/, "").split("/");
      if (parts.length <= 1) {
        if (isDir) {
          return "/";
        }
        return "";
      }
      parts.pop();
      parts = parts.join("/") + "/";
      if (isDir) {
        parts = "/" + parts; // fix weird rtl thing
      }

      return parts;
    },
    baseName(str) {
      let parts = str.replace(/(\/$|^\/)/, "").split("/");
      return parts.pop();
    },
    open() {
      if (!this.active) {
        mutations.showHover("search");
      }
    },
    close(event) {
      this.value = "";
      event.stopPropagation();
      mutations.closeHovers();
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
    },
    removeFromTypes(string) {
      if (string == null || string == "") {
        return false;
      }
      this.searchTypes = this.searchTypes.replace(string + " ", "");
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
      this.showHelp = false;
      if (event != undefined) {
        event.preventDefault();
      }
      if (this.value === "" || this.value.length < 3) {
        this.ongoing = false;
        this.results = [];
        this.noneMessage = "Not enough characters to search (min 3)";
        return;
      }
      let searchTypesFull = this.searchTypes;
      if (this.largerThan != "" && !this.isTypeSelectDisabled) {
        searchTypesFull = searchTypesFull + "type:largerThan=" + this.largerThan + " ";
      }
      if (this.smallerThan != "" && !this.isTypeSelectDisabled) {
        searchTypesFull = searchTypesFull + "type:smallerThan=" + this.smallerThan + " ";
      }
      let path = state.route.path;
      this.ongoing = true;

      this.results = await search(path, searchTypesFull + this.value);

      this.ongoing = false;
      if (this.results.length == 0) {
        this.noneMessage = "No results found in indexed search.";
      }
    },
    toggleHelp() {
      this.showHelp = !this.showHelp;
    },
  },
};
</script>

<style>
.main-input {
  width: 100%;
}

.searchContext {
  width: 100%;
  padding: 0.5em 1em;
  background: var(--blue);
  color: white;
  border-left: 1px solid gray;
  border-right: 1px solid gray;
  word-wrap: break-word;
}

#result-desktop > #result-list {
  max-height: 80vh;
  width: 35em;
  overflow: scroll;
  padding-bottom: 1em;
  -webkit-transition: width 0.3s ease 0s;
  transition: width 0.3s ease 0s;
  background-color: unset;
}

#result-desktop {
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
  -webkit-box-shadow: 0px 2em 50px 10px rgba(0, 0, 0, 0.3);
  box-shadow: 0px 2em 50px 10px rgba(0, 0, 0, 0.3);
  background-color: lightgray;
  max-height: 80vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

#search.active #result-desktop ul li a {
  display: flex;
  align-items: center;
  padding: 0.3em 0;
  margin-right: 0.3em;
}

#search #result-list.active {
  width: 65em !important;
  max-width: 85vw !important;
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

#search #input {
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

.text-container {
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

#search.active #input {
  background-color: var(--background);
  border-color: black;
  border-style: solid;
  border-bottom-style: none;
  border-bottom-right-radius: 0;
  border-bottom-left-radius: 0;
}

input.sizeInput:disabled {
  cursor: not-allowed;
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
  background: var(--blue);
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
  flex-direction: row;
  flex-wrap: nowrap;
  align-content: center;
  margin: 1em;
  justify-content: center;
}

.sizeInput {
  height: 100%;
  text-align: center;
  width: 5em;
  border-radius: 1em;
  padding: 1em;
  backdrop-filter: invert(0.1);
  border: none !important;
}

.sizeInputWrapper {
  border-radius: 1em;
  margin-left: 0.5em;
  margin-right: 0.5em;
  display: -ms-flexbox;
  display: flex;
  background-color: rgb(245, 245, 245);
  padding: 0.25em;
  height: 3em;
  -webkit-box-align: center;
  -ms-flex-align: center;
  align-items: center;
  border: 1px solid #ccc;
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
</style>
