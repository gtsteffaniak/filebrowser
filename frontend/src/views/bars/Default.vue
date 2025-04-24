<template>
  <header :class="{ 'dark-mode-header': isDarkMode }">
    <action
      v-if="notShare"
      icon="close_back"
      :label="$t('buttons.close')"
      @action="close()"
    />
    <title v-if="isSettings" class="topTitle">Settings</title>
    <title v-else class="topTitle">{{ req.name }}</title>
    <action :icon="iconName" @click="toggleOverflow" />
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
  mounted() {},
  computed: {
    iconName() {
      let icon = "more_vert";
      if (getters.currentPromptName() == "OverflowMenu") {
        icon = "keyboard_arrow_up";
      }
      return icon;
    },
    ismarkdownEditable() {
      return state.req.type == "text/markdown" && state.user.permissions.modify;
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
    toggleOverflow() {
      if (getters.currentPromptName() == "OverflowMenu") {
        mutations.closeHovers();
        return;
      } else {
        mutations.showHover({
          name: "OverflowMenu",
        });
      }
    },

    close() {
      mutations.closeHovers();

      if (getters.isSettings()) {
        // Use this.isSettings to access the computed property
        router.push({ path: "/files/", hash: "" });
        return;
      }

      if (getters.currentView() === "onlyOfficeEditor") {
        const current = window.location.pathname;
        const newpath = removeLastDir(current);
        window.location = newpath + "#" + state.req.name;
        return;
      }
      mutations.replaceRequest({});
      router.go(-1);
    },
  },
};
</script>
