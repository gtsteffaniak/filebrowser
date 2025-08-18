<template>
  <div class="viewer-background">
    <div v-if="loading" class="status-text">{{ $t('files.loading') }}</div>
    <div v-else-if="error" class="status-text error">{{ error }}</div>
    <div v-else class="docx-page" v-html="docxHtml"></div>
  </div>
</template>

<script>
import { defineComponent } from "vue";
import * as mammoth from "mammoth";
import { filesApi, publicApi } from "@/api";
import { state, mutations, getters } from "@/store";

export default defineComponent({
  name: "DocxViewer",
  data() {
    return {
      docxHtml: "",
      loading: false,
      error: "",
    };
  },
  watch: {
    "state.req.path": {
      handler(newPath, oldPath) {
        // Added a console log to the watcher to see when it runs
        console.log(
          `[DEBUG] Watcher triggered: Path changed from "${oldPath}" to "${newPath}"`
        );
        this.loadFile();
      },
      immediate: true,
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
  methods: {
    async loadFile() {
      try {
        const filename = state.req.name;
        // Check if the filename is valid and ends with .docx
        if (!filename || !filename.toLowerCase().endsWith(".docx")) {
          this.error = `This viewer only supports .docx files. Current file: "${
            filename || "Not available"
          }"`;
          console.error(`[3a] Filename check FAILED. Stopping execution.`);
          this.docxHtml = ""; // Ensure view is cleared
          return;
        }
        console.log("[3a] Filename check PASSED.");

        this.loading = true;
        this.error = "";
        this.docxHtml = "";

        console.log("[4] Getting download URL from API...");
        const downloadUrl = getters.isShare()
          ? publicApi.getDownloadURL({
              path: state.share.subPath,
              hash: state.share.hash,
              token: state.share.token,
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
        console.log("[5] Got download URL:", downloadUrl);

        console.log("[6] Fetching data from URL...");
        const response = await fetch(downloadUrl);
        console.log("[7] Fetch response received. Status:", response.status);

        if (!response.ok) {
          throw new Error(`Failed to download file (Status: ${response.status})`);
        }

        console.log("[8] Getting ArrayBuffer from response...");
        const arrayBuffer = await response.arrayBuffer();
        console.log(`[8a] Got ArrayBuffer. Size: ${arrayBuffer.byteLength} bytes.`);

        if (arrayBuffer.byteLength === 0) {
          throw new Error("Downloaded file is empty (0 bytes).");
        }

        console.log("[9] Passing ArrayBuffer to mammoth.js for conversion...");
        const result = await mammoth.convertToHtml({ arrayBuffer });
        this.docxHtml = result.value;
        console.log(
          "%c[10] SUCCESS: Document rendered.",
          "font-weight: bold; color: green;"
        );
      } catch (e) {
        console.error("%c[X] CATCH BLOCK ERROR:", "font-weight: bold; color: red;", e);
        this.error = e.message || "An unknown error occurred.";
      } finally {
        this.loading = false;
        console.log("[F] FINALLY block executed.");
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
  color:black;
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
