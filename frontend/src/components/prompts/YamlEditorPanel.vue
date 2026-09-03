<template>
  <div class="yaml-editor-panel">
    <div ref="editorHost" class="yaml-editor-host"></div>
    <p v-if="error" class="yaml-error">{{ error }}</p>
    <div class="yaml-actions">
      <button type="button" class="button button--flat button--grey" @click="$emit('cancel')">
        {{ $t("general.cancel") }}
      </button>
      <button type="button" class="button button--flat button--blue" @click="apply">
        {{ $t("general.save") }}
      </button>
    </div>
  </div>
</template>

<script>
import ace from "ace-builds";
import "ace-builds/src-min-noconflict/mode-yaml";
import "ace-builds/src-min-noconflict/theme-chrome";
import "ace-builds/src-min-noconflict/theme-tomorrow_night_bright";
import { getters } from "@/store";

export default {
  name: "YamlEditorPanel",
  props: {
    modelValue: {
      type: String,
      required: true,
    },
  },
  emits: ["update:modelValue", "apply", "cancel"],
  data() {
    return {
      editor: null,
      error: "",
    };
  },
  mounted() {
    this.editor = ace.edit(this.$refs.editorHost);
    this.editor.session.setMode("ace/mode/yaml");
    this.editor.setTheme(getters.isDarkMode() ? "ace/theme/tomorrow_night_bright" : "ace/theme/chrome");
    this.editor.setValue(this.modelValue || "", -1);
    this.editor.setOptions({
      fontSize: "14px",
      showPrintMargin: false,
      wrap: true,
    });
  },
  beforeUnmount() {
    this.editor?.destroy();
  },
  methods: {
    apply() {
      this.error = "";
      const text = this.editor?.getValue() ?? "";
      this.$emit("update:modelValue", text);
      this.$emit("apply", text);
    },
  },
};
</script>

<style scoped>
.yaml-editor-host {
  min-height: 280px;
  width: 100%;
  border: 1px solid var(--borderColor);
  border-radius: 0.5em;
}

.yaml-error {
  color: var(--red);
  margin: 0.5em 0 0;
}

.yaml-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.5em;
  margin-top: 0.75em;
}
</style>
