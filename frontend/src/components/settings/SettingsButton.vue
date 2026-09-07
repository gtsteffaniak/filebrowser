<template>
  <div class="toggle-container" :class="{ disabled }">
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
  },
  emits: ["click"],
};
</script>

<style scoped>
.toggle-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 1rem;
}

.toggle-name-container {
  display: flex;
  align-items: center;
}

.tooltip-info-icon {
  font-size: 1.2rem;
  cursor: pointer;
}

.toggle-container.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.toggle-container.disabled .toggle-name {
  color: #999;
}
</style>
