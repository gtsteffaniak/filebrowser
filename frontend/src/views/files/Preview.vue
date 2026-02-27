<template>
  <div id="previewer">
    <!-- Loading overlay during navigation transition -->
    <div v-if="isTransitioning" class="transition-loading">
      <LoadingSpinner size="medium" />
    </div>
    <div class="preview" :class="{
        'plyr-background-light': !isDarkMode && previewType == 'audio' && !useDefaultMediaPlayer,
        'plyr-background-dark': isDarkMode && previewType == 'audio' && !useDefaultMediaPlayer,
        'transitioning': isTransitioning }" v-if="!isDeleted">
      <ExtendedImage v-if="showImage && !isTransitioning" :src="raw" @navigate-previous="navigatePrevious" @navigate-next="navigateNext"/>

      <!-- Media Player Component -->
      <plyrViewer v-else-if="previewType == 'audio' || previewType == 'video'"
        ref="plyrViewer"
        :previewType="previewType"
        :raw="raw"
        :subtitlesList="subtitlesList"
        :req="req"
        :listing="listing"
        :useDefaultMediaPlayer="useDefaultMediaPlayer"
        :autoPlayEnabled="autoPlay"
        @play="autoPlay = true"
        :class="{'plyr-background': previewType == 'audio' && !useDefaultMediaPlayer}" />

      <div v-else-if="previewType == 'pdf'" class="pdf-wrapper">
        <iframe class="pdf" :src="raw"></iframe>
        <a v-if="isMobileSafari" :href="raw" target="_blank" class="button button--flat floating-btn">
          <div>
            <i class="material-icons">open_in_new</i>{{ $t("general.openFile") }}
          </div>
        </a>
      </div>

      <div v-else class="info">
        <div class="title">
          <i class="material-icons">feedback</i>
          {{ $t("files.noPreview") }}
        </div>
        <div v-if="permissions.download">
          <a target="_blank" :href="downloadUrl" class="button button--flat">
            <div>
              <i class="material-icons">file_download</i>{{ $t("general.download") }}
            </div>
          </a>
          <a target="_blank" :href="raw" class="button button--flat" v-if="req.type != 'directory'">
            <div>
              <i class="material-icons">open_in_new</i>{{ $t("general.openFile") }}
            </div>
          </a>
        </div>
        <div v-else>
          <p> {{ $t("files.noDownloadAccess") }} </p>
        </div>
        <p> {{ req.name }} </p>
      </div>
    </div>
  </div>
</template>
<script>
import { filesApi, publicApi } from "@/api";
import { url } from "@/utils";
import ExtendedImage from "@/components/files/ExtendedImage.vue";
import plyrViewer from "@/views/files/plyrViewer.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { state, getters, mutations } from "@/store";
import { getFileExtension } from "@/utils/files";
import { convertToVTT } from "@/utils/subtitles";
import { globalVars } from "@/utils/constants";

export default {
  name: "preview",
  components: {
    ExtendedImage,
    plyrViewer,
    LoadingSpinner,
  },
  data() {
    return {
      listing: null,
      name: "",
      fullSize: true,
      currentPrompt: null, // Replaces Vuex getter `currentPrompt`
      subtitlesList: [],
      isDeleted: false,
      tapTimeout: null,
    };
  },
  computed: {
    permissions() {
      return getters.permissions();
    },
    showImage() {
      if (state.req.type == "image/heic" || state.req.type == "image/heif") {
        if (this.isHeicAndViewable) {
          return true;
        }
        return false;
      }
      return this.previewType == 'image' || this.pdfConvertable;
    },
    autoPlay() {
      return getters.previewPerms().autoplayMedia;
    },
    useDefaultMediaPlayer() {
      return getters.previewPerms().defaultMediaPlayer === true;
    },
    isMobileSafari() {
      const userAgent = window.navigator.userAgent;
      const isIOS =
        /iPad|iPhone|iPod/.test(userAgent) && !window.MSStream;
      const isSafari = /^((?!chrome|android).)*safari/i.test(userAgent);
      return isIOS && isSafari;
    },
    isHeicAndViewable() {
      if (globalVars.enableHeicConversion || state.isSafari) {
        return true;
      }
      return false;
    },
    pdfConvertable() {
      if (!globalVars.muPdfAvailable) {
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
      return !!pdfConvertCompatibleFileExtensions[ext];
    },
    sidebarShowing() {
      return getters.isSidebarVisible();
    },
    previewType() {
      if (getters.fileViewingDisabled(state.req.name)) {
        return "preview";
      }
      return getters.previewType();
    },
    raw() {
      const showFullSizeHeic = state.req.type === "image/heic" && !state.isSafari && globalVars.mediaAvailable && !globalVars.disableHeicConversion;
      if (this.pdfConvertable || showFullSizeHeic) {
        if (getters.isShare()) {
          const previewPath = url.removeTrailingSlash(state.req.path);
          return publicApi.getPreviewURL(previewPath, "original");
        }
        return (
          filesApi.getPreviewURL(
            state.req.source,
            state.req.path,
            state.req.modified,
          ) + "&size=original"
        );
      }
      if (getters.isShare()) {
        return publicApi.getDownloadURL(
          {
            path: state.shareInfo.subPath,
            hash: state.shareInfo.hash,
            token: state.shareInfo.token,
          },
          [state.req.path],
          true,
        );
      }
      return filesApi.getDownloadURL(
        state.req.source,
        state.req.path,
        true,
      );
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    downloadUrl() {
      if (getters.isShare()) {
        return publicApi.getDownloadURL(
          {
            path: state.shareInfo.subPath,
            hash: state.shareInfo.hash,
            token: state.shareInfo.token,
          },
          [state.req.path],
        );
      }
      return filesApi.getDownloadURL(state.req.source, state.req.path);
    },
    isTransitioning() {
      return state.navigation.isTransitioning;
    },
    getSubtitles() {
      return this.subtitles();
    },
    req() {
      return state.req;
    },
    disableFileViewer() {
      return state.shareInfo?.disableFileViewer;
    },
  },
  watch: {
    async req() {
      if (!getters.isLoggedIn()) {
        return;
      }

      this.isDeleted = false;
      // Reload subtitles when navigating to a new video
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
  },
  async mounted() {
    // Check for pre-fetched parent directory items from Files.vue
    if (state.req.parentDirItems) {
      this.listing = state.req.parentDirItems;
    } else if (state.req.items) {
      this.listing = state.req.items;
    }
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
    // Clear navigation state when leaving preview
    mutations.clearNavigation();
  },
  methods: {
    async subtitles() {
      if (!state.req?.subtitles?.length) {
        return [];
      }
      let subs = [];
      // Fetch subtitle content for each track using the media API
      for (let index = 0; index < state.req.subtitles.length; index++) {
        const subtitleTrack = state.req.subtitles[index];
        try {
          // Fetch subtitle content from API using name and embedded
          const content = await filesApi.getSubtitleContent(
            state.req.source,
            state.req.path,
            subtitleTrack.name,
            subtitleTrack.embedded
          );
          if (!content || content.length === 0) {
            console.warn("Subtitle track has no content:", subtitleTrack.name);
            continue;
          }
          // Convert to VTT if needed
          let vttContent = content;
          if (!content.startsWith("WEBVTT")) {
            const ext = getFileExtension(subtitleTrack.name);
            vttContent = convertToVTT(ext, content);
          }
          if (vttContent.startsWith("WEBVTT")) {
            // Create a virtual file (Blob) and get a URL for it
            const blob = new Blob([vttContent], { type: "text/vtt" });
            const vttURL = URL.createObjectURL(blob);

            subs.push({
              name: subtitleTrack.name,
              src: vttURL,
              language: subtitleTrack.language
            });
          } else {
            console.warn(
              "Skipping subtitle track - no WEBVTT header after conversion:",
              subtitleTrack.name
            );
          }
        } catch (err) {
          console.error("Failed to load subtitle:", subtitleTrack.name, err);
        }
      }
      return subs;
    },
    async keyEvent(event) {
      if (getters.currentPromptName()) {
        return;
      }

      const { key } = event;

      switch (key) {
        case "Delete":
          this.showDeletePrompt();
          break;
        case "Escape":
        case "Backspace":
          this.close();
          break;
      }
    },
    async updatePreview() {
      let directoryPath = url.removeLastDir(state.req.path);

      // If directoryPath is empty, the file is in root - use '/' as the directory
      if (!directoryPath || directoryPath === '') {
        directoryPath = '/';
      }

      if (!this.listing || this.listing == "undefined") {
        // Try to use pre-fetched parent directory items first
        if (state.req.parentDirItems) {
          this.listing = state.req.parentDirItems;
        } else if (directoryPath !== state.req.path) {
          // Fetch directory listing (now with '/' for root files)
          try {
            let res;
            if (getters.isShare()) {
              // Use public API for shared files
              res = await publicApi.fetchPub(
                directoryPath,
                state.shareInfo?.hash,
              );
            } else {
              // Use regular files API for authenticated users
              res = await filesApi.fetchFiles(
                state.req.source,
                directoryPath,
              );
            }
            this.listing = res.items;
          } catch (error) {
            console.error("error Preview.vue", error);
            this.listing = [state.req];
          }
        } else {
          console.error("No listing found Preview.vue");
          // Shouldn't happen, but fallback to current item
          this.listing = [state.req];
        }
      }

      if (!this.listing) {
        this.listing = [state.req];
      }
      this.name = state.req.name;

      // Setup navigation using the new state management
      mutations.setupNavigation({
        listing: this.listing,
        currentItem: state.req,
        directoryPath: directoryPath
      });
    },

    prefetchUrl(item) {
      if (getters.isShare()) {
        return this.fullSize
          ? publicApi.getDownloadURL(
              {
                path: item.path,
                hash: state.shareInfo?.hash,
                token: state.shareInfo.token,
                inline: true,
              },
              [item.path],
            )
          : publicApi.getPreviewURL(state.shareInfo?.hash, item.path);
      }
      return this.fullSize
        ? filesApi.getDownloadURL(state.req.source, item.path, true)
        : filesApi.getPreviewURL(
            state.req.source,
            item.path,
            item.modified,
          );
    },
    resetPrompts() {
      this.currentPrompt = null;
    },
    showDeletePrompt() {
      const item = state.req;
      const previewUrl = item.hasPreview
        ? filesApi.getPreviewURL(item.source, item.path, item.modified)
        : null;
      mutations.showHover({
        name: "delete",
        props: {
          items: [{
            source: item.source,
            path: item.path,
            type: item.type,
            size: item.size,
            modified: item.modified,
            previewUrl: previewUrl,
          }],
        },
      });
    },
    toggleSize() {
      this.fullSize = !this.fullSize;
    },
    close() {
      mutations.replaceRequest({}); // Reset request data
      let uri = url.removeLastDir(state.route.path) + "/";
      this.$router.push({ path: uri });
    },
    download() {
      const items = [state.req];
      downloadFiles(items);
    },
    navigatePrevious() {
      if (state.navigation.previousLink) {
        this.$router.replace({ path: state.navigation.previousLink });
      }
    },
    navigateNext() {
      if (state.navigation.nextLink) {
        this.$router.replace({ path: state.navigation.nextLink });
      }
    },
  },
};
</script>

<style scoped>
/* Loading overlay for navigation transitions */
.transition-loading {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: var(--background);
  z-index: 10000;
  transition: 0.1s ease opacity;
}

.transition-loading .spinner {
  width: 70px;
  text-align: center;
}

.transition-loading .spinner > div {
  width: 18px;
  height: 18px;
  background-color: var(--textPrimary);
  border-radius: 100%;
  display: inline-block;
  animation: sk-bouncedelay 1.4s infinite ease-in-out both;
}

.transition-loading .spinner .bounce1 {
  animation-delay: -0.32s;
}

.transition-loading .spinner .bounce2 {
  animation-delay: -0.16s;
}

@keyframes sk-bouncedelay {
  0%, 80%, 100% {
    transform: scale(0);
  }
  40% {
    transform: scale(1.0);
  }
}

.preview.transitioning {
  opacity: 0.3;
  pointer-events: none;
}

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
