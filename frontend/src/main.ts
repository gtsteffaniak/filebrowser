import { createApp } from "vue";
import router from "./router"; // Adjust the path as per your setup
import App from "./App.vue"; // Adjust the path as per your setup
import { state } from "@/store"; // Adjust the path as per your setup
import i18n from "@/i18n";
import VueLazyload from "vue-lazyload";
import VuePlyr from "@skjnldsv/vue-plyr"; // Custom media player

import "./css/styles.css";

const app = createApp(App);

// Install additionals
app.use(VueLazyload);
app.use(i18n);
app.use(router);
app.use(VuePlyr);

// Ensure i18n functions are available globally (needed after removing Vite i18n plugin)
app.config.globalProperties.$t = i18n.global.t as any;
app.config.globalProperties.$tc = i18n.global.tc as any;
app.config.globalProperties.$te = i18n.global.te as any;
app.config.globalProperties.$d = i18n.global.d as any;
app.config.globalProperties.$n = i18n.global.n as any;

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

router.isReady().then(() => app.mount("#app"));
