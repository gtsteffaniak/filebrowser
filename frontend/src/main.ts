import { disableExternal } from "@/utils/constants";
import { createApp } from "vue";
import router from "@/router";
import App from "@/App.vue";

import "./css/styles.css";


const app = createApp(App);

app.use(router);

app.mixin({
  mounted() {
    // expose vue instance to components
    this.$el.__vue__ = this;
  },
});

// provide v-focus for components
app.directive("focus", {
  mounted: async (el) => {
    // initiate focus for the element
    el.focus();
  },
});


router.isReady().then(() => app.mount("#app"));