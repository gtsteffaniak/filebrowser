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
      return [];
    },
  },
  methods: {
    handleButtonClick(button) {
      // Execute the button's action
      if (typeof button.action === 'function') {
        try {
          button.action();
        } catch (error) {
          console.error('Error executing button action:', error);
        }
      }
    },
    getButtonClass(button) {
      // Default button classes
      let classes = 'button button--flat';
      // Add custom class if provided
      if (button.className) {
        classes += ` ${button.className}`;
      }
      return classes;
    },
  },
};
</script>
