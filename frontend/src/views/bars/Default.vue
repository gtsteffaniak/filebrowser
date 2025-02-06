<template>
  <header :class="{ 'dark-mode-header': isDarkMode }">
    <action v-if="notShare" icon="close" :label="$t('buttons.close')" @action="close()" />
    <title v-if="isSettings" class="topTitle">Settings</title>
    <title v-else class="topTitle">{{ req.name }}</title>
    <action v-if="ismarkdown" icon="edit" @action="edit()"/>
    <action v-else icon="hide_source" />
  </header>
</template>

<script>
import router from "@/router";
import { getters, state, mutations } from "@/store";
import { removeLastDir } from "@/utils/url";
import Action from "@/components/Action.vue";

export default {
  name: "listingView",
  components: {
    Action,
  },
  computed: {
    ismarkdown() {
      return state.req.type == "text/markdown";
    },
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
    async edit() {
      mutations.setMarkdownEdit(true);
      window.location.hash = "#edit";
    },
    close() {
      mutations.closeHovers();

      if (getters.isSettings()) {
        // Use this.isSettings to access the computed property
        router.push({ path: "/files/", hash: "" });
        return;
      }

      if (getters.currentView() === "onlyOfficeEditor") {
        // fixes a bug, but this fix also means scroll location memory is not preserved
        window.location = removeLastDir(getters.routePath()); // Load last page as if navigating normally
        return;
      }
      mutations.replaceRequest({});
      router.go(-1)
    },
  },
};
</script>
