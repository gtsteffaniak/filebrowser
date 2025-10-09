<template>
  <header v-if="!isOnlyOffice" :class="['flexbar', { 'dark-mode-header': isDarkMode }]">
    <action
      v-if="!disableNavButtons"
      icon="close_back"
      :label="$t('buttons.close')"
      :disabled="isDisabledMultiAction"
      @action="multiAction"
    />
    <search v-if="showSearch" />
    <title v-else class="topTitle">{{ getTopTitle }}</title>
    <action
      v-if="isListingView && !disableNavButtons"
      class="menu-button"
      :icon="viewIcon"
      :label="$t('buttons.switchView')"
      @action="switchView"
      :disabled="isDisabled"
    />
    <action
      class="overflow-menu-button"
      v-else-if="!isListingView && !showQuickSave"
      :icon="iconName"
      :disabled="noItems"
      @click="toggleOverflow"
    />
    <action
      class="save-button"
      v-else-if="showQuickSave"
      id="save-button"
      icon="save"
      :label="$t('buttons.save')"
      @action="save()"
    />
  </header>
</template>

<script>
import router from "@/router";
import buttons from "@/utils/buttons";
import { notify } from "@/notify";
import { eventBus } from "@/store/eventBus";
import { getters, state, mutations } from "@/store";
import Action from "@/components/Action.vue";
import Search from "@/components/Search.vue";
import { globalVars, shareInfo } from "@/utils/constants";
import { url } from "@/utils";

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
    getTopTitle() {
      if (getters.isSettings()) {
        return this.$t("sidebar.settings");
      }
      if (getters.isShare() && shareInfo.title && state.req.type === "directory") {
        return shareInfo.title;
      }
      return state.req.name;
    },
    showQuickSave() {
      if (getters.currentView() != "editor" || !state.user.permissions.modify) {
        return false;
      }
      return state.user.editorQuickSave;
    },
    disableNavButtons() {
      return (globalVars.disableNavButtons && this.isListingView) || (getters.isShare() && shareInfo.disableNavButtons);
    },
    isOnlyOffice() {
      return getters.currentView() === "onlyOfficeEditor";
    },
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
        list: "view_list",
        compact: "table_rows_narrow",
        normal: "view_module",
        gallery: "grid_view",
      };
      return icons[getters.viewMode()] || "grid_view";
    },
    isShare() {
      return getters.isShare();
    },
    noItems() {
      return !state.contextMenuHasItems;
    },
    showEdit() {
      return window.location.hash != "#edit" && state.user.permissions.modify;
    },
    showDelete() {
      return state.user.permissions.modify && getters.currentView() == "preview";
    },
    showSave() {
      return getters.currentView() == "editor" && state.user.permissions.modify;
    },
    showSearch() {
      return getters.isLoggedIn() && getters.currentView() === "listingView" && !getters.isShare();
    },
    isDisabled() {
      return state.isSearchActive || getters.currentPromptName() != "";
    },
    isDisabledMultiAction() {
      const shareDisabled = shareInfo.disableSidebar && getters.multibuttonState() === "menu";
      return this.isDisabled || (getters.isStickySidebar() && getters.multibuttonState() === "menu") || shareDisabled;
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
    async save() {
      const button = "save";
      buttons.loading("save");
      try {
        eventBus.emit("handleEditorValueRequest", "data");
        buttons.success(button);
        notify.showSuccess("File Saved!");
      } catch (e) {
        buttons.done(button);
        notify.showError("Error saving file: ", e);
      }
    },
    toggleOverflow() {
      if (getters.currentPromptName() === "OverflowMenu") {
        mutations.closeHovers();
      } else {
        mutations.showHover({ name: "OverflowMenu" });
      }
    },
    switchView() {
      mutations.closeHovers();
      const index = this.viewModes.indexOf(getters.viewMode());
      const next = (index + 1) % this.viewModes.length;
      const newViewMode = this.viewModes[next];
      mutations.updateDisplayPreferences({ viewMode: newViewMode });
      mutations.updateCurrentUser({ viewMode: newViewMode });
    },
    multiAction() {
      const cv = getters.currentView();
      if (cv == "listingView" || ( getters.isShare() && !getters.multibuttonState() === "close")) {
        mutations.toggleSidebar();
      } else if (cv == "settings" && state.isMobile) {
        mutations.toggleSidebar();
      } else {
        mutations.closeHovers();
        if (cv === "settings") {
          if (state.previousHistoryItem?.name) {
            console.log('multiAction', state.previousHistoryItem)
            url.goToItem(state.previousHistoryItem.source, state.previousHistoryItem.path, state.previousHistoryItem);
            return;
          }
          router.push({ path: "/files" });
          return;
        }
        if (getters.isPreviewView()) {
          if (state.previousHistoryItem?.name) {
            console.log('multiAction', state.previousHistoryItem)
            url.goToItem(state.previousHistoryItem.source, state.previousHistoryItem.path, state.previousHistoryItem);
            return;
          } else {
            // navigate to parent directory of current url
            const parentPath = url.removeLastDir(state.route.path);
            router.push({ path: parentPath });
          }
          return;
        }

        router.go(-1);
      }
    },
  },
};
</script>

<style scoped>
header button:hover {
  box-shadow: unset !important;
  -webkit-box-shadow: unset !important;
}
header {
  background-color: rgb(37 49 55 / 5%) !important;
}
/* Header with backdrop-filter support */
@supports (backdrop-filter: none) {
  header {
    backdrop-filter: blur(16px) invert(0.1);
  }
  .dark-mode-header {
    background-color: rgb(37 49 55 / 33%) !important;
  }
}
</style>