<template>
  <header>
    <slot />
  </header>
</template>

<script>
import { logoURL } from "@/utils/constants";
import Action from "@/components/header/Action.vue";

export default {
  name: "header-bar",
  props: ["showLogo", "showMenu"],
  components: {
    Action,
  },
  data: function () {
    return {
      logoURL,
    };
  },
  methods: {
    toggleSidebar() {
      if ( this.$store.state.show == "sidebar" ) {
        this.$store.commit("closeHovers");
      } else {
        this.$store.commit("showHover", "sidebar");
      }
    },
  },
};
</script>

<style></style>
