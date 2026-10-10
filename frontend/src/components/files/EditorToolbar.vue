<template>
  <div class="editor-toolbar no-select">
    <div class="editor-toolbar-sticky">
      <div
        v-for="btn in stickyToolbarButtons"
        :key="btn.id"
        class="md-toolbar-group"
      >
        <button
          type="button"
          class="editor-toolbar-btn"
          :title="btn.title"
          :aria-label="btn.title"
          :disabled="btn.disabled"
          @mousedown.prevent
          @click="btn.action"
        >
          <i class="material-symbols">{{ btn.icon }}</i>
        </button>
      </div>
    </div>
    <div
      v-for="btn in editorToolbarButtons"
      :key="btn.id"
      class="md-toolbar-group"
    >
      <button
        v-if="!btn.color && !btn.menu"
        type="button"
        class="editor-toolbar-btn"
        :title="btn.title"
        :aria-label="btn.title"
        :disabled="btn.disabled"
        @mousedown.prevent
        @click="btn.action"
      >
        <i class="material-symbols">{{ btn.icon }}</i>
      </button>
      <template v-else-if="btn.menu === 'align' || btn.menu === 'clipboard'">
        <button
          :ref="(el) => setIconMenuTriggerEl(btn.menu as 'align' | 'clipboard', el as HTMLElement | null)"
          type="button"
          class="editor-toolbar-btn"
          :title="btn.title"
          :aria-label="btn.title"
          @mousedown.prevent
          @click="toggleMenu(btn.menu)"
        >
          <i class="material-symbols">{{ btn.icon }}</i>
        </button>
        <Teleport to="body">
          <transition name="expand" @before-enter="expandBeforeEnter" @enter="expandEnter" @leave="expandLeave">
            <ul v-if="openMenu === btn.menu" :ref="(el) => setIconMenuEl(btn.menu as 'align' | 'clipboard', el as HTMLElement | null)" class="editor-toolbar-menu editor-toolbar-menu--icon-menu floating-window border-radius" :style="menuStyle">
              <li v-for="item in iconMenuItems(btn.menu)" :key="item.id">
                <button
                  type="button"
                  class="editor-toolbar-btn"
                  :title="item.title"
                  :aria-label="item.title"
                  :disabled="item.disabled"
                  @mousedown.prevent
                  @click="extBtnAction(item)"
                >
                  <i class="material-symbols">{{ item.icon }}</i>
                </button>
              </li>
            </ul>
          </transition>
        </Teleport>
      </template>
      <div v-else class="editor-toolbar-btn clickable">
        <button
          type="button"
          class="editor-toolbar-btn"
          :title="btn.title"
          :aria-label="btn.title"
          :disabled="btn.disabled"
          @mousedown.prevent
          @click="applyStoredColor(btn)"
        >
          <i class="material-symbols" :class="{ 'toolbar-color-glyph': selectedColor(btn) }">{{ btn.icon }}</i>
        </button>
        <span
          class="toolbar-color-indicator"
          :class="{ 'toolbar-color-indicator--empty': !selectedColor(btn) }"
          :style="selectedColor(btn) ? { backgroundColor: selectedColor(btn) } : null"
        >
          <input
            :ref="(el) => setColorInput(el as HTMLInputElement | null, btn.color ?? '')"
            type="color"
            class="color-input"
            :value="selectedColor(btn)"
            :aria-label="btn.title"
            @mousedown.stop
            @change="onColorChange(btn.color ?? '', ($event.target as HTMLInputElement).value, btn.applyColor)"
          />
        </span>
      </div>
    </div>
    <div class="editor-toolbar-sticky editor-toolbar-sticky--right">
      <button
        ref="extraMenuTrigger"
        type="button"
        class="editor-toolbar-btn"
        :title="$t('editor.md.moreOptions')"
        :aria-label="$t('editor.md.moreOptions')"
        @mousedown.prevent
        @click="toggleMenu('extra')"
      >
        <i class="material-symbols">more_horiz</i>
      </button>
      <Teleport to="body">
        <transition name="expand" @before-enter="expandBeforeEnter" @enter="expandEnter" @leave="expandLeave">
          <ul v-if="openMenu === 'extra'" ref="extraMenu" class="editor-toolbar-menu floating-window border-radius" :style="menuStyle">
            <li v-for="item in extraMenuItems" :key="item.id">
              <button
                type="button"
                class="editor-toolbar-btn editor-toolbar-menu-btn"
                :title="item.title"
                @mousedown.prevent
                @click="extBtnAction(item)"
              >
                <i class="material-symbols">{{ item.icon }}</i>
                <span>{{ item.title }}</span>
              </button>
            </li>
          </ul>
        </transition>
      </Teleport>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import ace from "ace-builds";
import type { Ace } from "ace-builds";
import { mutations, state, getters } from "@/store";
import { eventBus } from "@/store/eventBus";
import { removeLastDir } from "@/utils/url.js";
import { expandBeforeEnter, expandEnter, expandLeave } from "@/utils/expandTransition";
import { copyToClipboard } from "@/utils/clipboard.js";
import { notify } from "@/notify";

interface AnchorRange {
  start: Ace.Anchor;
  end: Ace.Anchor;
}

interface PendingSelection {
  kind: "image" | "video" | "audio";
  contextId: string;
  alt: string;
  range: AnchorRange;
}

const props = withDefaults(defineProps<{
  editor?: Ace.Editor | null;
  isMarkdown?: boolean;
  showSave?: boolean;
  saveHandler?: (() => Promise<void>) | null;
}>(), {
  editor: null,
  isMarkdown: false,
  showSave: true,
  saveHandler: null,
});


function formatImageDestination(dest: string): string {
  if (/[\s()]/.test(dest)) {
    return `<${dest.replace(/([\\<>])/g, "\\$1")}>`;
  }
  return dest;
}

function formatImageAltText(alt: string): string {
  return alt.replace(/([\\[\]])/g, "\\$1");
}

function formatHtmlAttrValue(value: string): string {
  return value.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

function advancePosition(pos: Ace.Point, str: string): Ace.Point {
  const parts = str.split("\n");
  if (parts.length === 1) return { row: pos.row, column: pos.column + str.length };
  return { row: pos.row + parts.length - 1, column: parts.at(-1)?.length ?? 0 };
}

interface ToolbarButton {
  id: string;
  icon: string;
  title: string;
  action?: () => void;
  disabled?: boolean;
  color?: string;
  applyColor?: (color: string) => void;
  sticky?: boolean;
  menu?: "align" | "clipboard";
}

defineOptions({ name: "editorToolbar" });

const { t } = useI18n();

const canUndo = ref(false);
const canRedo = ref(false);
const saveState = ref<"idle" | "saving" | "success" | "error">("idle");
let saveResetTimer: ReturnType<typeof setTimeout> | null = null;
const pendingSelection = ref<PendingSelection | null>(null);
const openMenu = ref<"extra" | "align" | "clipboard" | null>(null);
const menuPosition = ref({ top: 0, left: 0, right: 0 });
const lastColors = reactive(new Map<string, string>([
  ["mdFontColor", localStorage.getItem("mdFontColor") || ""],
  ["mdHighlightColor", localStorage.getItem("mdHighlightColor") || ""],
]));
const colorInputRefs = new Map<string, HTMLInputElement>();
const iconMenuTriggerEls = new Map<"align" | "clipboard", HTMLElement>();
const iconMenuEls = new Map<"align" | "clipboard", HTMLElement>();
const extraMenuTrigger = ref<HTMLElement | null>(null);
const extraMenu = ref<HTMLElement | null>(null);

const toolbarButtons = computed((): ToolbarButton[] => {
  // alwaysAvailable are for buttons that will be available for all filetypes, not just markdown.
  const alwaysAvailable: ToolbarButton[] = [
    { id: "undo", icon: "undo", title: t("editor.md.undo"), action: () => undo(), disabled: !canUndo.value, sticky: true },
    { id: "redo", icon: "redo", title: t("editor.md.redo"), action: () => redo(), disabled: !canRedo.value, sticky: true },
    { id: "find", icon: "search", title: t("general.search"), action: () => openFind() },
  ];
  if (props.showSave) {
    alwaysAvailable.unshift({
      id: "save",
      icon: { idle: "save", saving: "save", success: "check", error: "error" }[saveState.value],
      title: t("general.save"),
      action: () => save(),
      disabled: saveState.value === "saving",
      sticky: true,
    });
  }
  const isJson = state.req?.type === "application/json"
  if (isJson && getters.sourcePermissions().modify) {
    alwaysAvailable.push({
      id: "formatJSON",
      icon: "data_object",
      title: t("editor.json.formatJSON"),
      action: () => formatJSON(),
    });
  }
  if (!props.isMarkdown) {
    return [
      ...alwaysAvailable,
      ...clipboardMenuItems.value,
    ];
  }
  return [
    ...alwaysAvailable,
    { id: "clipboard", icon: "content_paste", title: t("editor.md.clipboardActions"), menu: "clipboard" },
    { id: "bold", icon: "format_bold", title: t("editor.md.bold"), action: () => wrapSelection("**", "**") },
    { id: "italic", icon: "format_italic", title: t("editor.md.italic"), action: () => wrapSelection("_", "_") },
    { id: "strikethrough", icon: "strikethrough_s", title: t("editor.md.strikethrough"), action: () => wrapSelection("~~", "~~") },
    { id: "heading", icon: "title", title: t("editor.md.heading"), action: () => cycleHeading() },
    { id: "quote", icon: "format_quote", title: t("editor.md.quote"), action: () => toggleLinePrefix("> ") },
    { id: "image", icon: "image", title: t("fileTypes.image"), action: () => insertImage() },
    { id: "align", icon: "format_align_left", title: t("editor.md.align"), menu: "align" },
    { id: "bulletList", icon: "format_list_bulleted", title: t("editor.md.bulletList"), action: () => toggleLinePrefix("- ") },
    { id: "numberedList", icon: "format_list_numbered", title: t("editor.md.numberedList"), action: () => applyNumberedList() },
    { id: "taskList", icon: "checklist", title: t("editor.md.taskList"), action: () => toggleTaskList() },
    { id: "fontColor", icon: "font_download", title: t("editor.md.fontColor"), color: "mdFontColor", applyColor: applyFontColor },
    { id: "highlight", icon: "ink_highlighter", title: t("editor.md.highlight"), color: "mdHighlightColor", applyColor: applyHighlightColor },
  ];
});

const stickyToolbarButtons = computed((): ToolbarButton[] => {
  return toolbarButtons.value.filter((btn) => btn.sticky);
});

const editorToolbarButtons = computed((): ToolbarButton[] => {
  return toolbarButtons.value.filter((btn) => !btn.sticky);
});

const alignMenuItems = computed((): ToolbarButton[] => {
  return [
    { id: "alignLeft", icon: "format_align_left", title: t("editor.md.alignLeft"), action: () => wrapSelection('<p align="left">', "</p>", t("editor.md.text")) },
    { id: "alignCenter", icon: "format_align_center", title: t("editor.md.alignCenter"), action: () => wrapSelection("<center>", "</center>", t("editor.md.text")) },
    { id: "alignRight", icon: "format_align_right", title: t("editor.md.alignRight"), action: () => wrapSelection('<p align="right">', "</p>", t("editor.md.text")) },
    { id: "alignJustify", icon: "format_align_justify", title: t("editor.md.justify"), action: () => wrapSelection('<p align="justify">', "</p>", t("editor.md.text")) },
  ];
});

const clipboardMenuItems = computed((): ToolbarButton[] => {
  return [
    { id: "copy", icon: "content_copy", title: t("general.copy"), action: () => copySelection() },
    { id: "cut", icon: "content_cut", title: t("general.cut"), action: () => cutSelection() },
    { id: "paste", icon: "content_paste", title: t("general.paste"), action: () => pasteClipboard() },
    { id: "selectAll", icon: "select_all", title: t("buttons.selectAll"), action: () => selectAllText() },
  ];
});

const extraMenuItems = computed((): ToolbarButton[] => {
  const editorSettings: ToolbarButton = {
    id: "editorSettings",
    icon: "settings",
    title: t("editor.settings.title"),
    action: () => openEditorSettings(),
  };
  if (!props.isMarkdown) {
    return [editorSettings];
  }
  return [
    editorSettings,
    { id: "code", icon: "code", title: t("editor.md.inlineCode"), action: () => wrapSelection("`", "`") },
    { id: "codeBlock", icon: "code_blocks", title: t("editor.md.codeBlock"), action: () => insertCodeBlock() },
    { id: "video", icon: "videocam", title: t("fileTypes.video"), action: () => insertVideo() },
    { id: "audio", icon: "music_note", title: t("fileTypes.audio"), action: () => insertAudio() },
    { id: "table", icon: "table", title: t("tools.activityViewer.tableView"), action: () => insertTable() },
    { id: "horizontalRule", icon: "horizontal_rule", title: t("editor.md.horizontalRule"), action: () => insertHorizontalRule() },
    { id: "inlineMath", icon: "functions", title: t("editor.md.inlineMath"), action: () => wrapSelection("$", "$", "E = mc^2") },
    { id: "displayMath", icon: "calculate", title: t("editor.md.displayMath"), action: () => wrapSelection("$$\n", "\n$$", "E = mc^2") },
    { id: "superscript", icon: "superscript", title: t("editor.md.superscript"), action: () => wrapSelection("<sup>", "</sup>", "2") },
    { id: "subscript", icon: "subscript", title: t("editor.md.subscript"), action: () => wrapSelection("<sub>", "</sub>", "2") },
    { id: "kbd", icon: "keyboard", title: t("threejs.keyboard"), action: () => wrapSelection("<kbd>", "</kbd>", "Ctrl") },
    { id: "link", icon: "link", title: t("general.links"), action: () => insertLink() },
  ];
});

const menuStyle = computed(() => {
  if (openMenu.value === "align" || openMenu.value === "clipboard") {
    return { top: `${menuPosition.value.top}px`, left: `${menuPosition.value.left}px`, transform: "translateX(-50%)" };
  }
  return { top: `${menuPosition.value.top}px`, right: `${menuPosition.value.right}px` };
});

function focusEditor() {
  if (props.editor) props.editor.focus();
}

function formatJSON() {
  const editor = props.editor;
  if (!editor) return;
  try {
    const textToFormat = editor.getValue();
    const parsed = JSON.parse(textToFormat);
    const next = state.editor.jsonFormatted
      ? JSON.stringify(parsed)
      : JSON.stringify(parsed, null, 2);
    if (next !== textToFormat) editor.setValue(next, -1);
    mutations.setEditorJsonFormatted(!state.editor.jsonFormatted);
    notify.showSuccessToast(t("editor.json.formatJSONSuccess"));
  } catch (e) {
    notify.showErrorToast(t("editor.json.invalidJSON", { message: e instanceof Error ? e.message : String(e) }));
  }
  focusEditor();
}

async function save() {
  if (!props.saveHandler || saveState.value === "saving") return;
  if (saveResetTimer) clearTimeout(saveResetTimer);
  saveState.value = "saving";
  try {
    await props.saveHandler();
    saveState.value = "success";
  } catch (_e) {
    // the editor already shows the error message
    saveState.value = "error";
  }
  saveResetTimer = setTimeout(() => {
    saveState.value = "idle";
  }, 1500);
  focusEditor();
}

function undo() {
  const editor = props.editor;
  if (!editor) return;
  editor.undo();
  refreshUndoState();
  focusEditor();
}

function redo() {
  const editor = props.editor;
  if (!editor) return;
  editor.redo();
  refreshUndoState();
  focusEditor();
}

function openFind() {
  const editor = props.editor;
  if (!editor) return;
  editor.execCommand("find");
}

function attachUndoListener(editor: Ace.Editor | null) {
  if (!editor) return;
  editor.session.on("change", refreshUndoState);
  refreshUndoState();
}

function detachUndoListener(editor: Ace.Editor | null) {
  if (!editor) return;
  editor.session.off("change", refreshUndoState);
}

function refreshUndoState() {
  const editor = props.editor;
  if (!editor) {
    canUndo.value = false;
    canRedo.value = false;
    return;
  }
  const undoManager = editor.session.getUndoManager();
  canUndo.value = undoManager.hasUndo();
  canRedo.value = undoManager.hasRedo();
}

function copySelection() {
  const editor = props.editor;
  if (!editor) return;
  const text = editor.getCopyText();
  if (text) void copyToClipboard(text);
  focusEditor();
}

function cutSelection() {
  const editor = props.editor;
  if (!editor) return;
  const text = editor.getCopyText();
  if (text) void copyToClipboard(text);
  editor.execCommand("cut");
  focusEditor();
}

async function pasteClipboard() {
  const editor = props.editor;
  if (!editor) return;
  focusEditor();
  try {
    const text = await navigator.clipboard.readText();
    if (text) {
      editor.execCommand("paste", { text });
    }
  } catch (_e) { /* ignore - probably blocked by browser */ }
}

function selectAllText() {
  const editor = props.editor;
  if (!editor) return;
  editor.execCommand("selectall");
}

function selectedLineRange() {
  const range = props.editor?.getSelectionRange();
  if (!range) return { startRow: 0, endRow: 0 };
  let endRow = range.end.row;
  if (endRow > range.start.row && range.end.column === 0) {
    endRow -= 1;
  }
  return { startRow: range.start.row, endRow };
}

function wrapSelection(before: string, after: string = before, placeholder: string = "") {
  const editor = props.editor;
  if (!editor) return;
  const range = editor.getSelectionRange();
  const selectedText = editor.getSelectedText();
  const trailingNewline = selectedText.match(/\r?\n$/)?.[0] || "";
  const text = selectedText ? selectedText.slice(0, selectedText.length - trailingNewline.length) : placeholder;
  const start = { row: range.start.row, column: range.start.column };
  if (selectedText) {
    editor.session.replace(range, `${before}${text}${after}${trailingNewline}`);
  } else {
    editor.session.insert(start, `${before}${text}${after}`);
  }
  const contentStart = advancePosition(start, before);
  const contentEnd = advancePosition(contentStart, text);
  editor.selection.setRange({ start: contentStart, end: contentEnd });
  focusEditor();
}

function applyFontColor(color: string) {
  wrapSelection(`<font color="${color}">`, "</font>", t("editor.md.text"));
}

function applyHighlightColor(color: string) {
  const style = color ? ` style="background-color: ${color}; --mark-color: ${color}"` : "";
  wrapSelection(`<mark${style}>`, "</mark>", t("editor.md.highlight"));
}

function selectedColor(btn: ToolbarButton): string {
  return (btn.color && lastColors.get(btn.color)) || "";
}

function applyStoredColor(btn: ToolbarButton) {
  if (btn.disabled || !btn.color || !btn.applyColor) return;
  const stored = selectedColor(btn);
  if (!stored) {
    colorInputRefs.get(btn.color)?.click();
    return;
  }
  btn.applyColor(stored);
}

function setColorInput(el: HTMLInputElement | null, color: string) {
  if (el) colorInputRefs.set(color, el);
}

function onColorChange(storageKey: string, color: string, apply?: (color: string) => void) {
  localStorage.setItem(storageKey, color);
  lastColors.set(storageKey, color);
  apply?.(color);
}

function toggleLinePrefix(prefix: string) {
  const editor = props.editor;
  if (!editor) return;
  const { startRow, endRow } = selectedLineRange();
  const session = editor.session;
  const lines = [];
  for (let row = startRow; row <= endRow; row++) {
    lines.push(session.getLine(row));
  }
  const nonBlank = lines.filter((line) => line.trim() !== "");
  const allPrefixed = (nonBlank.length ? nonBlank : lines).every((line) => line.startsWith(prefix));
  for (let row = startRow; row <= endRow; row++) {
    const line = session.getLine(row);
    if (allPrefixed) {
      if (line.startsWith(prefix)) {
        session.replace({ start: { row, column: 0 }, end: { row, column: prefix.length } }, "");
      }
    } else if (
      !line.startsWith(prefix)
      && (prefix === "> " || nonBlank.length === 0 || line.trim() !== "")
    ) {
      session.insert({ row, column: 0 }, prefix);
    }
  }
  focusEditor();
}

function applyNumberedList() {
  const editor = props.editor;
  if (!editor) return;
  const { startRow, endRow } = selectedLineRange();
  const session = editor.session;
  const lines = [];
  for (let row = startRow; row <= endRow; row++) {
    lines.push(session.getLine(row));
  }
  const nonBlank = lines.filter((line) => line.trim() !== "");
  const alreadyNumbered = (nonBlank.length ? nonBlank : lines).every((line) => /^\d+\.\s/.test(line));
  let num = 1;
  for (let row = startRow; row <= endRow; row++) {
    const line = session.getLine(row);
    const match = line.match(/^\d+\.\s/);
    if (alreadyNumbered) {
      if (match) {
        session.replace({ start: { row, column: 0 }, end: { row, column: match[0].length } }, "");
      }
    } else if (line.trim() !== "") {
      const prefix = `${num}. `;
      if (match) {
        session.replace({ start: { row, column: 0 }, end: { row, column: match[0].length } }, prefix);
      } else {
        session.insert({ row, column: 0 }, prefix);
      }
      num++;
    }
  }
  focusEditor();
}

function cycleHeading() {
  const editor = props.editor;
  if (!editor) return;
  const { startRow } = selectedLineRange();
  const session = editor.session;
  const line = session.getLine(startRow);
  const match = line.match(/^(#{1,6})\s/);
  const currentLevel = match?.[1]?.length ?? 0;
  const nextLevel = currentLevel === 0 ? 1 : (currentLevel >= 6 ? 0 : currentLevel + 1);
  const stripped = line.replace(/^#{1,6}\s*/, "");
  const newLine = nextLevel === 0 ? stripped : `${"#".repeat(nextLevel)} ${stripped}`;
  session.replace({ start: { row: startRow, column: 0 }, end: { row: startRow, column: line.length } }, newLine);
  focusEditor();
}

function insertCodeBlock() {
  const editor = props.editor;
  if (!editor) return;
  const selectedText = editor.getSelectedText();
  const range = editor.getSelectionRange();
  if (selectedText) {
    editor.session.replace(range, `\`\`\`\n${  selectedText  }\n\`\`\``);
  } else {
    editor.insert("```\n\n```");
    const pos = editor.getCursorPosition();
    editor.moveCursorTo(pos.row - 1, 0);
  }
  focusEditor();
}

function insertLink() {
  const editor = props.editor;
  if (!editor) return;
  const selectedText = editor.getSelectedText();
  const range = editor.getSelectionRange();
  const label = formatImageAltText(selectedText || t("editor.md.text"));
  const linkText = `[${label}](url)`;
  let insertionEnd: Ace.Point;
  if (selectedText) {
    insertionEnd = editor.session.replace(range, linkText);
  } else {
    editor.insert(linkText);
    insertionEnd = editor.getCursorPosition();
  }
  const urlEnd = insertionEnd.column - 1; // before the closing ')'
  const urlStart = urlEnd - "url".length;
  if (urlStart >= 0) {
    editor.selection.setRange({
      start: { row: insertionEnd.row, column: urlStart },
      end: { row: insertionEnd.row, column: urlEnd },
    });
  }
  focusEditor();
}

function openEditorSettings() {
  mutations.showPrompt({
    name: "EditorSettings",
  });
}

function insertBlock(content: string) {
  const editor = props.editor;
  if (!editor) return;
  const pos = editor.getCursorPosition();
  const line = editor.session.getLine(pos.row);
  const needsNewlineBefore = line.trim() !== "";
  editor.moveCursorTo(pos.row, line.length);
  editor.clearSelection();
  editor.insert(`${needsNewlineBefore ? "\n\n" : ""}${content}`);
  focusEditor();
}

function insertHorizontalRule() {
  insertBlock("---\n\n");
}

function insertImage() {
  openPathPicker("image", { allowedFileTypes: ["image/"] });
}

function insertVideo() {
  openPathPicker("video", { allowedFileTypes: ["video/"] });
}

function insertAudio() {
  openPathPicker("audio", { allowedFileTypes: ["audio/"] });
}

function openPathPicker(kind: "image" | "video" | "audio", pickerProps: Record<string, unknown>) {
  const editor = props.editor;
  if (!editor) return;
  const selectedText = editor.getSelectedText();
  const range = editor.getSelectionRange();
  const contextId = `md-toolbar-${kind}-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
  pendingSelection.value = {
    kind,
    contextId,
    alt: kind === "image" ? (selectedText || "") : "",
    range: {
      start: editor.session.doc.createAnchor(range.start.row, range.start.column),
      end: editor.session.doc.createAnchor(range.end.row, range.end.column),
    },
  };
  props.editor?.blur();
  mutations.showPrompt({
    name: "pathPicker",
    pinned: true,
    props: {
      currentPath: state.req?.path ? removeLastDir(state.req.path) : "/",
      currentSource: state.req?.source || state.sources.current,
      hideDestinationSource: true,
      showFiles: true,
      showFolders: true,
      requireFileSelection: true,
      selectionContextId: contextId,
      ...pickerProps,
    },
  });
}

function clearPendingSelection() {
  pendingSelection.value?.range.start.detach();
  pendingSelection.value?.range.end.detach();
  pendingSelection.value = null;
}

function buildImageMd(path: string, alt: string): string {
  const fileName = path.split("/").filter(Boolean).pop() || "image";
  const altText = alt || fileName.replace(/\.[^./]+$/, "");
  return `![${formatImageAltText(altText)}](${formatImageDestination(path)})`;
}

function buildMediaMd(path: string, tag: "video" | "audio"): string {
  return `<${tag} src="${formatHtmlAttrValue(path)}" controls></${tag}>`;
}

function onPathSelected(data: { path?: string; selectionContextId?: string }) {
  const pending = pendingSelection.value;
  if (!pending || !data || data.selectionContextId !== pending.contextId) {
    return;
  }
  const editor = props.editor;
  const path = data.path;
  clearPendingSelection();
  if (!editor || typeof path !== "string") return;
  const text = pending.kind === "image"
    ? buildImageMd(path, pending.alt)
    : buildMediaMd(path, pending.kind);
  const start = pending.range.start.getPosition();
  const end = pending.range.end.getPosition();
  const range = new (ace.require("ace/range").Range)(start.row, start.column, end.row, end.column);
  const insertionEnd = editor.session.replace(range, text);
  editor.moveCursorTo(insertionEnd.row, insertionEnd.column);
  editor.clearSelection();
}

function onPathPickerCancelled(data: { selectionContextId?: string }) {
  if (!pendingSelection.value || !data || data.selectionContextId !== pendingSelection.value.contextId) {
    return;
  }
  clearPendingSelection();
}

function toggleTaskList() {
  const editor = props.editor;
  if (!editor) return;
  const { startRow, endRow } = selectedLineRange();
  const session = editor.session;
  const taskPrefix = /^- \[[ xX]\] /;
  const unchecked = /^- \[ \] /;
  const checked = /^- \[[xX]\] /;
  const lines = [];
  for (let row = startRow; row <= endRow; row++) {
    lines.push(session.getLine(row));
  }
  const nonBlank = lines.filter((line) => line.trim() !== "");
  const target = nonBlank.length ? nonBlank : lines;
  const allUnchecked = target.every((line) => unchecked.test(line));
  const allChecked = target.every((line) => checked.test(line));
  const action = allUnchecked ? "check" : allChecked ? "remove" : "add";
  for (let row = startRow; row <= endRow; row++) {
    const line = session.getLine(row);
    const match = line.match(taskPrefix);
    if (action === "remove") {
      if (match) session.replace({ start: { row, column: 0 }, end: { row, column: match[0].length } }, "");
    } else if (match) {
      session.replace({ start: { row, column: 3 }, end: { row, column: 4 } }, action === "check" ? "x" : " ");
    } else if (nonBlank.length === 0 || line.trim() !== "") {
      session.insert({ row, column: 0 }, "- [ ] ");
    }
  }
  focusEditor();
}

function onPointerDown(e: PointerEvent) {
  const target = e.target as Node;
  const insideIconMenus =
    Array.from(iconMenuTriggerEls.values()).some((el) => el.contains(target))
    || Array.from(iconMenuEls.values()).some((el) => el.contains(target));
  const inside =
    extraMenuTrigger.value?.contains(target)
    || extraMenu.value?.contains(target)
    || insideIconMenus;
  if (!inside) closeMenu();
}

function toggleMenu(name: "extra" | "align" | "clipboard") {
  if (openMenu.value === name) {
    closeMenu();
    return;
  }
  const trigger = name === "extra" ? (extraMenuTrigger.value ?? undefined) : iconMenuTriggerEls.get(name);
  if (trigger) {
    const rect = trigger.getBoundingClientRect();
    if (name === "extra") {
      // Right-anchor to the viewport edge so it can't overflow off-screen
      menuPosition.value = { top: rect.bottom + 4, left: 0, right: window.innerWidth - rect.right };
    } else {
      // Center under the button
      menuPosition.value = { top: rect.bottom + 4, left: rect.left + rect.width / 2, right: 0 };
    }
  }
  props.editor?.blur();
  openMenu.value = name;
}

function closeMenu() {
  openMenu.value = null;
}

function setIconMenuTriggerEl(menu: "align" | "clipboard", el: HTMLElement | null) {
  if (el) iconMenuTriggerEls.set(menu, el);
  else iconMenuTriggerEls.delete(menu);
}

function setIconMenuEl(menu: "align" | "clipboard", el: HTMLElement | null) {
  if (el) iconMenuEls.set(menu, el);
  else iconMenuEls.delete(menu);
}

function iconMenuItems(menu: "align" | "clipboard"): ToolbarButton[] {
  return menu === "align" ? alignMenuItems.value : clipboardMenuItems.value;
}

function onKeyDown(e: KeyboardEvent) {
  if (e.key === "Escape" && openMenu.value) {
    closeMenu();
    focusEditor();
  }
}

function extBtnAction(item: ToolbarButton) {
  closeMenu();
  item.action?.();
}

function insertTable() {
  const column = t("editor.md.column");
  const cell = t("editor.md.cell");
  insertBlock(`| ${column} 1 | ${column} 2 |\n| --- | --- |\n| ${cell} 1 | ${cell} 2 |\n`);
}

watch(() => props.editor, (newEditor, oldEditor) => {
  detachUndoListener(oldEditor ?? null);
  attachUndoListener(newEditor);
  if (oldEditor && oldEditor !== newEditor) {
    clearPendingSelection();
  }
}, { immediate: true });

onMounted(() => {
  eventBus.on("pathSelected", onPathSelected);
  eventBus.on("pathPickerCancelled", onPathPickerCancelled);
  document.addEventListener("pointerdown", onPointerDown);
  document.addEventListener("keydown", onKeyDown);
  window.addEventListener("scroll", closeMenu, true);
  window.addEventListener("resize", closeMenu);
});

onBeforeUnmount(() => {
  if (saveResetTimer) clearTimeout(saveResetTimer);
  detachUndoListener(props.editor);
  clearPendingSelection();
  eventBus.off("pathSelected", onPathSelected);
  eventBus.off("pathPickerCancelled", onPathPickerCancelled);
  document.removeEventListener("pointerdown", onPointerDown);
  document.removeEventListener("keydown", onKeyDown);
  window.removeEventListener("scroll", closeMenu, true);
  window.removeEventListener("resize", closeMenu);
});
</script>

<style scoped>
.editor-toolbar {
  display: flex;
  align-items: center;
  background: var(--background);
  gap: 0.15em;
  padding: 0.35em 0;
  border-bottom: 1px solid var(--alt-background);
  flex-shrink: 0;
  overflow: auto hidden;
  scrollbar-width: none;
}

.editor-toolbar-btn {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2em;
  height: 2em;
  flex-shrink: 0;
  border: none;
  background: transparent;
  border-radius: var(--borderRadius);
  color: var(--textPrimary);
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.md-toolbar-group {
  display: flex;
  flex-shrink: 0;
}

/* sticky buttons at the left for easy access */
.editor-toolbar-sticky {
  left: 0;
  position: sticky;
  display: flex;
  z-index: 1;
  align-items: center;
  flex-shrink: 0;
  background: var(--background);
  border-right: 1px solid var(--alt-background);
  padding: 0.25em;
  margin: -0.35em 0;
}

.editor-toolbar-sticky--right {
  right: -1px;
  padding-right: calc(0.25em + 1px);
  border-left: 1px solid var(--alt-background);
  border-right: none;
}

/* overlaid on the icons bottom edge */
.toolbar-color-indicator {
  position: absolute;
  left: 50%;
  bottom: 0.15em;
  transform: translateX(-50%);
  width: 1.4em;
  height: 0.5em;
  border-radius: 0.2em;
  background-color: var(--alt-background);
  box-shadow: inset 0 0 0 1px var(--alt-background);
  cursor: pointer;
}

.editor-toolbar-btn .material-symbols.toolbar-color-glyph {
  font-size: 1em;
  transform: translateY(-0.2em);
}

.toolbar-color-indicator--empty {
  background-color: transparent;
  opacity: 0;
}

.color-input {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  margin: 0;
  padding: 0;
  border: none;
  opacity: 0;
  cursor: pointer;
}

.editor-toolbar-btn:disabled {
  opacity: 0.35;
}

.editor-toolbar-btn .material-symbols {
  font-size: 1.2em;
}

.editor-toolbar-menu {
  position: fixed;
  margin: 0;
  padding: 0.25em;
  padding-bottom: 0.65em;
  list-style: none;
  z-index: 9999;
}

.editor-toolbar-menu--icon-menu {
  display: flex;
  gap: 0.15em;
  padding-bottom: 0.5em;
}

/* same as editor-toolbar-btn but only the layout is what changes */
.editor-toolbar-menu-btn {
  width: 100%;
  height: auto;
  justify-content: flex-start;
  gap: 0.5em;
  padding: 0.5em 0.75em;
}

.editor-toolbar-menu-btn .material-symbols {
  font-size: 1.1em;
}

.expand-enter-active,
.expand-leave-active {
  overflow: hidden;
}
</style>
