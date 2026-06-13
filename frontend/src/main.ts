import { createApp } from "vue";
import i18n from "@/i18n";
import { state } from "@/store"; // Adjust the path as per your setup
import App from "./App.vue"; // Adjust the path as per your setup
import router from "./router"; // Adjust the path as per your setup

import "./css/styles.css";
import "./css/dark.css";
import { initPwaInstall } from "@/utils/pwaInstall";

initPwaInstall();

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
