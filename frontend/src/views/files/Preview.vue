<template>
  <div id="previewer" @mousemove="toggleNavigation" @touchstart="toggleNavigation">
    <div class="loading delayed" :class="{ 'dark-mode': isDarkMode }" v-if="loading">
      <div class="spinner">
        <div class="bounce1"></div>
        <div class="bounce2"></div>
        <div class="bounce3"></div>
      </div>
    </div>
    <template v-else>
      <div class="preview">
        <ExtendedImage v-if="req.type == 'image'" :src="raw"></ExtendedImage>
        <audio
          v-else-if="req.type == 'audio'"
          ref="player"
          :src="raw"
          controls
          :autoplay="autoPlay"
          @play="autoPlay = true"
        ></audio>
        <video
          v-else-if="req.type == 'video'"
          ref="player"
          :src="raw"
          controls
          :autoplay="autoPlay"
          @play="autoPlay = true"
        >
          <track
            kind="captions"
            v-for="(sub, index) in subtitles"
            :key="index"
            :src="sub"
            :label="'Subtitle ' + index"
            :default="index === 0"
          />
          Sorry, your browser doesn't support embedded videos, but don't worry, you can
          <a :href="downloadUrl">download it</a>
          and watch it with your favorite video player!
        </video>
        <object v-else-if="req.type == 'pdf'" class="pdf" :data="raw"></object>
        <div v-else-if="req.type == 'blob' || req.type == 'archive'" class="info">
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
            <a target="_blank" :href="raw" class="button button--flat" v-if="!req.isDir">
              <div>
                <i class="material-icons">open_in_new</i>{{ $t("buttons.openFile") }}
              </div>
            </a>
          </div>
        </div>
      </div>
    </template>

    <button
      @click="prev"
      @mouseover="hoverNav = true"
      @mouseleave="hoverNav = false"
      :class="{ hidden: !hasPrevious || !showNav }"
      :aria-label="$t('buttons.previous')"
      :title="$t('buttons.previous')"
    >
      <i class="material-icons">chevron_left</i>
    </button>
    <button
      @click="next"
      @mouseover="hoverNav = true"
      @mouseleave="hoverNav = false"
      :class="{ hidden: !hasNext || !showNav }"
      :aria-label="$t('buttons.next')"
      :title="$t('buttons.next')"
    >
      <i class="material-icons">chevron_right</i>
    </button>
    <link rel="prefetch" :href="previousRaw" />
    <link rel="prefetch" :href="nextRaw" />
  </div>
</template>
<script>
import { files as api } from "@/api";
import { resizePreview } from "@/utils/constants";
import url from "@/utils/url";
import throttle from "@/utils/throttle";
import ExtendedImage from "@/components/files/ExtendedImage.vue";
import { state, getters, mutations } from "@/store"; // Import your custom store

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
      fullSize: false,
      showNav: true,
      navTimeout: null,
      hoverNav: false,
      autoPlay: false,
      previousRaw: "",
      nextRaw: "",
      currentPrompt: null, // Replaces Vuex getter `currentPrompt`
      oldReq: {}, // Replace with your actual initial state
      jwt: "", // Replace with your actual initial state
      loading: false, // Replace with your actual initial state
    };
  },
  computed: {
    req() {
      return state.req;
    },
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
      return api.getDownloadURL(state.req);
    },
    raw() {
      if (state.req.type === "image" && !this.fullSize) {
        return api.getPreviewURL(state.req, "big");
      }
      return api.getDownloadURL(state.req, true);
    },
    showMore() {
      return getters.currentPromptName() === "more";
    },
    isResizeEnabled() {
      return resizePreview;
    },
    subtitles() {
      if (state.req.subtitles) {
        return api.getSubtitlesURL(state.req);
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
      if (this.currentPrompt !== null) {
        return;
      }

      if (event.which === 13 || event.which === 39) {
        // right arrow
        if (this.hasNext) this.next();
      } else if (event.which === 37) {
        // left arrow
        if (this.hasPrevious) this.prev();
      } else if (event.which === 27) {
        // esc
        this.close();
      }
    },
    async updatePreview() {
      if (this.$refs.player && this.$refs.player.paused && !this.$refs.player.ended) {
        this.autoPlay = false;
      }

      let dirs = state.route.fullPath.split("/");
      this.name = decodeURIComponent(dirs[dirs.length - 1]);

      if (!this.listing) {
        try {
          const path = url.removeLastDir(state.route.path);
          const res = await api.fetch(path);
          this.listing = res.items;
        } catch (e) {
          showError(e);
        }
      }

      this.previousLink = "";
      this.nextLink = "";

      for (let i = 0; i < this.listing.length; i++) {
        if (this.listing[i].name !== this.name) {
          continue;
        }

        for (let j = i - 1; j >= 0; j--) {
          if (mediaTypes.includes(this.listing[j].type)) {
            this.previousLink = this.listing[j].url;
            this.previousRaw = this.prefetchUrl(this.listing[j]);
            break;
          }
        }
        for (let j = i + 1; j < this.listing.length; j++) {
          if (mediaTypes.includes(this.listing[j].type)) {
            this.nextLink = this.listing[j].url;
            this.nextRaw = this.prefetchUrl(this.listing[j]);
            break;
          }
        }

        return;
      }
    },
    prefetchUrl(item) {
      if (item.type !== "image") {
        return "";
      }

      return this.fullSize
        ? api.getDownloadURL(item, true)
        : api.getPreviewURL(item, "big");
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
    }, 500),
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
