<template>
  <div id="editor-container">
    <div id="editor"></div>
  </div>
</template>

<script>
import { eventBus } from "@/store/eventBus";
import { state, getters } from "@/store";
import { filesApi } from "@/api";
import { notify } from "@/notify";
import ace, { version as ace_version } from "ace-builds";
import modelist from "ace-builds/src-noconflict/ext-modelist";
import "ace-builds/src-min-noconflict/theme-chrome";
import "ace-builds/src-min-noconflict/theme-twilight";
import "ace-builds/src-min-noconflict/mode-yaml";
import "ace-builds/src-min-noconflict/mode-json";

export default {
  name: "editor",
  props: {
    viewerMode: {
      type: Boolean,
      default: false
    },
    content: {
      type: String,
      default: ""
    },
    editorMode: {
      type: String,
      default: "yaml" // Default to YAML for config viewing
    },
    readOnly: {
      type: Boolean,
      default: null // null means auto-determine
    }
  },
  data: function () {
    return {
      editor: null, // The editor instance
      filename: "",
    };
  },
  computed: {
    isDarkMode() {
      return getters.isDarkMode();
    },
  },
  watch: {
    // Update editor content when prop changes in viewer mode
    content(newContent) {
      if (this.viewerMode && this.editor) {
        this.editor.setValue(newContent || "", -1); // -1 moves cursor to start
      }
    },
    // Update theme when dark mode changes
    isDarkMode(newValue) {
      if (this.editor) {
        this.editor.setTheme(newValue ? "ace/theme/twilight" : "ace/theme/chrome");
      }
    }
  },
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
    this.setupEditor();
  },
  methods: {
    setupEditor(attempt = 1) {
      try {
        // Handle viewer mode - bypass all req/route logic
        if (this.viewerMode) {
          return this.setupViewerMode();
        }

        // Original file editor logic
        this.filename = decodeURIComponent(this.$route.path.split("/").pop() || "");
        // Safety Check 1: Use the component's 'filename' data property for comparison

        // no need to do safety check for direct link
        if (getters.shareHash() == this.filename) {
          this.filename = "";
        }

        if (this.filename && state.req.name !== this.filename) {
          if (attempt < 5) {
            console.warn(
              `[Attempt ${attempt}/5] State filename ("${state.req.name}") does not match route filename ("${this.filename}"). Retrying in 500ms...`
            );
            setTimeout(() => this.setupEditor(attempt + 1), 500);
          } else {
            const errorMsg = `${this.$t("editor.syncFailed", { filename: this.filename })}`;
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

        const fileContent = state.req.content == "empty-file-x6OlSil" ? "" : state.req.content || "";
        this.editor = ace.edit(editorEl, {
          mode: modelist.getModeForPath(state.req.name).mode,
          value: fileContent,
          showPrintMargin: false,
          showGutter: true,
          showLineNumbers: true,
          theme: this.isDarkMode ? "ace/theme/twilight" : "ace/theme/chrome",
          readOnly: this.readOnly !== null ? this.readOnly : (state.req.type === "textImmutable"),
          wrap: false,
          enableMobileMenu: true,
        });

        this.editor.container.addEventListener("contextmenu", (event) => {
          event.preventDefault();
          event.stopPropagation();
        }, true);

        this.filename = decodeURIComponent(this.$route.path.split("/").pop());
      } catch (error) {
        notify.showError(this.$t("editor.uninitialized"));
      }
    },
    setupViewerMode() {
      const editorEl = document.getElementById("editor");
      if (!editorEl) {
        return;
      }

      ace.config.set(
        "basePath",
        `https://cdn.jsdelivr.net/npm/ace-builds@${ace_version}/src-min-noconflict/`
      );

      // Determine the ACE mode from the editorMode prop
      const aceMode = this.getAceMode(this.editorMode);

      this.editor = ace.edit(editorEl, {
        mode: aceMode,
        value: this.content || "",
        showPrintMargin: false,
        showGutter: true,
        showLineNumbers: true,
        theme: this.isDarkMode ? "ace/theme/twilight" : "ace/theme/chrome",
        readOnly: this.readOnly !== null ? this.readOnly : true, // Default to read-only in viewer mode
        wrap: false,
        enableMobileMenu: false, // Disable mobile menu in viewer mode
      });

      // Disable context menu in viewer mode
      this.editor.container.addEventListener("contextmenu", (event) => {
        event.preventDefault();
        event.stopPropagation();
      }, true);
    },
    getAceMode(mode) {
      const modeMap = {
        'yaml': 'ace/mode/yaml',
        'json': 'ace/mode/json',
        'javascript': 'ace/mode/javascript',
        'typescript': 'ace/mode/typescript',
        'html': 'ace/mode/html',
        'css': 'ace/mode/css',
        'markdown': 'ace/mode/markdown',
        'text': 'ace/mode/text',
        'xml': 'ace/mode/xml'
      };
      return modeMap[mode] || `ace/mode/${mode}`;
    },
    async handleEditorValueRequest() {
      // Skip save logic in viewer mode
      if (this.viewerMode) {
        return;
      }

      // Safety Check 2: Final verification before saving
      if (state.req.name !== this.filename) {
        // Corrected the error message to be more accurate
        notify.showError(this.$t("editor.saveAbortedMessage", { activeFile: state.req.name, tryingToSave: this.filename }));
        return;
      }
      try {
        if (this.editor) {
          if (getters.isShare()) {
            // TODO: add support for saving shared files
            notify.showError(this.$t("share.saveDisabled"));
            return;
          } else {
            // Use regular files API for authenticated users
            await filesApi.put(state.req.source, state.req.path, this.editor.getValue());
          }
        } else {
          notify.showError(this.$t("editor.uninitialized"));
          return;
        }
        notify.showSuccess(`${this.filename} saved successfully.`);
      } catch (error) {
        notify.showError(this.$t("editor.saveFailed"));
      }
    },
    keyEvent(event) {
      const { key, ctrlKey, metaKey } = event;
      if (getters.currentPromptName()) return;
      
      // Skip save shortcut in viewer mode
      if (this.viewerMode) return;
      
      if ((ctrlKey || metaKey) && key.toLowerCase() === "s") {
        event.preventDefault();
        this.handleEditorValueRequest();
      }
    },
  },
};
</script>

<style>
.ace_editor {
    font-size: 14px;
    line-height: 1.3;
}
/* Mobile menu */
.ace_mobile-menu {
    font-size: 16px !important;
    border-radius: 12px !important;
    padding: 10px !important;
    box-shadow: 0 8px 25px rgba(0, 0, 0, 0.4) !important;
}
.ace_mobile-menu .ace_menu-item {
    font-size: 16px !important;
    margin: 8px 0 !important;
    border-radius: 8px !important;
    text-align: center !important;
}
.ace_mobile-menu .ace_menu-item {
    display: flex !important;
    align-items: center !important;
    justify-content: center !important;
}
</style>