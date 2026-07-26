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
  </div>
</template>

<script lang="ts">
import type { PropType } from "vue";
import type { Ace } from "ace-builds";

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
  beforeUnmount() {
    this.detachUndoListener(this.editor);
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
        { id: "table", icon: "table", title: this.$t("editor.md.table"), action: () => this.insertTable() },
        { id: "horizontalRule", icon: "horizontal_rule", title: this.$t("editor.md.horizontalRule"), action: () => this.insertHorizontalRule() },
      ];
    },
  },
  methods: {
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
    wrapSelection(before: string, after: string = before) {
      const editor = this.editor;
      if (!editor) return;
      const range = editor.getSelectionRange();
      const selectedText = editor.getSelectedText();
      if (selectedText) {
        editor.session.replace(range, `${before}${selectedText}${after}`);
        editor.selection.setRange({
          start: { row: range.start.row, column: range.start.column + before.length },
          end: {
            row: range.end.row,
            column: range.end.column + (range.start.row === range.end.row ? before.length : 0),
          },
        });
      } else {
        const pos = editor.getCursorPosition();
        editor.insert(`${before}${after}`);
        editor.moveCursorTo(pos.row, pos.column + before.length);
        editor.clearSelection();
      }
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
        } else if (!line.startsWith(prefix)) {
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
        } else {
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
      const label = selectedText || "text";
      const linkText = `[${label}](url)`;
      if (selectedText) {
        editor.session.replace(range, linkText);
      } else {
        editor.insert(linkText);
      }
      const pos = editor.getCursorPosition();
      const urlEnd = pos.column - 1; // before the closing ')'
      const urlStart = urlEnd - "url".length;
      if (urlStart >= 0) {
        editor.selection.setRange({
          start: { row: pos.row, column: urlStart },
          end: { row: pos.row, column: urlEnd },
        });
      }
      this.focusEditor();
    },
    insertHorizontalRule() {
      const editor = this.editor;
      if (!editor) return;
      const pos = editor.getCursorPosition();
      const line = editor.session.getLine(pos.row);
      const needsNewlineBefore = line.trim() !== "";
        editor.moveCursorTo(pos.row, line.length);
      editor.clearSelection();
      editor.insert(`${needsNewlineBefore ? "\n\n" : ""}---\n\n`);
      this.focusEditor();
    },
    insertImage() {
      const editor = this.editor;
      if (!editor) return;
      const selectedText = editor.getSelectedText();
      const range = editor.getSelectionRange();
      const alt = selectedText || "alt text";
      const imageText = `![${alt}](url)`;
      if (selectedText) {
        editor.session.replace(range, imageText);
      } else {
        editor.insert(imageText);
      }
      const pos = editor.getCursorPosition();
      editor.selection.setRange({
        start: { row: pos.row, column: pos.column - 4 },
        end: { row: pos.row, column: pos.column - 1 },
      });
      this.focusEditor();
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
        } else if (!match) {
          session.insert({ row, column: 0 }, "- [ ] ");
        }
      }
      this.focusEditor();
    },
    insertTable() {
      const editor = this.editor;
      if (!editor) return;
      const pos = editor.getCursorPosition();
      const line = editor.session.getLine(pos.row);
      const needsNewlineBefore = line.trim() !== "";
      const table = "| Column 1 | Column 2 |\n| --- | --- |\n| Cell 1 | Cell 2 |\n";
      editor.moveCursorTo(pos.row, line.length);
      editor.clearSelection();
      editor.insert(`${needsNewlineBefore ? "\n\n" : ""}${table}`);
      this.focusEditor();
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
</style>
