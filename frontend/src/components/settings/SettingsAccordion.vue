<template>
  <div class="settings-accordion">
    <slot />
  </div>
</template>

<script>
import { SETTINGS_ACCORDION_KEY } from "./settingsAccordion.js";

export default {
  name: "SettingsAccordion",
  props: {
    modelValue: {
      type: String,
      default: null,
    },
  },
  emits: ["update:modelValue"],
  provide() {
    return {
      [SETTINGS_ACCORDION_KEY]: {
        expanded: () => this.modelValue,
        toggle: (name) => {
          const next = this.modelValue === name ? null : name;
          this.$emit("update:modelValue", next);
        },
      },
    };
  },
};
</script>
