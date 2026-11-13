<template>
  <router-view></router-view>
</template>

<script>
import { onMounted } from "vue";
import { mutations } from "@/store/index.ts"; // Import your store's mutations
import { scheduleFullIcons } from "@/utils/loadFullIcons.js";

mutations.setLoading("main-app", true);
export default {
  name: "app",
  computed: {},
  setup() {
    onMounted(async () => {
      if (document.fonts?.load) {
        try {
          await document.fonts.load("24px 'Material Symbols Core'");
        } catch {
          // Core icons still render once the preloaded font finishes loading.
        }
      }

      mutations.setLoading("main-app", false);
      const loadingLoader = document.querySelector("body > .loader");
      if (loadingLoader) {
        loadingLoader.remove();
      }

      scheduleFullIcons();
    });
  },
};
</script>
