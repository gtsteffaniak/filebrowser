<template>
  <div id="previewer">
    <div v-if="isTransitioning" class="transition-loading">
      <LoadingSpinner size="medium" />
    </div>
    <div class="preview" :class="{
      'transitioning': isTransitioning
    }" v-if="!isDeleted">
      <template v-if="showImage && !isTransitioning">
        <ExtendedImage :src="raw" @navigate-previous="navigatePrevious"
          @navigate-next="navigateNext" @close-preview="exitPreviewFromImageGesture" />

        <FloatingActionButton
          v-if="showSlideshowButton"
          icon="slideshow"
          position="top-right"
          :label="$t('slideshow.start')"
          @press="openSlideshowOptions"
        />
      </template>

      <div v-else-if="isPdf" class="pdf-wrapper">
        <div v-if="usePdfPreviewFallback" class="info">
          <div class="title">
            <i class="material-symbols">picture_as_pdf</i>
            {{ $t("files.mobilePdfPreviewHint") }}
          </div>
          <div class="preview-buttons" v-if="permissions.download">
            <a
              target="_blank"
              :href="downloadUrl"
              class="button button--flat preview-action--icon-only"
              :aria-label="$t('general.download', { suffix: '' })"
            >
              <i class="material-symbols">file_download</i>
            </a>
          </div>
        </div>
        <iframe
          v-else
          allow="web-share"
          class="pdf"
          :src="raw"
          :title="req.name || 'PDF'"
        ></iframe>
      </div>

      <div v-else class="info">
        <div class="title">
          <i class="material-symbols">feedback</i>
          {{ $t("files.noPreview") }}
        </div>
        <div class="preview-buttons" v-if="permissions.download">
          <a
            target="_blank"
            :href="downloadUrl"
            class="button button--flat preview-action--icon-only"
            :aria-label="$t('general.download', { suffix: '' })"
          >
            <i class="material-symbols">file_download</i>
          </a>
        </div>
        <div v-else>
          <p> {{ $t("files.noDownloadAccess") }} </p>
        </div>
      </div>
    </div>
  </div>
</template>
<script>
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import FloatingActionButton from "@/components/settings/FloatingActionButton.vue";
import { countMediaInListing } from "@/utils/slideshow.ts";
import { resourcesApi } from "@/api/index.js";
import { removeTrailingSlash } from "@/utils/url.js";
import { usePreviewNavigation } from "@/composables/preview/usePreviewNavigation.ts";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { state, getters, mutations } from "@/store/index.ts";
import { isRawImageMimeType } from "@/utils/mimetype.js";
import { globalVars } from "@/utils/constants.ts";
import { isPdfFile } from "@/utils/mediaFile.js";
import { shouldUsePdfPreviewFallback } from "@/utils/pdfPreview.js";
export default {
  name: "preview",
  components: {
    LoadingSpinner,
    FloatingActionButton,
    ExtendedImage: createAsyncComponent(() => import('@/components/files/ExtendedImage.vue')),
  },
  setup() {
    return usePreviewNavigation();
  },
  data() {
    return {
      fullSize: true,
      currentPrompt: null,
      isDeleted: false,
      tapTimeout: null,
    };
  },
  computed: {
    // Slideshow entry point: images only, and only when the folder has at least two images or videos.
    showSlideshowButton() {
      return this.previewType === 'image' && !this.isTransitioning && countMediaInListing(this.listing) >= 2;
    },
    permissions() {
      return getters.sourcePermissions();
    },
    showImage() {
      if (state.req.type === "image/heic" || state.req.type === "image/heif") {
        return this.isHeicAndViewable;
      }
      if (isRawImageMimeType(state.req.type)) {
        return true;
      }
      return this.previewType === 'image' || this.pdfConvertable;
    },
    isMobileSafari() {
      const userAgent = window.navigator.userAgent;
      const isIOS =
        /iPad|iPhone|iPod/.test(userAgent) && !window.MSStream;
      const isSafari = /^((?!chrome|android).)*safari/i.test(userAgent);
      return isIOS && isSafari;
    },
    // Safari: native HEIC in ExtendedImage. Other browsers: converted original when hasPreview.
    isHeicAndViewable() {
      if (state.isSafari) return true;
      return Boolean(state.req?.hasPreview);
    },
    pdfConvertable() {
      const ext = `.${state.req.name.split(".").pop().toLowerCase()}`;
      if (state.user.disableViewingExt.includes(ext)) {
        return false;
      }
      const list = globalVars.docPreviewExtensions || [];
      return list.includes(ext.slice(1));
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
    isPdf() {
      return (
        isPdfFile(state.req.type) || isPdfFile(state.req.name)
      );
    },
    usePdfPreviewFallback() {
      return this.isPdf && shouldUsePdfPreviewFallback();
    },
    raw() {
      const viewToken = state.req.viewToken;
      const typeHint = state.req.type || state.req.name;
      const isHeicOrHeif = state.req.type === "image/heic" || state.req.type === "image/heif";

      const getRawPreview = isRawImageMimeType(state.req.type);
      // HEIC/HEIF at size=original for non-Safari; Safari full size is resolved in ExtendedImage.
      const getHeicConvertedPreview = isHeicOrHeif && !state.isSafari && state.req?.hasPreview;
      if (this.pdfConvertable || getRawPreview || getHeicConvertedPreview) {
        if (getters.isShare()) {
          const previewPath = removeTrailingSlash(state.req.path);
          return resourcesApi.getPreviewURLPublic(previewPath, "original", state.req.viewToken);
        }
        return (
          `${resourcesApi.getPreviewURL(
            state.req.source,
            state.req.path,
            state.req.modified,
          )}&size=original`
        );
      }
      if (getters.isShare()) {
        return resourcesApi.getViewURL(
          state.req.source,
          state.req.path,
        viewToken,
          {
            path: state.shareInfo.subPath,
            hash: state.shareInfo.hash,
          },
          false,
          typeHint,
        );
      }
      return resourcesApi.getViewURL(state.req.source, state.req.path, viewToken, null, false, typeHint);
    },
    downloadUrl() {
      if (getters.isShare()) {
        return resourcesApi.getDownloadURLPublic(
          {
            path: state.shareInfo.subPath,
            hash: state.shareInfo.hash,
          },
          [state.req.path],
        );
      }
      return resourcesApi.getDownloadURL(state.req.source, state.req.path);
    },
    isTransitioning() {
      return state.navigation.isTransitioning;
    },
    req() {
      return state.req;
    },
    disableFileViewer() {
      return state.shareInfo.disableFileViewer;
    },
  },
  watch: {
    req: {
      immediate: true,
      async handler() {
        await this.loadPreviewForReq();
      },
    },
  },
  methods: {
    openSlideshowOptions() {
      mutations.showPrompt({
        name: 'SlideshowOptions',
        props: { startName: state.req.name },
      });
    },
    async loadPreviewForReq() {
      if (!getters.isLoggedIn() && !getters.isShare()) {
        return;
      }
      this.isDeleted = false;
      const path = state.req.path;
      await this.loadListingContext();

      if (state.req.path !== path) {
        return;
      }
      await this.updatePreview();
      this.selectCurrentItem();
    },
    prefetchUrl(item) {
      const viewToken = item.viewToken || state.req.viewToken;
      const typeHint = item.type || item.name;
      if (getters.isShare()) {
        return this.fullSize
          ? resourcesApi.getViewURL(
              state.req.source,
              item.path,
              viewToken,
              {
                path: item.path,
                hash: state.shareInfo?.hash,
              },
              false,
              typeHint,
            )
          : resourcesApi.getPreviewURLPublic(item.path);
      }
      return this.fullSize
        ? resourcesApi.getViewURL(state.req.source, item.path, viewToken, null, false, typeHint)
        : resourcesApi.getPreviewURL(
            state.req.source,
            item.path,
            item.modified,
          );
    },
    resetPrompts() {
      this.currentPrompt = null;
    },
    toggleSize() {
      this.fullSize = !this.fullSize;
    },
    download() {
      const items = [state.req];
      downloadFiles(items);
    },
  },
};
</script>

<style scoped>
.transition-loading {
  position: fixed;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: var(--background);
  z-index: 10000;
  transition: opacity 0.1s ease;
}

.transition-loading .spinner {
  width: 70px;
  text-align: center;
}

.transition-loading .spinner>div {
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
  background: rgb(0 0 0 / 50%);
  color: white;
}

.pdf-wrapper .floating-btn:hover {
  background: rgb(0 0 0 / 70%);
}

.preview-buttons {
  display: flex;
  flex-direction: row;
  gap: 1em;
}

</style>
