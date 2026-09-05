<template>
  <div id="editor-root" ref="editorRoot" :class="{ 'split-active': isSplitActive }">
    <div
      id="editor-container"
      :class="{ 'viewer-mode': viewerMode }"
      :style="isSplitActive ? { flexBasis: `${editorPanePercent}%` } : {}"
    >
      <EditorToolbar v-if="showEditorToolbar" :editor="editor" :is-markdown="isMarkdownFile" />
      <div id="editor" ref="editorEl"></div>
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

<script lang="ts">
import type { RouteLocationNormalized } from "vue-router";
import type { Ace } from "ace-builds";
import { state, getters, mutations } from "@/store";
import { resourcesApi } from "@/api";
import { pathsMatch, removeLastDir } from "@/utils/url.js";
import { notify } from "@/notify";
import ace, { version as ace_version } from "ace-builds";
import modelist from "ace-builds/src-noconflict/ext-modelist";
import "ace-builds/src-noconflict/ext-searchbox";
import "ace-builds/src-min-noconflict/theme-chrome";
import "ace-builds/src-min-noconflict/theme-tomorrow_night_bright";
import "ace-builds/src-min-noconflict/mode-yaml";
import "ace-builds/src-min-noconflict/mode-json";
import "ace-builds/src-min-noconflict/mode-markdown";
import EditorToolbar from "@/components/files/EditorToolbar.vue";
import MarkdownSplitView from "@/components/files/MarkdownSplitView.vue";
import { editorConfig } from "@/utils/editorConfig";
import { rejectPutIfQuotaExceeded } from "@/utils/uploadQuota";

type Req = typeof state.req;
interface AceRendererInternal { $gutterLayer: { $renderer: unknown } }

const THEME_DARK = "ace/theme/tomorrow_night_bright";
const THEME_LIGHT = "ace/theme/chrome";

export default {
  name: "editor",
  components: {
    EditorToolbar,
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
    editor: null as Ace.Editor | null, // The editor instance
    isDirty: false,
    savedContent: "", // content used for dirty comparisons
    suppressDirtyTracking: false,
    originalReq: null as Req | null,
    saveLocked: false, // Lock saves during req transitions
    saveUnlockTimer: null as ReturnType<typeof setTimeout> | null, // pending save-unlock timer
    statsUpdateTimer: null as ReturnType<typeof setTimeout> | null, // throttle stats update
    currentReqPath: null as string | null, // Track current path for transition detection
    navigationGuard: null as (() => void) | null, // Navigation guard to prevent navigation with unsaved changes
    isPromptOpen: false, // Track if prompt is currently open for avoid navigation
    pendingNavigation: null as RouteLocationNormalized | null, // Store pending navigation while prompt is open
    viewerResizeObserver: null as ResizeObserver | null,
    editorPanePercent: 50, // split-view editor pane width
    resizeContainerEl: null as HTMLElement | null, // #editor-root element, passed to MarkdownSplitView for divider drag geometry
    beforeUnloadHandler: null as ((event: BeforeUnloadEvent) => void) | null,
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
      if (!this.viewerMode && !this.permissions.modify) {
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
    showEditorToolbar() {
      return !this.editorReadOnly;
    },
    isSplitActive() {
      return !this.viewerMode && this.isMarkdownFile && state.editor.markdownSplitView && !state.isMobile && this.permissions.modify;
    },
    editorScrollRatio() {
      return state.editor.scrollRatio;
    },
    isTransitioning() {
      return state.navigation.isTransitioning;
    },
    editorFontSize() {
      return state.editor.fontSize;
    },
    wrapEditorContent() {
      return editorConfig.wrapEditorContent;
    },
    editorAceOptions() {
      return {
        keybinding: editorConfig.keybinding,
        tabSize: editorConfig.tabSize,
        overscroll: editorConfig.overscroll,
        showIndentGuides: editorConfig.showIndentGuides,
        showGutter: editorConfig.showGutter,
        fixedGutterWidth: editorConfig.fixedGutterWidth,
        showLineNumbers: editorConfig.showLineNumbers,
        relativeLineNumbers: editorConfig.relativeLineNumbers,
        customScrollbar: editorConfig.customScrollbar,
      };
    },
  },
  watch: {
    // Lock saves during navigation transitions
    isTransitioning(isTransitioning: boolean) {
      if (isTransitioning && !this.viewerMode) {
        this.saveLocked = true;
      } else if (!isTransitioning && !this.viewerMode) {
        // Unlock after a short delay to ensure req is fully loaded
        this.scheduleSaveUnlock(300);
      }
    },
    // Update originalReq and lock saves when req changes during navigation
    'req'(newReq: Req, oldReq: Req) {
      if (!this.viewerMode && newReq && (newReq.path !== oldReq?.path || newReq.source !== oldReq.source)) {
        // Update originalReq to the new file
        this.originalReq = newReq;
        this.isDirty = false; // Reset dirty flag for new file
        mutations.setEditorDirty(false);
        mutations.setEditorJsonFormatted(false);
        mutations.resetEditorScrollRatio(newReq.path);

        // Lock saves temporarily
        this.saveLocked = true;
        this.currentReqPath = newReq.path;

        // Unlock after content loads
        this.scheduleSaveUnlock(500);
      }
    },
    // Update editor content reactively
    editorContent(newContent: string) {
      if (this.editor) {
        const currentValue = this.editor.getValue();
        if (currentValue !== newContent) {
          this.suppressDirtyTracking = true;
          this.editor.setValue(newContent, -1); // -1 moves cursor to start
          this.editor.session.getUndoManager().reset();
          this.updateEditorStats();
          this.suppressDirtyTracking = false;
        }
        this.savedContent = newContent;
        this.isDirty = false;
        mutations.setEditorDirty(false);
        if (this.viewerMode) {
          this.$nextTick(() => {
            if (this.editor) {
              this.editor.resize();
            }
          });
        }
        if (this.isSplitActive) {
          (this.$refs.splitView as InstanceType<typeof MarkdownSplitView> | undefined)?.setLiveContent(newContent);
        }
      }
    },
    // Update editor language mode
    editorLanguageMode(newMode: string) {
      if (this.editor) {
        this.editor.session.setMode(newMode);
      }
    },
    // Update read-only state
    editorReadOnly(isReadOnly: boolean) {
      if (this.editor) {
        this.editor.setReadOnly(isReadOnly);
      }
    },
    // Update theme when dark mode changes
    isDarkMode(newValue: boolean) {
      if (this.editor) {
        this.editor.setTheme(newValue ? THEME_DARK : THEME_LIGHT);
      }
    },
    // Initialize navigation when state syncs for file editing
    isStateSynced(synced: boolean) {
      if (synced && !this.viewerMode && this.req) {
        this.initializeNavigation();
      }
    },
    editorScrollRatio() {
      if (this.viewerMode || !this.isMarkdownFile) return;
      if (state.editor.scrollSource === 'editor') return;
      (this.$refs.splitView as InstanceType<typeof MarkdownSplitView> | undefined)?.applyScrollRatio(state.editor.scrollRatio);
    },
    isSplitActive() {
      this.$nextTick(() => {
        if (this.editor) this.editor.resize();
      });
    },
    editorFontSize() {
      this.applyFontSize();
    },
    wrapEditorContent() {
      this.applyWrap();
    },
    editorAceOptions(cfg) {
      this.applyAceOptions(cfg);
    },
  },
  created() {
    window.addEventListener("keydown", this.keyEvent, true);

    // Show generic browser dialog if the user closes the tab, or try to close the browser with unsaved changes
    this.beforeUnloadHandler = (event: BeforeUnloadEvent) => {
      if (this.isDirty && !this.viewerMode) {
        event.preventDefault();
      }
    };
    window.addEventListener("beforeunload", this.beforeUnloadHandler);

    this.setupNavigationGuard();
  },
  beforeUnmount() {
    if (this.saveUnlockTimer) {
      clearTimeout(this.saveUnlockTimer);
      this.saveUnlockTimer = null;
    }
    if (this.statsUpdateTimer) {
      clearTimeout(this.statsUpdateTimer);
      this.statsUpdateTimer = null;
    }
    if (this.viewerResizeObserver) {
      this.viewerResizeObserver.disconnect();
      this.viewerResizeObserver = null;
    }

    window.removeEventListener("keydown", this.keyEvent, true);
    if (this.beforeUnloadHandler) {
      window.removeEventListener("beforeunload", this.beforeUnloadHandler);
    }

    if (this.editor) {
      this.editor.session.off('changeScrollTop', this.handleEditorScroll);
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
  unmounted() {
    if (this.editor) {
      this.editor.destroy();
      this.editor = null;
    }
  },
  mounted: function () {
    this.resizeContainerEl = (this.$refs.editorRoot as HTMLElement | undefined) || null;
    if (this.viewerMode) {
      this.$nextTick(() => {
        this.$nextTick(() => {
          this.initializeEditor();
          this.applyFontSize();
          this.setupViewerResizeObserver();
        });
      });
      return;
    }

    this.originalReq = this.req;
    this.currentReqPath = this.req?.path ?? null;
    if (this.isMarkdownFile && this.req?.path) {
      mutations.resetEditorScrollRatio(this.req.path);
    }
    this.initializeEditor(state.editor.scrollRatio);

    // Register save handler so other components can trigger save
    mutations.setEditorSaveHandler(() => this.handleEditorValueRequest());
    this.applyFontSize();
    this.setupViewerResizeObserver();
  },
  methods: {
    scheduleSaveUnlock(delay: number) {
      if (this.saveUnlockTimer) {
        clearTimeout(this.saveUnlockTimer);
      }
      this.saveUnlockTimer = setTimeout(() => {
        this.saveUnlockTimer = null;
        if (!state.navigation.isTransitioning && this.req?.path === this.currentReqPath) {
          this.saveLocked = false;
        }
      }, delay);
    },
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

      let listing: unknown;

      if (this.req.items) {
        listing = this.req.items;
      } else if (this.req.parentDirItems) {
        // Use pre-fetched parent directory items from Files.vue
        listing = this.req.parentDirItems;
      } else if (directoryPath !== this.req.path) {
        // Fetch directory listing (now with '/' for root files)
        try {
          let res: { items: unknown; };
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
    initializeEditor(initialScrollRatio: number = state.editor.scrollRatio) {
      const editorEl = this.$refs.editorEl as HTMLElement | undefined;
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
          showGutter: editorConfig.showGutter,
          showLineNumbers: editorConfig.showLineNumbers,
          relativeLineNumbers: editorConfig.relativeLineNumbers,
          theme: this.isDarkMode ? THEME_DARK : THEME_LIGHT,
          readOnly: this.editorReadOnly,
          wrap: !!editorConfig.wrapEditorContent,
          enableMobileMenu: false,
          useWorker: true,
          cursorStyle: "smooth",
          highlightGutterLine: true,
          animatedScroll: true,
          displayIndentGuides: editorConfig.showIndentGuides,
          fixedWidthGutter: editorConfig.fixedGutterWidth,
          tabSize: editorConfig.tabSize,
          scrollPastEnd: editorConfig.overscroll,
          customScrollbar: editorConfig.customScrollbar,
          keyboardHandler: editorConfig.keybinding || null,
          fontSize: `${state.editor.fontSize}px`,
        });

        this.savedContent = this.editorContent;
        this.editor.session.getUndoManager().reset(); // To avoid redo to an empty file on fresh mount
        this.editor.commands.removeCommand("showSettingsMenu");

        const editorInstance = this.editor;
        editorInstance.on('change', () => {
          if (this.editor !== editorInstance) return;
          if (this.suppressDirtyTracking) return;
          const dirty = editorInstance.getValue() !== this.savedContent;
          if (this.isDirty !== dirty) {
            this.isDirty = dirty;
            mutations.setEditorDirty(dirty);
          }
          this.scheduleStatsUpdate();
          (this.$refs.splitView as InstanceType<typeof MarkdownSplitView> | undefined)?.handleEditorChange();
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
              if (this.editor !== editorInstance) return;
              if (this.isSplitActive) {
                (this.$refs.splitView as InstanceType<typeof MarkdownSplitView> | undefined)?.setLiveContent(editorInstance.getValue());
              }
              (this.$refs.splitView as InstanceType<typeof MarkdownSplitView> | undefined)?.applyScrollRatio(initialScrollRatio, true);
              requestAnimationFrame(() => {
                requestAnimationFrame(() => {
                  if (this.editor !== editorInstance) return;
                  editorInstance.session.on('changeScrollTop', this.handleEditorScroll);
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
    getValue(): string {
      return this.editor?.getValue() ?? this.editorContent;
    },
    getAceMode(mode: string): string {
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

      const content = this.editor.getValue();
      const newBytes = new TextEncoder().encode(content).length;
      const oldBytes = this.originalReq?.size ?? 0;
      const quotaPath = removeLastDir(this.originalReq.path) || "/";
      if (await rejectPutIfQuotaExceeded(quotaPath, newBytes, oldBytes)) {
        const errorMsg = this.$t("quotas.errors.exceeded");
        throw new Error(errorMsg);
      }

      if (getters.isShare()) {
        // Save the file
        await resourcesApi.putPublic(state.shareInfo.hash, this.originalReq.path, content);
      } else {
        // Save the file
        await resourcesApi.put(this.originalReq.source, this.originalReq.path, content);
      }

      notify.showSuccessToast(`${this.originalReq.name} saved successfully.`);
      this.savedContent = this.editor.getValue();
      mutations.setRequestContent(this.savedContent);
      this.isDirty = false;
      mutations.setEditorDirty(false);
    },
    async keyEvent(event: KeyboardEvent) {
      const { key, ctrlKey, metaKey } = event;
      if (getters.currentPromptName()) return;
      if ((ctrlKey || metaKey) && key === ",") {
        event.preventDefault();
        event.stopPropagation();
        this.openEditorSettings();
        return;
      }
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
    openEditorSettings() {
      mutations.showPrompt({
        name: "EditorSettings",
      });
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
          try {
            await this.handleEditorValueRequest();
          } catch (_e) {
            this.isPromptOpen = false;
            return;
          }
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
      if (!this.editor) return undefined;
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
      const validWord = text.split(/\s+/).filter((t: string) => /[a-zA-Z0-9]/.test(t));
      const words = validWord.length;

      return { lines, words, chars };
    },
    updateEditorStats() {
      if (!this.editor) return;
      const stats = this.getSelectedStats();
      if (!stats) return;
      const { lines, words, chars } = stats;
      const isMarkdown = this.isMarkdownFile;
      if (isMarkdown) {
        mutations.setEditorStats({ lines, words, chars });
      } else {
        // For other files, show only lines
        // Just lines because will be a bit misleading to count words if we are viewing code for example
        mutations.setEditorStats({ lines, words: null, chars: null });
      }
    },
    scheduleStatsUpdate() {
      if (this.statsUpdateTimer) {
        clearTimeout(this.statsUpdateTimer);
      }
      this.statsUpdateTimer = setTimeout(() => {
        this.statsUpdateTimer = null;
        this.updateEditorStats();
      }, 185);
    },
    applyFontSize() {
      if (this.editor) {
        this.editor.setOption('fontSize', `${state.editor.fontSize}px`);
      }
    },
    applyWrap() {
      if (this.editor) {
        this.editor.setOption('wrap', !!editorConfig.wrapEditorContent);
      }
    },
    applyAceOptions(cfg = editorConfig) {
      if (!this.editor) return;
      const wasRelative = this.editor.getOption('relativeLineNumbers');
      this.editor.setOption('keyboardHandler', cfg.keybinding || null);
      this.editor.setOption('tabSize', cfg.tabSize);
      this.editor.setOption('scrollPastEnd', cfg.overscroll);
      this.editor.setOption('displayIndentGuides', cfg.showIndentGuides);
      this.editor.setOption('showGutter', cfg.showGutter);
      this.editor.setOption('fixedWidthGutter', cfg.fixedGutterWidth);
      this.editor.setOption('showLineNumbers', cfg.showLineNumbers);
      this.editor.setOption('relativeLineNumbers', cfg.relativeLineNumbers);
      this.editor.setOption('customScrollbar', cfg.customScrollbar);
      if (wasRelative && !cfg.relativeLineNumbers && cfg.showLineNumbers) {
        const gutterLayer = (this.editor.renderer as unknown as AceRendererInternal).$gutterLayer;
        if (gutterLayer?.$renderer) {
          gutterLayer.$renderer = null;
        }
      }
    },
    handleEditorScroll() {
      (this.$refs.splitView as InstanceType<typeof MarkdownSplitView> | undefined)?.handleEditorScroll();
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
  background-color: #151515 !important; /* original of the theme is #000000 */
}

.ace-tomorrow-night-bright .ace_marker-layer .ace_active-line {
  background: #232323;
}

#editor-root.split-active .ace_scrollbar {
  scrollbar-width: none;
}

#editor-root.split-active .ace_scrollbar::-webkit-scrollbar {
  display: none;
}
</style>
