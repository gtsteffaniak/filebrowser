<template>
  <button @click="action" :aria-label="label" :title="label" class="action">
    <i class="material-icons">{{ icon }}</i>
    <span>{{ label }}</span>
    <span v-if="counter > 0" class="counter">{{ counter }}</span>
  </button>
</template>

<script>
import { state, getters, mutations } from "@/store"; // Import your custom store

export default {
  name: "action",
  props: ["icon", "label", "counter", "show"],
  methods: {
    action: function () {
      console.log("show",this.show)

      if (this.show) {
        mutations.showHover(this.show);
      }
      this.$emit("action");
    },
  },
};
</script>

<style></style>
