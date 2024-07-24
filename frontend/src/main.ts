import { createApp } from 'vue';
import router from './router'; // Adjust the path as per your setup
import App from './App.vue'; // Adjust the path as per your setup
import { state } from './store'; // Adjust the path as per your setup
import i18n from './i18n'; // Adjust the path as per your setup

import './css/styles.css';

const app = createApp(App);

// Global mixin to expose Vue instance to components
app.mixin({
  mounted() {
    // Expose Vue instance to components
    (this.$el as any).__vue__ = this;
  },
});

// Global directive v-focus
app.directive('focus', {
  mounted(el) {
    // Initiate focus for the element
    el.focus();
  },
});

// Install router
app.use(router);

// Install i18n
app.use(i18n);

// Provide state to the entire application
app.provide('state', state);

// Wait for router to be ready before mounting the app
router.isReady().then(() => {
  app.mount('#app');
});
