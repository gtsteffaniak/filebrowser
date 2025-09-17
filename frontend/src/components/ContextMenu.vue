<template>
  <transition
    name="expand"
    @before-enter="beforeEnter"
    @enter="enter"
    @leave="leave"
  >
    <div
      id="context-menu"
      ref="contextMenu"
      v-if="showContext"
      :style="centered ? {} : { top: posY + 'px', left: posX + 'px' }"
      class="button no-select fb-shadow"
      :class="{ 'dark-mode': isDarkMode, 'centered': centered }"
      :key="showCreate ? 'create-mode' : 'normal-mode'"
    >
      <div class="context-menu-header">
        <div
          class="action button clickable"
          v-if="!isSearchActive && userPerms.modify && !isShare"
          @click="toggleShowCreate"
        >
          <i v-if="!showCreate" class="material-icons">add</i>
          <i v-if="showCreate" class="material-icons">arrow_back</i>
        </div>
        <div
          v-if="selectedCount > 0"
          @mouseleave="hideTooltip"
          @mouseenter="showTooltip($event, $t('buttons.selectedCount'))"
          class="button selected-count-header"
        >
          <span>{{ selectedCount }}</span>
        </div>
      </div>
      <hr class="divider">
      <action
        v-if="showCreate && !isSearchActive && userPerms.modify"
        icon="create_new_folder"
        :label="$t('sidebar.newFolder')"
        @action="showHover('newDir')"
      />
      <action
        v-if="showCreate && userPerms.modify && !isSearchActive"
        icon="note_add"
        :label="$t('sidebar.newFile')"
        @action="showHover('newFile')"
      />
      <action
        v-if="showCreate && userPerms.modify && !isSearchActive"
        icon="file_upload"
        :label="$t('buttons.upload')"
        @action="uploadFunc"
      />
      <action
        v-if="!showCreate && selectedCount == 1"
        icon="info"
        :label="$t('buttons.info')"
        show="info"
      />

      <action
        v-if="(!showCreate && selectedCount > 0)"
        icon="file_download"
        :label="$t('buttons.download')"
        @action="startDownload"
        :counter="selectedCount"
      />
      <action
        v-if="selectedCount <= 1 && showShare"
        icon="share"
        :label="$t('buttons.share')"
        @action="showShareHover"
      />
      <action
        v-if="!showCreate && selectedCount == 1 && userPerms.modify && !isSearchActive"
        icon="mode_edit"
        :label="$t('buttons.rename')"
        @action="showRenameHover"
      />
      <action
        v-if="!showCreate && selectedCount > 0 && userPerms.modify"
        icon="content_copy"
        :label="$t('buttons.copyFile')"
        show="copy"
      />
      <action
        v-if="!showCreate && selectedCount == 1 && isSearchActive"
        icon="folder"
        :label="$t('buttons.openParentFolder')"
        @action="openParentFolder"
      />
      <action
        v-if="!showCreate && selectedCount > 0 && userPerms.modify"
        icon="forward"
        :label="$t('buttons.moveFile')"
        show="move"
      />
      <action
        v-if="showDelete"
        icon="delete"
        :label="$t('buttons.delete')"
        show="delete"
      />
      <action
        v-if="showAccess"
        icon="lock"
        :label="$t('access.rules')"
        @action="showAccessHover"
      />
      <action
        v-if="showSelectMultiple"
        icon="check_circle"
        :label="$t('buttons.selectMultiple')"
        @action="toggleMultipleSelection"
      />
    </div>
  </transition>
  <transition
    name="expand"
    @before-enter="beforeEnter"
    @enter="enter"
    @leave="leave"
  >
    <div
      id="context-menu"
      ref="contextMenu"
      v-if="showOverflow"
      :style="{
        top: '3em',
        right: '1em',
      }"
      class="button no-select fb-shadow"
      :class="{ 'dark-mode': isDarkMode }"
    >
      <action v-if="showGoToRaw" icon="open_in_new" :label="$t('buttons.openFile')" @action="goToRaw()" />
      <action v-if="shouldShowParentFolder()" icon="folder" :label="$t('buttons.openParentFolder')" @action="openParentFolder" />
      <action v-if="hasDownload" icon="file_download" :label="$t('buttons.download')" @action="startDownload" />
      <action v-if="showEdit" icon="edit" :label="$t('buttons.edit')" @action="edit()" />
      <action v-if="showSave" icon="save" :label="$t('buttons.save')" @action="save()" />
      <action v-if="showDelete" icon="delete" :label="$t('buttons.delete')" show="delete" />
    </div>
  </transition>
</template>

<script>
import downloadFiles from "@/utils/download";
import { state, getters, mutations } from "@/store";
import Action from "@/components/Action.vue";
import { globalVars } from "@/utils/constants.js";
import buttons from "@/utils/buttons";
import { notify } from "@/notify";
import { eventBus } from "@/store/eventBus";
import { filesApi, publicApi } from "@/api";
import { url } from "@/utils";
export default {
  name: "ContextMenu",
  components: {
    Action,
  },
  data() {
    return {
      posX: 0,
      posY: 0,
      showCreate: false,
      isAnimating: false,
      createStateInitialized: false,
    };
  },
  props: {
    showCentered: {
      type: Boolean,
      default: false,
    },
  },
  computed: {
    isShare() {
      return getters.isShare();
    },
    showSelectMultiple() {
      if (this.isMultiple || this.isSearchActive) {
        return false;
      }
      if (state.user?.showSelectMultiple) {
        return true;
      }
      if (getters.isMobile()) {
        return true;
      }
      return false
    },
    hasOverflowItems() {
      return this.showEdit || this.showDelete || this.showSave || this.showGoToRaw || this.hasDownload;
    },
    showGoToRaw() {
      const cv = getters.currentView();
      return cv == "preview" || cv == "markdownViewer" || cv == "editor";
    },
    showEdit() {
      const cv = getters.currentView();
      if (getters.isShare()) {
        // TODO: add support for editing shared files
        return false;
      }
      return cv == "markdownViewer" && state.user?.permissions?.modify;
    },
    showDelete() {
      const cv = getters.currentView();
      if (getters.isShare() || !state.user?.permissions?.modify) {
        // TODO: add support for deleting shared files
        return false;
      }
      const showDelete = cv != "settings" && !this.showCreate && this.selectedCount > 0 && !this.isSearchActive;
      return showDelete;
    },
    hasDownload() {
      return this.selectedCount > 0;
    },
    isPreview() {
      const cv = getters.currentView();
      return (
        cv == "preview" ||
        cv == "onlyOfficeEditor" ||
        cv == "markdownViewer" ||
        cv == "epubViewer" ||
        cv == "docViewer" ||
        cv == "editor"
      );
    },
    showSave() {
      if (getters.isShare()) {
        // TODO: add support for saving shared files
        return false;
      }
      return getters.currentView() == "editor" && state.user?.permissions?.modify;
    },
    showOverflow() {
      return getters.currentPromptName() == "OverflowMenu";
    },
    showAccess() {
      return state.user?.permissions?.admin && this.showCreate;
    },
    showShare() {
      if (getters.isShare()) {
        return false;
      }
      return state.user?.permissions?.share;
    },
    showContext() {
      return getters.currentPromptName() == "ContextMenu";
    },
    onlyofficeEnabled() {
      return globalVars.onlyOfficeUrl !== "";
    },
    isSearchActive() {
      return state.isSearchActive;
    },
    isMultiple() {
      return state.multiple;
    },
    user() {
      return state.user;
    },
    centered() {
      return this.showCentered || this.isMobileDevice || !this.posX || !this.posY;
    },
    isMobileDevice() {
      return state.isMobile;
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    selectedCount() {
      return getters.selectedCount();
    },
    userPerms() {
      return {
        upload: state.user?.permissions?.modify && state.selected.length > 0,
        share: state.user?.permissions?.share,
        modify: state.user?.permissions?.modify,
      };
    },
    currentPrompt() {
      return getters.currentPrompt();
    },
  },
  watch: {
    hasOverflowItems: {
      handler(hasItems) {
        mutations.setContextMenuHasItems(hasItems);
      },
      immediate: true,
    },
    showContext: {
      handler(newVal) {
        if (newVal) {
          // Always set positions when not animating to check for position props.
          if (!this.isAnimating) {
            this.setPositions();
          }
          // Initialize create state only once per menu session
          if (!this.createStateInitialized) {
            this.initializeCreateState();
            this.createStateInitialized = true;
          }
        } else {
          // Reset the flag when menu is hidden so it reinitializes next time
          this.createStateInitialized = false;
        }
      },
      immediate: true
    }
  },
  methods: {
    hideTooltip() {
      mutations.hideTooltip();
    },
    showTooltip(event, text) {
      mutations.showTooltip({
        content: text,
        x: event.clientX,
        y: event.clientY,
      });
    },
    toggleShowCreate() {
      this.showCreate = !this.showCreate;
    },
    shouldShowParentFolder() {
      return this.isPreview && state.req.path != "/";
    },
    showAccessHover() {
      mutations.showHover({
        name: "access",
        props: {
          sourceName: state.sources.current,
          path: state.req?.path || "",
        },
      });
    },
    // Animation methods
    beforeEnter(el) {
      this.isAnimating = true;
      el.style.height = '0';
      el.style.opacity = '0';
    },
    enter(el, done) {
      el.style.transition = '';
      el.style.height = '0';
      el.style.opacity = '0';
      // Force reflow
      void el.offsetHeight;
      // Calculate the height after ensuring all content is rendered
      this.$nextTick(() => {
        // Temporarily set to auto to get true height, then measure
        el.style.height = 'auto';
        el.style.visibility = 'hidden';
        void el.offsetHeight; // Force reflow
        const fullHeight = el.scrollHeight;
        const fullWidth = el.scrollWidth;

        // Adjust position now that we have dimensions
        const BUFFER = 8;
        const screenWidth = window.innerWidth;
        const screenHeight = window.innerHeight;
        let newX = this.posX;
        let newY = this.posY;

        if (newX + fullWidth + BUFFER > screenWidth) newX = screenWidth - fullWidth - BUFFER;
        if (newX < BUFFER) newX = BUFFER;
        if (newY + fullHeight + BUFFER > screenHeight) newY = screenHeight - fullHeight - BUFFER;
        if (newY < BUFFER) newY = BUFFER;

        this.posX = newX;
        this.posY = newY;

        // Reset to 0 for animation
        el.style.height = '0';
        el.style.visibility = 'visible';
        el.style.transition = 'height 0.3s, opacity 0.3s';
        void el.offsetHeight; // Force reflow
        // Animate to full height
        el.style.height = fullHeight + 'px';
        el.style.opacity = '1';
        setTimeout(() => {
          this.isAnimating = false;
          done();
        }, 300);
      });
    },
    leave(el, done) {
      this.isAnimating = true;
      el.style.transition = 'height 0.3s, opacity 0.3s';
      el.style.height = el.scrollHeight + 'px';
      void el.offsetHeight;
      el.style.height = '0';
      el.style.opacity = '0';
      setTimeout(() => {
        this.isAnimating = false;
        done();
      }, 300);
    },
    startShowCreate() {
      if (getters.isShare()) {
        return;
      }
      this.showCreate = true;
    },
    uploadFunc() {
      mutations.showHover("upload");
    },
    showHover(value) {
      return mutations.showHover(value);
    },
    showShareHover() {
      mutations.showHover({
        name: "share",
        props: {
          item: getters.selectedCount() == 1 ? getters.getFirstSelected() : state.req
        },
      });
    },
    showRenameHover() {
      mutations.showHover({
        name: "rename",
        props: {
          item: getters.selectedCount() == 1 ? getters.getFirstSelected() : state.req
        },
      });
    },
    setPositions() {
      const contextProps = getters.currentPrompt().props;
      this.posX = contextProps.posX;
      this.posY = contextProps.posY;
    },
    initializeCreateState() {
      // Only set initial showCreate state, don't override user choices
      if (state.selected.length > 0 || getters.isShare()) {
        this.showCreate = false;
      } else {
        this.showCreate = true;
      }
    },
    toggleMultipleSelection() {
      mutations.setMultiple(!state.multiple);
      mutations.closeHovers();
    },
    startDownload() {
      mutations.closeHovers();
      const items = state.selected.length > 0 ? state.selected : [state.req];
      downloadFiles(items);
    },
    goToRaw() {
      if (getters.isShare()) {
        window.open(publicApi.getDownloadURL(state.share, state.req.path, true), "_blank");
        mutations.closeHovers();
        return;
      }
      const downloadUrl = filesApi.getDownloadURL(
        state.req?.source || "",
        state.req?.path || "",
        true,
        false
      );
      window.open(downloadUrl, "_blank");
      mutations.closeHovers();
    },
    async edit() {
      window.location.hash = "#edit";
    },
    async save() {
      const button = "save";
      buttons.loading("save");
      try {
        eventBus.emit("handleEditorValueRequest", "data");
        buttons.success(button);
        notify.showSuccess("File Saved!");
      } catch (e) {
        buttons.done(button);
        notify.showError(`Error saving file: ${e}`);
      }

      mutations.closeHovers();
    },
    showUpload() {
      mutations.showHover({
        name: "upload",
        props: {
          filesToReplace: state.selected.map((item) => item.name || ""),
        },
      });
    },
    openParentFolder() {
      const item = state.selected.length > 0 ? state.selected[0] : state.req;
      let parentPath = url.removeLastDir(item.path);
      if (parentPath == "") {
        parentPath = "/";
      }
      url.goToItem(state.req.source, parentPath);
    },
  },
};
</script>

<style scoped>
#context-menu {
  position: absolute;
  z-index: 1000;
  background-color: var(--background);
  max-width: 20em;
  min-width: 15em;
  min-height: 4em;
  height: auto;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

#context-menu.centered {
  top: 50% !important;
  left: 50% !important;
  -webkit-transform: translate(-50%, -50%);
  transform: translate(-50%, -50%);
}

.selected-count-header {
  border-radius: 0.5em;
  cursor: unset;
  margin-bottom: 0.5em;
}

#context-menu .action {
  width: auto;
  display: flex;
  align-items: center;
}

#context-menu > span {
  display: inline-block;
  margin-left: 1em;
  color: var(--textPrimary);
  margin-right: auto;
}

#context-menu .action span {
  display: none;
}

/* Animation styles */
.expand-enter-active,
.expand-leave-active {
  transition: height 0.3s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}

.expand-enter,
.expand-leave-to {
  height: 0 !important;
  opacity: 0;
}

.context-menu-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
</style>
