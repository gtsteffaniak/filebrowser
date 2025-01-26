<template>
  <header :class="{ 'dark-mode-header': isDarkMode }">
    <action v-if="notShare" icon="close" :label="$t('buttons.close')" @action="close()" />
    <title v-if="isSettings" class="topTitle">Settings</title>
    <title v-else class="topTitle">{{ req.name }}</title>
    <action icon="hide_source" />
  </header>
</template>

<script>
import { url } from "@/utils";
import router from "@/router";
import { getters, state, mutations } from "@/store";
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
  methods: {
    close() {
      mutations.closeHovers();

      if (getters.isSettings()) {
        // Use this.isSettings to access the computed property
        router.push({ path: "/files/", hash: "" });
        return;
      }

      mutations.replaceRequest({});
      router.go(-1)
    },
  },
};
</script>
