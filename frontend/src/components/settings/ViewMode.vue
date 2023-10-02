<template>
  <select v-on:change="change" :value="viewMode">
    <option v-for="(viewMode, value) in viewModes" :key="value" :value="value">
      {{viewMode}}
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
      viewModes: {
        card: "normal",
        gallery: "gallery",
        list: "list",
        compactList: "compact",
      },
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
