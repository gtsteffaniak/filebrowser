<template>
  <div class="markdown-toolbar no-select">
    <button
      v-for="btn in markdownToolbarButtons"
      :key="btn.id"
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
    <div>
      <button
        ref="extraMenuTrigger"
        type="button"
        class="md-toolbar-btn"
        :title="$t('editor.md.moreOptions')"
        :aria-label="$t('editor.md.moreOptions')"
        @mousedown.prevent
        @click="toggleExtraMenu"
      >
        <i class="material-symbols">more_horiz</i>
      </button>
      <Teleport to="body">
        <transition name="expand" @before-enter="expandBeforeEnter" @enter="expandEnter" @leave="expandLeave">
          <ul v-if="extraMenuOpen" ref="extraMenu" class="md-toolbar-menu floating-window border-radius" :class="{ 'dark-mode': isDarkMode }" :style="menuStyle">
            <li v-for="item in extraMenuItems" :key="item.id" @mousedown.prevent @click="extBtnAction(item)">
              <i class="material-symbols">{{ item.icon }}</i>
              <span>{{ item.title }}</span>
            </li>
          </ul>
        </transition>
      </Teleport>
    </div>
  </div>
</template>

<script lang="ts">
import type { PropType } from "vue";
import type { Ace } from "ace-builds";
import { mutations, state, getters } from "@/store";
import { eventBus } from "@/store/eventBus";
import { removeLastDir } from "@/utils/url.js";
import { expandBeforeEnter, expandEnter, expandLeave } from "@/utils/expandTransition";

interface PendingImageRange {
  start: { row: number; column: number };
  end: { row: number; column: number };
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

function advancePosition(pos: Ace.Point, str: string): Ace.Point {
  const parts = str.split("\n");
  if (parts.length === 1) return { row: pos.row, column: pos.column + str.length };
  return { row: pos.row + parts.length - 1, column: parts[parts.length - 1].length };
}

interface ToolbarButton {
  id: string;
  icon: string;
  title: string;
  action: () => void;
  disabled?: boolean;
}

export default {
  name: "markdownToolbar",
  props: {
    editor: {
      type: Object as PropType<Ace.Editor | null>,
      default: null,
    },
  },
  data: () => ({
    canUndo: false,
    canRedo: false,
    pendingImageContextId: null as string | null,
    pendingImageRange: null as PendingImageRange | null,
    pendingImageAlt: "",
    extraMenuOpen: false,
    menuPosition: { top: 0, right: 0 },
  }),
  watch: {
    editor: {
      immediate: true,
      handler(newEditor: Ace.Editor | null, oldEditor: Ace.Editor | null) {
        this.detachUndoListener(oldEditor);
        this.attachUndoListener(newEditor);
      },
    },
  },
  mounted() {
    eventBus.on("pathSelected", this.imagePathSelected);
    eventBus.on("pathPickerCancelled", this.imagePathPickerCancelled);
    document.addEventListener("pointerdown", this.onPointerDown);
    window.addEventListener("scroll", this.closeExtraMenu, true);
    window.addEventListener("resize", this.closeExtraMenu);
  },
  beforeUnmount() {
    this.detachUndoListener(this.editor);
    eventBus.off("pathSelected", this.imagePathSelected);
    eventBus.off("pathPickerCancelled", this.imagePathPickerCancelled);
    document.removeEventListener("pointerdown", this.onPointerDown);
    window.removeEventListener("scroll", this.closeExtraMenu, true);
    window.removeEventListener("resize", this.closeExtraMenu);
  },
  computed: {
    markdownToolbarButtons(): ToolbarButton[] {
      return [
        { id: "undo", icon: "undo", title: this.$t("editor.md.undo"), action: () => this.undo(), disabled: !this.canUndo },
        { id: "redo", icon: "redo", title: this.$t("editor.md.redo"), action: () => this.redo(), disabled: !this.canRedo },
        { id: "bold", icon: "format_bold", title: this.$t("editor.md.bold"), action: () => this.wrapSelection("**", "**") },
        { id: "italic", icon: "format_italic", title: this.$t("editor.md.italic"), action: () => this.wrapSelection("_", "_") },
        { id: "strikethrough", icon: "strikethrough_s", title: this.$t("editor.md.strikethrough"), action: () => this.wrapSelection("~~", "~~") },
        { id: "heading", icon: "title", title: this.$t("editor.md.heading"), action: () => this.cycleHeading() },
        { id: "quote", icon: "format_quote", title: this.$t("editor.md.quote"), action: () => this.toggleLinePrefix("> ") },
        { id: "code", icon: "code", title: this.$t("editor.md.inlineCode"), action: () => this.wrapSelection("`", "`") },
        { id: "codeBlock", icon: "code_blocks", title: this.$t("editor.md.codeBlock"), action: () => this.insertCodeBlock() },
        { id: "link", icon: "link", title: this.$t("editor.md.link"), action: () => this.insertLink() },
        { id: "image", icon: "image", title: this.$t("editor.md.image"), action: () => this.insertImage() },
        { id: "bulletList", icon: "format_list_bulleted", title: this.$t("editor.md.bulletList"), action: () => this.toggleLinePrefix("- ") },
        { id: "numberedList", icon: "format_list_numbered", title: this.$t("editor.md.numberedList"), action: () => this.applyNumberedList() },
        { id: "taskList", icon: "checklist", title: this.$t("editor.md.taskList"), action: () => this.toggleTaskList() },
        { id: "table", icon: "table", title: this.$t("tools.activityViewer.tableView"), action: () => this.insertTable() },
        { id: "horizontalRule", icon: "horizontal_rule", title: this.$t("editor.md.horizontalRule"), action: () => this.insertHorizontalRule() },
      ];
    },
    extraMenuItems(): ToolbarButton[] {
      return [
        { id: "inlineMath", icon: "functions", title: this.$t("editor.md.inlineMath"), action: () => this.wrapSelection("$", "$", "E = mc^2") },
        { id: "displayMath", icon: "calculate", title: this.$t("editor.md.displayMath"), action: () => this.wrapSelection("$$\n", "\n$$", "E = mc^2") },
        { id: "highlight", icon: "ink_highlighter", title: this.$t("editor.md.highlight"), action: () => this.wrapSelection("<mark>", "</mark>", this.$t("editor.md.highlight")) },
        { id: "superscript", icon: "superscript", title: this.$t("editor.md.superscript"), action: () => this.wrapSelection("<sup>", "</sup>", "2") },
        { id: "subscript", icon: "subscript", title: this.$t("editor.md.subscript"), action: () => this.wrapSelection("<sub>", "</sub>", "2") },
        { id: "kbd", icon: "keyboard", title: this.$t("threejs.keyboard"), action: () => this.wrapSelection("<kbd>", "</kbd>", "Ctrl") },
      ];
    },
    menuStyle() {
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
      const label = selectedText || this.$t("editor.md.text");
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
      const editor = this.editor;
      if (!editor) return;
      const selectedText = editor.getSelectedText();
      const range = editor.getSelectionRange();
      this.pendingImageAlt = selectedText || "";
      this.pendingImageRange = {
        start: { row: range.start.row, column: range.start.column },
        end: { row: range.end.row, column: range.end.column },
      };
      const selectionContextId = `md-toolbar-image-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
      this.pendingImageContextId = selectionContextId;
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
          allowedFileTypes: ["image/"],
          selectionContextId,
        },
      });
    },
    imagePathSelected(data: { path?: string; selectionContextId?: string }) {
      if (!this.pendingImageContextId || !data || data.selectionContextId !== this.pendingImageContextId) {
        return;
      }
      this.pendingImageContextId = null;
      const range = this.pendingImageRange;
      const alt = this.pendingImageAlt;
      this.pendingImageRange = null;
      this.pendingImageAlt = "";
      const editor = this.editor;
      if (!editor || !range || typeof data.path !== "string") return;
      const fileName = data.path.split("/").filter(Boolean).pop() || "image";
      const altText = alt || fileName.replace(/\.[^./]+$/, "");
      const imageText = `![${formatImageAltText(altText)}](${formatImageDestination(data.path)})`;
      const insertionEnd = editor.session.replace(range as Ace.Range, imageText);
      editor.moveCursorTo(insertionEnd.row, insertionEnd.column);
      editor.clearSelection();
      this.focusEditor();
    },
    imagePathPickerCancelled(data: { selectionContextId?: string }) {
      if (!this.pendingImageContextId || !data || data.selectionContextId !== this.pendingImageContextId) {
        return;
      }
      this.pendingImageContextId = null;
      this.pendingImageRange = null;
      this.pendingImageAlt = "";
    },
    toggleTaskList() {
      const editor = this.editor;
      if (!editor) return;
      const { startRow, endRow } = this.selectedLineRange();
      const session = editor.session;
      const taskPrefix = /^- \[[ xX]\] /;
      const lines = [];
      for (let row = startRow; row <= endRow; row++) {
        lines.push(session.getLine(row));
      }
      const nonBlank = lines.filter((line) => line.trim() !== "");
      const allTasked = (nonBlank.length ? nonBlank : lines).every((line) => taskPrefix.test(line));
      for (let row = startRow; row <= endRow; row++) {
        const line = session.getLine(row);
        const match = line.match(taskPrefix);
        if (allTasked) {
          if (match) {
            session.replace({ start: { row, column: 0 }, end: { row, column: match[0].length } }, "");
          }
        } else if (!match && (nonBlank.length === 0 || line.trim() !== "")) {
          session.insert({ row, column: 0 }, "- [ ] ");
        }
      }
      this.focusEditor();
    },
    onPointerDown(e: PointerEvent) {
      const target = e.target as Node;
      const trigger = this.$refs.extraMenuTrigger as HTMLElement | undefined;
      const menu = this.$refs.extraMenu as HTMLElement | undefined;
      if (trigger?.contains(target) || menu?.contains(target)) return;
      this.extraMenuOpen = false;
    },
    toggleExtraMenu() {
      if (this.extraMenuOpen) {
        this.closeExtraMenu();
        return;
      }
      const trigger = this.$refs.extraMenuTrigger as HTMLElement | undefined;
      if (trigger) {
        const rect = trigger.getBoundingClientRect();
        this.menuPosition = { top: rect.bottom + 4, right: window.innerWidth - rect.right };
      }
      this.extraMenuOpen = true;
    },
    closeExtraMenu() {
      this.extraMenuOpen = false;
    },
    extBtnAction(item: ToolbarButton) {
      this.extraMenuOpen = false;
      item.action();
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
  padding: 0.35em 0.5em;
  border-bottom: 1px solid var(--alt-background);
  flex-shrink: 0;
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
}

.markdown-toolbar::-webkit-scrollbar {
  display: none;
  height: 0;
}

.md-toolbar-btn {
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

.md-toolbar-btn:hover {
  background-color: var(--alt-background);
}

.md-toolbar-btn:disabled {
  opacity: 0.35;
  cursor: default;
}

.md-toolbar-btn:disabled:hover {
  background-color: transparent;
}

.md-toolbar-btn .material-symbols {
  font-size: 1.2em;
}

.md-toolbar-menu {
  position: fixed;
  margin: 0;
  padding: 0.25em;
  list-style: none;
  min-width: 11em;
  z-index: 9999;
}

.md-toolbar-menu li {
  display: flex;
  align-items: center;
  gap: 0.5em;
  padding: 0.5em 0.75em;
  border-radius: 0.5em;
  cursor: pointer;
  white-space: nowrap;
  transition: background-color 0.15s ease;
}

.md-toolbar-menu li:hover {
  background-color: var(--alt-background);
}

.md-toolbar-menu li .material-symbols {
  font-size: 1.1em;
}

.expand-enter-active,
.expand-leave-active {
  transition: height 0.3s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}

.expand-enter,
.expand-leave-to {
  height: 0 !important;
  opacity: 0;
}
</style>
