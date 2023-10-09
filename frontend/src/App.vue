<template>
    <router-view></router-view>
</template>

<script>

// eslint-disable-next-line no-undef
__webpack_public_path__ = window.FileBrowser.StaticURL + "/";

export default {
  name: "app",
  computed: {
  },
  mounted() {
    const loading = document.getElementById("loading");
    loading.classList.add("done");
    setTimeout(() => {
      loading.parentNode.removeChild(loading);
    }, 200);
  },
};
</script>

<style>
/* Always load styles.css */
@import "./css/styles.css";
@import "./css/dark.css";
</style>
