<template>
  <router-view></router-view>
</template>

<script>
import { onMounted } from "vue";
import { mutations } from "@/store"; // Import your store's mutations
mutations.setLoading("main-app", true);
export default {
  name: "app",
  computed: {},
  setup() {
    onMounted(() => {
      mutations.setLoading("main-app", false);
      // Query the loading element and remove it from the DOM
      const loadingDiv = document.getElementById("loading");
      if (loadingDiv) {
        loadingDiv.remove();
      }
    });
  },
};
</script>

<style>
/* Always load styles.css */
@import "./css/styles.css";
@import "./css/dark.css";
</style>
