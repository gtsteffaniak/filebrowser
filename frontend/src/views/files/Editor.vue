<template>
  <div id="editor-container">
    <div id="editor"></div>
  </div>
</template>

<script>
import { router } from "@/router";
import { eventBus } from "@/store/eventBus";
import { state, getters } from "@/store";
import { filesApi } from "@/api";
import url from "@/utils/url.js";
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
    breadcrumbs() {
      let parts = state.route.path.split("/");

      if (parts[0] === "") {
        parts.shift();
      }

      if (parts[parts.length - 1] === "") {
        parts.pop();
      }

      let breadcrumbs = [];

      for (let i = 0; i < parts.length; i++) {
        breadcrumbs.push({ name: decodeURIComponent(parts[i]) });
      }

      breadcrumbs.shift();

      if (breadcrumbs.length > 3) {
        while (breadcrumbs.length !== 4) {
          breadcrumbs.shift();
        }

        breadcrumbs[0].name = "...";
      }

      return breadcrumbs;
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
      filesApi.put(state.route.path, this.editor.getValue());
    },
    back() {
      let uri = url.removeLastDir(state.route.path) + "/";
      this.$router.push({ path: uri });
    },
    keyEvent(event) {
      const { key, ctrlKey, metaKey } = event;
      if (getters.currentPromptName() != null) {
        return;
      }
      if (key == "Backspace") {
        // go back
        let currentPath = state.route.path.replace(/\/+$/, "");
        let newPath = currentPath.substring(0, currentPath.lastIndexOf("/"));
        router.push({ path: newPath });
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
