<template>
  <header
    v-if="!isOnlyOffice"
    class="fixed top-0 left-0 z-5 flex h-(--header-height) w-full items-center justify-between gap-1 border-b border-divider bg-surface/85 px-2 text-foreground backdrop-blur-md"
  >
    <action
      v-if="!disableNavButtons"
      icon="close_back"
      :label="$t('general.close')"
      :disabled="isDisabledMultiAction"
      @action="multiAction"
    />
    <div
      v-if="showSearch && !isSearchActive"
      class="search-bar-container flex h-9 min-w-0 max-w-xl flex-1 cursor-pointer items-center gap-2 rounded-full bg-surface-2/70 px-3 transition-colors hover:bg-surface-2 max-md:max-w-[60%]"
      :class="{ 'cursor-not-allowed opacity-50': isDisabled }"
      @click="openSearch"
    >
      <i class="material-symbols select-none text-lg leading-none text-muted">search</i>
      <input
        type="text"
        id="search-bar-input"
        class="w-full cursor-pointer border-none bg-transparent text-sm text-foreground outline-none placeholder:text-muted"
        :placeholder="$t('general.search', { suffix: '...' })"
        readonly
      />
    </div>
    <title v-if="!showSearch" class="topTitle block min-w-0 flex-1 truncate px-4 text-center text-lg font-medium">{{ getTopTitle }}</title>
    <action
      v-if="showHeaderSwitchView && !disableNavButtons"
      class="menu-button"
      :icon="viewIcon"
      :label="$t('buttons.switchView')"
      @action="switchView"
      :disabled="isDisabled"
    />
    <action
      class="overflow-menu-button"
      v-else-if="!showHeaderSwitchView && !showQuickSave"
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
      if (getters.currentView() !== "editor" || !state.user.permissions.modify) {
        return false;
      }
      return state.user.editorQuickSave;
    },
    disableNavButtons() {
      const isShare = getters.isShare();
      const regularDisabled = globalVars.disableNavButtons && this.isListingView;
      const shareDisabled = isShare && state.shareInfo?.hideNavButtons && getters.currentView() === "listingView";
      const uploadShare = isShare && state.shareInfo?.shareType === "upload"
      return regularDisabled || shareDisabled || uploadShare;
    },
    isOnlyOffice() {
      return getters.currentView() === "onlyOfficeEditor";
    },
    isListingView() {
      return getters.currentView() === "listingView";
    },
    isAdvancedSearchRoute() {
      return (state.route?.path || "") === "/tools/advancedSearch";
    },
    showHeaderSwitchView() {
      return this.isListingView || this.isAdvancedSearchRoute;
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
      return !state.contextMenuHasItems && !getters.isPreviewView();
    },
    showEdit() {
      return window.location.hash !== "#edit" && state.user.permissions.modify;
    },
    showDelete() {
      return state.user.permissions.modify && getters.currentView() === "preview";
    },
    showSave() {
      return getters.currentView() === "editor" && state.user.permissions.modify;
    },
    showSearch() {
      return getters.isLoggedIn() && getters.currentView() === "listingView" && !getters.isShare();
    },
    isSearchActive() {
      return state.isSearchActive;
    },
    isDisabled() {
      return state.isSearchActive || getters.currentPromptName() !== "";
    },
    isDisabledMultiAction() {
      const regularDisabled = getters.isStickySidebar() && getters.multibuttonState() === "menu";
      const shareDisabled = state.shareInfo?.disableSidebar && getters.multibuttonState() === "menu";
      return this.isDisabled || regularDisabled || shareDisabled;
    },
    showSwitchView() {
      return this.showHeaderSwitchView;
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
      if (!state.isSearchActive && !this.isDisabled) {
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
        mutations.closeHovers();
      } else {
        mutations.showPrompt({ name: "OverflowMenu" });
      }
    },
    /** Match StatusBar.adjustViewMode: list vs compact, icons vs gallery from gallery size. */
    resolveViewModeForFamily(baseMode) {
      const size = state.user?.gallerySize ?? 5;
      if (baseMode === "list") {
        return size <= 3 ? "compact" : "list";
      }
      if (baseMode === "icons") {
        return size <= 4 ? "icons" : "gallery";
      }
      return baseMode;
    },
    /** Map concrete viewMode to one of the three switch-view families (list|compact → list, etc.). */
    viewModeCycleIndex(mode) {
      if (mode === "list" || mode === "compact") {
        return 0;
      }
      if (mode === "normal") {
        return 1;
      }
      if (mode === "gallery" || mode === "icons") {
        return 2;
      }
      return 1;
    },
    switchView() {
      mutations.closeHovers();
      const current = getters.viewMode();
      const cycleIndex = this.viewModeCycleIndex(current);
      const nextIndex = (cycleIndex + 1) % this.viewModes.length;
      const baseMode = this.viewModes.at(nextIndex);
      const newViewMode = this.resolveViewModeForFamily(baseMode);
      mutations.updateDisplayPreferences({ viewMode: newViewMode });
      void mutations.updateCurrentUser({ viewMode: newViewMode });
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
      if (cv === "listingView" || ( getters.isShare() && getters.multibuttonState() !== "close") || cv === "tools") {
        mutations.toggleSidebar();
      } else if (cv === "settings" && state.isMobile) {
        mutations.toggleSidebar();
      } else {
        mutations.closeHovers();
        if (cv === "settings") {
          if (state.previousHistoryItem?.name) {
            url.goToItem(
              state.previousHistoryItem.source,
              state.previousHistoryItem.path,
              state.previousHistoryItem,
              false,
              state.previousHistoryItem.isShare
            );
            return;
          }
          if (state.shareInfo?.hash && state.req?.source === state.shareInfo.hash) {
            url.goToItem(state.shareInfo.hash, state.req.path, {}, false, true);
            return;
          }
          // otherwise navigate to files
          void router.push({ path: "/files" });
          return;
        }
        if (getters.isPreviewView()) {
          if (state.previousHistoryItem?.name) {
            url.goToItem(state.previousHistoryItem.source, state.previousHistoryItem.path, state.previousHistoryItem, false, state.previousHistoryItem.isShare);
            return;
          } else {
            // navigate to parent directory of current url
            const parentPath = url.removeLastDir(state.route.path);
            void router.push({ path: parentPath });
          }
          return;
        }

        router.go(-1);
      }
    },
    showSaveBeforeExitPrompt(onConfirmAction) {
      mutations.showPrompt({
        name: "SaveBeforeExit",
        pinned: true,
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
/* Action label text is hidden in the header; icons only. */
header .action span {
  display: none;
}

/* Square hover targets instead of the legacy circle + global inset-shadow hack. */
header .action,
header .action i {
  border-radius: 0.5rem;
}

header .action:not(.disabled):not(:disabled):hover {
  background-color: var(--surface-hover);
  box-shadow: none !important;
  -webkit-box-shadow: none !important;
}
</style>