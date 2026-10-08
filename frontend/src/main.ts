import { createApp } from "vue";
import i18n from "@/i18n/index.ts";
import { state } from "@/store/index.ts";
// biome-ignore lint/correctness/noUnresolvedImports: disabled temporarily
import App from "./App.vue";
import router from "./router/index.ts";

import "./css/styles.css";
import { initPwaInstall } from "@/utils/pwaInstall.js";
import { defaultDarkMode, syncDocumentTheme } from "@/utils/theme.js";
import { initViewportLayoutListener } from "@/utils/viewport.js";

initPwaInstall();
initViewportLayoutListener();
syncDocumentTheme(defaultDarkMode());

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

app.mixin({
  mounted() {
    // expose vue instance to components
    this.$el.__vue__ = this;
  },
});

void router.isReady().then(() => app.mount("#app"));
