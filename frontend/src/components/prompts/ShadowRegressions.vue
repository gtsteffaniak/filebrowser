<template>
  <div class="card-content no-buttons prompt-panel">
    <div class="prompt-controls">
      <button
        type="button"
        class="button button--flat button--grey"
        :disabled="clearing"
        @click="clearAll"
      >
        {{ $t("notifications.clearAll") }}
      </button>
    </div>
    <div class="editor-container">
      <Editor
        :viewer-mode="true"
        :content="regressionsContent"
        editor-mode="json"
        :read-only="true"
      />
    </div>
  </div>
</template>

<script>
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import { notify } from "@/notify/index.ts";
import { getShadowRegressions, clearShadowRegressions } from "@/api/settings.js";

export default {
  name: "ShadowRegressions",
  components: {
    Editor: createAsyncComponent(() => import("@/views/files/Editor.vue")),
  },
  props: {
    promptId: {
      type: [String, Number],
      default: null,
    },
  },
  data() {
    return {
      regressionsContent: "",
      clearing: false,
    };
  },
  mounted() {
    void this.loadRegressions();
  },
  methods: {
    async loadRegressions() {
      try {
        const entries = await getShadowRegressions();
        this.regressionsContent = JSON.stringify(entries, null, 2);
      } catch (e) {
        console.error(e);
        notify.showErrorToast(this.$t("media.shadowRegressionsLoadFailed"));
        this.regressionsContent = "";
      }
    },
    async clearAll() {
      this.clearing = true;
      try {
        await clearShadowRegressions();
        notify.showSuccessToast(this.$t("media.shadowRegressionsCleared"));
        await this.loadRegressions();
      } catch (e) {
        console.error(e);
        notify.showErrorToast(this.$t("media.shadowRegressionsClearFailed"));
      } finally {
        this.clearing = false;
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
