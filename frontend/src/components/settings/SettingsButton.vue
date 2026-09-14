<template>
  <div
    class="settings-button"
    :class="[
      valueRow ? 'settings-button--value-row' : 'toggle-container',
      { disabled },
    ]"
  >
    <div class="toggle-name-container">
      <span class="toggle-name">{{ name }}</span>
      <HelpTooltipIcon v-if="description" :text="description" />
    </div>
    <button
      type="button"
      class="button button--settings-control"
      :disabled="disabled"
      :aria-label="ariaLabel || name"
      @click="$emit('click')"
    >
      <i class="material-symbols-outlined">open_in_new</i>
    </button>
  </div>
</template>

<script>
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";

export default {
  name: "SettingsButton",
  components: { HelpTooltipIcon },
  props: {
    name: {
      type: String,
      required: true,
    },
    description: {
      type: String,
      default: "",
    },
    ariaLabel: {
      type: String,
      default: "",
    },
    disabled: {
      type: Boolean,
      default: false,
    },
    /** Render as a single row inside ProfileEnforceableField (no nested toggle-container). */
    valueRow: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["click"],
};
</script>

<style scoped>
.settings-button.toggle-container,
.settings-button--value-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 1rem;
  width: 100%;
}

.settings-button--value-row {
  min-height: unset;
}

.toggle-name-container {
  display: flex;
  align-items: center;
}

.tooltip-info-icon {
  font-size: 1.2rem;
  cursor: pointer;
}

.settings-button.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.settings-button.disabled .toggle-name {
  color: #999;
}
</style>
