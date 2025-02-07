<template>
  <header :class="{ 'dark-mode-header': isDarkMode }">
    <action v-if="notShare" icon="close" :label="$t('buttons.close')" @action="close()" />
    <title v-if="isSettings" class="topTitle">Settings</title>
    <title v-else class="topTitle">{{ req.name }}</title>
    <action v-if="ismarkdownEditable" icon="edit" @action="edit()"/>
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
    ismarkdownEditable() {
      return state.req.type == "text/markdown" && state.user.perm.modify;
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
        const path = removeLastDir(getters.routePath())
        window.location = path + "#" + state.req.name;
        return;
      }
      mutations.replaceRequest({});
      router.go(-1)
    },
  },
};
</script>
