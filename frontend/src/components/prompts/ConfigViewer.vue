<template>
  <div class="card-content no-buttons prompt-panel">
    <div class="settings-items">
      <ToggleSwitch
        class="item"
        v-model="showFull"
        @update:modelValue="fetchConfig"
        :name="$t('settings.configViewerShowFull')"
      />
      <ToggleSwitch
        class="item"
        v-model="showComments"
        @update:modelValue="fetchConfig"
        :name="$t('settings.configViewerShowComments')"
      />
    </div>

    <div class="editor-container">
      <Editor
        :viewer-mode="true"
        :content="configContent"
        :editor-mode="'yaml'"
        :read-only="true"
      />
    </div>
  </div>
</template>

<script>
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import * as settingsApi from "@/api/settings";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "ConfigViewer",
  components: {
    ToggleSwitch,
    Editor: createAsyncComponent(() => import('@/views/files/Editor.vue')),
  },
  data() {
    return {
      showFull: false,
      showComments: false,
      configContent: "",
      latestConfigRequestId: 0,
    };
  },
  mounted() {
    void this.fetchConfig();
  },
  methods: {
    async fetchConfig() {
      const requestId = ++this.latestConfigRequestId;
      try {
        const response = await settingsApi.config(this.showFull, this.showComments);
        const text = await response.text();
        if (requestId !== this.latestConfigRequestId) {
          return;
        }
        this.configContent = text;
      } catch (e) {
        if (requestId !== this.latestConfigRequestId) {
          return;
        }
        console.error(e);
        const errorMessage =
          e && typeof e === "object" && "message" in e ? String(e.message) : "Unknown error";
        this.configContent = `Error loading config: ${errorMessage}`;
      }
    },
  },
};
</script>

<style scoped>
.prompt-panel {
  display: flex;
  flex-direction: column;
  gap: 0.75em;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

.prompt-controls {
  flex-shrink: 0;
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
