<template>
  <select v-on:change="change" :value="viewMode">
    <option v-for="mode in viewModes" :key="mode" :value="mode">
      {{ mode }}
    </option>
  </select>
</template>

<script>
export default {
  name: "ViewMode",
  props: ["viewMode"],
  data() {
    return {
      viewModes: ['list', 'compact', 'normal', 'gallery'],
    };
  },
  methods: {
    change(event) {
      this.$emit("update:viewMode", event.target.value);
    },
  },
};
</script>
