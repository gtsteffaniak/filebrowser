<template>
  <div class="viewer-background">
    <div v-if="loading" class="status-text">{{ $t('general.loading', { suffix: "..." }) }}</div>
    <div v-else-if="error" class="status-text error">{{ error }}</div>
    <div v-else class="docx-page" v-html="docxHtml"></div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { resourcesApi } from "@/api";
import { ensureViewToken, refreshViewToken, requestViewIdentity } from "@/api/viewToken.js";
import { state, mutations, getters } from "@/store";
import { sanitizeDocxHtml } from "@/utils/docxPreview";
import { removeLastDir } from "@/utils/url.js";

defineOptions({ name: "docViewer" });

const docxHtml = ref("");
const loading = ref(false);
const error = ref("");

let navigationUpdateTimeout: ReturnType<typeof setTimeout> | null = null;
let lastNavigationUpdatePath: string | null = null;

async function updateNavigationForCurrentItem() {
  if (!state.req || state.req.type === 'directory') {
    return;
  }
  let directoryPath = removeLastDir(state.req.path);

  // If directoryPath is empty, the file is in root - use '/' as the directory
  if (!directoryPath || directoryPath === '') {
    directoryPath = '/';
  }

  let listing;

  // Try to get listing from current request first
  if (state.req.items) {
    listing = state.req.items;
  } else if (state.req.parentDirItems) {
    // Use pre-fetched parent directory items from Files.vue
    listing = state.req.parentDirItems;
  } else if (directoryPath !== state.req.path) {
    // Fetch directory listing (now with '/' for root files)
    try {
      let res;
      if (getters.isShare()) {
        res = await resourcesApi.fetchFilesPublic(directoryPath, state.shareInfo.hash);
      } else {
        res = await resourcesApi.fetchFiles(state.req.source, directoryPath);
      }
      listing = res.items;
    } catch (err) {
      console.error("error DocViewer.vue", err);
      listing = [state.req]; // Fallback to current item only
    }
  } else {
    listing = [state.req];
  }
  mutations.setupNavigation({
    listing: listing,
    currentItem: state.req,
    directoryPath: directoryPath
  });
}

let loadSeq = 0;

async function loadFile() {
  const seq = ++loadSeq;
  try {
    const filename = state.req.name;
    // Check if the filename is valid and ends with .docx
    if (!filename?.toLowerCase().endsWith(".docx")) {
      error.value = `This viewer only supports .docx files. Current file: "${
        filename || "Not available"
      }"`;
      docxHtml.value = ""; // Ensure view is cleared
      return;
    }
    loading.value = true;
    error.value = "";
    docxHtml.value = "";

    const viewIdentity = requestViewIdentity(state.req);
    let viewToken: string | undefined = state.req.viewToken;
    try {
      viewToken = await ensureViewToken(state.req.source);
      if (requestViewIdentity(state.req) === viewIdentity) {
        mutations.setRequestViewToken(viewToken);
      }
    } catch (err) {
      console.warn("Failed to refresh view token for DOCX preview:", err);
    }

    const downloadUrl = getters.isShare()
      ? resourcesApi.getViewURL(
          state.req.source,
          state.req.path,
          viewToken,
          {
            path: state.shareInfo.subPath,
            hash: state.shareInfo.hash,
          },
          false,
          state.req.type || state.req.name,
        )
      : resourcesApi.getViewURL(
          state.req.source,
          state.req.path,
          viewToken,
          null,
          false,
          state.req.type || state.req.name,
        );

    if (!downloadUrl) {
      throw new Error("Could not retrieve a valid download URL from the API.");
    }

    let response = await fetch(downloadUrl);

    if (response.status === 403 && state.req?.source) {
      try {
        const refreshed = await refreshViewToken(state.req.source, viewToken);
        viewToken = refreshed.viewToken;
        if (requestViewIdentity(state.req) === viewIdentity) {
          mutations.setRequestViewToken(viewToken);
        }
        const retryUrl = getters.isShare()
          ? resourcesApi.getViewURL(
              state.req.source,
              state.req.path,
              viewToken,
              {
                path: state.shareInfo.subPath,
                hash: state.shareInfo.hash,
              },
              false,
              state.req.type || state.req.name,
            )
          : resourcesApi.getViewURL(
              state.req.source,
              state.req.path,
              viewToken,
              null,
              false,
              state.req.type || state.req.name,
            );
        if (retryUrl) {
          response = await fetch(retryUrl);
        }
      } catch (refreshErr) {
        console.warn("Failed to refresh view token after 403:", refreshErr);
      }
    }
    if (!response.ok) {
      throw new Error(`Failed to download file (Status: ${response.status})`);
    }

    const arrayBuffer = await response.arrayBuffer();

    if (arrayBuffer.byteLength === 0) {
      throw new Error("Downloaded file is empty (0 bytes).");
    }

    const mammothModule = await import("mammoth");
    const mammoth = mammothModule.default ?? mammothModule;
    const { convertToHtml } = mammoth;
    const result = await convertToHtml({ arrayBuffer });
    if (seq !== loadSeq) return;
    docxHtml.value = sanitizeDocxHtml(result.value);
  } catch (e) {
    if (seq !== loadSeq) return;
    error.value = (e as Error).message || "An unknown error occurred.";
  } finally {
    if (seq === loadSeq) loading.value = false;
  }
}

watch(() => state.req.path, () => { void loadFile() }, { immediate: true });

watch(() => state.req, (newReq) => {
  if (newReq?.path && newReq.name) {
    // Prevent duplicate navigation updates for the same path
    if (lastNavigationUpdatePath === newReq.path) {
      return;
    }
    // Clear any pending navigation update
    if (navigationUpdateTimeout) {
      clearTimeout(navigationUpdateTimeout);
    }
    lastNavigationUpdatePath = newReq.path;

    // Debounce navigation updates to prevent rapid firing
    navigationUpdateTimeout = setTimeout(() => {
      void updateNavigationForCurrentItem();
      navigationUpdateTimeout = null;
    }, 50);
    mutations.resetSelected();
    mutations.addSelected({
      name: newReq.name,
      path: newReq.path,
      size: newReq.size,
      type: newReq.type,
      source: newReq.source,
      modified: newReq.modified,
      hasPreview: newReq.hasPreview,
    });
  }
}, { immediate: true });

onMounted(() => {
  mutations.resetSelected();
  mutations.addSelected({
    name: state.req.name,
    path: state.req.path,
    size: state.req.size,
    type: state.req.type,
    source: state.req.source,
    modified: state.req.modified,
    hasPreview: state.req.hasPreview,
  });
});

onBeforeUnmount(() => {
  // Clean up any pending navigation update
  if (navigationUpdateTimeout) {
    clearTimeout(navigationUpdateTimeout);
    navigationUpdateTimeout = null;
  }
});
</script>

<style scoped>
.viewer-background {
  width: 100%;
  height: 100%;
  overflow: hidden auto;
  background-color: #f0f2f5;
  padding: 2em;
  box-sizing: border-box;
}

.docx-page {
  background: white;
  width: min(8.5in, 100%);
  min-height: 11in;
  margin: 0 auto;
  padding: clamp(1em, 8vw, 1in);
  box-shadow: 0 0 10px rgb(0 0 0 / 15%);
  box-sizing: border-box;
  color: black;
  overflow-wrap: anywhere;
}

.docx-page :deep(img),
.docx-page :deep(svg),
.docx-page :deep(video) {
  max-width: 100%;
  height: auto;
}

.docx-page :deep(table) {
  display: block;
  max-width: 100%;
  overflow-x: auto;
  border-collapse: collapse;
  margin: 1em 0;
}

.docx-page :deep(td),
.docx-page :deep(th) {
  border: 1px solid black;
  padding: 0.4em 0.7em;
  vertical-align: top;
  text-align: left;
}

.docx-page :deep(th) {
  background-color: #f3f4f6;
  font-weight: 600;
}

.docx-page :deep(td > p),
.docx-page :deep(th > p) {
  margin: 0;
}

.docx-page :deep(pre) {
  max-width: 100%;
  overflow-x: auto;
}

.status-text {
  text-align: center;
  padding: 3em;
  font-family: sans-serif;
  color: #333;
  font-size: 1.2em;
}

.status-text.error {
  color: var(--red);
}

@media (width <= 8.5in) {
  .viewer-background {
    padding: 0;
  }
  .docx-page {
    min-height: 100%;
    box-shadow: none;
  }
}
</style>
