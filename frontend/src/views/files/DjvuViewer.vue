<template>
  <div class="djvu-container">
    <div v-if="!isReady" class="loading-indicator">
      <p>{{ $t("general.loading", { suffix: "..." }) }}</p>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent, markRaw } from "vue";
import { state, mutations, getters } from "@/store";
import { resourcesApi } from "@/api";
import { removeLastDir } from "@/utils/url";

declare const DjVu: any;

function loadScript(src: string): Promise<HTMLScriptElement> {
  return new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = src;
    script.onload = () => resolve(script);
    script.onerror = () => reject(new Error(`Failed to load script: ${src}`));
    document.head.appendChild(script);
  });
}

export default defineComponent({
  name: "djvuViewer",
  data() {
    return {
      isReady: false,
      viewer: null as any,
      viewerEl: null as HTMLDivElement | null,
    };
  },
  async mounted() {
    mutations.resetSelected();
    mutations.addSelected({
      name: state.req.name,
      path: state.req.path,
      size: state.req.size,
      type: state.req.type,
      source: state.req.source,
    });
    try {
      // 1. Load the vendored DjVu.js scripts once (they declare global
      //    const/let bindings that cannot be re-declared)
      if (typeof DjVu === "undefined" || !DjVu.Viewer) {
        const baseURL = (window as any).globalVars?.baseURL || "/";
        const staticBase = baseURL + "public/static/js";

        await loadScript(`${staticBase}/djvu.js`);
        await loadScript(`${staticBase}/djvu_viewer.js`);
      }

      // 2. Get the download URL for the DjVu file
      const djvuUrl = getters.isShare()
        ? resourcesApi.getDownloadURLPublic(
            {
              path: state.shareInfo.subPath,
              hash: state.shareInfo.hash,
              token: state.shareInfo.token,
            },
            [state.req.path]
          )
        : resourcesApi.getDownloadURL(
            state.req.source,
            state.req.path,
            false,
            false
          );

      const viewerEl = document.createElement("div");
      viewerEl.id = "djvu-viewer-overlay";
      document.body.appendChild(viewerEl);
      this.viewerEl = viewerEl;

      // markRaw prevents Vue from wrapping the viewer in a reactive
      // Proxy, which would interfere with React's internal state.
      this.viewer = markRaw(new DjVu.Viewer());
      this.viewer.render(viewerEl);
      await this.viewer.loadDocumentByUrl(djvuUrl);

      this.isReady = true;
    } catch (error) {
      this.onLoadComponentError(error);
    }
  },
  beforeUnmount() {
    if (this.viewer) {
      try {
        this.viewer.destroy();
      } catch (e) {
        console.warn("DjVu viewer cleanup:", e);
      }
    }
    if (this.viewerEl) {
      this.viewerEl.remove();
    }
  },
  methods: {
    close() {
      const current = window.location.pathname;
      const newPath = removeLastDir(current);
      window.location.href = newPath + "#" + state.req.name;
    },
    onLoadComponentError(error: any) {
      console.error("Error loading DjVu file:", error);
    },
  },
});
</script>

<style>
#djvu-viewer-overlay {
  position: fixed;
  top: 3.8em;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 999;
  background-color: var(--surfaceSecondary);
}
</style>

<style scoped>
.djvu-container {
  width: 100%;
  height: 100%;
}

.loading-indicator {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100%;
  font-size: 1.2em;
  color: #6c757d;
}
</style>
