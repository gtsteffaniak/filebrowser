<template>
  <header :class="['flexbar', { 'dark-mode-header': isDarkMode }]">
    <action
      v-if="!isShare"
      icon="close_back"
      :label="$t('buttons.close')"
      @action="multiAction"
    />
    <search v-if="showSearch" />
    <title v-else-if="isSettings" class="topTitle">Settings</title>
    <title v-else class="topTitle">{{ req.name }}</title>
    <action
      v-if="isListingView"
      class="menu-button"
      :icon="viewIcon"
      :label="$t('buttons.switchView')"
      @action="switchView"
      :disabled="isSearchActive"
    />
    <action
      v-else-if="!isShare"
      :icon="iconName"
      :disabled="noItems"
      @click="toggleOverflow"
    />
  </header>
</template>

<script>
import router from "@/router";
import { getters, state, mutations } from "@/store";
import { removeLastDir } from "@/utils/url";
import Action from "@/components/Action.vue";
import Search from "@/components/Search.vue";

export default {
  name: "UnifiedHeader",
  components: {
    Action,
    Search,
  },
  data() {
    return {
      viewModes: ["list", "compact", "normal", "gallery"],
    };
  },
  computed: {
    isListingView() {
      return getters.currentView() == "listingView";
    },
    iconName() {
      return getters.currentPromptName() === "OverflowMenu"
        ? "keyboard_arrow_up"
        : "more_vert";
    },
    viewIcon() {
      const icons = {
        list: "view_module",
        compact: "view_module",
        normal: "grid_view",
        gallery: "view_list",
      };
      return icons[state.user.viewMode] || "grid_view";
    },
    isShare() {
      return getters.currentView() == "share";
    },
    noItems() {
      return !this.showEdit && !this.showSave && !this.showDelete;
    },
    showEdit() {
      return window.location.hash == "#edit" && state.user.permissions.modify;
    },
    showDelete() {
      return state.user.permissions.modify && getters.currentView() == "preview";
    },
    showSave() {
      return getters.currentView() == "editor" && state.user.permissions.modify;
    },
    showSearch() {
      return getters.isLoggedIn() && getters.currentView() === "listingView";
    },
    isSearchActive() {
      return state.isSearchActive;
    },
    showSwitchView() {
      return getters.currentView() === "listingView";
    },
    showSidebarToggle() {
      return getters.currentView() === "listingView";
    },
    req() {
      return state.req;
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    isSettings() {
      return getters.isSettings();
    },
  },
  methods: {
    toggleOverflow() {
      if (getters.currentPromptName() === "OverflowMenu") {
        mutations.closeHovers();
      } else {
        mutations.showHover({ name: "OverflowMenu" });
      }
    },
    switchView() {
      mutations.closeHovers();
      const index = this.viewModes.indexOf(state.user.viewMode);
      const next = (index + 1) % this.viewModes.length;
      mutations.updateCurrentUser({ viewMode: this.viewModes[next] });
    },
    multiAction() {
      const listingView = getters.currentView();
      if (listingView == "listingView") {
        mutations.toggleSidebar();
      } else {
        mutations.closeHovers();
        if (listingView == "settings") {
          if (!state.route.path.includes("/settings/users/")) {
            router.push({ path: "/files/", hash: "" });
            return;
          }
        }
        if (listingView === "onlyOfficeEditor") {
          const current = window.location.pathname;
          const newpath = removeLastDir(current);
          window.location = newpath + "#" + state.req.name;
          return;
        }
        mutations.replaceRequest({});
        router.go(-1);
      }
    },
  },
};
</script>
