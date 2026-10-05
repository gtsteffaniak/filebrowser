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

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, onUnmounted, ref, shallowRef, watch } from "vue";
import { useRoute, useRouter, type RouteLocationNormalized } from "vue-router";
import { useI18n } from "vue-i18n";
import type { Ace } from "ace-builds";
import { state, getters, mutations } from "@/store";
import { resourcesApi } from "@/api";
import { pathsMatch, removeLastDir } from "@/utils/url.js";
import { notify } from "@/notify";
import ace, { version as ace_version } from "ace-builds";
import modelist from "ace-builds/src-noconflict/ext-modelist";
import "ace-builds/src-noconflict/ext-searchbox";
import "ace-builds/src-min-noconflict/ext-language_tools";
import "ace-builds/src-min-noconflict/theme-chrome";
import "ace-builds/src-min-noconflict/theme-tomorrow_night_bright";
import "ace-builds/src-min-noconflict/mode-yaml";
import "ace-builds/src-min-noconflict/mode-json";
import "ace-builds/src-min-noconflict/mode-markdown";
import EditorToolbar from "@/components/files/EditorToolbar.vue";
import MarkdownSplitView from "@/components/files/MarkdownSplitView.vue";
import { editorConfig, type EditorConfig } from "@/utils/editorConfig";
import { rejectPutIfQuotaExceeded } from "@/utils/uploadQuota";

type Req = typeof state.req;
interface AceRendererInternal { $gutterLayer: { $renderer: unknown } }

const props = defineProps({
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
});

const THEME_DARK = "ace/theme/tomorrow_night_bright";
const THEME_LIGHT = "ace/theme/chrome";

defineOptions({ name: "editor" });

const route = useRoute();
const router = useRouter();
const { t } = useI18n();

const editorRoot = ref<HTMLElement | null>(null);
const editorEl = ref<HTMLElement | null>(null);
const splitView = ref<InstanceType<typeof MarkdownSplitView> | null>(null);

const editor = shallowRef<Ace.Editor | null>(null); // The editor instance
const originalReq = ref<Req | null>(null);
const editorPanePercent = ref(50); // split-view editor pane width
const resizeContainerEl = ref<HTMLElement | null>(null); // #editor-root element, passed to MarkdownSplitView for divider drag geometry

let isDirty = false;
let savedContent = ""; // content used for dirty comparisons
let suppressDirtyTracking = false;
let saveLocked = false; // Lock saves during req transitions
let saveUnlockTimer: ReturnType<typeof setTimeout> | null = null; // pending save-unlock timer
let statsUpdateTimer: ReturnType<typeof setTimeout> | null = null; // throttle stats update
let currentReqPath: string | null = null; // Track current path for transition detection
let navigationGuard: (() => void) | null = null; // Navigation guard to prevent navigation with unsaved changes
let isPromptOpen = false; // Track if prompt is currently open for avoid navigation
let pendingNavigation: RouteLocationNormalized | null = null; // Store pending navigation while prompt is open
let viewerResizeObserver: ResizeObserver | null = null;

const permissions = computed(() => getters.sourcePermissions());
const isDarkMode = computed(() => getters.isDarkMode());
const req = computed(() => state.req);
// Current filename from route
const routeFilename = computed(() => {
  if (props.viewerMode) return null;
  const filename = decodeURIComponent(route.path.split("/").pop() || "");
  return getters.shareHash() === filename ? "" : filename;
});
// Check if state and route are synchronized
const isStateSynced = computed(() => {
  if (props.viewerMode) return true;
  if (!originalReq.value || !req.value) return false;
  if (getters.isShare()) {
    const subPath = state.shareInfo?.subPath;
    if (subPath === undefined || subPath === null) return false;
    if (!pathsMatch(req.value.path, subPath)) return false;
    return pathsMatch(originalReq.value.path, req.value.path);
  }
  if (!routeFilename.value) return false;
  return originalReq.value.name === routeFilename.value;
});
// Editor content to display
const editorContent = computed(() => {
  if (props.viewerMode) {
    return props.content || "";
  }
  if (!isStateSynced.value) {
    return ""; // Show blank content until synced
  }
  return req.value.content === "empty-file-x6OlSil" ? "" : (req.value.content || "");
});
// Editor mode/language
const editorLanguageMode = computed(() => {
  if (props.viewerMode) {
    return getAceMode(props.editorMode);
  }
  if (!isStateSynced.value || !req.value) {
    return "ace/mode/text";
  }

  return modelist.getModeForPath(req.value.name ?? "").mode;
});
// Editor read-only state
const editorReadOnly = computed(() => {
  if (!props.viewerMode && !permissions.value.modify) {
    return true;
  }
  if (props.readOnly !== null) {
    return props.readOnly;
  }
  if (props.viewerMode) {
    return true;
  }
  if (!isStateSynced.value) {
    return true; // Read-only until synced
  }
  return req.value.type === "textImmutable";
});
const isMarkdownFile = computed(() => {
  if (props.viewerMode) return false;
  const type = req.value?.type;
  return type === "text/markdown" || type === "text/x-markdown";
});
const showEditorToolbar = computed(() => !editorReadOnly.value);
const isSplitActive = computed(() =>
  !props.viewerMode && isMarkdownFile.value && state.editor.markdownSplitView && !getters.isMobile() && permissions.value.modify
);
const editorAceOptions = computed(() => ({
  keybinding: editorConfig.keybinding,
  tabSize: editorConfig.tabSize,
  overscroll: editorConfig.overscroll,
  indentedSoftWrap: editorConfig.indentedSoftWrap,
  showIndentGuides: editorConfig.showIndentGuides,
  showGutter: editorConfig.showGutter,
  fixedGutterWidth: editorConfig.fixedGutterWidth,
  showLineNumbers: editorConfig.showLineNumbers,
  relativeLineNumbers: editorConfig.relativeLineNumbers,
  customScrollbar: editorConfig.customScrollbar,
  enableAutocompletion: editorConfig.enableAutocompletion,
  enableLiveAutocompletion: editorConfig.enableAutocompletion && editorConfig.enableLiveAutocompletion,
}));

// Lock saves during navigation transitions
watch(() => state.navigation.isTransitioning, (isTransitioning) => {
  if (isTransitioning && !props.viewerMode) {
    saveLocked = true;
  } else if (!isTransitioning && !props.viewerMode) {
    // Unlock after a short delay to ensure req is fully loaded
    scheduleSaveUnlock(300);
  }
});

// Update originalReq and lock saves when req changes during navigation
watch(req, (newReq, oldReq) => {
  if (!props.viewerMode && newReq && (newReq.path !== oldReq?.path || newReq.source !== oldReq.source)) {
    // Update originalReq to the new file
    originalReq.value = newReq;
    isDirty = false; // Reset dirty flag for new file
    mutations.setEditorDirty(false);
    mutations.setEditorJsonFormatted(false);
    mutations.resetEditorScrollRatio(newReq.path ?? "");
    // Lock saves temporarily
    saveLocked = true;
    currentReqPath = newReq.path ?? null;
    // Unlock after content loads
    scheduleSaveUnlock(500);
  }
});

// Update editor content reactively
watch(editorContent, (newContent) => {
  if (editor.value) {
    const currentValue = editor.value.getValue();
    if (currentValue !== newContent) {
      suppressDirtyTracking = true;
      editor.value.setValue(newContent, -1); // -1 moves cursor to start
      editor.value.session.getUndoManager().reset();
      updateEditorStats();
      suppressDirtyTracking = false;
    }
    savedContent = newContent;
    isDirty = false;
    mutations.setEditorDirty(false);
    if (props.viewerMode) {
      void nextTick(() => {
        if (editor.value) {
          editor.value.resize();
        }
      });
    }
    if (isSplitActive.value) {
      splitView.value?.setLiveContent(newContent);
    }
  }
});

// Update editor language mode
watch(editorLanguageMode, (newMode) => {
  if (editor.value) {
    editor.value.session.setMode(newMode);
  }
});

// Update read-only state
watch(editorReadOnly, (isReadOnly) => {
  if (editor.value) {
    editor.value.setReadOnly(isReadOnly);
  }
});

// Update theme when dark mode changes
watch(isDarkMode, (newValue) => {
  if (editor.value) {
    editor.value.setTheme(newValue ? THEME_DARK : THEME_LIGHT);
  }
});

// Initialize navigation when state syncs for file editing
watch(isStateSynced, (synced) => {
  if (synced && !props.viewerMode && req.value) {
    initializeNavigation();
  }
});

watch(() => state.editor.scrollRatio, () => {
  if (props.viewerMode || !isMarkdownFile.value) return;
  if (state.editor.scrollSource === 'editor') return;
  splitView.value?.applyScrollRatio(state.editor.scrollRatio);
});

watch(isSplitActive, () => {
  void nextTick(() => {
    if (editor.value) editor.value.resize();
  });
});

watch(() => state.editor.fontSize, applyFontSize);

watch(() => editorConfig.wrapEditorContent, applyWrap);

watch(editorAceOptions, (cfg) => applyAceOptions(cfg));

function scheduleSaveUnlock(delay: number) {
  if (saveUnlockTimer) {
    clearTimeout(saveUnlockTimer);
  }
  saveUnlockTimer = setTimeout(() => {
    saveUnlockTimer = null;
    if (!state.navigation.isTransitioning && req.value?.path === currentReqPath) {
      saveLocked = false;
    }
  }, delay);
}

function setupViewerResizeObserver() {
  if (typeof ResizeObserver === "undefined" || !editor.value) {
    return;
  }
  viewerResizeObserver = new ResizeObserver(() => {
    if (editor.value) {
      editor.value.resize();
    }
  });
  viewerResizeObserver.observe(editor.value.container);
  void nextTick(() => {
    if (editor.value) {
      editor.value.resize();
    }
  });
}

function initializeNavigation() {
  if (!req.value || req.value.type === 'directory') {
    return;
  }

  mutations.resetSelected();
  mutations.addSelected({
    name: req.value.name ?? "",
    path: req.value.path ?? "",
    size: req.value.size,
    type: req.value.type,
    source: req.value.source,
    modified: req.value.modified,
    hasPreview: req.value.hasPreview,
  });

  void updateNavigationForCurrentItem();
}

async function updateNavigationForCurrentItem() {
  if (!req.value || req.value.type === 'directory') {
    return;
  }
  let directoryPath = removeLastDir(req.value.path);

  // If directoryPath is empty, the file is in root - use '/' as the directory
  if (!directoryPath || directoryPath === '') {
    directoryPath = '/';
  }
  let listing: unknown;

  if (req.value.items) {
    listing = req.value.items;
  } else if (req.value.parentDirItems) {
    // Use pre-fetched parent directory items from Files.vue
    listing = req.value.parentDirItems;
  } else if (directoryPath !== req.value.path) {
    // Fetch directory listing (now with '/' for root files)
    try {
      let res: { items: unknown; };
      if (getters.isShare()) {
        res = await resourcesApi.fetchFilesPublic(directoryPath, state.shareInfo.hash);
      } else {
        res = await resourcesApi.fetchFiles(req.value.source, directoryPath);
      }
      listing = res.items;
    } catch (error) {
      console.error("error Editor.vue", error);
      listing = [req.value];
    }
  } else {
    listing = [req.value];
  }
  mutations.setupNavigation({
    listing: listing,
    currentItem: req.value,
    directoryPath: directoryPath
  });
}

function initializeEditor(initialScrollRatio: number = state.editor.scrollRatio) {
  if (!editorEl.value) {
    return;
  }
  try {
    ace.config.set(
      "basePath",
      `https://cdn.jsdelivr.net/npm/ace-builds@${ace_version}/src-min-noconflict/`
    );

    const editorInstance = ace.edit(editorEl.value, {
      mode: editorLanguageMode.value,
      value: editorContent.value,
      showPrintMargin: false,
      showGutter: editorConfig.showGutter,
      showLineNumbers: editorConfig.showLineNumbers,
      relativeLineNumbers: editorConfig.relativeLineNumbers,
      theme: isDarkMode.value ? THEME_DARK : THEME_LIGHT,
      readOnly: editorReadOnly.value,
      wrap: !!editorConfig.wrapEditorContent,
      indentedSoftWrap: editorConfig.indentedSoftWrap,
      enableMobileMenu: false,
      enableBasicAutocompletion: editorConfig.enableAutocompletion,
      enableLiveAutocompletion: editorConfig.enableAutocompletion && editorConfig.enableLiveAutocompletion,
      enableSnippets: editorConfig.enableAutocompletion,
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
    editor.value = editorInstance;

    savedContent = editorContent.value;
    editorInstance.session.getUndoManager().reset(); // To avoid redo to an empty file on fresh mount
    editorInstance.commands.removeCommand("showSettingsMenu");

    editorInstance.on('change', () => {
      if (editor.value !== editorInstance) return;
      if (suppressDirtyTracking) return;
      scheduleStatsUpdate(editorInstance);
      splitView.value?.handleEditorChange();
    });
    // Initialize navigation for file editing mode when synced
    if (isStateSynced.value && !props.viewerMode) {
      initializeNavigation();
    }
    updateEditorStats();
    editorInstance.selection.on('changeSelection', () => {
      updateEditorStats();
    });
    if (!props.viewerMode) {
      if (isMarkdownFile.value) {
        void nextTick(() => {
          if (editor.value !== editorInstance) return;
          if (isSplitActive.value) {
            splitView.value?.setLiveContent(editorInstance.getValue());
          }
          splitView.value?.applyScrollRatio(initialScrollRatio, true);
          requestAnimationFrame(() => {
            requestAnimationFrame(() => {
              if (editor.value !== editorInstance) return;
              editorInstance.session.on('changeScrollTop', handleEditorScroll);
            });
          });
        });
      } else {
        editorInstance.session.on('changeScrollTop', handleEditorScroll);
      }
    }
  } catch (_e) {
    notify.showError(t("editor.uninitialized"));
  }
}

function getValue(): string {
  return editor.value?.getValue() ?? editorContent.value;
}

function getAceMode(mode: string): string {
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
}

async function handleEditorValueRequest() {
  // Skip save logic in viewer mode
  if (props.viewerMode) {
    return;
  }
  // Check if navigation is transitioning
  if (state.navigation.isTransitioning) {
    const errorMsg = "Please wait for navigation to complete before saving.";
    notify.showError(errorMsg);
    throw new Error(errorMsg);
  }
  // Check if save is locked due to req transition
  if (saveLocked) {
    const errorMsg = "Please wait a moment before saving.";
    notify.showError(errorMsg);
    throw new Error(errorMsg);
  }
  // Filename protection - ensure state is synced before saving
  const original = originalReq.value;
  if (!isStateSynced.value || !original) {
    const errorMsg = t("editor.saveAbortedMessage", {
      activeFile: original?.name || "unknown",
      tryingToSave: routeFilename.value || "unknown"
    });
    notify.showError(errorMsg);
    throw new Error(errorMsg);
  }
  if (!editor.value) {
    const errorMsg = t("editor.uninitialized");
    notify.showError(errorMsg);
    throw new Error(errorMsg);
  }

  const content = editor.value.getValue();
  const newBytes = new TextEncoder().encode(content).length;
  const oldBytes = original.size ?? 0;
  const quotaPath = removeLastDir(original.path) || "/";
  if (await rejectPutIfQuotaExceeded(quotaPath, newBytes, oldBytes)) {
    const errorMsg = t("quotas.errors.exceeded");
    throw new Error(errorMsg);
  }
  if (getters.isShare()) {
    // Save the file
    await resourcesApi.putPublic(state.shareInfo.hash, original.path, content);
  } else {
    // Save the file
    await resourcesApi.put(original.source, original.path, content);
  }

  notify.showSuccessToast(`${original.name} saved successfully.`);
  savedContent = editor.value.getValue();
  mutations.setRequestContent(savedContent);
  isDirty = false;
  mutations.setEditorDirty(false);
}

function stopEnterPropagation(event: KeyboardEvent) {
  if (event.key === "Enter") {
    event.stopPropagation();
  }
}

function keyEvent(event: KeyboardEvent) {
  const { key, ctrlKey, metaKey } = event;
  if (getters.currentPromptName()) return;
  if ((ctrlKey || metaKey) && key === ",") {
    event.preventDefault();
    event.stopPropagation();
    openEditorSettings();
    return;
  }
  // Skip save shortcut in viewer mode
  if (props.viewerMode) return;
  if ((ctrlKey || metaKey) && key.toLowerCase() === "s") {
    event.preventDefault();
    handleEditorValueRequest().catch(() => { /* ignore */ });
  }
}

function openEditorSettings() {
  mutations.showPrompt({
    name: "EditorSettings",
  });
}

function setupNavigationGuard() {
  if (props.viewerMode) return;

  navigationGuard = router.beforeEach((to, from, next) => {
    // If prompt is already open, block any new navigation attempts
    if (isPromptOpen) {
      if (getters.currentPromptName() === "SaveBeforeExit") {
        next(false);
        return;
      }
      isPromptOpen = false;
      pendingNavigation = null;
    }
    // Check if we are navigating to a different route
    const isDifferentRoute = to.path !== from.path || to.hash !== from.hash;

    if (isDirty && !props.viewerMode && isDifferentRoute && req.value) {
      next(false);
      pendingNavigation = to;
      showSaveBeforeExitPrompt();
      return;
    }
    next();
  });
}

function showSaveBeforeExitPrompt() {
  isPromptOpen = true;
  mutations.showPrompt({
    name: "SaveBeforeExit",
    pinned: true,
    confirm: async () => {
      try {
        await handleEditorValueRequest();
      } catch (_e) {
        isPromptOpen = false;
        return;
      }
      isDirty = false;
      mutations.setEditorDirty(false);
      executePendingNavigation();
    },
    discard: () => {
      // Discard changes and exit
      isDirty = false;
      mutations.setEditorDirty(false);
      executePendingNavigation();
    },
    cancel: () => {
      // Keep editing - block navigation
      cancelPendingNavigation();
    },
  });
}

function executePendingNavigation() {
  isPromptOpen = false;
  const target = pendingNavigation;
  pendingNavigation = null;
  if (target) {
    void router.push(target.fullPath);
  }
}

function cancelPendingNavigation() {
  isPromptOpen = false;
  pendingNavigation = null;
}

function getSelectedStats() {
  if (!editor.value) return undefined;
  const session = editor.value.session;
  const selectionRange = editor.value.selection.getRange();
  const isSelectionEmpty =
    selectionRange.start.row === selectionRange.end.row &&
    selectionRange.start.column === selectionRange.end.column;

  let text: string, lines: number;
  if (!isSelectionEmpty) {
    text = editor.value.getSelectedText();
    lines = text ? text.split('\n').length : 0;
  } else {
    text = session.getValue();
    lines = session.getLength();
  }

  const chars = text.length;
  const validWord = text.split(/\s+/).filter((word: string) => /[a-zA-Z0-9]/.test(word));
  const words = validWord.length;

  return { lines, words, chars };
}

function updateEditorStats() {
  if (!editor.value) return;
  const stats = getSelectedStats();
  if (!stats) return;
  const { lines, words, chars } = stats;
  if (isMarkdownFile.value) {
    mutations.setEditorStats({ lines, words, chars });
  } else {
    // For other files, show only lines
    // Just lines because will be a bit misleading to count words if we are viewing code for example
    mutations.setEditorStats({ lines, words: null, chars: null });
  }
}

function scheduleStatsUpdate(editorInstance?: Ace.Editor) {
  if (statsUpdateTimer) {
    clearTimeout(statsUpdateTimer);
  }
  statsUpdateTimer = setTimeout(() => {
    statsUpdateTimer = null;
    updateEditorStats();
    if (editorInstance && editor.value === editorInstance) {
      const dirty = editorInstance.getValue() !== savedContent;
      if (isDirty !== dirty) {
        isDirty = dirty;
        mutations.setEditorDirty(dirty);
      }
    }
  }, 150);
}

function applyFontSize() {
  if (editor.value) {
    editor.value.setOption('fontSize', `${state.editor.fontSize}px`);
  }
}

function applyWrap() {
  if (editor.value) {
    editor.value.setOption('wrap', !!editorConfig.wrapEditorContent);
  }
}

function applyAceOptions(cfg: Omit<EditorConfig, "wrapEditorContent"> = editorConfig) {
  if (!editor.value) return;
  const wasRelative = editor.value.getOption('relativeLineNumbers');
  editor.value.setOption('keyboardHandler', cfg.keybinding || null);
  editor.value.setOption('tabSize', cfg.tabSize);
  editor.value.setOption('scrollPastEnd', cfg.overscroll);
  editor.value.setOption('indentedSoftWrap', cfg.indentedSoftWrap);
  editor.value.setOption('displayIndentGuides', cfg.showIndentGuides);
  editor.value.setOption('showGutter', cfg.showGutter);
  editor.value.setOption('fixedWidthGutter', cfg.fixedGutterWidth);
  editor.value.setOption('showLineNumbers', cfg.showLineNumbers);
  editor.value.setOption('relativeLineNumbers', cfg.relativeLineNumbers);
  editor.value.setOption('customScrollbar', cfg.customScrollbar);
  editor.value.setOption('enableBasicAutocompletion', cfg.enableAutocompletion);
  editor.value.setOption('enableLiveAutocompletion', cfg.enableAutocompletion && cfg.enableLiveAutocompletion);
  editor.value.setOption('enableSnippets', cfg.enableAutocompletion);
  if (wasRelative && !cfg.relativeLineNumbers && cfg.showLineNumbers) {
    const gutterLayer = (editor.value.renderer as unknown as AceRendererInternal).$gutterLayer;
    if (gutterLayer?.$renderer) {
      gutterLayer.$renderer = null;
    }
  }
}

function handleEditorScroll() {
  splitView.value?.handleEditorScroll();
}

// Show generic browser dialog if the user closes the tab, or try to close the browser with unsaved changes
const beforeUnloadHandler = (event: BeforeUnloadEvent) => {
  if (isDirty && !props.viewerMode) {
    event.preventDefault();
  }
};

window.addEventListener("keydown", keyEvent, true);
window.addEventListener("beforeunload", beforeUnloadHandler);
setupNavigationGuard();

onMounted(() => {
  resizeContainerEl.value = editorRoot.value;
  resizeContainerEl.value?.addEventListener("keydown", stopEnterPropagation); // to avoid trigger prompts primary button when the editor is embedded
  if (props.viewerMode) {
    void nextTick(() => {
      void nextTick(() => {
        initializeEditor();
        applyFontSize();
        setupViewerResizeObserver();
      });
    });
    return;
  }

  originalReq.value = req.value;
  currentReqPath = req.value?.path ?? null;
  if (isMarkdownFile.value && req.value?.path) {
    mutations.resetEditorScrollRatio(req.value.path);
  }
  initializeEditor(state.editor.scrollRatio);
  // Register save handler so other components can trigger save
  mutations.setEditorSaveHandler(() => handleEditorValueRequest());
  applyFontSize();
  setupViewerResizeObserver();
});

onBeforeUnmount(() => {
  if (saveUnlockTimer) {
    clearTimeout(saveUnlockTimer);
    saveUnlockTimer = null;
  }
  if (statsUpdateTimer) {
    clearTimeout(statsUpdateTimer);
    statsUpdateTimer = null;
  }
  if (viewerResizeObserver) {
    viewerResizeObserver.disconnect();
    viewerResizeObserver = null;
  }

  window.removeEventListener("keydown", keyEvent, true);
  window.removeEventListener("beforeunload", beforeUnloadHandler);

  if (editor.value) {
    editor.value.session.off('changeScrollTop', handleEditorScroll);
  }
  // Clear navigation guard
  if (navigationGuard) {
    navigationGuard();
  }
  // Clear dirty state and save handler when leaving editor
  mutations.setEditorDirty(false);
  mutations.setEditorSaveHandler(null);
  mutations.setEditorStats({ lines: 0, words: 0, chars: 0 });
});

onUnmounted(() => {
  resizeContainerEl.value?.removeEventListener("keydown", stopEnterPropagation);
  resizeContainerEl.value = null;
  if (editor.value) {
    editor.value.destroy();
    editor.value = null;
  }
});

defineExpose({ getValue });
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

#editor-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
  overflow: hidden;
  position: relative;
}

#editor-root.split-active #editor-container {
  flex: 0 0 auto;
  min-width: 0;
  position: relative;
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
  z-index: 1 !important;

  --ace-dark-bg: #151515;
  --ace-popup-bg: #1a1a1a;
  --ace-control-bg: #232323;
}

/* make sure the text selection is detected */
.ace_content {
  user-select: text;
}

/* Text selection color */
.ace_editor .ace_selection {
  background-color: color-mix(in srgb, var(--primaryColor) 25%, transparent) !important;
}

.ace_editor .ace_selection.ace_start {
  box-shadow: 0 0 3px 0 color-mix(in srgb, var(--primaryColor) 40%, transparent) !important;
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

.ace_editor.ace_dark {
  background-color: var(--ace-dark-bg) !important;
}

.ace_editor.ace_dark .ace_marker-layer .ace_active-line {
  background: color-mix(in srgb, var(--surfaceSecondary) 35%, transparent);
}

#editor-root.split-active .ace_scrollbar {
  scrollbar-width: none;
}

#editor-root.split-active .ace_scrollbar::-webkit-scrollbar {
  display: none;
}

.ace_editor .ace_search {
  color: var(--textPrimary);
  border-color: var(--divider);
}

.ace_editor.ace_dark .ace_search {
  background-color: var(--ace-popup-bg);
}

.ace_editor .ace_search_field {
  color: var(--textPrimary);
  border: 1px solid var(--divider);
}

.ace_editor.ace_dark .ace_search_field {
  background-color: var(--ace-dark-bg);
}

.ace_editor .ace_search .ace_button,
.ace_editor .ace_search .ace_searchbtn,
.ace_editor .ace_search .ace_replacebtn {
  border-color: var(--divider);
  color: var(--textPrimary);
}

.ace_editor.ace_dark .ace_search .ace_button,
.ace_editor.ace_dark .ace_search .ace_searchbtn,
.ace_editor.ace_dark .ace_search .ace_replacebtn {
  background-color: var(--ace-control-bg);
}

.ace_editor .ace_search .ace_button:hover,
.ace_editor .ace_search .ace_searchbtn:hover {
  background-color: var(--hoverOverlay);
}

.ace_editor.ace_autocomplete .ace_marker-layer .ace_active-line {
  background-color: color-mix(in srgb, var(--primaryColor) 16%, transparent) !important;
}

.ace_editor.ace_autocomplete .ace_marker-layer .ace_line-hover,
.ace_editor.ace_autocomplete .ace_marker-layer .ace_line {
  border-color: color-mix(in srgb, var(--primaryColor) 30%, transparent) !important;
  background: color-mix(in srgb, var(--primaryColor) 15%, transparent) !important;
}

.ace_editor.ace_autocomplete .ace_completion-highlight {
  color: var(--primaryColor) !important;
}

.ace_prompt_container {
  background: var(--ace-popup-bg) !important;
}

.ace_prompt_container .ace-tm {
  background-color: var(--ace-popup-bg) !important;
  color: var(--textPrimary) !important;
  border: 1px solid color-mix(in srgb, var(--divider) 45%, transparent);
}

.ace_prompt_container .ace-tm .ace_cursor {
  color: var(--textPrimary) !important;
}

.ace_editor .ace_tooltip,
.ace_editor .ace_doc-tooltip {
  color: var(--textSecondary);
  border: 1px solid color-mix(in srgb, var(--divider) 45%, transparent);
}

.ace_editor.ace_dark .ace_tooltip,
.ace_editor.ace_dark .ace_doc-tooltip {
  background-color: var(--ace-popup-bg);
}

</style>
