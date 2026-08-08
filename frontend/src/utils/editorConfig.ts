import { reactive } from 'vue';
import { getObjectProperty } from './object';

const STORAGE_KEY = 'editorConfig';

export interface EditorConfig {
  wrapEditorContent: boolean;
  keybinding: string;
  tabSize: number;
  overscroll: number;
  showIndentGuides: boolean;
  showGutter: boolean;
  fixedGutterWidth: boolean;
  showLineNumbers: boolean;
  relativeLineNumbers: boolean;
  customScrollbar: boolean;
}

const DEFAULTS: EditorConfig = {
  wrapEditorContent: false,
  keybinding: '',
  tabSize: 4,
  overscroll: 0,
  showIndentGuides: true,
  showGutter: true,
  fixedGutterWidth: true,
  showLineNumbers: true,
  relativeLineNumbers: false,
  customScrollbar: false,
};

function load(): EditorConfig {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (!stored) return { ...DEFAULTS };
    const parsed = JSON.parse(stored) as unknown;
    if (!parsed || typeof parsed !== 'object') return { ...DEFAULTS };

    const validEntries: [string, unknown][] = [];
    for (const key of Object.keys(DEFAULTS)) {
      const value = getObjectProperty(parsed as Record<string, unknown>, key);
      const fallback = getObjectProperty(DEFAULTS, key);
      if (typeof value === typeof fallback) {
        validEntries.push([key, value]);
      }
    }
    return { ...DEFAULTS, ...Object.fromEntries(validEntries) } as unknown as EditorConfig;
  } catch (_) {
    return { ...DEFAULTS };
  }
}

function persist(): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(editorConfig));
  } catch (_) { /* ignore */ }
}

export const editorConfig: EditorConfig = reactive(load());

export function saveEditorConfig(partial: Partial<EditorConfig>): void {
  Object.assign(editorConfig, partial);
  persist();
}

export function resetEditorConfig(): void {
  Object.assign(editorConfig, DEFAULTS);
  persist();
}
