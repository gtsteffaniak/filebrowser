<template>
  <div class="card-content" v-html="body"></div>

  <div class="card-actions">
    <button
      v-for="(button, index) in displayButtons"
      :key="index"
      :class="getButtonClass(button)"
      @click="handleButtonClick(button)"
      :aria-label="button.label"
      :title="button.label"
    >
      {{ button.label }}
    </button>
  </div>
</template>

<script>
import { mutations } from "@/store";

export default {
  name: "generic-prompt",
  props: {
    title: {
      type: String,
      required: true,
    },
    body: {
      type: String,
      required: true,
    },
    buttons: {
      type: Array,
      required: false,
      default: () => [],
    },
  },
  computed: {
    displayButtons() {
      // If buttons are provided, use them
      if (this.buttons && this.buttons.length > 0) {
        return this.buttons;
      }
      // Otherwise, return a default close button
      return [
        {
          label: this.$t('general.close'),
          action: () => {
            // Just close the prompt
          },
        },
      ];
    },
  },
  methods: {
    handleButtonClick(button) {
      // Execute the button's action
      if (typeof button.action === 'function') {
        button.action();
      }
      // Close the prompt unless the button specifies to keep it open
      if (button.keepOpen !== true) {
        mutations.closeTopHover();
      }
    },
    getButtonClass(button) {
      // Default button classes
      let classes = 'button button--flat';
      // Add custom class if provided
      if (button.className) {
        classes += ` ${button.className}`;
      } else if (button.primary) {
        // Primary button style (default)
        // No additional class needed
      } else {
        // Secondary button style (grey)
        classes += ' button--grey';
      }
      return classes;
    },
  },
};
</script>
