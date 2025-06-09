<template>
  <div id="editor-container">
    <div id="editor"></div>
  </div>
</template>

<script>
import { eventBus } from "@/store/eventBus";
import { state, getters, mutations } from "@/store";
import { filesApi } from "@/api";
// Assuming 'notify' is a utility you have for showing notifications
import { notify } from "@/notify";
import ace, { version as ace_version } from "ace-builds";
import modelist from "ace-builds/src-noconflict/ext-modelist";
import "ace-builds/src-min-noconflict/theme-chrome";
import "ace-builds/src-min-noconflict/theme-twilight";

export default {
  name: "editor",
  data: function () {
    return {
      editor: null, // The editor instance
      // Initialize filename from the route it's created with
      filename: "",
    };
  },
  computed: {
    isDarkMode() {
      return getters.isDarkMode();
    },
  },
  // Use beforeRouteUpdate to react to file changes
  beforeRouteUpdate(to, from, next) {

    // Destroy the old editor instance to ensure a clean state
    if (this.editor) {
      this.editor.destroy();
      this.editor = null;
    }

    // Call setupEditor on the next DOM update cycle
    this.$nextTick(() => {
      this.setupEditor();
    });

    // Continue with the navigation
    next();
  },
  created() {
    window.addEventListener("keydown", this.keyEvent);
    eventBus.on("handleEditorValueRequest", this.handleEditorValueRequest);
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
    if (this.editor) {
      this.editor.destroy();
    }
  },
  mounted: function () {
    // This will run only when the component is first added to the page
    this.setupEditor();
  },
  methods: {
    setupEditor(attempt = 1) {
      this.filename = decodeURIComponent(this.$route.path.split("/").pop())
      // Safety Check 1: Use the component's 'filename' data property for comparison
      if (state.req.name !== this.filename) {
        if (attempt < 5) {
          console.warn(
            `[Attempt ${attempt}/5] State filename ("${state.req.name}") does not match route filename ("${this.filename}"). Retrying in 500ms...`
          );
          setTimeout(() => this.setupEditor(attempt + 1), 500);
        } else {
          const errorMsg = `[FATAL] Failed to sync state with the route for "${this.filename}" after 5 attempts. Aborting editor setup to prevent data corruption.`;
          console.error(errorMsg);
          notify.showError(errorMsg); // Using the custom notifier
        }
        return;
      }

      console.log(
        "State and route are in sync. Proceeding with editor setup for",
        this.filename
      );
      const editorEl = document.getElementById("editor");
      if (!editorEl) {
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
        theme: this.isDarkMode ? "ace/theme/twilight" : "ace/theme/chrome",
        readOnly: state.req.type === "textImmutable",
        wrap: false,
      });
      this.filename = decodeURIComponent(this.$route.path.split("/").pop())
    },
    handleEditorValueRequest() {
      // Safety Check 2: Final verification before saving
      if (state.req.name !== this.filename) {
        // Corrected the error message to be more accurate
        const errorMsg = `CRITICAL: Save operation aborted. The application's active file ("${state.req.name}") does not match the file you are trying to save ("${this.filename}").`;
        notify.showError(errorMsg);
        return;
      }

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
