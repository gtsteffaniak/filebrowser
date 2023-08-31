<template>
  <router-view></router-view>
</template>

<script>
// eslint-disable-next-line no-undef

export default {
  name: "app",
  mounted() {
    const loading = document.getElementById("loading");
    loading.classList.add("done");

    setTimeout(function () {
      loading.parentNode.removeChild(loading);
    }, 200);
  },
};
</script>

<style>
@import "./css/styles.css";
</style>
