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
            <p>No results found in indexed search.</p>
          </div>
        </div>
        <template v-if="isEmpty">
          <template v-if="value.length === 0">
            <div class="boxes">
              <h3>{{ $t("search.types") }}</h3>
              <div>
                <div tabindex="0" v-for="(v, k) in boxes" :key="k" role="button" @click="init('type:' + k)"
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
        <template>
          <p v-show="isEmpty && isRunning" id="renew">
            <i class="material-icons spin">autorenew</i>
          </p>
          <div class="searchPrompt" v-show="isEmpty && !isRunning">
            <p>No results found in indexed search.</p>
            <div class="helpButton" @click="toggleHelp()">Toggle Search Help</div>
          </div>

          <div class="helpText" v-if="showHelp">
            Search additional terms separated by <code>|</code>, for example <code>"test|not"</code> searches for both terms independently
            <p>Note: searching files by size may have significantly longer search times since it cannot rely on the index alone.
               The search looks for only files that match all other conditions first, then checks the filesize and returns matching results.</p>
          </div>
          <template>
            <div class="boxes">
              <ButtonGroup :buttons="folderSelect" @button-clicked="init" @remove-button-clicked="removeInit" />
              <ButtonGroup :buttons="typeSelect" @button-clicked="init" @remove-button-clicked="removeInit" />
              <ButtonGroup :buttons="sizeSelect" @button-clicked="init" @remove-button-clicked="removeInit" />
            </div>
          </template>
        </template>
      </div>
    </div>
  </div>
</template>

<style>
.helpText{
  padding:1em
}
.helpButton {
  text-align: center;
  background: var(--background);
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
  "larger=100": { label: "larger than 100MB", icon: "arrow_forward_ios" },
  "smaller=100": { label: "smaller than 100MB ", icon: "arrow_back_ios" },
};

export default {
  components: {
    ButtonGroup,
  },
  name: "search",
  data: function () {
    return {
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
      sizeSelect: [
        { label: "Smaller than 100MB", value: "type:smaller=100" },
        { label: "Larger than 100MB", value: "type:larger=100" },
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
        this.reset();
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
        this.reset();
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
      let parts = str.replace(/\/$/, "").split("/");
      parts.pop();
      return parts.join("/") + "/";
    },
    baseName(str) {
      let parts = str.split("/");
      if (str.endsWith("/")) {
        return parts[parts.length - 2] + "/";
      } else {
        return parts[parts.length - 1];
      }
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
    init(string) {
      if (string == null || string == "") {
        return false
      }
      this.value = `${string} ${this.value}`;
      if (this.isMobile) {
        this.$refs.input.focus();
      }
    },
    removeInit(string) {
      if (string == null || string == "") {
        return false
      }
      this.value = this.value.replace(string + " ", "");
      if (this.isMobile) {
        this.$refs.input.focus();
      }
    },
    reset() {
      this.ongoing = false;
      this.resultsCount = 50;
      this.results = [];
    },
    async submit(event) {
      event.preventDefault();
      const words = this.value.split(" ").filter((word) => word.length < 3);
      if (this.value === "" || words.length > 0) {
        return;
      }
      let path = this.$route.path;
      this.ongoing = true;
      try {
        this.results = await search(path, this.value);
      } catch (error) {
        this.$showError(error);
      }
      this.ongoing = false;
    },
    toggleHelp(){
      this.showHelp = !this.showHelp
    }
  },
};
</script>