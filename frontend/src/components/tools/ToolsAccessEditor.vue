<template>
  <div :class="rootClass">
    <p v-if="description">{{ description }}</p>
    <div class="settings-items tools-access-list">
      <div v-for="entry in displayEntries" :key="entry.toolId" class="tool-access-item">
        <i class="material-symbols tool-access-icon" aria-hidden="true">{{ entry.icon }}</i>
        <ToggleSwitch
          class="item tool-access-toggle"
          :enforceable="isDefaultsMode"
          :model-value="entry.enabled"
          :enforced="entry.enforced"
          :disabled="disabled || entry.toggleDisabled"
          :value-tooltip="entry.toggleTooltip"
          :name="entry.name"
          :description="entry.description || entry.toggleDescription"
          @update:model-value="(value) => onEnabledChange(entry.toolId, value)"
          @update:enforced="(value) => onEnforcedChange(entry.toolId, value)"
        />
      </div>
    </div>
  </div>
  <div v-if="showPromptActions" class="card-actions">
    <button
      type="button"
      class="button button--flat"
      :disabled="disabled"
      @click="$emit('cancel')"
    >
      {{ $t("general.cancel") }}
    </button>
    <button
      type="button"
      class="button button--flat button--blue"
      :disabled="disabled"
      @click="$emit('save')"
    >
      {{ $t("general.save") }}
    </button>
  </div>
</template>

<script>
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { tools } from "@/utils/constants";
import { normalizeUserToolAccess } from "@/utils/toolAccess";

export default {
  name: "ToolsAccessEditor",
  components: {
    ToggleSwitch,
  },
  props: {
    mode: {
      type: String,
      default: "defaults",
    },
    modelValue: {
      type: Array,
      default: () => [],
    },
    userToolAccess: {
      type: Object,
      default: () => ({}),
    },
    description: {
      type: String,
      default: "",
    },
    disabled: {
      type: Boolean,
      default: false,
    },
    showPromptActions: {
      type: Boolean,
      default: true,
    },
    embedded: {
      type: Boolean,
      default: false,
    },
    promptShell: {
      type: Boolean,
      default: null,
    },
  },
  emits: ["update:modelValue", "update:userToolAccess", "save", "cancel"],
  computed: {
    isDefaultsMode() {
      return this.mode === "defaults";
    },
    usesPromptShell() {
      if (this.promptShell !== null) {
        return this.promptShell;
      }
      return this.embedded;
    },
    rootClass() {
      const classes = ["tools-access-editor"];
      if (this.usesPromptShell) {
        classes.push("card-content", "prompt-panel");
      }
      return classes;
    },
    catalog() {
      return tools();
    },
    displayEntries() {
      if (this.isDefaultsMode) {
        const byId = new Map(
          (Array.isArray(this.modelValue) ? this.modelValue : []).map((item) => [
            item.toolId,
            item,
          ])
        );
        return this.catalog.map((tool) => {
          const item = byId.get(tool.id) || { toolId: tool.id, enabled: true, enforced: false };
          return {
            toolId: tool.id,
            name: tool.name,
            description: tool.description,
            icon: tool.icon,
            enabled: !!item.enabled,
            enforced: !!item.enforced,
            toggleDisabled: false,
            toggleTooltip: "",
            toggleDescription: "",
          };
        });
      }
      return this.catalog.map((tool) => {
        const policyItem = (Array.isArray(this.modelValue) ? this.modelValue : []).find(
          (entry) => entry?.toolId === tool.id
        );
        const enforced = !!policyItem?.enforced;
        const userValue = this.userToolAccess?.[tool.id];
        const enabled = typeof userValue === "boolean" ? userValue : this.defaultEnabledFor(tool.id);
        return {
          toolId: tool.id,
          name: tool.name,
          description: tool.description,
          icon: tool.icon,
          enabled,
          enforced,
          toggleDisabled: enforced,
          toggleTooltip: enforced ? this.$t("profileSettings.enforcedByAdmin") : "",
          toggleDescription: "",
        };
      });
    },
  },
  methods: {
    defaultEnabledFor(toolId) {
      const item = (Array.isArray(this.modelValue) ? this.modelValue : []).find(
        (entry) => entry?.toolId === toolId
      );
      return item ? !!item.enabled : true;
    },
    onEnabledChange(toolId, enabled) {
      if (this.isDefaultsMode) {
        this.emitDefaultsUpdate(toolId, { enabled });
        return;
      }
      this.$emit(
        "update:userToolAccess",
        normalizeUserToolAccess(
          {
            ...(this.userToolAccess || {}),
            [toolId]: enabled,
          },
          this.modelValue
        )
      );
    },
    onEnforcedChange(toolId, enforced) {
      if (!this.isDefaultsMode) {
        return;
      }
      this.emitDefaultsUpdate(toolId, { enforced });
    },
    emitDefaultsUpdate(toolId, patch) {
      const current = Array.isArray(this.modelValue) ? [...this.modelValue] : [];
      const index = current.findIndex((item) => item?.toolId === toolId);
      const existing = index >= 0 ? current.at(index) : undefined;
      const base = existing
        ? { ...existing }
        : { toolId, enabled: true, enforced: false };
      const nextItem = { ...base, ...patch, toolId };
      const next = index >= 0
        ? current.map((item, itemIndex) => (itemIndex === index ? nextItem : item))
        : [...current, nextItem];
      this.$emit("update:modelValue", next);
    },
  },
};
</script>

<style scoped>
.tools-access-editor {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.tools-access-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.tool-access-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.35em;
  padding-left: 0.75em;
  border-radius: var(--borderRadius);
  transition: background-color 0.15s ease;
}

.tools-access-list > .tool-access-item:hover {
  background-color: var(--surfaceSecondary);
}

.tool-access-item:hover :deep(.toggle-container--enforceable),
.tool-access-item:hover :deep(.toggle-container--enforceable:hover) {
  background-color: transparent;
}

.tool-access-icon {
  flex-shrink: 0;
  color: var(--primaryColor);
}

.tool-access-toggle {
  flex: 1 1 auto;
  min-width: 0;
}

.tool-access-toggle :deep(.toggle-container--enforceable) {
  padding: 0;
}
</style>
