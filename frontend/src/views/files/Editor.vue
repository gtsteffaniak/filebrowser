<template>
  <div id="editor-container">
    <div id="editor"></div>
  </div>
</template>

<script>
import { eventBus } from "@/store/eventBus";
import { state, getters } from "@/store";
import { filesApi } from "@/api";
import ace from "ace-builds/src-min-noconflict/ace.js";
import "ace-builds/src-min-noconflict/theme-chrome";
import "ace-builds/src-min-noconflict/theme-twilight";

export default {
  name: "editor",
  data: function () {
    return {};
  },
  computed: {
    isDarkMode() {
      return getters.isDarkMode();
    },
  },
  created() {
    window.addEventListener("keydown", this.keyEvent);
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
    this.editor.destroy();
  },
  mounted: function () {
    // this is empty content string "empty-file-x6OlSil" which is used to represent empty text file
    const fileContent =
      state.req.content == "empty-file-x6OlSil" ? "" : state.req.content || "";
    this.editor = ace.edit("editor", {
      value: fileContent,
      showPrintMargin: false,
      theme: "ace/theme/chrome",
      readOnly: state.req.type === "textImmutable",
      wrap: false,
    });
    // Set the basePath for Ace Editor
    ace.config.set("basePath", "/node_modules/ace-builds/src-min-noconflict");
    if (this.isDarkMode) {
      this.editor.setTheme("ace/theme/twilight");
    }
    eventBus.on("handleEditorValueRequest", this.handleEditorValueRequest);
  },
  methods: {
    handleEditorValueRequest() {
      filesApi.put(state.req.path, state.req.source, this.editor.getValue());
    },
    back() {
      this.$router.push(-1);
    },
    keyEvent(event) {
      const { key, ctrlKey, metaKey } = event;
      if (getters.currentPromptName() != null) {
        return;
      }
      if (!ctrlKey && !metaKey) {
        return;
      }
      switch (key.toLowerCase()) {
        case "s":
          event.preventDefault();
          this.save();
          break;

        default:
          // No action for other keys
          return;
      }
    },
  },
};
</script>
