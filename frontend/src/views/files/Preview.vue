<template>
  <PreviewShell>
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
      <PreviewNotice
        v-if="usePdfPreviewFallback"
        icon="picture_as_pdf"
        :title="$t('files.mobilePdfPreviewHint')"
        :download-url="permissions.download ? downloadUrl : undefined"
      />
      <iframe
        v-else
        allow="web-share"
        class="pdf"
        :src="raw"
        :title="req.name || 'PDF'"
      ></iframe>
    </div>

    <PreviewNotice
      v-else
      icon="feedback"
      :title="$t('files.noPreview')"
      :download-url="permissions.download ? downloadUrl : undefined"
    >
      <div v-if="!permissions.download">
        <p> {{ $t("files.noDownloadAccess") }} </p>
      </div>
    </PreviewNotice>
  </PreviewShell>
</template>
<script>
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import FloatingActionButton from "@/components/settings/FloatingActionButton.vue";
import { countMediaInListing } from "@/utils/slideshow.ts";
import { resourcesApi } from "@/api/index.js";
import PreviewNotice from "@/components/files/PreviewNotice.vue";
import PreviewShell from "@/components/files/PreviewShell.vue";
import { removeTrailingSlash } from "@/utils/url.js";
import { usePreviewNavigation } from "@/composables/preview/usePreviewNavigation.ts";
import { state, getters, mutations } from "@/store/index.ts";
import { isRawImageMimeType } from "@/utils/mimetype.js";
import { globalVars } from "@/utils/constants.ts";
import { isPdfFile } from "@/utils/mediaFile.js";
import { shouldUsePdfPreviewFallback } from "@/utils/pdfPreview.js";
export default {
  name: "preview",
  components: {
    PreviewNotice,
    PreviewShell,
    FloatingActionButton,
    ExtendedImage: createAsyncComponent(() => import('@/components/files/ExtendedImage.vue')),
  },
  setup() {
    return usePreviewNavigation();
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
      const path = state.req.path;
      await this.loadListingContext();

      if (state.req.path !== path) {
        return;
      }
      await this.updatePreview();
      this.selectCurrentItem();
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
  background: rgb(0 0 0 / 50%);
  color: white;
}

.pdf-wrapper .floating-btn:hover {
  background: rgb(0 0 0 / 70%);
}
</style>
