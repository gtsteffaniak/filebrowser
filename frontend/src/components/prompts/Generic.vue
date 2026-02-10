<template>
  <div class="card-content">
    <component v-if="componentName" :is="componentName" v-bind="componentProps"/>
    <div v-else-if="body" v-html="body"></div>
    <div class="card-actions" v-if="displayButtons.length > 0">
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
  </div>
</template>

<script>
import FileList from "@/components/files/FileList.vue";

export default {
  name: "generic-prompt",
  components: {
    FileList,
  },
  props: {
    title: {
      type: String,
      required: true,
    },
    body: {
      type: String,
      required: false,
      default: "",
    },
    componentName: {
      type: String,
      required: false,
      default: "",
    },
    componentProps: {
      type: Object,
      required: false,
      default: () => ({}),
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
      if (Array.isArray(this.buttons)) {
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

<style scoped>
.card-content {
  position: relative;
}

</style>