<template>
  <div id="search" @click="open" v-bind:class="{ active, ongoing }">
    <div id="input">
      <button v-if="active" class="action" @click="close" :aria-label="$t('buttons.close')" :title="$t('buttons.close')">
        <i class="material-icons">close</i>
      </button>
      <i v-else class="material-icons">search</i>
      <input type="text" @keyup.exact="keyup" @input="submit" ref="input" :autofocus="active" v-model.trim="value"
        :aria-label="$t('search.search')" :placeholder="$t('search.search')" />
    </div>
    <div v-if="isMobile && active" id="result" :class="{ hidden: !active }" ref="result">
      <div id="result-list">
        <div class="button" style="width: 100%">
          Search Context: {{ getContext(this.$route.path) }}
        </div>
        <ul v-show="results.length > 0">
          <li v-for="(s, k) in results" :key="k" @click.stop.prevent="navigateTo(s.url)" style="cursor: pointer">
            <router-link to="#" event="">
              <i v-if="s.dir" class="material-icons folder-icons"> folder </i>
              <i v-else-if="s.audio" class="material-icons audio-icons"> volume_up </i>
              <i v-else-if="s.image" class="material-icons image-icons"> photo </i>
              <i v-else-if="s.video" class="material-icons video-icons"> movie </i>
              <i v-else-if="s.archive" class="material-icons archive-icons"> archive </i>
              <i v-else class="material-icons file-icons"> insert_drive_file </i>
              <span class="text-container">
                {{ basePath(s.path) }}<b>{{ baseName(s.path) }}</b>
              </span>
            </router-link>
          </li>
        </ul>
        <p v-show="isEmpty && isRunning" id="renew">
          <i class="material-icons spin">autorenew</i>
        </p>
        <div v-show="isEmpty && !isRunning">
          <div class="searchPrompt" v-show="isEmpty && !isRunning">
            <p>{{ noneMessage }}</p>
          </div>
        </div>
        <template v-if="isEmpty">
          <template v-if="value.length === 0">
            <div class="boxes">
              <h3>{{ $t("search.types") }}</h3>
              <div>
                <div tabindex="0" v-for="(v, k) in boxes" :key="k" role="button" @click="addToTypes('type:' + k)"
                  :aria-label="v.label">
                  <i class="material-icons">{{ v.icon }}</i>
                  <p>{{ v.label }}</p>
                </div>
              </div>
            </div>
          </template>
        </template>
      </div>
    </div>
    <div v-if="!isMobile && active" id="result-desktop" ref="result">
      <div id="result-list">
        <div class="button fluid">
          Search Context: {{ getContext(this.$route.path) }}
        </div>
        <template>
          <p v-show="isEmpty && isRunning" id="renew">
            <i class="material-icons spin">autorenew</i>
          </p>
          <div class="searchPrompt" v-show="isEmpty && !isRunning">
            <p>{{noneMessage}}</p>
            <div class="helpButton" @click="toggleHelp()">Toggle Search Help</div>
          </div>
          <div class="helpText" v-if="showHelp">
            <p>Search occurs on each character you type (3 character minimum for search terms).</p>
            <p><b>The index:</b> Search utilizes the index which automatically gets updated on the configured interval
              (default: 5 minutes).
              Searching when the program has just started may result in incomplete results.</p>
            <p><b>Filter by type:</b> You can have multiple type filters by adding <code>type:condition</code> followed by
              search terms.</p>
            <p><b>Multiple Search terms:</b> Additional terms separated by <code>|</code>,
              for example <code>"test|not"</code> searches for both terms independently.</p>
            <p><b>File size:</b> Searching files by size may have significantly longer search times.</p>
          </div>
          <template>
              <ButtonGroup :buttons="folderSelect" @button-clicked="addToTypes" @remove-button-clicked="removeFromTypes"
                @disableAll="folderSelectClicked()" @enableAll="resetButtonGroups()" />
              <ButtonGroup :buttons="typeSelect" @button-clicked="addToTypes" @remove-button-clicked="removeFromTypes"
                :isDisabled="isTypeSelectDisabled" />
              <div class="sizeConstraints">
                <div class="input sizeInputWrapper">
                  <p>Smaller Than:</p>
                  <input class="sizeInput" v-model="smallerThan" type="text" placeholder="Enter number">
                  <p>MB</p>
                </div>
                <div class="input sizeInputWrapper">
                  <p>Larger Than:</p>
                  <input class="sizeInput" v-model="largerThan" type="text" placeholder="Enter number">
                  <p>MB</p>
                </div>
              </div>
          </template>
        </template>
        <ul v-show="results.length > 0">
          <li v-for="(s, k) in results" :key="k" @click.stop.prevent="navigateTo(s.url)" style="cursor: pointer">
            <router-link to="#" event="">
              <i v-if="s.dir" class="material-icons folder-icons"> folder </i>
              <i v-else-if="s.audio" class="material-icons audio-icons"> volume_up </i>
              <i v-else-if="s.image" class="material-icons image-icons"> photo </i>
              <i v-else-if="s.video" class="material-icons video-icons"> movie </i>
              <i v-else-if="s.archive" class="material-icons archive-icons"> archive </i>
              <i v-else class="material-icons file-icons"> insert_drive_file </i>
              <span class="text-container">
                {{ basePath(s.path) }}<b>{{ baseName(s.path) }}</b>
              </span>
            </router-link>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>

<style>

.helpText {
  padding: 1em
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
  text-align: center;
  width: 5em;
  border: solid !important;
  border-radius: 1em;
  padding: 1em;
}
.sizeInputWrapper {
  margin: auto;
  border-radius: 1em;
  display: flex;
  flex-wrap: nowrap;
  justify-content: space-evenly;
}

.helpButton {
  cursor: pointer;
  text-align: center;
  background: lightgray;
  background-color: lightgray;
  padding: .25em;
  border-radius: .25em;
}

.searchPrompt {
  display: flex;
  flex-direction: column;
  align-content: center;
  justify-content: center;
  align-items: center;
}
</style>

<script>
import ButtonGroup from "./ButtonGroup.vue";
import { mapState, mapGetters, mapMutations } from "vuex";
import { search } from "@/api";

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
      searchTypes: " ",
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
        { label: "Documents", value: "type:docs" },
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
  watch: {
    show(val, old) {
      this.active = val === "search";
      if (old === "search" && !this.active) {
        if (this.reload) {
          this.setReload(true);
        }

        document.body.style.overflow = "auto";
        this.ongoing = false;
        this.results = [];
        this.value = "";
        this.active = false;
        this.$refs.input.blur();
      } else if (this.active) {
        this.reload = false;
        this.$refs.input.focus();
        document.body.style.overflow = "hidden";
      }
    },
    value() {
      if (this.results.length) {
        this.ongoing = false;
        this.results = [];
      }
    },
  },
  computed: {
    ...mapState(["user", "show"]),
    ...mapGetters(["isListing"]),
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
      return this.showHelp
    },
  },
  mounted() {
    window.addEventListener("resize", this.handleResize);
    this.handleResize(); // Call this once to set the initial width
  },
  methods: {
    handleResize() {
      this.width = window.innerWidth;
    },
    async navigateTo(url) {
      this.closeHovers();
      await this.$nextTick();
      setTimeout(() => this.$router.push(url), 0);
    },
    getContext(url) {
      url = url.slice(1);
      let path = "./" + url.substring(url.indexOf("/") + 1);
      return path.replace(/\/+$/, "") + "/";
    },
    basePath(str) {
      if (!str.includes("/")) {
        return "";
      }
      let parts = str.replace(/(\/$|^\/)/, "").split("/"); //remove first and last slash
      parts.pop();
      parts = parts.join("/") + "/"
      if (str.endsWith("/")) {
        parts = "/" + parts // weird rtl bug
      }
      return parts;
    },
    baseName(str) {
      let parts = str.replace(/(\/$|^\/)/, "").split("/")
      return parts.pop()
    },
    ...mapMutations(["showHover", "closeHovers", "setReload"]),
    open() {
      this.showHover("search");
    },
    close(event) {
      event.stopPropagation();
      this.closeHovers();
    },
    keyup(event) {
      if (event.keyCode === 27) {
        this.close(event);
        return;
      }
      this.results.length === 0;
    },
    addToTypes(string){
      console.log(string)
      if (string == null || string == "") {
        return false
      }
      this.searchTypes = string+" "+this.searchTypes
      console.log(this.searchTypes)
    },
    removeFromTypes(string){
      console.log(string)
      if (string == null || string == "") {
        return false
      }
      this.searchTypes = this.searchTypes.replace(string + " ", "");
      if (this.isMobile) {
        this.$refs.input.focus();
      }
      console.log(this.searchTypes)
    },
    folderSelectClicked() {
      this.isTypeSelectDisabled = true;  // Disable the other ButtonGroup
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
        this.noneMessage = "Not enough characters to search (min 3)"
        return
      }
      let searchTypesFull = this.searchTypes
      if (this.largerThan != "" ){
        searchTypesFull = searchTypesFull+"type:largerThan="+this.largerThan+" "
      }
      if (this.smallerThan != "" ){
        searchTypesFull = searchTypesFull+"type:smallerThan="+this.smallerThan+" "
      }
      let path = this.$route.path;
      this.ongoing = true;
      try {
        this.results = await search(path, searchTypesFull + this.value);
      } catch (error) {
        this.$showError(error);
      }
      if (this.results.length == 0 && this.ongoing == false){
        this.noneMessage = "No results found in indexed search."
      }
      this.ongoing = false;
    },
    toggleHelp() {
      this.showHelp = !this.showHelp
    }
  },
};
</script>