<template>
  <select v-on:change="change" :value="viewMode">
    <option v-for="(viewMode) in viewModes" :key="viewMode" :value="viewMode">
      {{ viewMode }}
    </option>
  </select>
</template>

<script>
export default {
  name: "viewMode",
  props: ["viewMode"],
  data() {
    let dataObj = {
      viewMode: "list",
      viewModes: ['list', 'compact', 'normal', 'gallery'],
    };

    Object.defineProperty(dataObj, "viewMode", {
      configurable: false,
      writable: false,
    });

    return dataObj;
  },
  methods: {
    change(event) {
      this.$emit("update:viewMode", event.target.value);
    },
  },
};
</script>
