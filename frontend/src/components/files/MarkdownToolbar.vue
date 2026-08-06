<template>
  <div class="markdown-toolbar no-select">
    <div class="md-toolbar-sticky">
      <div
        v-for="btn in stickyToolbarButtons"
        :key="btn.id"
        class="md-toolbar-group"
      >
        <button
          type="button"
          class="md-toolbar-btn"
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
      v-for="btn in toolbarButtons"
      :key="btn.id"
      class="md-toolbar-group"
    >
      <button
        v-if="!btn.color && !btn.menu"
        type="button"
        class="md-toolbar-btn"
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
          :ref="(el) => setIconMenuTriggerEl(btn.menu, el as HTMLElement | null)"
          type="button"
          class="md-toolbar-btn"
          :title="btn.title"
          :aria-label="btn.title"
          @mousedown.prevent
          @click="toggleMenu(btn.menu)"
        >
          <i class="material-symbols">{{ btn.icon }}</i>
        </button>
        <Teleport to="body">
          <transition name="expand" @before-enter="expandBeforeEnter" @enter="expandEnter" @leave="expandLeave">
            <ul v-if="openMenu === btn.menu" :ref="(el) => setIconMenuEl(btn.menu, el as HTMLElement | null)" class="md-toolbar-menu md-toolbar-menu--icon-menu floating-window border-radius" :class="{ 'dark-mode': isDarkMode }" :style="menuStyle">
              <li v-for="item in iconMenuItems(btn.menu)" :key="item.id">
                <button
                  type="button"
                  class="md-toolbar-btn"
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
      <div v-else class="md-toolbar-btn clickable">
        <button
          type="button"
          class="md-toolbar-btn"
          :title="btn.title"
          :aria-label="btn.title"
          :disabled="btn.disabled"
          @mousedown.prevent
          @click="applyStoredColor(btn)"
        >
          <i class="material-symbols" :class="{ 'md-toolbar-color-glyph': selectedColor(btn) }">{{ btn.icon }}</i>
        </button>
        <span
          class="md-toolbar-color-indicator"
          :class="{ 'md-toolbar-color-indicator--empty': !selectedColor(btn) }"
          :style="selectedColor(btn) ? { backgroundColor: selectedColor(btn) } : null"
        >
          <input
            :ref="(el) => setColorInput(el as HTMLInputElement | null, btn.color)"
            type="color"
            class="color-input"
            :value="selectedColor(btn)"
            :aria-label="btn.title"
            @mousedown.stop
            @change="onColorChange(btn.color, ($event.target as HTMLInputElement).value, btn.applyColor)"
          />
        </span>
      </div>
    </div>
    <div class="md-toolbar-sticky md-toolbar-sticky--right">
      <button
        ref="extraMenuTrigger"
        type="button"
        class="md-toolbar-btn"
        :title="$t('editor.md.moreOptions')"
        :aria-label="$t('editor.md.moreOptions')"
        @mousedown.prevent
        @click="toggleMenu('extra')"
      >
        <i class="material-symbols">more_horiz</i>
      </button>
      <Teleport to="body">
        <transition name="expand" @before-enter="expandBeforeEnter" @enter="expandEnter" @leave="expandLeave">
          <ul v-if="openMenu === 'extra'" ref="extraMenu" class="md-toolbar-menu floating-window border-radius" :class="{ 'dark-mode': isDarkMode }" :style="menuStyle">
            <li v-for="item in extraMenuItems" :key="item.id">
              <button
                type="button"
                class="md-toolbar-btn md-toolbar-menu-btn"
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

<script lang="ts">
import type { PropType } from "vue";
import ace from "ace-builds";
import type { Ace } from "ace-builds";
import { mutations, state, getters } from "@/store";
import { eventBus } from "@/store/eventBus";
import { removeLastDir } from "@/utils/url.js";
import { expandBeforeEnter, expandEnter, expandLeave } from "@/utils/expandTransition";
import { copyToClipboard } from "@/utils/clipboard.js";

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
  return { row: pos.row + parts.length - 1, column: parts[parts.length - 1].length };
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

export default {
  name: "markdownToolbar",
  props: {
    editor: {
      type: Object as PropType<Ace.Editor | null>,
      default: null,
    },
    isMarkdown: {
      type: Boolean,
      default: false,
    },
  },
  data: () => ({
    canUndo: false,
    canRedo: false,
    pendingSelection: null as PendingSelection | null,
    openMenu: null as "extra" | "align" | "clipboard" | null,
    menuPosition: { top: 0, left: 0, right: 0 },
    lastColors: new Map<string, string>([
      ["mdFontColor", localStorage.getItem("mdFontColor") || ""],
      ["mdHighlightColor", localStorage.getItem("mdHighlightColor") || ""],
    ]),
    colorInputRefs: new Map<string, HTMLInputElement>(),
    iconMenuTriggerEls: new Map<"align" | "clipboard", HTMLElement>(),
    iconMenuEls: new Map<"align" | "clipboard", HTMLElement>(),
  }),
  watch: {
    editor: {
      immediate: true,
      handler(newEditor: Ace.Editor | null, oldEditor: Ace.Editor | null) {
        this.detachUndoListener(oldEditor);
        this.attachUndoListener(newEditor);
        if (oldEditor && oldEditor !== newEditor) {
          this.clearPendingSelection();
        }
      },
    },
  },
  mounted() {
    eventBus.on("pathSelected", this.onPathSelected);
    eventBus.on("pathPickerCancelled", this.onPathPickerCancelled);
    document.addEventListener("pointerdown", this.onPointerDown);
    document.addEventListener("keydown", this.onKeyDown);
    window.addEventListener("scroll", this.closeMenu, true);
    window.addEventListener("resize", this.closeMenu);
  },
  beforeUnmount() {
    this.detachUndoListener(this.editor);
    this.clearPendingSelection();
    eventBus.off("pathSelected", this.onPathSelected);
    eventBus.off("pathPickerCancelled", this.onPathPickerCancelled);
    document.removeEventListener("pointerdown", this.onPointerDown);
    document.removeEventListener("keydown", this.onKeyDown);
    window.removeEventListener("scroll", this.closeMenu, true);
    window.removeEventListener("resize", this.closeMenu);
  },
  computed: {
    markdownToolbarButtons(): ToolbarButton[] {
      const alwaysAvailable: ToolbarButton[] = [
        { id: "undo", icon: "undo", title: this.$t("editor.md.undo"), action: () => this.undo(), disabled: !this.canUndo, sticky: true },
        { id: "redo", icon: "redo", title: this.$t("editor.md.redo"), action: () => this.redo(), disabled: !this.canRedo, sticky: true },
        { id: "find", icon: "search", title: this.$t("general.search"), action: () => this.openFind() },
      ];
      if (!this.isMarkdown) {
        return [
          ...alwaysAvailable,
          { id: "copy", icon: "content_copy", title: this.$t("general.copy"), action: () => this.copySelection() },
          { id: "cut", icon: "content_cut", title: this.$t("editor.md.cut"), action: () => this.cutSelection() },
          { id: "paste", icon: "content_paste", title: this.$t("editor.md.paste"), action: () => this.pasteClipboard() },
          { id: "selectAll", icon: "select_all", title: this.$t("buttons.selectAll"), action: () => this.selectAllText() },
        ];
      }
      return [
        ...alwaysAvailable,
        { id: "clipboard", icon: "content_paste", title: this.$t("editor.md.clipboardActions"), menu: "clipboard" },
        { id: "bold", icon: "format_bold", title: this.$t("editor.md.bold"), action: () => this.wrapSelection("**", "**") },
        { id: "italic", icon: "format_italic", title: this.$t("editor.md.italic"), action: () => this.wrapSelection("_", "_") },
        { id: "strikethrough", icon: "strikethrough_s", title: this.$t("editor.md.strikethrough"), action: () => this.wrapSelection("~~", "~~") },
        { id: "heading", icon: "title", title: this.$t("editor.md.heading"), action: () => this.cycleHeading() },
        { id: "quote", icon: "format_quote", title: this.$t("editor.md.quote"), action: () => this.toggleLinePrefix("> ") },
        { id: "image", icon: "image", title: this.$t("fileTypes.image"), action: () => this.insertImage() },
        { id: "align", icon: "format_align_left", title: this.$t("editor.md.align"), menu: "align" },
        { id: "bulletList", icon: "format_list_bulleted", title: this.$t("editor.md.bulletList"), action: () => this.toggleLinePrefix("- ") },
        { id: "numberedList", icon: "format_list_numbered", title: this.$t("editor.md.numberedList"), action: () => this.applyNumberedList() },
        { id: "taskList", icon: "checklist", title: this.$t("editor.md.taskList"), action: () => this.toggleTaskList() },
        { id: "fontColor", icon: "font_download", title: this.$t("editor.md.fontColor"), color: "mdFontColor", applyColor: this.applyFontColor },
        { id: "highlight", icon: "ink_highlighter", title: this.$t("editor.md.highlight"), color: "mdHighlightColor", applyColor: this.applyHighlightColor },
      ];
    },
    stickyToolbarButtons(): ToolbarButton[] {
      return this.markdownToolbarButtons.filter((btn) => btn.sticky);
    },
    toolbarButtons(): ToolbarButton[] {
      return this.markdownToolbarButtons.filter((btn) => !btn.sticky);
    },
    alignMenuItems(): ToolbarButton[] {
      return [
        { id: "alignLeft", icon: "format_align_left", title: this.$t("editor.md.alignLeft"), action: () => this.wrapSelection('<p align="left">', "</p>", this.$t("editor.md.text")) },
        { id: "alignCenter", icon: "format_align_center", title: this.$t("editor.md.alignCenter"), action: () => this.wrapSelection("<center>", "</center>", this.$t("editor.md.text")) },
        { id: "alignRight", icon: "format_align_right", title: this.$t("editor.md.alignRight"), action: () => this.wrapSelection('<p align="right">', "</p>", this.$t("editor.md.text")) },
        { id: "alignJustify", icon: "format_align_justify", title: this.$t("editor.md.justify"), action: () => this.wrapSelection('<p align="justify">', "</p>", this.$t("editor.md.text")) },
      ];
    },
    clipboardMenuItems(): ToolbarButton[] {
      return [
        { id: "copy", icon: "content_copy", title: this.$t("general.copy"), action: () => this.copySelection() },
        { id: "cut", icon: "content_cut", title: this.$t("editor.md.cut"), action: () => this.cutSelection() },
        { id: "paste", icon: "content_paste", title: this.$t("editor.md.paste"), action: () => this.pasteClipboard() },
        { id: "selectAll", icon: "select_all", title: this.$t("buttons.selectAll"), action: () => this.selectAllText() },
      ];
    },
    extraMenuItems(): ToolbarButton[] {
      const editorSettings: ToolbarButton = {
        id: "editorSettings",
        icon: "settings",
        title: this.$t("editor.settings"),
        action: () => this.openEditorSettings(),
      };
      if (!this.isMarkdown) {
        return [editorSettings];
      }
      return [
        editorSettings,
        { id: "code", icon: "code", title: this.$t("editor.md.inlineCode"), action: () => this.wrapSelection("`", "`") },
        { id: "codeBlock", icon: "code_blocks", title: this.$t("editor.md.codeBlock"), action: () => this.insertCodeBlock() },
        { id: "video", icon: "videocam", title: this.$t("fileTypes.video"), action: () => this.insertVideo() },
        { id: "audio", icon: "music_note", title: this.$t("fileTypes.audio"), action: () => this.insertAudio() },
        { id: "table", icon: "table", title: this.$t("tools.activityViewer.tableView"), action: () => this.insertTable() },
        { id: "horizontalRule", icon: "horizontal_rule", title: this.$t("editor.md.horizontalRule"), action: () => this.insertHorizontalRule() },
        { id: "inlineMath", icon: "functions", title: this.$t("editor.md.inlineMath"), action: () => this.wrapSelection("$", "$", "E = mc^2") },
        { id: "displayMath", icon: "calculate", title: this.$t("editor.md.displayMath"), action: () => this.wrapSelection("$$\n", "\n$$", "E = mc^2") },
        { id: "superscript", icon: "superscript", title: this.$t("editor.md.superscript"), action: () => this.wrapSelection("<sup>", "</sup>", "2") },
        { id: "subscript", icon: "subscript", title: this.$t("editor.md.subscript"), action: () => this.wrapSelection("<sub>", "</sub>", "2") },
        { id: "kbd", icon: "keyboard", title: this.$t("threejs.keyboard"), action: () => this.wrapSelection("<kbd>", "</kbd>", "Ctrl") },
        { id: "link", icon: "link", title: this.$t("general.links"), action: () => this.insertLink() },
      ];
    },
    menuStyle() {
      if (this.openMenu === "align" || this.openMenu === "clipboard") {
        return { top: `${this.menuPosition.top}px`, left: `${this.menuPosition.left}px`, transform: "translateX(-50%)" };
      }
      return { top: `${this.menuPosition.top}px`, right: `${this.menuPosition.right}px` };
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
  },
  methods: {
    expandBeforeEnter,
    expandEnter,
    expandLeave,
    focusEditor() {
      if (this.editor) this.editor.focus();
    },
    undo() {
      const editor = this.editor;
      if (!editor) return;
      editor.undo();
      this.refreshUndoState();
      this.focusEditor();
    },
    redo() {
      const editor = this.editor;
      if (!editor) return;
      editor.redo();
      this.refreshUndoState();
      this.focusEditor();
    },
    openFind() {
      const editor = this.editor;
      if (!editor) return;
      editor.execCommand("find");
    },
    attachUndoListener(editor: Ace.Editor | null) {
      if (!editor) return;
      editor.session.on("change", this.refreshUndoState);
      this.refreshUndoState();
    },
    detachUndoListener(editor: Ace.Editor | null) {
      if (!editor) return;
      editor.session.off("change", this.refreshUndoState);
    },
    refreshUndoState() {
      const editor = this.editor;
      if (!editor) {
        this.canUndo = false;
        this.canRedo = false;
        return;
      }
      const undoManager = editor.session.getUndoManager();
      this.canUndo = undoManager.hasUndo();
      this.canRedo = undoManager.hasRedo();
    },
    copySelection() {
      const editor = this.editor;
      if (!editor) return;
      const text = editor.getCopyText();
      if (text) void copyToClipboard(text);
      this.focusEditor();
    },
    cutSelection() {
      const editor = this.editor;
      if (!editor) return;
      const text = editor.getCopyText();
      if (text) void copyToClipboard(text);
      editor.execCommand("cut");
      this.focusEditor();
    },
    async pasteClipboard() {
      const editor = this.editor;
      if (!editor) return;
      this.focusEditor();
      try {
        const text = await navigator.clipboard.readText();
        if (text) {
          editor.execCommand("paste", { text });
        }
      } catch (e) { /* ignore - probably blocked by browser */ }
    },
    selectAllText() {
      const editor = this.editor;
      if (!editor) return;
      editor.execCommand("selectall");
      this.focusEditor();
    },
    selectedLineRange() {
      const range = this.editor.getSelectionRange();
      let endRow = range.end.row;
      if (endRow > range.start.row && range.end.column === 0) {
        endRow -= 1;
      }
      return { startRow: range.start.row, endRow };
    },
    wrapSelection(before: string, after: string = before, placeholder: string = "") {
      const editor = this.editor;
      if (!editor) return;
      const range = editor.getSelectionRange();
      const selectedText = editor.getSelectedText();
      const text = selectedText || placeholder;
      const start = { row: range.start.row, column: range.start.column };
      if (selectedText) {
        editor.session.replace(range, `${before}${text}${after}`);
      } else {
        editor.session.insert(start, `${before}${text}${after}`);
      }
      const contentStart = advancePosition(start, before);
      const contentEnd = advancePosition(contentStart, text);
      editor.selection.setRange({ start: contentStart, end: contentEnd });
      this.focusEditor();
    },
    applyFontColor(color: string) {
      this.wrapSelection(`<font color="${color}">`, "</font>", this.$t("editor.md.text"));
    },
    applyHighlightColor(color: string) {
      const style = color ? ` style="background-color: ${color}"` : "";
      this.wrapSelection(`<mark${style}>`, "</mark>", this.$t("editor.md.highlight"));
    },
    selectedColor(btn: ToolbarButton): string {
      return (btn.color && this.lastColors.get(btn.color)) || "";
    },
    applyStoredColor(btn: ToolbarButton) {
      if (btn.disabled || !btn.color || !btn.applyColor) return;
      const stored = this.selectedColor(btn);
      if (!stored) {
        this.colorInputRefs.get(btn.color)?.click();
        return;
      }
      btn.applyColor(stored);
    },
    setColorInput(el: HTMLInputElement | null, color: string) {
      if (el) this.colorInputRefs.set(color, el);
    },
    onColorChange(storageKey: string, color: string, apply?: (color: string) => void) {
      localStorage.setItem(storageKey, color);
      this.lastColors.set(storageKey, color);
      apply?.(color);
    },
    toggleLinePrefix(prefix: string) {
      const editor = this.editor;
      if (!editor) return;
      const { startRow, endRow } = this.selectedLineRange();
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
      this.focusEditor();
    },
    applyNumberedList() {
      const editor = this.editor;
      if (!editor) return;
      const { startRow, endRow } = this.selectedLineRange();
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
      this.focusEditor();
    },
    cycleHeading() {
      const editor = this.editor;
      if (!editor) return;
      const { startRow } = this.selectedLineRange();
      const session = editor.session;
      const line = session.getLine(startRow);
      const match = line.match(/^(#{1,6})\s/);
      const currentLevel = match ? match[1].length : 0;
      const nextLevel = currentLevel === 0 ? 1 : (currentLevel >= 6 ? 0 : currentLevel + 1);
      const stripped = line.replace(/^#{1,6}\s*/, "");
      const newLine = nextLevel === 0 ? stripped : `${"#".repeat(nextLevel)} ${stripped}`;
      session.replace({ start: { row: startRow, column: 0 }, end: { row: startRow, column: line.length } }, newLine);
      this.focusEditor();
    },
    insertCodeBlock() {
      const editor = this.editor;
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
      this.focusEditor();
    },
    insertLink() {
      const editor = this.editor;
      if (!editor) return;
      const selectedText = editor.getSelectedText();
      const range = editor.getSelectionRange();
      const label = formatImageAltText(selectedText || this.$t("editor.md.text"));
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
      this.focusEditor();
    },
    openEditorSettings() {
      mutations.showPrompt({
        name: "EditorSettings",
      });
    },
    insertBlock(content: string) {
      const editor = this.editor;
      if (!editor) return;
      const pos = editor.getCursorPosition();
      const line = editor.session.getLine(pos.row);
      const needsNewlineBefore = line.trim() !== "";
      editor.moveCursorTo(pos.row, line.length);
      editor.clearSelection();
      editor.insert(`${needsNewlineBefore ? "\n\n" : ""}${content}`);
      this.focusEditor();
    },
    insertHorizontalRule() {
      this.insertBlock("---\n\n");
    },
    insertImage() {
      this.openPathPicker("image", { allowedFileTypes: ["image/"] });
    },
    insertVideo() {
      this.openPathPicker("video", { allowedFileTypes: ["video/"] });
    },
    insertAudio() {
      this.openPathPicker("audio", { allowedFileTypes: ["audio/"] });
    },
    openPathPicker(kind: "image" | "video" | "audio", pickerProps: Record<string, unknown>) {
      const editor = this.editor;
      if (!editor) return;
      const selectedText = editor.getSelectedText();
      const range = editor.getSelectionRange();
      const contextId = `md-toolbar-${kind}-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
      this.pendingSelection = {
        kind,
        contextId,
        alt: kind === "image" ? (selectedText || "") : "",
        range: {
          start: editor.session.doc.createAnchor(range.start.row, range.start.column),
          end: editor.session.doc.createAnchor(range.end.row, range.end.column),
        },
      };
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
    },
    clearPendingSelection() {
      this.pendingSelection?.range.start.detach();
      this.pendingSelection?.range.end.detach();
      this.pendingSelection = null;
    },
    buildImageMd(path: string, alt: string): string {
      const fileName = path.split("/").filter(Boolean).pop() || "image";
      const altText = alt || fileName.replace(/\.[^./]+$/, "");
      return `![${formatImageAltText(altText)}](${formatImageDestination(path)})`;
    },
    buildMediaMd(path: string, tag: "video" | "audio"): string {
      return `<${tag} src="${formatHtmlAttrValue(path)}" controls></${tag}>`;
    },
    onPathSelected(data: { path?: string; selectionContextId?: string }) {
      const pending = this.pendingSelection;
      if (!pending || !data || data.selectionContextId !== pending.contextId) {
        return;
      }
      const editor = this.editor;
      const path = data.path;
      this.clearPendingSelection();
      if (!editor || typeof path !== "string") return;
      const text = pending.kind === "image"
        ? this.buildImageMd(path, pending.alt)
        : this.buildMediaMd(path, pending.kind);
      const start = pending.range.start.getPosition();
      const end = pending.range.end.getPosition();
      const range = new (ace.require("ace/range").Range)(start.row, start.column, end.row, end.column);
      const insertionEnd = editor.session.replace(range, text);
      editor.moveCursorTo(insertionEnd.row, insertionEnd.column);
      editor.clearSelection();
      this.focusEditor();
    },
    onPathPickerCancelled(data: { selectionContextId?: string }) {
      if (!this.pendingSelection || !data || data.selectionContextId !== this.pendingSelection.contextId) {
        return;
      }
      this.clearPendingSelection();
    },
    toggleTaskList() {
      const editor = this.editor;
      if (!editor) return;
      const { startRow, endRow } = this.selectedLineRange();
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
      this.focusEditor();
    },
    onPointerDown(e: PointerEvent) {
      const target = e.target as Node;
      const extraMenuTrigger = this.$refs.extraMenuTrigger as HTMLElement | undefined;
      const extraMenu = this.$refs.extraMenu as HTMLElement | undefined;
      const insideIconMenus =
        Array.from(this.iconMenuTriggerEls.values()).some((el) => el.contains(target))
        || Array.from(this.iconMenuEls.values()).some((el) => el.contains(target));
      const inside =
        extraMenuTrigger?.contains(target)
        || extraMenu?.contains(target)
        || insideIconMenus;
      if (!inside) this.closeMenu();
    },
    toggleMenu(name: "extra" | "align" | "clipboard") {
      if (this.openMenu === name) {
        this.closeMenu();
        return;
      }
      const trigger = name === "extra" ? (this.$refs.extraMenuTrigger as HTMLElement | undefined) : this.iconMenuTriggerEls.get(name);
      if (trigger) {
        const rect = trigger.getBoundingClientRect();
        if (name === "extra") {
          // Right-anchor to the viewport edge so it can't overflow off-screen
          this.menuPosition = { top: rect.bottom + 4, left: 0, right: window.innerWidth - rect.right };
        } else {
          // Center under the button
          this.menuPosition = { top: rect.bottom + 4, left: rect.left + rect.width / 2, right: 0 };
        }
      }
      this.openMenu = name;
    },
    closeMenu() {
      this.openMenu = null;
    },
    setIconMenuTriggerEl(menu: "align" | "clipboard", el: HTMLElement | null) {
      if (el) this.iconMenuTriggerEls.set(menu, el);
      else this.iconMenuTriggerEls.delete(menu);
    },
    setIconMenuEl(menu: "align" | "clipboard", el: HTMLElement | null) {
      if (el) this.iconMenuEls.set(menu, el);
      else this.iconMenuEls.delete(menu);
    },
    iconMenuItems(menu: "align" | "clipboard"): ToolbarButton[] {
      return menu === "align" ? this.alignMenuItems : this.clipboardMenuItems;
    },
    onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape" && this.openMenu) {
        this.closeMenu();
        this.focusEditor();
      }
    },
    extBtnAction(item: ToolbarButton) {
      this.closeMenu();
      item.action?.();
    },
    insertTable() {
      const column = this.$t("editor.md.column");
      const cell = this.$t("editor.md.cell");
      this.insertBlock(`| ${column} 1 | ${column} 2 |\n| --- | --- |\n| ${cell} 1 | ${cell} 2 |\n`);
    },
  },
};
</script>

<style scoped>
.markdown-toolbar {
  display: flex;
  align-items: center;
  gap: 0.15em;
  padding: 0.35em 0;
  border-bottom: 1px solid var(--alt-background);
  flex-shrink: 0;
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
}

.md-toolbar-btn {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2em;
  height: 2em;
  flex-shrink: 0;
  border: none;
  background: transparent;
  border-radius: 0.5em;
  color: inherit;
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.md-toolbar-group {
  display: flex;
  flex-shrink: 0;
}

/* for undo/redo sticky at the left for easy access */
.md-toolbar-sticky {
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

.md-toolbar-sticky--right {
  right: -1px;
  /*margin-left: auto;*/
  padding-right: calc(0.25em + 1px);
  border-left: 1px solid var(--alt-background);
  border-right: none;
}

/* overlaid on the icons bottom edge */
.md-toolbar-color-indicator {
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

.md-toolbar-btn .material-symbols.md-toolbar-color-glyph {
  font-size: 1em;
  transform: translateY(-0.2em);
}

.md-toolbar-color-indicator--empty {
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

.md-toolbar-btn:disabled {
  opacity: 0.35;
}

.md-toolbar-btn .material-symbols {
  font-size: 1.2em;
}

.md-toolbar-menu {
  position: fixed;
  margin: 0;
  padding: 0.25em;
  padding-bottom: 0.65em;
  list-style: none;
  z-index: 9999;
}

.md-toolbar-menu--icon-menu {
  display: flex;
  gap: 0.15em;
  padding-bottom: 0.5em;
}

/* same as md-toolbar-btn but only the layout is what changes */
.md-toolbar-menu-btn {
  width: 100%;
  height: auto;
  justify-content: flex-start;
  gap: 0.5em;
  padding: 0.5em 0.75em;
}

.md-toolbar-menu-btn .material-symbols {
  font-size: 1.1em;
}

.expand-enter-active,
.expand-leave-active {
  overflow: hidden;
}
</style>
