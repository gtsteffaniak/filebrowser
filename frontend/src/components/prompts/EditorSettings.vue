<template>
  <div class="card-content editor-settings-content settings-items">
    <ToggleSwitch
      class="item"
      :name="$t('profileSettings.wrapEditorContent')"
      :description="$t('profileSettings.wrapEditorContentDescription')"
      v-model="wrapEditorContent"
    />
  </div>
  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--grey"
      @click="resetEditorConfig"
      :aria-label="$t('player.visualizer.resetDefaults')"
      :title="$t('player.visualizer.resetDefaults')"
    > {{ $t("player.visualizer.resetDefaults") }}
    </button>
    <button
      type="button"
      class="button button--flat"
      @click="closeTopPrompt"
      :aria-label="$t('general.ok')"
      :title="$t('general.ok')"
    > {{ $t("general.ok") }}
    </button>
  </div>
</template>

<script lang="ts">
import { mutations } from "@/store";
import { editorConfig, saveEditorConfig, resetEditorConfig } from "@/utils/editorConfig";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "EditorSettings",
  components: { ToggleSwitch },
  computed: {
    wrapEditorContent: {
      get(): boolean { return editorConfig.wrapEditorContent; },
      set(value: boolean): void { saveEditorConfig({ wrapEditorContent: value }); },
    },
  },
  methods: {
    closeTopPrompt(): void {
      mutations.closeTopPrompt();
    },
    resetEditorConfig(): void {
      resetEditorConfig();
    },
  },
};
</script>

<style scoped>
.settings-items .item {
  padding-top: 0.75em;
  padding-bottom: 0.75em;
}

.editor-settings-content {
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 0 0.75em;
}
</style>
