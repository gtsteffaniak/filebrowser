<template>
  <div id="previewer" @mousemove="toggleNavigation" @touchstart="toggleNavigation">
    <div class="preview">
      <ExtendedImage v-if="getSimpleType(currentItem.type) == 'image'" :src="raw">
      </ExtendedImage>
      <audio v-else-if="getSimpleType(currentItem.type) == 'audio'" ref="player" :src="raw" controls
        :autoplay="autoPlay" @play="autoPlay = true"></audio>
      <video v-else-if="getSimpleType(currentItem.type) == 'video'" ref="player" :src="raw" controls
        :autoplay="autoPlay" @play="autoPlay = true">
        <track kind="captions" v-for="(sub, index) in subtitles" :key="index" :src="sub" :label="'Subtitle ' + index"
          :default="index === 0" />
        Sorry, your browser doesn't support embedded videos, but don't worry, you can
        <a :href="downloadUrl">download it</a>
        and watch it with your favorite video player!
      </video>
      <object v-else-if="getSimpleType(currentItem.type) == 'pdf'" class="pdf" :data="raw"></object>
      <div v-else class="info">
        <div class="title">
          <i class="material-icons">feedback</i>
          {{ $t("files.noPreview") }}
        </div>
        <div>
          <a target="_blank" :href="downloadUrl" class="button button--flat">
            <div>
              <i class="material-icons">file_download</i>{{ $t("buttons.download") }}
            </div>
          </a>
          <a target="_blank" :href="raw" class="button button--flat" v-if="currentItem.type != 'directory'">
            <div>
              <i class="material-icons">open_in_new</i>{{ $t("buttons.openFile") }}
            </div>
          </a>
        </div>
      </div>
    </div>

    <button @click="prev" @mouseover="hoverNav = true" @mouseleave="hoverNav = false"
      :class="{ hidden: !hasPrevious || !showNav }" :aria-label="$t('buttons.previous')"
      :title="$t('buttons.previous')">
      <i class="material-icons">chevron_left</i>
    </button>
    <button @click="next" @mouseover="hoverNav = true" @mouseleave="hoverNav = false"
      :class="{ hidden: !hasNext || !showNav }" :aria-label="$t('buttons.next')" :title="$t('buttons.next')">
      <i class="material-icons">chevron_right</i>
    </button>
    <link rel="prefetch" :href="previousRaw" />
    <link rel="prefetch" :href="nextRaw" />
  </div>
</template>
<script>
import { filesApi } from "@/api";
import { resizePreview } from "@/utils/constants";
import url from "@/utils/url.js";
import throttle from "@/utils/throttle";
import ExtendedImage from "@/components/files/ExtendedImage.vue";
import { state, getters, mutations } from "@/store";
import { getTypeInfo } from "@/utils/mimetype";

const mediaTypes = ["image", "video", "audio", "blob"];

export default {
  name: "preview",
  components: {
    ExtendedImage,
  },
  data() {
    return {
      previousLink: "",
      nextLink: "",
      listing: null,
      name: "",
      fullSize: true,
      showNav: true,
      navTimeout: null,
      hoverNav: false,
      autoPlay: false,
      previousRaw: "",
      nextRaw: "",
      currentPrompt: null, // Replaces Vuex getter `currentPrompt`
      oldReq: {}, // Replace with your actual initial state
      currentItem: {
        name: "",
        path: "",
        url: "",
        modified: "",
        type: "",
      },
    };
  },
  computed: {
    isDarkMode() {
      return getters.isDarkMode();
    },
    hasPrevious() {
      return this.previousLink !== "";
    },
    hasNext() {
      return this.nextLink !== "";
    },
    downloadUrl() {
      return filesApi.getDownloadURL(this.currentItem.url);
    },
    raw() {
      if (this.currentItem.url == "" || this.currentItem.url == undefined) {
        return;
      }
      const previewUrl = this.fullSize
        ? filesApi.getDownloadURL(this.currentItem.url, "large")
        : filesApi.getPreviewURL(
          this.currentItem.url,
          "small",
          this.currentItem.modified
        );
      return previewUrl;
    },
    showMore() {
      return getters.currentPromptName() === "more";
    },
    isResizeEnabled() {
      return resizePreview;
    },
    subtitles() {
      if (this.currentItem.subtitles) {
        return filesApi.getSubtitlesURL(this.currentItem);
      }
      return [];
    },
  },
  watch: {
    $route() {
      if (!getters.isLoggedIn()) {
        return;
      }
      this.updatePreview();
      this.toggleNavigation();
    },
  },
  async mounted() {
    window.addEventListener("keydown", this.key);
    this.listing = this.oldReq.items;
    this.updatePreview();
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.key);
  },
  methods: {
    getSimpleType(mimetype) {
      return getTypeInfo(mimetype).simpleType;
    },
    deleteFile() {
      this.currentPrompt = {
        name: "delete",
        confirm: () => {
          this.listing = this.listing.filter((item) => item.name !== this.name);
          if (this.hasNext) {
            this.next();
          } else if (!this.hasPrevious && !this.hasNext) {
            this.close();
          } else {
            this.prev();
          }
        },
      };
    },
    prev() {
      this.hoverNav = false;
      this.$router.replace({ path: this.previousLink });
    },
    next() {
      this.hoverNav = false;
      this.$router.replace({ path: this.nextLink });
    },
    key(event) {
      if (getters.currentPromptName() != null) {
        return;
      }

      const { key } = event;

      switch (key) {
        case "ArrowRight":
          if (this.hasNext) {
            this.next();
          }
          break;
        case "ArrowLeft":
          if (this.hasPrevious) {
            this.prev();
          }
          break;
        case ("Escape", "Backspace"):
          this.close();
          break;
      }
    },
    async updatePreview() {
      if (this.$refs.player && this.$refs.player.paused && !this.$refs.player.ended) {
        this.autoPlay = false;
      }
      let parts = state.route.path.split("/");
      this.name = decodeURI(parts.pop("/"));
      if (!this.listing) {
        const path = url.removeLastDir(state.route.path);
        const res = await filesApi.fetchFiles(path);
        this.listing = res.items;
      }
      this.previousLink = "";
      this.nextLink = "";
      const path = state.req.path;

      let directoryPath = path.substring(0, path.lastIndexOf("/"));
      if (directoryPath == "") {
        directoryPath = "/";
      }
      for (let i = 0; i < this.listing.length; i++) {
        if (this.listing[i].name !== this.name) {
          continue;
        }
        this.currentItem = this.listing[i];
        for (let j = i - 1; j >= 0; j--) {
          let composedListing = this.listing[j];
          composedListing.path = directoryPath + "/" + composedListing.name;
          if (mediaTypes.includes(composedListing.type.split("/")[0])) {
            this.previousLink = composedListing.url;
            this.previousRaw = this.prefetchUrl(composedListing);
            break;
          }
        }
        for (let j = i + 1; j < this.listing.length; j++) {
          let composedListing = this.listing[j];
          composedListing.path = directoryPath + "/" + composedListing.name;
          if (mediaTypes.includes(composedListing.type.split("/")[0])) {
            this.nextLink = composedListing.url;
            this.nextRaw = this.prefetchUrl(composedListing);
            break;
          }
        }
        return;
      }
    },
    prefetchUrl(item) {
      return this.fullSize
        ? filesApi.getDownloadURL(item.path, true)
        : filesApi.getPreviewURL(item.path, "large", item.modified);
    },
    openMore() {
      this.currentPrompt = "more";
    },
    resetPrompts() {
      this.currentPrompt = null;
    },
    toggleSize() {
      this.fullSize = !this.fullSize;
    },
    toggleNavigation: throttle(function () {
      this.showNav = true;

      if (this.navTimeout) {
        clearTimeout(this.navTimeout);
      }

      this.navTimeout = setTimeout(() => {
        this.showNav = false || this.hoverNav;
        this.navTimeout = null;
      }, 1500);
    }, 100),
    close() {
      mutations.replaceRequest({}); // Reset request data
      let uri = url.removeLastDir(state.route.path) + "/";
      this.$router.push({ path: uri });
    },
    download() {
      window.open(this.downloadUrl);
    },
  },
};
</script>
