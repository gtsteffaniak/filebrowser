<template>
  <div
    id="search"
    @click="open"
    v-bind:class="{ active, ongoing, 'dark-mode': isDarkMode }"
  >
    <!-- Search input section -->
    <div id="input">
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
          <li
            v-for="(s, k) in results"
            :key="k"
            @click.stop.prevent="navigateTo(s.url)"
            style="cursor: pointer"
          >
            <router-link to="#" event="">
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
        <template v-if="isEmpty">
          <!-- Reset filters button -->
          <button
            class="mobile-boxes"
            v-if="value.length === 0 && !showBoxes"
            @click="resetSearchFilters()"
          >
            Reset filters
          </button>
          <!-- Box types when no search input is present -->
          <template v-if="value.length === 0 && showBoxes">
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
          </template>
        </template>
      </div>
    </div>

    <!-- Search results for desktop -->
    <div v-show="!isMobile && active" id="result-desktop" ref="result">
      <div class="searchContext">Search Context: {{ getContext }}</div>
      <div id="result-list">
        <template>
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
            <p>
              <b>File size:</b> Searching files by size may have significantly longer
              search times.
            </p>
          </div>
          <template>
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
          </template>
        </template>
        <!-- List of search results -->
        <ul v-show="results.length > 0">
          <li
            v-for="(s, k) in results"
            :key="k"
            @click.stop.prevent="navigateTo(s.url)"
            style="cursor: pointer"
          >
            <router-link to="#" event="">
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
import { darkMode } from "@/utils/constants";
import { state, getters, mutations } from "@/store";

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
      width: window.innerWidth,
      active: false,
      ongoing: false,
      results: [],
      reload: false,
      scrollable: null,
    };
  },
  computed: {
    showOverlay: function () {
      return state.prompts.length > 0 && state.prompts[0].name !== "more";
    },
    isDarkMode() {
      return state.user && Object.prototype.hasOwnProperty.call(state.user, "darkMode")
        ? state.user.darkMode
        : darkMode;
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
    isMobile() {
      return this.width <= 800;
    },
    isRunning() {
      return this.ongoing;
    },
    searchHelp() {
      return this.showHelp;
    },
    getContext() {
      let path = this.$route.path;
      path = path.slice(1);
      path = "./" + path.substring(path.indexOf("/") + 1);
      path = path.replace(/\/+$/, "") + "/";
      return path;
    },
  },
  methods: {
    handleResize() {
      this.width = window.innerWidth;
    },
    async navigateTo(url) {
      mutations.closeHovers();
      await this.$nextTick();
      setTimeout(() => this.$router.push(url), 0);
    },
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
      mutations.showHover("search");
    },
    close(event) {
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
      if (this.isMobile) {
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
      event.preventDefault();
      if (this.value === "" || this.value.length < 3) {
        this.ongoing = false;
        this.results = [];
        this.noneMessage = "Not enough characters to search (min 3)";
        return;
      }
      let searchTypesFull = this.searchTypes;
      if (this.largerThan != "") {
        searchTypesFull = searchTypesFull + "type:largerThan=" + this.largerThan + " ";
      }
      if (this.smallerThan != "") {
        searchTypesFull = searchTypesFull + "type:smallerThan=" + this.smallerThan + " ";
      }
      let path = this.$route.path;
      this.ongoing = true;
      try {
        this.results = await search(path, searchTypesFull + this.value);
      } catch (error) {
        this.$showError(error);
      }
      this.ongoing = false;
      if (this.results.length == 0) {
        this.noneMessage = "No results found in indexed search.";
      }
    },
    toggleHelp() {
      this.showHelp = !this.showHelp;
    },
  },
  mounted() {
    window.addEventListener("resize", this.handleResize);
  },
  beforeDestroy() {
    window.removeEventListener("resize", this.handleResize);
  },
};
</script>
