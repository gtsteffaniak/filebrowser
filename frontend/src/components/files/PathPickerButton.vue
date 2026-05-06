<template>
  <div
    role="button"
    tabindex="0"
    :aria-label="resolvedAriaLabel"
    class="searchContext clickable button unified-path-picker"
    @click="openPicker"
    @keydown.enter.prevent="openPicker"
    @keydown.space.prevent="openPicker"
  >
    {{ buttonLabel }}
  </div>
</template>

<script>
import { mutations, state } from "@/store";
import { eventBus } from "@/store/eventBus";

function normalizeContextId() {
  return `path-picker-btn-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
}

export default {
  name: "PathPickerButton",

  props: {
    path: {
      type: String,
      default: "/",
    },
    source: {
      type: String,
      default: "",
    },
    ariaLabel: {
      type: String,
      default: undefined,
    },
    /** i18n or plain string when nothing chosen yet. */
    placeholder: {
      type: String,
      default: undefined,
    },
    showFiles: {
      type: Boolean,
      default: false,
    },
    showFolders: {
      type: Boolean,
      default: true,
    },
    hideDestinationSource: {
      type: Boolean,
      default: false,
    },
    requireFileSelection: {
      type: Boolean,
      default: false,
    },
    allowedFileTypes: {
      type: Array,
      default: null,
    },
    listTitle: {
      type: String,
      default: null,
    },
  },

  emits: ["update:path", "update:source", "navigate"],

  data() {
    return {
      pendingContextId: null,
    };
  },

  computed: {
    resolvedAriaLabel() {
      if (this.ariaLabel) {
        return this.ariaLabel;
      }
      return this.buttonLabel;
    },
    buttonLabel() {
      const hasSource = typeof this.source === "string" && this.source.length > 0;
      const p =
        typeof this.path === "string" && this.path.length > 0 ? this.path : "/";
      if (!hasSource) {
        return (
          this.placeholder !== undefined && this.placeholder !== null && this.placeholder !== ""
            ? this.placeholder
            : this.$t("tools.fileWatcher.chooseFile")
        );
      }
      return `${p} (${this.source})`;
    },
  },

  mounted() {
    eventBus.on("pathSelected", this.onPathSelected);
    eventBus.on("pathPickerCancelled", this.onPathPickerCancelled);
  },

  beforeUnmount() {
    eventBus.off("pathSelected", this.onPathSelected);
    eventBus.off("pathPickerCancelled", this.onPathPickerCancelled);
  },

  methods: {
    openPicker() {
      const currentPath =
        typeof this.path === "string" && this.path.length > 0 ? this.path : "/";
      const currentSource =
        (typeof this.source === "string" && this.source.length > 0
          ? this.source
          : state.sources.current) || Object.keys(state.sources.info || {})[0] || "";
      this.pendingContextId = normalizeContextId();
      mutations.showPrompt({
        name: "pathPicker",
        props: {
          currentPath,
          currentSource,
          showFiles: this.showFiles,
          showFolders: this.showFolders,
          hideDestinationSource: this.hideDestinationSource,
          requireFileSelection: this.requireFileSelection,
          allowedFileTypes: this.allowedFileTypes,
          listTitle: this.listTitle,
          selectionContextId: this.pendingContextId,
        },
      });
    },

    onPathSelected(data) {
      if (!this.pendingContextId || !data) {
        return;
      }
      if (data.selectionContextId !== this.pendingContextId) {
        return;
      }
      this.pendingContextId = null;
      if (typeof data.path === "string") {
        this.$emit("update:path", data.path);
      }
      if (typeof data.source === "string") {
        this.$emit("update:source", data.source);
      }
      this.$emit("navigate");
    },

    onPathPickerCancelled(data) {
      if (!this.pendingContextId || !data) {
        return;
      }
      if (data.selectionContextId !== this.pendingContextId) {
        return;
      }
      this.pendingContextId = null;
    },
  },
};
</script>

<style scoped>
.unified-path-picker {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  max-width: 100%;
  min-width: 0;
  word-break: break-word;
  box-sizing: border-box;
}
</style>
