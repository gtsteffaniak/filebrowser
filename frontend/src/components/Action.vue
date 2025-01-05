<template>
  <button @click="action" :aria-label="label" :title="label" class="action no-select">
    <i class="material-icons">{{ icon }}</i>
    <span>{{ label }}</span>
    <span v-if="counter > 0" class="counter">{{ counter }}</span>
  </button>
</template>

<script>
import { mutations } from "@/store"; // Import your custom store

export default {
  name: "action",
  props: ["icon", "label", "counter", "show"],
  methods: {
    action: function () {
      if (this.show) {
        mutations.showHover(this.show);
      }
      this.$emit("action");
    },
  },
};
</script>

<style>

</style>
