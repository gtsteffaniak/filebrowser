<template>
  <header :class="{ 'dark-mode-header': isDarkMode }">
    <action v-if="notShare" icon="close" :label="$t('buttons.close')" @action="close()" />
    <title v-if="isSettings" class="topTitle">Settings</title>
    <title v-else class="topTitle">{{ req.name }}</title>
  </header>
</template>

<script>
import { getters, state } from "@/store";
import Action from "@/components/Action.vue";

export default {
  name: "listingView",
  components: {
    Action,
  },

  computed: {
    notShare() {
      return getters.currentView() != "share";
    },
    req() {
      return state.req;
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
  },
};
</script>
