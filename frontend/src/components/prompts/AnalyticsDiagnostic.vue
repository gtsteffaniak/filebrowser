<template>
  <div class="card-content no-buttons prompt-panel">
    <div class="editor-container">
      <Editor
        :viewer-mode="true"
        :content="previewContent"
        :editor-mode="'json'"
        :read-only="true"
      />
    </div>
  </div>
</template>

<script>
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import { notify } from "@/notify";
import * as settingsApi from "@/api/settings";

export default {
  name: "AnalyticsDiagnostic",
  components: {
    Editor: createAsyncComponent(() => import('@/views/files/Editor.vue')),
  },
  data() {
    return {
      previewContent: "",
    };
  },
  mounted() {
    void this.loadPreview();
  },
  methods: {
    async loadPreview() {
      try {
        const preview = await settingsApi.getAnalyticsPreview();
        this.previewContent = JSON.stringify(preview, null, 2);
      } catch (e) {
        console.error(e);
        notify.showErrorToast(this.$t("settings.analyticsPreviewFailed"));
        this.previewContent = "";
      }
    },
  },
};
</script>

<style scoped>
.prompt-panel {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

.editor-container {
  position: relative;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--divider, #ccc);
  border-radius: 4px;
}
</style>
