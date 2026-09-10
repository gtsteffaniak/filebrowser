import { beforeEach, describe, expect, it, vi } from "vitest";

const { storeMock } = vi.hoisted(() => ({
  storeMock: {
    state: {
      req: null,
      editor: { jsonFormatted: false },
    },
    getters: {
      sourcePermissions: vi.fn(() => ({ modify: true })),
      isDarkMode: vi.fn(() => false),
    },
    mutations: {
      setEditorJsonFormatted: vi.fn(),
    },
  },
}));

vi.mock("@/store", () => storeMock);
vi.mock("@/store/eventBus", () => ({
  eventBus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() },
}));
vi.mock("ace-builds", () => ({ default: {} }));

import EditorToolbar from "./EditorToolbar.vue";

const { toolbarButtons } = EditorToolbar.computed;
const { applyHighlightColor, wrapSelection } = EditorToolbar.methods;

function createEditor(selectedText = "", start = { row: 2, column: 3 }) {
  const range = { start, end: { row: start.row, column: start.column + selectedText.length } };
  return {
    range,
    editor: {
      getSelectionRange: vi.fn(() => range),
      getSelectedText: vi.fn(() => selectedText),
      session: {
        insert: vi.fn(),
        replace: vi.fn(),
      },
      selection: {
        setRange: vi.fn(),
      },
    },
  };
}

describe("EditorToolbar toolbar availability", () => {
  beforeEach(() => {
    storeMock.state.req = null;
    storeMock.getters.sourcePermissions.mockReturnValue({ modify: true });
  });

  function getPlainTextButtons() {
    return toolbarButtons.call({
      $t: (key) => key,
      canUndo: false,
      canRedo: false,
      isMarkdown: false,
      clipboardMenuItems: [],
    });
  }

  it("builds the toolbar before file information is available", () => {
    expect(() => getPlainTextButtons()).not.toThrow();
    expect(getPlainTextButtons().map(({ id }) => id)).toEqual(["undo", "redo", "find"]);
  });

  it("offers JSON formatting only for modifiable JSON files", () => {
    storeMock.state.req = { type: "application/json" };
    expect(getPlainTextButtons().map(({ id }) => id)).toContain("formatJSON");

    storeMock.getters.sourcePermissions.mockReturnValue({ modify: false });
    expect(getPlainTextButtons().map(({ id }) => id)).not.toContain("formatJSON");

    storeMock.getters.sourcePermissions.mockReturnValue({ modify: true });
    storeMock.state.req = { type: "text/plain" };
    expect(getPlainTextButtons().map(({ id }) => id)).not.toContain("formatJSON");
  });
});

describe("EditorToolbar.wrapSelection", () => {
  it("does nothing when the editor is unavailable", () => {
    expect(wrapSelection.call({ editor: null }, "**", "**", "text")).toBeUndefined();
  });

  it("inserts and selects a placeholder for an empty selection", () => {
    const { editor } = createEditor();
    const focusEditor = vi.fn();

    wrapSelection.call({ editor, focusEditor }, "<mark>", "</mark>", "highlight");

    expect(editor.session.insert).toHaveBeenCalledWith(
      { row: 2, column: 3 },
      "<mark>highlight</mark>",
    );
    expect(editor.session.replace).not.toHaveBeenCalled();
    expect(editor.selection.setRange).toHaveBeenCalledWith({
      start: { row: 2, column: 9 },
      end: { row: 2, column: 18 },
    });
    expect(focusEditor).toHaveBeenCalledOnce();
  });

  it.each([
    ["a selected line with LF", "alpha\n", "**alpha**\n"],
    ["a selected line with CRLF", "alpha\r\n", "**alpha**\r\n"],
  ])("keeps the line ending outside the closing marker for %s", (_label, selectedText, replacement) => {
    const { editor, range } = createEditor(selectedText);
    const focusEditor = vi.fn();

    wrapSelection.call({ editor, focusEditor }, "**", "**");

    expect(editor.session.replace).toHaveBeenCalledWith(range, replacement);
    expect(editor.selection.setRange).toHaveBeenCalledWith({
      start: { row: 2, column: 5 },
      end: { row: 2, column: 10 },
    });
    expect(focusEditor).toHaveBeenCalledOnce();
  });

  it("keeps an internal newline inside the markers and the final newline outside", () => {
    const { editor, range } = createEditor("alpha\nbeta\n");

    wrapSelection.call({ editor, focusEditor: vi.fn() }, "_", "_");

    expect(editor.session.replace).toHaveBeenCalledWith(range, "_alpha\nbeta_\n");
    expect(editor.selection.setRange).toHaveBeenCalledWith({
      start: { row: 2, column: 4 },
      end: { row: 3, column: 4 },
    });
  });
});

describe("EditorToolbar.applyHighlightColor", () => {
  it("sets both the inline background and Markdown viewer custom property", () => {
    const context = {
      $t: vi.fn(() => "highlight"),
      wrapSelection: vi.fn(),
    };

    applyHighlightColor.call(context, "#12abef");

    expect(context.wrapSelection).toHaveBeenCalledWith(
      '<mark style="background-color: #12abef; --mark-color: #12abef">',
      "</mark>",
      "highlight",
    );
  });

  it("omits the style attribute when the default highlight color is requested", () => {
    const context = {
      $t: vi.fn(() => "highlight"),
      wrapSelection: vi.fn(),
    };

    applyHighlightColor.call(context, "");

    expect(context.wrapSelection).toHaveBeenCalledWith("<mark>", "</mark>", "highlight");
  });
});
