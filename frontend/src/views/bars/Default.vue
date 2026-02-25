<template>
  <header v-if="!isOnlyOffice" :class="['flexbar', { 'dark-mode-header': isDarkMode }]">
    <action
      v-if="!disableNavButtons"
      icon="close_back"
      :label="$t('general.close')"
      :disabled="isDisabledMultiAction"
      @action="multiAction"
    />
    <div class="search-bar-container" v-if="showSearch && !isSearchActive" @click="openSearch">
      <i class="material-icons">search</i>
      <input 
        type="text" 
        id="search-bar-input" 
        :placeholder="$t('general.search', { suffix: '...' })" 
        readonly
      />
    </div>
    <title v-if="!showSearch" class="topTitle">{{ getTopTitle }}</title>
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
      :label="$t('general.save')"
      @action="save()"
    />
  </header>
</template>

<script>
import router from "@/router";
import buttons from "@/utils/buttons";
import { notify } from "@/notify";
import { getters, state, mutations } from "@/store";
import Action from "@/components/Action.vue";
import { globalVars } from "@/utils/constants";
import { url } from "@/utils";

export default {
  name: "UnifiedHeader",
  components: {
    Action,
  },
  data() {
    return {
      viewModes: ["list", "normal", "icons"],
    };
  },
  computed: {
    getTopTitle() {
      if (getters.isSettings()) {
        return this.$t("general.settings");
      }
      if (getters.isShare()) {
        if (state.req?.type === "directory" || state.shareInfo?.shareType === "upload") {
          return state.shareInfo?.title;
        }
        return state.req.name;
      }
      const currentTool = getters.currentTool();
      if (currentTool) {
        return currentTool.name;
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
      const isShare = getters.isShare();
      const regularDisabled = globalVars.disableNavButtons && this.isListingView;
      const shareDisabled = isShare && state.shareInfo?.hideNavButtons && getters.currentView() == "listingView";
      const uploadShare = isShare && state.shareInfo?.shareType === "upload"
      return regularDisabled || shareDisabled || uploadShare;
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
        compact: "view_list",
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
    isSearchActive() {
      return state.isSearchActive;
    },
    isDisabled() {
      return state.isSearchActive || getters.currentPromptName() != "";
    },
    isDisabledMultiAction() {
      const regularDisabled = getters.isStickySidebar() && getters.multibuttonState() === "menu";
      const shareDisabled = state.shareInfo?.disableSidebar && getters.multibuttonState() === "menu";
      return this.isDisabled || regularDisabled || shareDisabled;
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
    openSearch() {
      if (!state.isSearchActive) {
        mutations.closeHovers();
        mutations.closeSidebar();
        mutations.resetSelected();
        mutations.setSearch(true);
        // this is hear to allow for animation
        setTimeout(() => {
          const resultList = document.getElementById("result-list");
          resultList.classList.add("active");
          document.getElementById("search-input").focus();
        }, 100);
      }
    },
    async save() {
      const button = "save";
      buttons.loading("save");
      try {
        // Call the editor's save handler directly
        if (state.editorSaveHandler) {
          await state.editorSaveHandler();
          buttons.success(button);
          // Note: Success notification is shown by the editor
        } else {
          const errorMsg = "No editor save handler registered";
          notify.showError(errorMsg);
          throw new Error(errorMsg);
        }
      } catch (e) {
        buttons.done(button);
        // Note: Error notification is already shown by the editor
        throw e; // Re-throw so caller knows save failed
      }
    },
    toggleOverflow() {
      if (getters.currentPromptName() === "OverflowMenu") {
        mutations.closeTopHover();
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

      // Check for unsaved editor changes before navigation
      if (cv === "editor" && state.editorDirty) {
        this.showSaveBeforeExitPrompt(() => this.performNavigation(cv));
        return;
      }

      this.performNavigation(cv);
    },
    performNavigation(cv) {
      if (cv == "listingView" || ( getters.isShare() && !getters.multibuttonState() === "close")) {
        mutations.toggleSidebar();
      } else if (cv == "settings" && state.isMobile) {
        mutations.toggleSidebar();
      } else {
        mutations.closeHovers();
        if (cv === "settings") {
          if (state.previousHistoryItem?.name) {
            url.goToItem(state.previousHistoryItem.source, state.previousHistoryItem.path, state.previousHistoryItem);
            return;
          }
          router.push({ path: "/files" });
          return;
        }
        if (getters.isPreviewView()) {
          if (state.previousHistoryItem?.name) {
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
    showSaveBeforeExitPrompt(onConfirmAction) {
      mutations.showHover({
        name: "SaveBeforeExit",
        pinnedHover: true,
        confirm: async () => {
          // Save and exit - trigger the save action
          // If save fails, this will throw and be caught by SaveBeforeExit component
          await this.save();
          mutations.setEditorDirty(false);
          onConfirmAction();
        },
        discard: () => {
          // Discard changes and exit
          mutations.setEditorDirty(false);
          onConfirmAction();
        },
        cancel: () => {
          // Keep editing - do nothing
        },
      });
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

.search-bar-container {
  display: flex;
  align-items: center;
  background-color: rgba(100, 100, 100, 0.2);
  border-radius: 1em;
  padding: 0.5em 0.75em;
  transition: background-color 0.2s ease;
  gap: 0.5em;
  min-width: 35em;
  max-width: 300px;
  flex: 1;
  height: 3em;
  box-sizing: border-box;
}

@media (max-width: 768px) {
  .search-bar-container {
    min-width: unset;
    max-width: 60%;
  }
}

.search-bar-container:hover {
  background-color: rgba(100, 100, 100, 0.3);
}

.search-bar-container .material-icons {
  font-size: 1.25em;
  user-select: none;
}

#search-bar-input {
  background: transparent;
  border: none;
  outline: none;
  color: rgba(255, 255, 255, 0.9);
  width: 100%;
  font-size: 0.95em;
  user-select: none;
}

#search-bar-input::placeholder {
  color: gray;
}


.dark-mode-header .search-bar-container {
  background-color: rgba(100, 100, 100, 0.2);
}

.dark-mode-header .search-bar-container:hover {
  background-color: rgba(255, 255, 255, 0.15);
}
</style>