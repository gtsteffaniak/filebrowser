<template>
  <div class="yaml-editor-panel" :class="{ 'yaml-editor-panel--fill': fill }">
    <div class="yaml-editor-host">
      <Editor
        ref="editor"
        :viewer-mode="true"
        :content="modelValue"
        :editor-mode="'yaml'"
        :read-only="false"
      />
    </div>
    <p v-if="error" class="yaml-error">{{ error }}</p>
    <div v-if="!fill" class="yaml-actions">
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
import { createAsyncComponent } from "@/utils/asyncComponent.js";

export default {
  name: "YamlEditorPanel",
  components: {
    Editor: createAsyncComponent(() => import('@/views/files/Editor.vue')),
  },
  props: {
    modelValue: {
      type: String,
      required: true,
    },
    fill: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["update:modelValue", "apply", "cancel"],
  data() {
    return {
      error: "",
    };
  },
  methods: {
    getValue() {
      return this.$refs.editor?.getValue() ?? this.modelValue ?? "";
    },
    apply() {
      this.error = "";
      const text = this.getValue();
      this.$emit("update:modelValue", text);
      this.$emit("apply", text);
    },
  },
};
</script>

<style scoped>
.yaml-editor-panel--fill {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 23em;
  overflow: hidden;
}

.yaml-editor-panel--fill .yaml-editor-host {
  flex: 1 1 auto;
  min-height: 0;
}

.yaml-editor-host {
  position: relative;
  width: 100%;
  border: 1px solid var(--borderColor);
  border-radius: 0.5em;
  overflow: hidden;
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
