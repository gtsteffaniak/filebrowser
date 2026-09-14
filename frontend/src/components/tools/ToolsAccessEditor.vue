<template>
  <div :class="rootClass">
    <p v-if="description">{{ description }}</p>
    <div class="settings-items tools-access-list">
      <div v-for="entry in displayEntries" :key="entry.toolId" class="tool-access-item">
        <div class="tool-access-header">
          <i class="material-symbols tool-access-icon">{{ entry.icon }}</i>
          <div class="tool-access-text">
            <span class="tool-access-name">{{ entry.name }}</span>
            <span v-if="entry.description" class="tool-access-description">{{ entry.description }}</span>
          </div>
        </div>
        <ToggleSwitch
          class="item"
          :enforceable="isDefaultsMode"
          :model-value="entry.enabled"
          :enforced="entry.enforced"
          :disabled="disabled || entry.toggleDisabled"
          :value-tooltip="entry.toggleTooltip"
          :name="entry.name"
          :description="entry.toggleDescription"
          @update:model-value="(value) => onEnabledChange(entry.toolId, value)"
          @update:enforced="(value) => onEnforcedChange(entry.toolId, value)"
        />
      </div>
    </div>
    <div v-if="showPromptActions" class="card-actions">
      <button
        type="button"
        class="button button--flat button--grey"
        :disabled="disabled"
        @click="$emit('cancel')"
      >
        {{ $t("general.cancel") }}
      </button>
      <button
        type="button"
        class="button button--flat"
        :disabled="disabled"
        @click="$emit('save')"
      >
        {{ $t("general.save") }}
      </button>
    </div>
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
  },
  emits: ["update:modelValue", "update:userToolAccess", "save", "cancel"],
  computed: {
    isDefaultsMode() {
      return this.mode === "defaults";
    },
    rootClass() {
      const classes = ["tools-access-editor"];
      if (this.embedded) {
        classes.push("card-content", "prompt-panel");
      } else {
        classes.push("card-content");
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
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  gap: 0.75rem;
}

.tools-access-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
}

.tool-access-item {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.tool-access-header {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
}

.tool-access-icon {
  color: var(--primaryColor);
}

.tool-access-text {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.tool-access-name {
  font-weight: 500;
}

.tool-access-description {
  font-size: 0.9em;
  color: var(--textSecondary);
}

.tools-access-editor .card-actions {
  flex-shrink: 0;
  margin-top: auto;
}
</style>
