<template>
  <div class="viewer-background">
    <div v-if="loading" class="status-text">{{ $t('general.loading', { suffix: "..." }) }}</div>
    <div v-else-if="error" class="status-text error">{{ error }}</div>
    <div v-else class="docx-page" v-html="docxHtml"></div>
  </div>
</template>

<script>
import { defineComponent } from "vue";
import * as mammoth from "mammoth";
import { filesApi, publicApi } from "@/api";
import { state, mutations, getters } from "@/store";
import { url } from "@/utils";

export default defineComponent({
  name: "DocxViewer",
  data() {
    return {
      docxHtml: "",
      loading: false,
      error: "",
      navigationUpdateTimeout: null,
      lastNavigationUpdatePath: null,
    };
  },
  computed: {
    req() {
      return state.req;
    },
  },
  watch: {
    "state.req.path": {
      handler() {
        this.loadFile();
      },
      immediate: true,
    },
    // Watch for req changes (similar to Preview.vue)
    req: {
      handler(newReq) {
        if (newReq && newReq.path && newReq.name) {
          // Prevent duplicate navigation updates for the same path
          if (this.lastNavigationUpdatePath === newReq.path) {
            return;
          }

          // Clear any pending navigation update
          if (this.navigationUpdateTimeout) {
            clearTimeout(this.navigationUpdateTimeout);
          }

          this.lastNavigationUpdatePath = newReq.path;

          // Debounce navigation updates to prevent rapid firing
          this.navigationUpdateTimeout = setTimeout(() => {
            this.updateNavigationForCurrentItem();
            this.navigationUpdateTimeout = null;
          }, 50);

          // Update selected items immediately
          mutations.resetSelected();
          mutations.addSelected({
            name: newReq.name,
            path: newReq.path,
            size: newReq.size,
            type: newReq.type,
            source: newReq.source,
          });
        }
      },
      immediate: true
    },
  },
  mounted() {
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
    // Clean up any pending navigation update
    if (this.navigationUpdateTimeout) {
      clearTimeout(this.navigationUpdateTimeout);
      this.navigationUpdateTimeout = null;
    }
  },
  methods: {
    async updateNavigationForCurrentItem() {
      if (!state.req || state.req.type === 'directory') {
        return;
      }

      // Use same directory path calculation as Preview.vue
      let directoryPath = url.removeLastDir(state.req.path);
      
      // If directoryPath is empty, the file is in root - use '/' as the directory
      if (!directoryPath || directoryPath === '') {
        directoryPath = '/';
      }
      
      let listing = null;

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
            res = await publicApi.fetchPub(directoryPath, state.shareInfo.hash);
          } else {
            res = await filesApi.fetchFiles(state.req.source, directoryPath);
          }
          listing = res.items;
        } catch (error) {
          console.error("error DocViewer.vue", error);
          listing = [state.req]; // Fallback to current item only
        }
      } else {
        // Shouldn't happen, but fallback to current item
        console.error("No listing found DocViewer.vue");
        listing = [state.req];
      }

      mutations.setupNavigation({
        listing: listing,
        currentItem: state.req,
        directoryPath: directoryPath
      });
    },
    async loadFile() {
      try {
        const filename = state.req.name;
        // Check if the filename is valid and ends with .docx
        if (!filename || !filename.toLowerCase().endsWith(".docx")) {
          this.error = `This viewer only supports .docx files. Current file: "${
            filename || "Not available"
          }"`;
          this.docxHtml = ""; // Ensure view is cleared
          return;
        }

        this.loading = true;
        this.error = "";
        this.docxHtml = "";

        const downloadUrl = getters.isShare()
          ? publicApi.getDownloadURL({
              path: state.shareInfo.subPath,
              hash: state.shareInfo.hash,
              token: state.shareInfo.token,
            }, [state.req.path])
          : filesApi.getDownloadURL(
              state.req.source,
              state.req.path,
              false,
              true
            );

        if (!downloadUrl) {
          throw new Error("Could not retrieve a valid download URL from the API.");
        }

        const response = await fetch(downloadUrl);

        if (!response.ok) {
          throw new Error(`Failed to download file (Status: ${response.status})`);
        }

        const arrayBuffer = await response.arrayBuffer();

        if (arrayBuffer.byteLength === 0) {
          throw new Error("Downloaded file is empty (0 bytes).");
        }

        const result = await mammoth.convertToHtml({ arrayBuffer });
        this.docxHtml = result.value;
      } catch (e) {
        this.error = e.message || "An unknown error occurred.";
      } finally {
        this.loading = false;
      }
    },
  },
});
</script>

<style scoped>
/* Styles remain the same */
.viewer-background {
  width: 100%;
  height: 100%;
  overflow-y: auto;
  background-color: #f0f2f5;
  padding: 2em;
  box-sizing: border-box;
}
.docx-page {
  background: white;
  width: 8.5in;
  min-height: 11in;
  margin: 0 auto;
  padding: 1in;
  box-shadow: 0 0 10px rgba(0, 0, 0, 0.15);
  box-sizing: border-box;
  color: black;
}
.status-text {
  text-align: center;
  padding: 3em;
  font-family: sans-serif;
  color: #333;
  font-size: 1.2em;
}
.status-text.error {
  color: #d9534f;
}
@media (max-width: 8.5in) {
  .viewer-background {
    padding: 0;
  }
  .docx-page {
    width: 100%;
    min-height: 100%;
    margin: 0;
    padding: 1em;
    box-shadow: none;
  }
}
</style>
