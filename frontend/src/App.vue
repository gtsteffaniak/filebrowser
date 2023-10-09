<template>
  <router-view></router-view>
</template>

<script>
import { mapState } from "vuex";

// eslint-disable-next-line no-undef
__webpack_public_path__ = window.FileBrowser.StaticURL + "/";

export default {
  name: "app",
  computed: {
    ...mapState(["user"]),
    isDarkMode() {
      return this.user.darkMode;
    },
  },
  mounted() {
  },
};
</script>

<style>
/* Always load styles.css */
@import "./css/styles.css";
</style>

<!-- Use a conditional style block to load dark mode styles -->
<style v-if="isDarkMode">
  @import url('./css/dark.css');
</style>
