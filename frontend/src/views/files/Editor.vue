<template>
  <div id="editor-container">
    <div id="editor"></div>
  </div>
</template>

<script>
import { eventBus } from "@/store/eventBus";
import { state, getters, mutations } from "@/store";
import { filesApi, publicApi } from "@/api";
import { url } from "@/utils";
import { notify } from "@/notify";
import ace, { version as ace_version } from "ace-builds";
import modelist from "ace-builds/src-noconflict/ext-modelist";
import "ace-builds/src-min-noconflict/theme-github";
import "ace-builds/src-min-noconflict/theme-tomorrow_night_bright";
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
      isDirty: false,
      originalReq: null,
      saveLocked: false, // Lock saves during req transitions
      currentReqPath: null, // Track current path for transition detection
    };
  },
  computed: {
    isDarkMode() {
      return getters.isDarkMode();
    },
    req() {
      return state.req;
    },
    // Current filename from route
    routeFilename() {
      if (this.viewerMode) return null;
      const filename = decodeURIComponent(this.$route.path.split("/").pop() || "");
      return getters.shareHash() === filename ? "" : filename;
    },
    // Check if state and route are synchronized
    isStateSynced() {
      if (this.viewerMode) return true;
      if (!this.routeFilename || !this.originalReq) return false;
      return this.originalReq.name === this.routeFilename;
    },
    // Editor content to display
    editorContent() {
      if (this.viewerMode) {
        return this.content || "";
      }
      
      if (!this.isStateSynced) {
        return ""; // Show blank content until synced
      }
      
      return this.req.content === "empty-file-x6OlSil" ? "" : (this.req.content || "");
    },
    // Editor mode/language
    editorLanguageMode() {
      if (this.viewerMode) {
        return this.getAceMode(this.editorMode);
      }
      
      if (!this.isStateSynced || !this.req) {
        return "ace/mode/text";
      }
      
      return modelist.getModeForPath(this.req.name).mode;
    },
    // Editor read-only state
    editorReadOnly() {
      if (this.readOnly !== null) {
        return this.readOnly;
      }
      
      if (this.viewerMode) {
        return true;
      }
      
      if (!this.isStateSynced) {
        return true; // Read-only until synced
      }
      
      return this.req.type === "textImmutable";
    },
  },
  watch: {
    // Lock saves during navigation transitions
    'state.navigation.isTransitioning'(isTransitioning) {
      if (isTransitioning && !this.viewerMode) {
        this.saveLocked = true;
      } else if (!isTransitioning && !this.viewerMode) {
        // Unlock after a short delay to ensure req is fully loaded
        setTimeout(() => {
          this.saveLocked = false;
        }, 300);
      }
    },
    // Update originalReq and lock saves when req changes during navigation
    'req'(newReq, oldReq) {
      if (!this.viewerMode && oldReq && newReq && newReq.path !== oldReq.path) {
        // Update originalReq to the new file
        this.originalReq = newReq;
        this.isDirty = false; // Reset dirty flag for new file
        
        // Lock saves temporarily
        this.saveLocked = true;
        this.currentReqPath = newReq.path;
        
        // Unlock after content loads
        setTimeout(() => {
          if (this.req.path === this.currentReqPath) {
            this.saveLocked = false;
          }
        }, 500);
      }
    },
    // Update editor content reactively
    editorContent(newContent) {
      if (this.editor) {
        const currentValue = this.editor.getValue();
        if (currentValue !== newContent) {
          this.editor.setValue(newContent, -1); // -1 moves cursor to start
          this.isDirty = false;
        }
      }
    },
    // Update editor language mode
    editorLanguageMode(newMode) {
      if (this.editor) {
        this.editor.session.setMode(newMode);
      }
    },
    // Update read-only state
    editorReadOnly(isReadOnly) {
      if (this.editor) {
        this.editor.setReadOnly(isReadOnly);
      }
    },
    // Update theme when dark mode changes
    isDarkMode(newValue) {
      if (this.editor) {
        this.editor.setTheme(newValue ? "ace/theme/twilight" : "ace/theme/chrome");
      }
    },
    // Initialize navigation when state syncs for file editing
    isStateSynced(synced) {
      if (synced && !this.viewerMode && this.req) {
        this.initializeNavigation();
      }
    }
  },
  created() {
    window.addEventListener("keydown", this.keyEvent);
    eventBus.on("handleEditorValueRequest", this.handleEditorValueRequest);
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
    if (this.isDirty) {
      if (confirm("You have unsaved changes. Do you want to save them?")) {
        this.handleEditorValueRequest();
      }
    }
    if (this.editor) {
      this.editor.destroy();
    }
  },
  mounted: function () {
    this.initializeEditor();
    this.originalReq = this.req;
  },
  methods: {
    initializeNavigation() {
      if (!this.req || this.req.type === 'directory') {
        return;
      }

      mutations.resetSelected();
      mutations.addSelected({
        name: this.req.name,
        path: this.req.path,
        size: this.req.size,
        type: this.req.type,
        source: this.req.source,
      });

      this.updateNavigationForCurrentItem();
    },

    async updateNavigationForCurrentItem() {
      if (!this.req || this.req.type === 'directory') {
        return;
      }

      let directoryPath = url.removeLastDir(this.req.path);
      
      // If directoryPath is empty, the file is in root - use '/' as the directory
      if (!directoryPath || directoryPath === '') {
        directoryPath = '/';
      }
      
      let listing = null;

      if (this.req.items) {
        listing = this.req.items;
      } else if (this.req.parentDirItems) {
        // Use pre-fetched parent directory items from Files.vue
        listing = this.req.parentDirItems;
      } else if (directoryPath !== this.req.path) {
        // Fetch directory listing (now with '/' for root files)
        try {
          let res;
          if (getters.isShare()) {
            res = await publicApi.fetchPub(directoryPath, state.share.hash);
          } else {
            res = await filesApi.fetchFiles(this.req.source, directoryPath);
          }
          listing = res.items;
        } catch (error) {
          listing = [this.req];
        }
      } else {
        // Shouldn't happen, but fallback to current item
        listing = [this.req];
      }

      mutations.setupNavigation({
        listing: listing,
        currentItem: this.req,
        directoryPath: directoryPath
      });
    },
    initializeEditor() {
      const editorEl = document.getElementById("editor");
      if (!editorEl) {
        return;
      }

      try {
        ace.config.set(
          "basePath",
          `https://cdn.jsdelivr.net/npm/ace-builds@${ace_version}/src-min-noconflict/`
        );

        this.editor = ace.edit(editorEl, {
          mode: this.editorLanguageMode,
          value: this.editorContent,
          showPrintMargin: false,
          showGutter: true,
          showLineNumbers: true,
          theme: this.isDarkMode ? "ace/theme/tomorrow_night_bright" : "ace/theme/github",
          readOnly: this.editorReadOnly,
          wrap: false,
          enableMobileMenu: !this.viewerMode,
          useWorker: false,
          scrollPastEnd: 0.5,
          cursorStyle: "smooth",
          highlightGutterLine: true,
          animatedScroll: true,
          displayIndentGuides: true,
          fixedWidthGutter: true,
        });

        this.editor.setOption('displayIndentGuides', true);

        this.editor.on('change', () => {
          this.isDirty = true;
        });

        // Initialize navigation for file editing mode when synced
        if (this.isStateSynced && !this.viewerMode) {
          this.initializeNavigation();
        }
      } catch (error) {
        notify.showError(this.$t("editor.uninitialized"));
      }
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

      // Check if navigation is transitioning
      if (state.navigation.isTransitioning) {
        notify.showError("Please wait for navigation to complete before saving.");
        return;
      }

      // Check if save is locked due to req transition
      if (this.saveLocked) {
        notify.showError("Please wait a moment before saving.");
        return;
      }

      // Filename protection - ensure state is synced before saving
      if (!this.isStateSynced) {
        notify.showError(this.$t("editor.saveAbortedMessage", { 
          activeFile: this.originalReq?.name || "unknown", 
          tryingToSave: this.routeFilename || "unknown" 
        }));
        return;
      }

      if (!this.editor) {
        notify.showError(this.$t("editor.uninitialized"));
        return;
      }

      try {
        if (getters.isShare()) {
          // TODO: add support for saving shared files
          notify.showError(this.$t("share.saveDisabled"));
          return;
        }

        // Save the file
        await filesApi.put(this.originalReq.source, this.originalReq.path, this.editor.getValue());
        notify.showSuccess(`${this.originalReq.name} saved successfully.`);
        this.isDirty = false;
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
    line-height: 1.4;
    -webkit-user-select: text !important;
    -moz-user-select: text !important;
    -ms-user-select: text !important;
    user-select: text !important;
}

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
    display: flex !important;
    align-items: center !important;
    justify-content: center !important;
}

/* make sure the text selection is detected*/
.ace_content {
    -webkit-user-select: text;
    -moz-user-select: text;
    -ms-user-select: text;
    user-select: text;
}

/* Text selection color */
.ace_editor .ace_selection {
    background-color: color-mix(in srgb, var(--primaryColor) 25%, transparent) !important;
}

.ace_editor .ace_selection.ace_start {
    box-shadow: 0 0 3px 0px color-mix(in srgb, var(--primaryColor) 40%, transparent) !important;
}

.ace_editor .ace_gutter-active-line {
    background-color: color-mix(in srgb, var(--primaryColor) 20%, transparent) !important;
    color: var(--primaryColor) !important;
    font-weight: bold !important;
}

/* Indent lines */
.ace_editor .ace_indent-guide {
  border-right: 1px solid color-mix(in srgb, var(--primaryColor) 50%, transparent) !important;
  opacity: 1 !important;
  z-index: 5 !important;
}

.ace_editor .ace_indent-guide-active {
  border-right: 1px solid color-mix(in srgb, var(--primaryColor) 75%, transparent) !important;
}

/* Lightened Tomorrow Night Bright Theme, was too dark */
.ace-tomorrow-night-bright {
  background-color: #1f1f1f !important; /* original of the theme is #000000 */
}
</style>