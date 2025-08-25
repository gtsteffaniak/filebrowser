<template>
  <div id="previewer" @mousemove="toggleNavigation" @touchstart="toggleNavigation">
    <div class="preview" v-if="!isDeleted">
      <ExtendedImage v-if="previewType == 'image' || pdfConvertable && !disableFileViewer" :src="raw" 
        @navigate-previous="prev"
        @navigate-next="next">
      </ExtendedImage>
      <audio v-else-if="previewType == 'audio' && !disableFileViewer" ref="player" :src="raw" controls :autoplay="autoPlay"
        @play="autoPlay = true"></audio>
      <video v-else-if="previewType == 'video' && !disableFileViewer" ref="player" :src="raw" controls :autoplay="autoPlay"
        @play="autoPlay = true">
        <track kind="captions" v-for="(sub, index) in subtitlesList" :key="index" :src="sub.src"
          :label="'Subtitle ' + sub.name" :default="index === 0" />
      </video>

      <div v-else-if="previewType == 'pdf' && !disableFileViewer" class="pdf-wrapper">
        <iframe class="pdf" :src="raw"></iframe>
        <a v-if="isMobileSafari" :href="raw" target="_blank" class="button button--flat floating-btn">
          <div>
            <i class="material-icons">open_in_new</i>{{ $t("buttons.openFile") }}
          </div>
        </a>
      </div>

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
          <a target="_blank" :href="raw" class="button button--flat" v-if="req.type != 'directory'">
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
import { filesApi, publicApi } from "@/api";
import { url } from "@/utils";
import throttle from "@/utils/throttle";
import ExtendedImage from "@/components/files/ExtendedImage.vue";
import { state, getters, mutations } from "@/store";
import { getFileExtension } from "@/utils/files";
import { convertToVTT } from "@/utils/subtitles";
import { getTypeInfo } from "@/utils/mimetype";
import { muPdfAvailable } from "@/utils/constants";
import { shareInfo } from "@/utils/constants";
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
      previousRaw: "",
      nextRaw: "",
      currentPrompt: null, // Replaces Vuex getter `currentPrompt`
      subtitlesList: [],
      isDeleted: false,
    };
  },
  computed: {
    autoPlay() {
      return state.user.preview.autoplayMedia;
    },
    isMobileSafari() {
      const userAgent = window.navigator.userAgent;
      const isIOS = /iPad|iPhone|iPod/.test(userAgent) && !window.MSStream;
      const isSafari = /^((?!chrome|android).)*safari/i.test(userAgent);
      return isIOS && isSafari;
    },
    pdfConvertable() {
      if (!muPdfAvailable) {
        return false;
      }
      const ext = "." + state.req.name.split(".").pop().toLowerCase(); // Ensure lowercase and dot
      const pdfConvertCompatibleFileExtensions = {
        ".xps": true,
        ".mobi": true,
        ".fb2": true,
        ".cbz": true,
        ".svg": true,
        ".docx": true,
        ".pptx": true,
        ".xlsx": true,
        ".hwp": true,
        ".hwpx": true,
      };
      if (state.user.disableViewingExt.includes(ext)) {
        return false;
      }
      return !!pdfConvertCompatibleFileExtensions[ext] ;
    },
    sidebarShowing() {
      return getters.isSidebarVisible();
    },
    previewType() {
      return getters.previewType();
    },
    raw() {
      if (this.pdfConvertable) {
        if (getters.isShare()) {
          const previewPath = url.removeTrailingSlash(state.req.path);
          return publicApi.getPreviewURL(previewPath,"original");
        }
        return (
          filesApi.getPreviewURL(state.req.source, state.req.path, state.req.modified) + "&size=original"
        );
      }
      if (getters.isShare()) {
        return publicApi.getDownloadURL({
          path: state.share.subPath,
          hash: state.share.hash,
          token: state.share.token,
        }, [state.req.path], true);
      }
      return filesApi.getDownloadURL(state.req.source, state.req.path, true);
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
      if (getters.isShare()) {
        return publicApi.getDownloadURL({
          path: state.share.subPath,
          hash: state.share.hash,
          token: state.share.token,
        }, [state.req.path]);
      }
      return filesApi.getDownloadURL(state.req.source, state.req.path);
    },
    getSubtitles() {
      return this.subtitles();
    },
    req() {
      return state.req;
    },
    deletedItem() {
      return state.deletedItem;
    },
    disableFileViewer() {
      return shareInfo.disableFileViewer;
    },
  },
  watch: {
    deletedItem() {
      if (!state.deletedItem) {
        return;
      }
      this.isDeleted = true;
      this.listing = null; // Invalidate the listing to force a refresh
      this.nextRaw = "";
      this.previousRaw = "";
      if (this.hasNext) {
        this.next();
      } else if (!this.hasPrevious && !this.hasNext) {
        this.close();
      } else {
        this.prev();
      }
      mutations.setDeletedItem(false);
    },
    req() {
      if (!getters.isLoggedIn()) {
        return;
      }
      this.isDeleted = false;
      this.updatePreview();
      this.toggleNavigation();
      mutations.resetSelected();
      mutations.addSelected({
        name: state.req.name,
        path: state.req.path,
        size: state.req.size,
        type: state.req.type,
        source: state.req.source,
      });
    },
  },
  async mounted() {
    if (state.req.items) {
      this.listing = state.req.items;
    }
    mutations.setDeletedItem(false);
    window.addEventListener("keydown", this.keyEvent);
    this.subtitlesList = await this.subtitles();
    this.updatePreview();
    mutations.resetSelected();
    mutations.addSelected({
      name: state.req.name,
      path: state.req.path,
      size: state.req.size,
      type: state.req.type,
      source: state.req.source,
    });
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
  },
  methods: {
    async subtitles() {
      if (!state.req.subtitles || state.req.subtitles.length === 0) {
        return [];
      }
      let subs = [];
      for (const subtitleFile of state.req.subtitles) {
        const ext = getFileExtension(subtitleFile);
        const path = url.removeLastDir(state.req.path) + "/" + subtitleFile;

        let resp;
        if (getters.isShare()) {
          // Use public API for shared files
          resp = await publicApi.fetchPub(path, state.share.hash, "", true);
        } else {
          // Use regular files API for authenticated users
          resp = await filesApi.fetchFiles(state.req.source, path, true);
        }

        let vttContent = resp.content;
        // Convert SRT to VTT (assuming srt2vtt() does this)
        vttContent = convertToVTT(ext, resp.content);
        // Create a virtual file (Blob) and get a URL for it
        const blob = new Blob([vttContent], { type: "text/vtt" });
        const vttURL = URL.createObjectURL(blob);
        subs.push({
          name: ext,
          src: vttURL,
        });
      }
      return subs;
    },
    prev() {
      this.hoverNav = false;
      this.$router.replace({ path: this.previousLink });
    },
    next() {
      this.hoverNav = false;
      this.$router.replace({ path: this.nextLink });
    },
    async keyEvent(event) {
      if (getters.currentPromptName()) {
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
        case "Delete":
          mutations.showHover("delete")
          break;
        case "Escape":
        case "Backspace":
          this.close();
          break;
      }
    },
    async updatePreview() {
      // Try to autoplay media, handle browser restrictions
      if (this.autoPlay && (this.previewType === "video" || this.previewType === "audio")) {
        this.$nextTick(() => {
          if (this.$refs.player) {
            const playPromise = this.$refs.player.play();
            if (playPromise !== undefined) {
              playPromise.catch((error) => {
                if (this.$refs.player) {
                  console.log("autoplay failed", error);
                  this.$refs.player.muted = true;
                  this.$refs.player.play();
                }
              });
            }
          }
        });
      }
      const directoryPath = url.removeLastDir(state.req.path);
      if (!this.listing || this.listing == "undefined") {
        let res;
        if (getters.isShare()) {
          // Use public API for shared files
          res = await publicApi.fetchPub(directoryPath, state.share.hash);
        } else {
          // Use regular files API for authenticated users
          res = await filesApi.fetchFiles(state.req.source, directoryPath);
        }
      }
      if (!this.listing) {
        this.listing = [state.req]
      }
      this.name = state.req.name;
      this.previousLink = "";
      this.nextLink = "";

      for (let i = 0; i < this.listing.length; i++) {
        if (this.listing[i].name !== this.name) {
          continue;
        }
        for (let j = i - 1; j >= 0; j--) {
          let composedListing = this.listing[j];
          composedListing.path = directoryPath + "/" + composedListing.name;
          this.previousLink = url.buildItemUrl(composedListing.source, composedListing.path);
          if (getTypeInfo(composedListing.type).simpleType == "image") {
            this.previousRaw = this.prefetchUrl(composedListing);
          }
          break;
        }
        for (let j = i + 1; j < this.listing.length; j++) {
          let composedListing = this.listing[j];
          composedListing.path = directoryPath + "/" + composedListing.name;
          this.nextLink = url.buildItemUrl(composedListing.source, composedListing.path);
          if (getTypeInfo(composedListing.type).simpleType == "image") {
            this.nextRaw = this.prefetchUrl(composedListing);
          }
          break;
        }
        return;
      }
    },
    prefetchUrl(item) {
      if (getters.isShare()) {
        return this.fullSize
          ? publicApi.getDownloadURL({
              path: item.path,
              hash: state.share.hash,
              token: state.share.token,
              inline: true,
            }, [item.path])
          : publicApi.getPreviewURL(state.share.hash, item.path);
      }
      return this.fullSize
        ? filesApi.getDownloadURL(state.req.source, item.path, true)
        : filesApi.getPreviewURL(state.req.source, item.path, item.modified);
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
      const items = [state.req];
      downloadFiles(items);
    },
  },
};
</script>

<style scoped>
.pdf-wrapper {
  position: relative;
  width: 100%;
  height: 100%;
}

.pdf-wrapper .pdf {
  width: 100%;
  height: 100%;
  border: 0;
}

.pdf-wrapper .floating-btn {
  background: rgba(0, 0, 0, 0.5);
  color: white;
}

.pdf-wrapper .floating-btn:hover {
  background: rgba(0, 0, 0, 0.7);
}
</style>
