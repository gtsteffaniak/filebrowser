<template>
  <div class="form-flex-group">
    <select class="input" v-on:change="change" :value="viewMode">
      <option v-for="mode in viewModes" :key="mode" :value="mode">
        {{ mode }}
      </option>
    </select>
  </div>
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
