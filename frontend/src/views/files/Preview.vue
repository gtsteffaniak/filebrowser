<template>
  <PreviewShell>
    <template v-if="showImage && !isTransitioning">
      <ExtendedImage :src="raw" @navigate-previous="navigatePrevious"
        @navigate-next="navigateNext" @close-preview="exitPreviewFromImageGesture" />

      <FloatingActionButton
        v-if="showSlideshowButton"
        icon="slideshow"
        position="top-right"
        :label="t('slideshow.start')"
        @press="openSlideshowOptions"
      />
    </template>

    <div v-else-if="isPdf" class="pdf-wrapper">
      <PreviewNotice
        v-if="usePdfPreviewFallback"
        icon="picture_as_pdf"
        :title="t('files.mobilePdfPreviewHint')"
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
      :title="t('files.noPreview')"
      :download-url="permissions.download ? downloadUrl : undefined"
    >
      <div v-if="!permissions.download">
        <p> {{ t("files.noDownloadAccess") }} </p>
      </div>
    </PreviewNotice>
  </PreviewShell>
</template>

<script setup lang="ts">
import { computed, watch } from "vue";
import { resourcesApi } from "@/api/index.js";
import PreviewNotice from "@/components/files/PreviewNotice.vue";
import PreviewShell from "@/components/files/PreviewShell.vue";
import FloatingActionButton from "@/components/settings/FloatingActionButton.vue";
import { usePreviewNavigation } from "@/composables/preview/usePreviewNavigation.ts";
import { useI18n } from "@/i18n/index.ts";
import { getters, mutations, state } from "@/store/index.ts";
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import { globalVars } from "@/utils/constants.ts";
import { isPdfFile } from "@/utils/mediaFile.js";
import { isRawImageMimeType } from "@/utils/mimetype.js";
import { shouldUsePdfPreviewFallback } from "@/utils/pdfPreview.js";
import { countMediaInListing } from "@/utils/slideshow.ts";
import { removeTrailingSlash } from "@/utils/url.js";

const ExtendedImage = createAsyncComponent(() => import("@/components/files/ExtendedImage.vue"));

const { t } = useI18n();

const { listing, loadListingContext, updatePreview, selectCurrentItem, navigatePrevious, navigateNext, exitPreviewFromImageGesture } =
  usePreviewNavigation();

const req = computed(() => state.req);
const isTransitioning = computed(() => state.navigation.isTransitioning);
const permissions = computed(() => getters.sourcePermissions());

// Slideshow entry point: images only, and only when the folder has at least two images or videos.
const showSlideshowButton = computed(
  () => previewType.value === "image" && !isTransitioning.value && countMediaInListing(listing.value) >= 2,
);

const previewType = computed(() => {
  if (getters.fileViewingDisabled(state.req.name ?? "")) {
    return "preview";
  }
  return getters.previewType();
});

const pdfConvertable = computed(() => {
  const ext = `.${(state.req.name ?? "").split(".").pop()?.toLowerCase()}`;
  if (state.user.disableViewingExt?.includes(ext)) {
    return false;
  }
  const list: string[] = globalVars.docPreviewExtensions || [];
  return list.includes(ext.slice(1));
});

// Safari: native HEIC in ExtendedImage. Other browsers: converted original when hasPreview.
const isHeicAndViewable = computed(() => state.isSafari || Boolean(state.req?.hasPreview));

const showImage = computed(() => {
  if (state.req.type === "image/heic" || state.req.type === "image/heif") {
    return isHeicAndViewable.value;
  }
  if (isRawImageMimeType(state.req.type)) {
    return true;
  }
  return previewType.value === "image" || pdfConvertable.value;
});

const isPdf = computed(() => isPdfFile(state.req.type) || isPdfFile(state.req.name));
const usePdfPreviewFallback = computed(() => isPdf.value && shouldUsePdfPreviewFallback());

const raw = computed(() => {
  const viewToken = state.req.viewToken;
  const typeHint = state.req.type || state.req.name;
  const isHeicOrHeif = state.req.type === "image/heic" || state.req.type === "image/heif";

  const getRawPreview = isRawImageMimeType(state.req.type);
  // HEIC/HEIF at size=original for non-Safari; Safari full size is resolved in ExtendedImage.
  const getHeicConvertedPreview = isHeicOrHeif && !state.isSafari && state.req?.hasPreview;
  if (pdfConvertable.value || getRawPreview || getHeicConvertedPreview) {
    if (getters.isShare()) {
      const previewPath = removeTrailingSlash(state.req.path);
      return resourcesApi.getPreviewURLPublic(previewPath, "original", state.req.viewToken);
    }
    return `${resourcesApi.getPreviewURL(state.req.source, state.req.path, state.req.modified)}&size=original`;
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
});

const downloadUrl = computed(() => {
  if (getters.isShare()) {
    return resourcesApi.getDownloadURLPublic(
      {
        path: state.shareInfo.subPath,
        hash: state.shareInfo.hash,
      },
      [state.req.path ?? ""],
    );
  }
  return resourcesApi.getDownloadURL(state.req.source, state.req.path);
});

function openSlideshowOptions() {
  mutations.showPrompt({
    name: "SlideshowOptions",
    props: { startName: state.req.name },
  });
}

async function loadPreviewForReq() {
  if (!getters.isLoggedIn() && !getters.isShare()) {
    return;
  }
  const path = state.req.path;
  await loadListingContext();

  if (state.req.path !== path) {
    return;
  }
  await updatePreview();
  selectCurrentItem();
}

watch(
  () => state.req,
  async () => {
    await loadPreviewForReq();
  },
  { immediate: true },
);
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
