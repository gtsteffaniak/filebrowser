import { createApp } from "vue";
import i18n from "@/i18n/index.ts";
import { state, getters } from "@/store/index.ts";
import { emitStateChanged } from "@/store/eventBus.ts";
// biome-ignore lint/correctness/noUnresolvedImports: disabled temporarily
import App from "./App.vue";
import { router } from "./router/index.ts";

import "./css/styles.css";
import { initPwaInstall } from "@/utils/pwaInstall.js";
import { defaultDarkMode, syncDocumentTheme, watchSystemTheme } from "@/utils/theme.js";
import { initViewportLayoutListener } from "@/utils/viewport.js";

initPwaInstall();
initViewportLayoutListener();
syncDocumentTheme(defaultDarkMode());
// Live-update the theme when the OS switches appearance while in "auto" mode.
watchSystemTheme(() => {
  syncDocumentTheme(getters.isDarkMode());
  // Re-render bound components too (sidebar toggle icon, profile settings).
  emitStateChanged();
});

const app = createApp(App);

// Install additionals
app.use(i18n);
app.use(router);

// Provide state to the entire application
app.provide("state", state);

// provide v-focus for components
app.directive("focus", {
  mounted: (el) => {
    // A longer timeout is sometimes needed to win a "focus race"
    // against other parts of the app that might be managing focus.
    setTimeout(() => {
      el.focus();
    }, 100);
  },
});

if (import.meta.env.DEV) {
  app.mixin({
    mounted() {
      if (this.$el?.nodeType === 1) {
        this.$el.__vue__ = this;
      }
    },
  });
}

void router.isReady().then(() => app.mount("#app"));
