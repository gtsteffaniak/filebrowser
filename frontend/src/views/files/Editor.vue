<template>
  <div id="editor-container">
    <div id="editor"></div>
  </div>
</template>

<script>
import { eventBus } from "@/store/eventBus";
import { state, getters } from "@/store";
import { filesApi } from "@/api";
import ace, { version as ace_version } from "ace-builds";
import modelist from "ace-builds/src-noconflict/ext-modelist";
import "ace-builds/src-min-noconflict/theme-chrome";
import "ace-builds/src-min-noconflict/theme-twilight";

export default {
  name: "editor",
  data: function () {
    return {
      editor: null, // The editor instance
    };
  },
  computed: {
    isDarkMode() {
      return getters.isDarkMode();
    },
  },
  watch: {
    $route() {
      if (this.editor) {
        this.editor.destroy();
        this.editor = null;
      }
      // Wait for the DOM to update after the route change
      this.$nextTick(() => {
        this.setupEditor();
      });
    },
  },
  created() {
    window.addEventListener("keydown", this.keyEvent);
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
    if (this.editor) {
      this.editor.destroy();
    }
    eventBus.off("handleEditorValueRequest", this.handleEditorValueRequest);
  },
  mounted: function () {
    // Wait for the initial DOM render to complete
    this.$nextTick(() => {
      this.setupEditor();
    });
    eventBus.on("handleEditorValueRequest", this.handleEditorValueRequest);
  },
  methods: {
    setupEditor() {
      const editorEl = document.getElementById("editor");
      if (!editorEl) {
        console.warn(
          "Editor component mounted, but #editor div was not found in the DOM. Aborting setup."
        );
        return;
      }

      ace.config.set(
        "basePath",
        `https://cdn.jsdelivr.net/npm/ace-builds@${ace_version}/src-min-noconflict/`
      );

      const fileContent =
        state.req.content == "empty-file-x6OlSil" ? "" : state.req.content || "";

      this.editor = ace.edit(editorEl, {
        mode: modelist.getModeForPath(state.req.name).mode,
        value: fileContent,
        showPrintMargin: false,
        theme: "ace/theme/chrome",
        readOnly: state.req.type === "textImmutable",
        wrap: false,
      });

      if (this.isDarkMode) {
        this.editor.setTheme("ace/theme/twilight");
      }
    },
    handleEditorValueRequest() {
      if (this.editor) {
        filesApi.put(state.req.path, state.req.source, this.editor.getValue());
      }
    },
    keyEvent(event) {
      const { key, ctrlKey, metaKey } = event;
      if (getters.currentPromptName() != null) return;
      if (!ctrlKey && !metaKey) return;
      if (key.toLowerCase() === "s") {
        event.preventDefault();
        this.handleEditorValueRequest();
      }
    },
  },
};
</script>
