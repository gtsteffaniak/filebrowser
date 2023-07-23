<template>
  <header>
    <action
      class="menu-button"
      icon="menu"
      :label="$t('buttons.toggleSidebar')"
      @action="toggleSidebar()"
    />

    <slot />

    <div id="dropdown" :class="{ active: this.$store.state.show === 'more' }">
      <slot name="actions" />
    </div>

  </header>
</template>

<script>
import { logoURL } from "@/utils/constants";
import Action from "@/components/header/Action";

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
