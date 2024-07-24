import { createApp } from 'vue';
import router from './router'; // Adjust the path as per your setup
import App from './App.vue'; // Adjust the path as per your setup
import { state } from '@/store'; // Adjust the path as per your setup
import i18n, { isRtl } from "@/i18n";
import VueLazyload from "vue-lazyload";

import './css/styles.css';

const app = createApp(App);

// provide v-focus for components
app.directive("focus", {
  mounted: async (el) => {
    // initiate focus for the element
    el.focus();
  },
});

// Install additionals
app.use(VueLazyload);
app.use(i18n);
app.use(router);

// Provide state to the entire application
app.provide('state', state);

// provide v-focus for components
app.directive("focus", {
  mounted: async (el) => {
    // initiate focus for the element
    el.focus();
  },
});

app.mixin({
  mounted() {
    // expose vue instance to components
    this.$el.__vue__ = this;
  },
});

router.isReady().then(() => app.mount("#app"));