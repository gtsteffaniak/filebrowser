import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { storeMock } = vi.hoisted(() => ({
  storeMock: {
    state: {
      editor: { scrollRatio: 0, fontSize: 14 },
      navigation: { isTransitioning: false },
    },
    getters: {},
    mutations: {
      setEditorDirty: vi.fn(),
    },
  },
}));

vi.mock("@/store", () => storeMock);
vi.mock("ace-builds", () => ({
  default: { config: { set: vi.fn() }, edit: vi.fn() },
  version: "test",
}));
vi.mock("ace-builds/src-noconflict/ext-modelist", () => ({
  default: { getModeForPath: vi.fn(() => ({ mode: "ace/mode/text" })) },
}));
vi.mock("ace-builds/src-noconflict/ext-searchbox", () => ({}));
vi.mock("ace-builds/src-min-noconflict/theme-chrome", () => ({}));
vi.mock("ace-builds/src-min-noconflict/theme-tomorrow_night_bright", () => ({}));
vi.mock("ace-builds/src-min-noconflict/mode-yaml", () => ({}));
vi.mock("ace-builds/src-min-noconflict/mode-json", () => ({}));
vi.mock("ace-builds/src-min-noconflict/mode-markdown", () => ({}));

import Editor from "./Editor.vue";

const { scheduleStatsUpdate } = Editor.methods;

function createContext(overrides = {}) {
  const editor = { getValue: vi.fn(() => "saved") };
  return {
    editor,
    savedContent: "saved",
    isDirty: false,
    statsUpdateTimer: null,
    updateEditorStats: vi.fn(),
    ...overrides,
  };
}

describe("Editor.scheduleStatsUpdate", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    storeMock.mutations.setEditorDirty.mockClear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("updates stats and marks changed editor content dirty after 150ms", () => {
    const context = createContext();
    context.editor.getValue.mockReturnValue("changed");

    scheduleStatsUpdate.call(context, context.editor);
    vi.advanceTimersByTime(149);

    expect(context.updateEditorStats).not.toHaveBeenCalled();
    expect(storeMock.mutations.setEditorDirty).not.toHaveBeenCalled();

    vi.advanceTimersByTime(1);

    expect(context.statsUpdateTimer).toBeNull();
    expect(context.updateEditorStats).toHaveBeenCalledOnce();
    expect(context.isDirty).toBe(true);
    expect(storeMock.mutations.setEditorDirty).toHaveBeenCalledWith(true);
  });

  it("clears dirty state when content returns to the saved value", () => {
    const context = createContext({ isDirty: true });

    scheduleStatsUpdate.call(context, context.editor);
    vi.advanceTimersByTime(150);

    expect(context.isDirty).toBe(false);
    expect(storeMock.mutations.setEditorDirty).toHaveBeenCalledWith(false);
  });

  it("does not emit a redundant dirty mutation when the state is unchanged", () => {
    const context = createContext();

    scheduleStatsUpdate.call(context, context.editor);
    vi.advanceTimersByTime(150);

    expect(context.updateEditorStats).toHaveBeenCalledOnce();
    expect(storeMock.mutations.setEditorDirty).not.toHaveBeenCalled();
  });

  it("debounces rapid changes so only the final update runs", () => {
    const context = createContext();
    context.editor.getValue.mockReturnValue("first edit");

    scheduleStatsUpdate.call(context, context.editor);
    vi.advanceTimersByTime(100);
    context.editor.getValue.mockReturnValue("final edit");
    scheduleStatsUpdate.call(context, context.editor);
    vi.advanceTimersByTime(149);

    expect(context.updateEditorStats).not.toHaveBeenCalled();

    vi.advanceTimersByTime(1);
    expect(context.updateEditorStats).toHaveBeenCalledOnce();
    expect(context.editor.getValue).toHaveBeenCalledOnce();
    expect(storeMock.mutations.setEditorDirty).toHaveBeenCalledWith(true);
  });

  it("ignores dirty tracking for an editor instance that is no longer active", () => {
    const staleEditor = { getValue: vi.fn(() => "changed") };
    const context = createContext();

    scheduleStatsUpdate.call(context, staleEditor);
    vi.advanceTimersByTime(150);

    expect(context.updateEditorStats).toHaveBeenCalledOnce();
    expect(staleEditor.getValue).not.toHaveBeenCalled();
    expect(context.isDirty).toBe(false);
    expect(storeMock.mutations.setEditorDirty).not.toHaveBeenCalled();
  });

  it("can schedule a stats-only update without changing dirty state", () => {
    const context = createContext({ isDirty: true });

    scheduleStatsUpdate.call(context);
    vi.advanceTimersByTime(150);

    expect(context.updateEditorStats).toHaveBeenCalledOnce();
    expect(context.isDirty).toBe(true);
    expect(context.editor.getValue).not.toHaveBeenCalled();
    expect(storeMock.mutations.setEditorDirty).not.toHaveBeenCalled();
  });
});
