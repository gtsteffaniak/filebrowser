<template>
  <div id="editor-container">
    <div id="editor"></div>
  </div>
</template>

<script>
import { eventBus } from "@/store/eventBus";
import { state,mutations } from "@/store";
import { files as api } from "@/api";
import url from "@/utils/url";
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
      let parts = this.$route.path.split("/");

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
    const fileContent = state.req.content || "";
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
    eventBus.$on("handleEditorValueRequest", this.handleEditorValueRequest);
  },
  methods: {
    handleEditorValueRequest() {
      try {
        api.put(this.$route.path, this.editor.getValue());
      } catch (e) {
        this.$showError(e);
      }
    },
    back() {
      let uri = url.removeLastDir(this.$route.path) + "/";
      this.$router.push({ path: uri });
    },
    keyEvent(event) {
      if (!event.ctrlKey && !event.metaKey) {
        return;
      }

      if (String.fromCharCode(event.which).toLowerCase() !== "s") {
        return;
      }
      event.preventDefault();
      this.save();
    },
    close() {
      mutations.updateRequest({});
      let uri = url.removeLastDir(this.$route.path) + "/";
      this.$router.push({ path: uri });
    },
  },
};
</script>
