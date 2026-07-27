<template>
  <div id="editor-root" ref="editorRoot" :class="{ 'split-active': isSplitActive }">
    <div
      id="editor-container"
      :class="{ 'viewer-mode': viewerMode }"
      :style="isSplitActive ? { flexBasis: `${editorPanePercent}%` } : {}"
    >
      <MarkdownToolbar v-if="showMarkdownToolbar" :editor="editor" />
      <div id="editor"></div>
    </div>
    <MarkdownSplitView
      v-if="isMarkdownFile"
      ref="splitView"
      :editor="editor"
      :active="isSplitActive"
      :resize-container="resizeContainerEl"
      @resize="editorPanePercent = $event"
    />
  </div>
</template>

<script>
import { state, getters, mutations } from "@/store";
import { resourcesApi } from "@/api";
import {pathsMatch, removeLastDir } from "@/utils/url.js";
import { notify } from "@/notify";
import ace, { version as ace_version } from "ace-builds";
import modelist from "ace-builds/src-noconflict/ext-modelist";
import "ace-builds/src-min-noconflict/theme-github";
import "ace-builds/src-min-noconflict/theme-tomorrow_night_bright";
import "ace-builds/src-min-noconflict/mode-yaml";
import "ace-builds/src-min-noconflict/mode-json";
import "ace-builds/src-min-noconflict/mode-markdown";
import MarkdownToolbar from "@/components/files/MarkdownToolbar.vue";
import MarkdownSplitView from "@/components/files/MarkdownSplitView.vue";

const THEME_DARK = "ace/theme/tomorrow_night_bright";
const THEME_LIGHT = "ace/theme/chrome";

export default {
  name: "editor",
  components: {
    MarkdownToolbar,
    MarkdownSplitView,
  },
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
  data: () => ({
    editor: null, // The editor instance
    isDirty: false,
    suppressDirtyTracking: false,
    originalReq: null,
    saveLocked: false, // Lock saves during req transitions
    currentReqPath: null, // Track current path for transition detection
    navigationGuard: null, // Navigation guard to prevent navigation with unsaved changes
    isPromptOpen: false, // Track if prompt is currently open for avoid navigation
    pendingNavigation: null, // Store pending navigation while prompt is open
    viewerResizeObserver: null,
    editorPanePercent: 50, // split-view editor pane width
    resizeContainerEl: null, // #editor-root element, passed to MarkdownSplitView for divider drag geometry
  }),
  computed: {
    permissions() {
      return getters.sourcePermissions();
    },
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
      if (!this.originalReq || !this.req) return false;
      if (getters.isShare()) {
        const subPath = state.shareInfo?.subPath;
        if (subPath === undefined || subPath === null) return false;
        if (!pathsMatch(this.req.path, subPath)) return false;
        return pathsMatch(this.originalReq.path, this.req.path);
      }
      if (!this.routeFilename) return false;
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
      if (!this.permissions.modify) {
        return true;
      }
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
    isMarkdownFile() {
      if (this.viewerMode) return false;
      const type = this.req?.type;
      return type === "text/markdown" || type === "text/x-markdown";
    },
    showMarkdownToolbar() {
      return this.isMarkdownFile && !this.editorReadOnly;
    },
    isSplitActive() {
      return !this.viewerMode && this.isMarkdownFile && state.editor.markdownSplitView && !state.isMobile && this.permissions.modify;
    },
    editorScrollRatio() {
      return state.editor.scrollRatio;
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
        mutations.setEditorDirty(false);
        mutations.resetEditorScrollRatio(newReq.path);

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
          this.suppressDirtyTracking = true;
          this.editor.setValue(newContent, -1); // -1 moves cursor to start
          this.editor.session.getUndoManager().reset();
          this.updateEditorStats();
          this.suppressDirtyTracking = false;
          this.isDirty = false;
          mutations.setEditorDirty(false);
        }
        if (this.viewerMode) {
          this.$nextTick(() => {
            if (this.editor) {
              this.editor.resize();
            }
          });
        }
        if (this.isSplitActive) {
          this.$refs.splitView?.setLiveContent(newContent);
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
        this.editor.setTheme(newValue ? THEME_DARK : THEME_LIGHT);
      }
    },
    // Initialize navigation when state syncs for file editing
    isStateSynced(synced) {
      if (synced && !this.viewerMode && this.req) {
        this.initializeNavigation();
      }
    },
    editorScrollRatio() {
      if (this.viewerMode || !this.isMarkdownFile) return;
      if (state.editor.scrollSource === 'editor') return;
      this.$refs.splitView?.applyScrollRatio(state.editor.scrollRatio);
    },
    isSplitActive() {
      this.$nextTick(() => {
        if (this.editor) this.editor.resize();
      });
    },
  },
  created() {
    window.addEventListener("keydown", this.keyEvent);

    // Show generic browser dialog if the user closes the tab, or try to close the browser with unsaved changes
    this.beforeUnloadHandler = (event) => {
      if (this.isDirty && !this.viewerMode) {
        event.preventDefault();
      }
    };
    window.addEventListener("beforeunload", this.beforeUnloadHandler);

    this.setupNavigationGuard();
  },
  beforeUnmount() {
    if (this.viewerResizeObserver) {
      this.viewerResizeObserver.disconnect();
      this.viewerResizeObserver = null;
    }

    window.removeEventListener("keydown", this.keyEvent);
    window.removeEventListener("beforeunload", this.beforeUnloadHandler);

    if (this.editor) {
      this.editor.session.off('changeScrollTop', this.handleEditorScroll);
      this.editor.destroy();
      this.editor = null;
    }

    if (this.readOnly) {
      return;
    }

    // Clear navigation guard
    if (this.navigationGuard) {
      this.navigationGuard();
    }

    // Clear dirty state and save handler when leaving editor
    mutations.setEditorDirty(false);
    mutations.setEditorSaveHandler(null);
    mutations.setEditorStats({ lines: 0, words: 0, chars: 0 });
  },
  mounted: function () {
    this.resizeContainerEl = this.$refs.editorRoot || null;
    if (this.viewerMode) {
      this.$nextTick(() => {
        this.$nextTick(() => {
          this.initializeEditor();
          this.applyFontSize();
          this.setupViewerResizeObserver();
        });
      });
      this.$watch(() => state.editor.fontSize, () => {
        this.applyFontSize();
      });
      return;
    }

    this.originalReq = this.req;
    this.initializeEditor();
    if (this.isMarkdownFile && this.req?.path) {
      mutations.resetEditorScrollRatio(this.req.path);
    }

    // Register save handler so other components can trigger save
    mutations.setEditorSaveHandler(() => this.handleEditorValueRequest());
    this.applyFontSize();
    this.setupViewerResizeObserver();
    // Watch font size changes
    this.$watch(() => state.editor.fontSize, () => {
      this.applyFontSize();
    });
  },
  methods: {
    setupViewerResizeObserver() {
      if (typeof ResizeObserver === "undefined" || !this.editor) {
        return;
      }
      this.viewerResizeObserver = new ResizeObserver(() => {
        if (this.editor) {
          this.editor.resize();
        }
      });
      this.viewerResizeObserver.observe(this.editor.container);
      this.$nextTick(() => {
        if (this.editor) {
          this.editor.resize();
        }
      });
    },
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
        modified: this.req.modified,
        hasPreview: this.req.hasPreview,
      });

      void this.updateNavigationForCurrentItem();
    },

    async updateNavigationForCurrentItem() {
      if (!this.req || this.req.type === 'directory') {
        return;
      }

      let directoryPath = removeLastDir(this.req.path);

      // If directoryPath is empty, the file is in root - use '/' as the directory
      if (!directoryPath || directoryPath === '') {
        directoryPath = '/';
      }

      let listing;

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
            res = await resourcesApi.fetchFilesPublic(directoryPath, state.shareInfo.hash);
          } else {
            res = await resourcesApi.fetchFiles(this.req.source, directoryPath);
          }
          listing = res.items;
        } catch (error) {
          console.error("error Editor.vue", error);
          listing = [this.req];
        }
      } else {
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
          theme: this.isDarkMode ? THEME_DARK : THEME_LIGHT,
          readOnly: this.editorReadOnly,
          wrap: state.wrapEditor || false,
          enableMobileMenu: !this.viewerMode,
          useWorker: true,
          cursorStyle: "smooth",
          highlightGutterLine: true,
          animatedScroll: true,
          displayIndentGuides: true,
          fixedWidthGutter: true,
          fontSize: `${state.editor.fontSize}px`,
        });

        this.editor.setOption('displayIndentGuides', true);
        this.editor.session.getUndoManager().reset(); // To avoid redo to an empty file on fresh mount

        this.editor.on('change', () => {
          if (this.suppressDirtyTracking) return;
          const dirty = this.editor.getValue() !== this.editorContent;
          this.isDirty = dirty;
          mutations.setEditorDirty(dirty);
          this.updateEditorStats();
          this.$refs.splitView?.handleEditorChange();
        });

        // Initialize navigation for file editing mode when synced
        if (this.isStateSynced && !this.viewerMode) {
          this.initializeNavigation();
        }
        this.updateEditorStats();
        this.editor.selection.on('changeSelection', () => {
          this.updateEditorStats();
        });
        if (!this.viewerMode) {
          if (this.isMarkdownFile) {
            this.$nextTick(() => {
              this.$refs.splitView?.applyScrollRatio(state.editor.scrollRatio, true);
              requestAnimationFrame(() => {
                requestAnimationFrame(() => {
                  if (!this.editor) return;
                  this.editor.session.on('changeScrollTop', this.handleEditorScroll);
                });
              });
            });
          } else {
            this.editor.session.on('changeScrollTop', this.handleEditorScroll);
          }
        }
      } catch (_e) {
        notify.showError(this.$t("editor.uninitialized"));
      }
    },
    getAceMode(mode) {
      switch (mode) {
        case 'yaml': return 'ace/mode/yaml';
        case 'json': return 'ace/mode/json';
        case 'javascript': return 'ace/mode/javascript';
        case 'typescript': return 'ace/mode/typescript';
        case 'html': return 'ace/mode/html';
        case 'css': return 'ace/mode/css';
        case 'markdown': return 'ace/mode/markdown';
        case 'text': return 'ace/mode/text';
        case 'xml': return 'ace/mode/xml';
        default: return `ace/mode/${mode}`;
      }
    },
    async handleEditorValueRequest() {
      // Skip save logic in viewer mode
      if (this.viewerMode) {
        return;
      }

      // Check if navigation is transitioning
      if (state.navigation.isTransitioning) {
        const errorMsg = "Please wait for navigation to complete before saving.";
        notify.showError(errorMsg);
        throw new Error(errorMsg);
      }

      // Check if save is locked due to req transition
      if (this.saveLocked) {
        const errorMsg = "Please wait a moment before saving.";
        notify.showError(errorMsg);
        throw new Error(errorMsg);
      }

      // Filename protection - ensure state is synced before saving
      if (!this.isStateSynced) {
        const errorMsg = this.$t("editor.saveAbortedMessage", {
          activeFile: this.originalReq?.name || "unknown",
          tryingToSave: this.routeFilename || "unknown"
        });
        notify.showError(errorMsg);
        throw new Error(errorMsg);
      }

      if (!this.editor) {
        const errorMsg = this.$t("editor.uninitialized");
        notify.showError(errorMsg);
        throw new Error(errorMsg);
      }

      if (getters.isShare()) {
        // Save the file
        await resourcesApi.putPublic(state.shareInfo.hash, this.originalReq.path, this.editor.getValue());
      } else {
        // Save the file
        await resourcesApi.put(this.originalReq.source, this.originalReq.path, this.editor.getValue());
      }

      notify.showSuccessToast(`${this.originalReq.name} saved successfully.`);
      this.isDirty = false;
      mutations.setEditorDirty(false);
    },
    async keyEvent(event) {
      const { key, ctrlKey, metaKey } = event;
      if (getters.currentPromptName()) return;

      // Skip save shortcut in viewer mode
      if (this.viewerMode) return;

      if ((ctrlKey || metaKey) && key.toLowerCase() === "s") {
        event.preventDefault();
        try {
          await this.handleEditorValueRequest();
        } catch (_e) {
          // ignore
        }
      }
    },
    setupNavigationGuard() {
      if (this.viewerMode) return;

      this.navigationGuard = this.$router.beforeEach((to, from, next) => {
        // If prompt is already open, block any new navigation attempts
        if (this.isPromptOpen) {
          if (getters.currentPromptName() === "SaveBeforeExit") {
            next(false);
            return;
          }
          this.isPromptOpen = false;
          this.pendingNavigation = null;
        }

        // Check if we are navigating to a different route
        const isDifferentRoute = to.path !== from.path || to.hash !== from.hash;

        if (this.isDirty && !this.viewerMode && isDifferentRoute && this.req) {
          next(false);
          this.pendingNavigation = to;
          this.showSaveBeforeExitPrompt();
          return;
        }
        next();
      });
    },
    showSaveBeforeExitPrompt() {
      this.isPromptOpen = true;
      mutations.showPrompt({
        name: "SaveBeforeExit",
        pinned: true,
        confirm: async () => {
          await this.handleEditorValueRequest();
          this.isDirty = false;
          mutations.setEditorDirty(false);
          this.executePendingNavigation();
        },
        discard: () => {
          // Discard changes and exit
          this.isDirty = false;
          mutations.setEditorDirty(false);
          this.executePendingNavigation();
        },
        cancel: () => {
          // Keep editing - block navigation
          this.cancelPendingNavigation();
        },
      });
    },
    executePendingNavigation() {
      this.isPromptOpen = false;
      const target = this.pendingNavigation;
      this.pendingNavigation = null;
      if (target) {
        this.$router.push(target.fullPath);
      }
    },
    cancelPendingNavigation() {
      this.isPromptOpen = false;
      this.pendingNavigation = null;
    },
    getSelectedStats() {
      if (!this.editor) return;
      const session = this.editor.session;
      const selectionRange = this.editor.selection.getRange();
      const isSelectionEmpty =
        selectionRange.start.row === selectionRange.end.row &&
        selectionRange.start.column === selectionRange.end.column;

      let text, lines;
      if (!isSelectionEmpty) {
        text = this.editor.getSelectedText();
        lines = text ? text.split('\n').length : 0;
      } else {
        text = session.getValue();
        lines = session.getLength();
      }

      const chars = text.length;
      const validWord = text.split(/\s+/).filter(t => /[a-zA-Z0-9]/.test(t));
      const words = validWord.length;

      return { lines, words, chars };
    },
    updateEditorStats() {
      if (!this.editor) return;
      const { lines, words, chars } = this.getSelectedStats();
      const isMarkdown = this.isMarkdownFile;
      if (isMarkdown) {
        mutations.setEditorStats({ lines, words, chars });
      } else {
        // For other files, show only lines
        // Just lines because will be a bit misleading to count words if we are viewing code for example
        mutations.setEditorStats({ lines, words: null, chars: null });
      }
    },
    applyFontSize() {
      if (this.editor) {
        this.editor.setOption('fontSize', `${state.editor.fontSize}px`);
      }
    },
    handleEditorScroll() {
      this.$refs.splitView?.handleEditorScroll();
    },
  },
};

</script>

<style scoped>
#editor-root {
  height: 100%;
}

#editor-root.split-active {
  display: flex;
  height: 100%;
  width: 100%;
}

#editor-root.split-active #editor-container {
  flex: 0 0 auto;
  min-width: 0;
  position: relative;
}

#editor-container {
  display: flex;
  flex-direction: column;
  height: 100%;
}

#editor-container #editor {
  flex: 1;
  min-height: 0;
}

#editor-container.viewer-mode {
  position: absolute;
  inset: 0;
}

</style>

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

#editor-root.split-active .ace_scrollbar {
  scrollbar-width: none;
}

#editor-root.split-active .ace_scrollbar::-webkit-scrollbar {
  display: none;
}
</style>
