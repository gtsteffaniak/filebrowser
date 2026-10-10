<template>
  <div class="card-content editor-settings-content settings-items">
    <div v-for="field in fields" :key="field.key" class="setting-row item">
      <div class="setting-label">
        <span :id="`editor-${field.key}-label`">{{ field.label }}</span>
        <i
          class="no-select material-symbols-outlined tooltip-info-icon"
          @mouseenter="showTooltip($event, field.desc)"
          @mouseleave="hideTooltip"
        >help</i>
      </div>
      <div class="setting-control">
        <ExpandDropdown
          v-if="field.type === 'dropdown'"
          :options="field.options"
          :model-value="(config[field.key] as string | number)"
          @update:model-value="setValue(field, $event)"
          :aria-label="field.aria"
        />
        <input
          v-else
          type="number"
          class="input"
          :value="config[field.key]"
          @change="setValue(field, ($event.target as HTMLInputElement).value)"
          :min="field.min"
          :max="field.max"
          :aria-label="field.aria"
        />
      </div>
    </div>
    <ToggleSwitch
      v-for="toggle in toggles"
      :key="toggle.key"
      class="item"
      :name="toggle.label"
      :description="toggle.desc"
      :model-value="(config[toggle.key] as boolean)"
      @update:model-value="setValue(toggle, $event)"
    />
  </div>
  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--grey"
      @click="resetEditorConfig"
      :aria-label="$t('player.visualizer.resetDefaults')"
      :title="$t('player.visualizer.resetDefaults')"
    > {{ $t("player.visualizer.resetDefaults") }}
    </button>
    <button
      type="button"
      class="button button--flat"
      @click="closeTopPrompt"
      :aria-label="$t('general.ok')"
      :title="$t('general.ok')"
    > {{ $t("general.ok") }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { mutations } from "@/store";
import { editorConfig, saveEditorConfig, resetEditorConfig, type EditorConfig } from "@/utils/editorConfig";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

interface EditorSettingField {
  key: keyof EditorConfig;
  label: string;
  desc: string;
  type?: "dropdown" | "number";
  numeric?: boolean;
  aria?: string;
  min?: number;
  max?: number;
  options?: { value: string | number; label: string }[];
}

defineProps({
  promptId: { type: [String, Number], default: null },
});

defineOptions({ name: "EditorSettings" });

const { t } = useI18n();
const config = editorConfig;

const fields = computed<EditorSettingField[]>(() => [
  {
    key: "keybinding",
    type: "dropdown",
    label: t("editor.settings.keybinding.label"),
    desc: t("editor.settings.keybinding.description"),
    aria: t("editor.settings.keybinding.label"),
    options: [
      { value: "", label: t("editor.settings.keybinding.ace") },
      { value: "ace/keyboard/vim", label: t("editor.settings.keybinding.vim") },
      { value: "ace/keyboard/emacs", label: t("editor.settings.keybinding.emacs") },
      { value: "ace/keyboard/sublime", label: t("editor.settings.keybinding.sublime") },
      { value: "ace/keyboard/vscode", label: t("editor.settings.keybinding.vscode") },
    ],
  },
  {
    key: "tabSize",
    type: "number",
    label: t("editor.settings.tabSize.label"),
    desc: t("editor.settings.tabSize.description"),
    aria: t("editor.settings.tabSize.label"),
    min: 1,
    max: 16,
  },
  {
    key: "overscroll",
    type: "dropdown",
    numeric: true,
    label: t("editor.settings.overscroll.label"),
    desc: t("editor.settings.overscroll.description"),
    aria: t("editor.settings.overscroll.label"),
    options: [
      { value: 0, label: t("editor.settings.overscroll.none") },
      { value: 0.5, label: t("editor.settings.overscroll.half") },
      { value: 1, label: t("editor.settings.overscroll.full") },
    ],
  },
]);

const toggles = computed<EditorSettingField[]>(() => {
  const items: (EditorSettingField | false)[] = [
    { key: "wrapEditorContent", label: t("editor.settings.wrapContent"), desc: t("editor.settings.wrapContentDescription") },
    config.wrapEditorContent && {
      key: "indentedSoftWrap",
      label: t("editor.settings.indentedSoftWrap"),
      desc: t("editor.settings.indentedSoftWrapDescription"),
    },
    { key: "showIndentGuides", label: t("editor.settings.showIndentGuides"), desc: t("editor.settings.showIndentGuidesDescription") },
    { key: "showGutter", label: t("editor.settings.showGutter"), desc: t("editor.settings.showGutterDescription") },
    { key: "fixedGutterWidth", label: t("editor.settings.fixedGutterWidth"), desc: t("editor.settings.fixedGutterWidthDescription") },
    { key: "showLineNumbers", label: t("editor.settings.showLineNumbers"), desc: t("editor.settings.showLineNumbersDescription") },
    { key: "relativeLineNumbers", label: t("editor.settings.relativeLineNumbers"), desc: t("editor.settings.relativeLineNumbersDescription") },
    { key: "customScrollbar", label: t("editor.settings.customScrollbar"), desc: t("editor.settings.customScrollbarDescription") },
    { key: "enableAutocompletion", label: t("editor.settings.autocompletion"), desc: t("editor.settings.autocompletionDescription") },
    config.enableAutocompletion && {
      key: "enableLiveAutocompletion",
      label: t("editor.settings.liveAutocompletion"),
      desc: t("editor.settings.liveAutocompletionDescription"),
    },
  ];
  return items.filter((item): item is EditorSettingField => item !== false);
});

function setValue(field: EditorSettingField, value: string | number | boolean) {
  let val: string | number | boolean = value;
  if (field.type === "number") {
    val = Math.min(field.max as number, Math.max(field.min as number, Math.round(Number(value)) || (field.min as number)));
  } else if (field.numeric) {
    val = Number(value);
  }
  saveEditorConfig({ [field.key]: val } as Partial<EditorConfig>);
}

function closeTopPrompt(): void {
  mutations.closeTopPrompt();
}

function showTooltip(event: MouseEvent, text: string): void {
  mutations.showTooltip({
    content: text,
    x: event.clientX,
    y: event.clientY,
  });
}

function hideTooltip(): void {
  mutations.hideTooltip();
}
</script>

<style scoped>
.settings-items .item {
  padding-top: 0.75em;
  padding-bottom: 0.75em;
}

.editor-settings-content {
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 0 0.75em;
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1em;
  min-width: 0;
}

.setting-row .setting-label span {
  color: var(--textPrimary);
  font-size: 0.95em;
  flex-shrink: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.setting-label {
  display: flex;
  align-items: center;
  gap: 0.3em;
  min-width: 0;
  flex-shrink: 1;
}

.setting-control {
  flex: 1 1 auto;
  min-width: 0;
  max-width: 13em;
  display: flex;
  justify-content: flex-end;
}

.setting-control input[type="number"] {
  width: 100%;
  max-width: 6em;
  text-align: right;
}
</style>
