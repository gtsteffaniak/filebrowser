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
      class="button no-select floating-window"
      :class="{ 'dark-mode': isDarkMode, 'centered': centered }"
      :key="showCreate ? 'create-mode' : 'normal-mode'"
    >
      <div class="context-menu-header">
        <div
          class="action button clickable"
          v-if="showCreateButton"
          @click="toggleShowCreate"
        >
          <i v-if="!showCreate" class="material-icons">add</i>
          <i v-else class="material-icons">arrow_back</i>
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
      <hr v-if="showDivider" class="divider">
      <action
        v-if="showCreateActions"
        icon="create_new_folder"
        :label="$t('files.newFolder')"
        @action="showNewDirHover"
      />
      <action
        v-if="showCreateActions"
        icon="note_add"
        :label="$t('files.newFile')"
        @action="showHover('newFile')"
      />
      <action
        v-if="showCreateActions"
        icon="file_upload"
        :label="$t('general.upload')"
        @action="showUploadHover"
      />
      <action
        v-if="showInfo"
        icon="info"
        :label="$t('general.info')"
        @action="showInfoHover"
      />

      <action
        v-if="showDownload"
        icon="file_download"
        :label="$t('general.download')"
        @action="startDownload"
      />
      <action
        v-if="showArchive"
        icon="archive"
        :label="$t('prompts.archive')"
        @action="showArchiveHover"
      />
      <action
        v-if="showUnarchive"
        icon="unarchive"
        :label="$t('prompts.unarchive')"
        @action="showUnarchiveHover"
      />
      <action
        v-if="showShareAction"
        icon="share"
        :label="$t('general.share')"
        @action="showShareHover"
      />
      <action
        v-if="showRename"
        icon="mode_edit"
        :label="$t('general.rename')"
        @action="showRenameHover"
      />
      <action
        v-if="showCopy"
        icon="content_copy"
        :label="$t('buttons.copyFile')"
        @action="showCopyHover"
      />
      <action
        v-if="showOpenParentFolder"
        icon="folder"
        :label="$t('buttons.openParentFolder')"
        @action="openParentFolder"
      />
      <action
        v-if="showGoToItem"
        icon="folder"
        :label="$t('buttons.goToItem')"
        @action="goToItem"
      />
      <action
        v-if="showMove"
        icon="forward"
        :label="$t('buttons.moveFile')"
        @action="showMoveHover"
      />
      <action
        v-if="showSelectAll"
        icon="select_all"
        :label="$t('buttons.selectAll')"
        @action="selectAllItems"
      />
      <action
        v-if="showDelete"
        icon="delete"
        :label="$t('general.delete')"
        @action="showDeleteHover"
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
      class="button no-select floating-window"
      :class="{ 'dark-mode': isDarkMode }"
    >
      <action icon="info" :label="$t('general.info')" @action="showInfoHover"/>
      <action v-if="showGoToRaw" icon="open_in_new" :label="$t('general.openFile')" @action="goToRaw()" />
      <action v-if="shouldShowParentFolder()" icon="folder" :label="$t('buttons.openParentFolder')" @action="openParentFolder" />
      <action v-if="isPreview && permissions.modify" icon="mode_edit" :label="$t('general.rename')" @action="showRenameHoverForPreview" />
      <action v-if="hasDownload && !req.isDir" icon="visibility" :label="$t('buttons.watchFile')" @action="watchFile()" />
      <action v-if="hasDownload" icon="file_download" :label="$t('general.download')" @action="startDownload" />
      <action v-if="showUnarchiveInOverflow" icon="folder_open" :label="$t('prompts.unarchive')" @action="showUnarchiveHoverFromPreview" />
      <action v-if="showEdit" icon="edit" :label="$t('general.edit')" @action="edit()" />
      <action v-if="showSave" icon="save" :label="$t('general.save')" @action="save()" />
      <action v-if="showDelete" icon="delete" :label="$t('general.delete')" show="delete" />
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
import { filesApi, publicApi } from "@/api";
import { url } from "@/utils";

function isArchivePath(pathOrName) {
  if (!pathOrName || typeof pathOrName !== "string") return false;
  const lower = pathOrName.toLowerCase();
  return lower.endsWith(".zip") || lower.endsWith(".tar.gz") || lower.endsWith(".tgz");
}

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
    createOnly: {
      type: Boolean,
      default: false,
    },
    showCentered: {
      type: Boolean,
      default: false,
    },
    items: {
      type: Array,
      default: null, // Array of item objects { name, path, source, isDir, type, ... }
    },
  },
  computed: {
    // Either from prop or from state
    providedItems() {
      if (this.items) return this.items;
      // Fallback to global selection (indices or objects)
      if (state.selected.length === 0) return [];
      // Map to actual items from state.req
      if (typeof state.selected[0] === 'number') {
        return state.selected.map(index => state.req.items[index]);
      }
      return state.selected;
    },
    selectedCount() {
      return this.providedItems.length;
    },
    firstSelected() {
      return this.providedItems[0] || null;
    },
    showGoToItem() {
      return this.isDuplicateFinder && this.selectedCount == 1;
    },
    isDuplicateFinder() {
      return getters.currentView() === "duplicateFinder";
    },
    permissions() {
      return getters.permissions();
    },
    req() {
      return state.req;
    },
    isShare() {
      return getters.isShare();
    },
    showCreateActions() {
      if (this.isDuplicateFinder) return false;
      return this.showCreate && !this.isSearchActive;
    },
    showInfo() {
      if (this.isDuplicateFinder) return this.selectedCount == 1;
      return !this.showCreate && this.selectedCount == 1;
    },
    showDownload() {
      if (this.isDuplicateFinder) return false;
      return !this.showCreate && this.permissions.download && this.selectedCount > 0;
    },
    showArchive() {
      if (this.isDuplicateFinder || getters.isShare()) return false;
      if (!this.permissions.create) return false;
      return !this.showCreate && this.selectedCount > 0 && !this.showUnarchive;
    },
    showUnarchive() {
      if (this.isDuplicateFinder || getters.isShare()) return false;
      if (!this.permissions.create) return false;
      if (this.selectedCount !== 1) return false;
      const item = this.firstSelected;
      return item && isArchivePath(item.path || item.from || item.name);
    },
    showShareAction() {
      if (this.isDuplicateFinder) return false;
      return (this.showCreate || this.selectedCount == 1) && this.showShare;
    },
    showRename() {
      if (this.isDuplicateFinder) return false;
      return !this.showCreate && this.selectedCount == 1 && this.permissions.modify && !this.isSearchActive;
    },
    showCopy() {
      if (this.isDuplicateFinder) return false;
      return !this.showCreate && this.selectedCount > 0 && this.permissions.modify;
    },
    showOpenParentFolder() {
      return !this.showCreate && this.selectedCount == 1 && (this.isSearchActive || this.isDuplicateFinder);
    },
    showMove() {
      if (this.isDuplicateFinder) return false;
      return !this.showCreate && this.selectedCount > 0 && this.permissions.modify;
    },
    showSelectAll() {
      if (this.isDuplicateFinder) return false;
      return !this.showCreate && !this.isSearchActive && this.req?.items?.length > 0;
    },
    showCreateButton() {
      if (this.isDuplicateFinder || this.createOnly) return false;
      return !this.isSearchActive && this.permissions.create && !this.isShare;
    },
    showDivider() {
      if (this.isDuplicateFinder || this.createOnly) return false;
      if (getters.isShare()) {
        return state.shareInfo?.allowCreate
      }
      return state.user?.permissions?.create || state.user?.permissions?.share || state.user?.permissions?.admin;
    },
    showSelectMultiple() {
      if (this.isDuplicateFinder) return false;
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
      return this.showEdit || this.showDelete || this.showSave || this.showGoToRaw || this.hasDownload || this.showUnarchiveInOverflow;
    },
    showUnarchiveInOverflow() {
      if (!this.permissions.archive || getters.isShare()) return false;
      const req = state.req;
      return req && !req.isDir && isArchivePath(req.path || req.name);
    },
    showGoToRaw() {
      if (!this.permissions.download) {
        return false;
      }
      const cv = getters.currentView();
      return cv == "preview" || cv == "markdownViewer" || cv == "editor";
    },
    showEdit() {
      const cv = getters.currentView();
      return cv == "markdownViewer" && this.permissions.modify;
    },
    showDelete() {
      if (this.isDuplicateFinder) return false;
      if (this.selectedCount == 0) {
        return false;
      }
      const cv = getters.currentView();
      const showDelete = cv != "settings" && !this.isSearchActive && this.permissions.delete;
      return showDelete;
    },
    hasDownload() {
      return this.selectedCount > 0 && this.permissions.download;
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
      const allowEdit = this.permissions.modify || (getters.isShare() && state.shareInfo.allowEdit);
      return getters.currentView() == "editor" && allowEdit;
    },
    showOverflow() {
      return getters.currentPromptName() == "OverflowMenu";
    },
    showAccess() {
      if (this.isDuplicateFinder) return false;
      if (getters.isShare()) {
        return false;
      }
      return this.permissions.admin && this.showCreate;
    },
    showShare() {
      if (getters.isShare()) {
        return false;
      }
      return this.permissions.share;
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
    showInfoHover() {
      mutations.closeContextMenus();
      mutations.showHover({
        name: "info",
        props: {
          item: this.firstSelected,
        },
      });
    },
    goToItem() {
      const item = this.firstSelected;
      url.goToItem(item.source, item.path, {}, true);
    },
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
      if (!this.permissions.create) {
        this.showCreate = false;
        return;
      }
      this.showCreate = !this.showCreate;
    },
    shouldShowParentFolder() {
      return this.isPreview && state.req.path != "/";
    },
    showAccessHover() {
      mutations.closeContextMenus();
      let sourceName = this.firstSelected?.source || state.req.source;
      let path = this.firstSelected?.path || state.req.path;
      if (this.firstSelected && !this.firstSelected.isDir) {
        path = url.removeLastDir(path) + '/';
      }
      mutations.showHover({
        name: "access",
        props: {
          sourceName: sourceName,
          path: path,
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
      if (!this.permissions.create) {
        return;
      }
      this.showCreate = true;
    },
    showHover(value) {
      return mutations.showHover(value);
    },
    showShareHover() {
      mutations.closeContextMenus();
      mutations.showHover({
        name: "share",
        props: {
          item: this.selectedCount == 1 ? this.firstSelected : state.req
        },
      });
    },
    showRenameHover() {
      mutations.closeContextMenus();
      mutations.showHover({
        name: "rename",
        props: {
          item: this.selectedCount == 1 ? this.firstSelected : state.req,
          parentItems: []
        },
      });
    },
    showRenameHoverForPreview() {
      mutations.closeTopHover(); // Close the ContextMenu (if it was open from preview)
      // Get parent items from the listing
      const parentItems = state.navigation.listing || [];
      mutations.showHover({
        name: "rename",
        props: {
          item: state.req,
          parentItems: parentItems,
        },
      });
    },
    setPositions() {
      const contextProps = getters.currentPrompt().props;
      this.posX = contextProps.posX;
      this.posY = contextProps.posY;
    },
    initializeCreateState() {
      // If createOnly is set, always show create actions
      if (this.createOnly) {
        this.showCreate = true;
        return;
      }
      // Only set initial showCreate state, don't override user choices
      if (this.selectedCount > 0 || !this.permissions.create) {
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
      mutations.closeTopHover();
      const items = this.providedItems;
      downloadFiles(items);
    },
    showDeleteHover() {
      mutations.closeContextMenus();
      mutations.showHover({
        name: 'delete',
        props: {
          items: this.providedItems,
        },
      });
    },
    showMoveHover() {
      mutations.closeContextMenus();
      mutations.showHover({
        name: 'move',
        props: {
          items: this.providedItems,
          operation: 'move',
        },
      });
    },
    showCopyHover() {
      mutations.closeContextMenus();
      mutations.showHover({
        name: 'copy',
        props: {
          items: this.providedItems,
          operation: 'copy',
        },
      });
    },
    showNewDirHover() {
      mutations.closeContextMenus();
      // If the context menu was triggered on a directory, pass its path as base
      const selectedItem = this.firstSelected;
      let base = null;
      if (selectedItem && selectedItem.isDir) {
        // Pass both path and source
        base = {
          path: selectedItem.path,
          source: selectedItem.source,
        };
      }
      mutations.showHover({
        name: "newDir",
        props: {
          base: base,
        },
      });
    },
    showArchiveHover() {
      mutations.closeTopHover();
      const items = this.providedItems.map(item => ({
        path: item.path,
        name: item.name,
        source: item.source || state.req.source,
      }));
      if (items.length === 0) return;
      mutations.showHover({
        name: "archive",
        props: {
          items,
          source: state.req.source,
          currentPath: state.req.path || "/",
        },
      });
    },
    showUnarchiveHover() {
      mutations.closeTopHover();
      const item = this.firstSelected;
      if (!item) return;
      this.openUnarchivePrompt(item);
    },
    showUnarchiveHoverFromPreview() {
      mutations.closeTopHover();
      const req = state.req;
      if (!req) return;
      this.openUnarchivePrompt({ path: req.path, source: req.source, name: req.name });
    },
    openUnarchivePrompt(item) {
      const path = item.path || item.from;
      const source = item.source || state.req.source;
      mutations.showHover({
        name: "unarchive",
        props: {
          item: { path, source, name: item.name },
        },
      });
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
      mutations.closeContextMenus();
    },
    watchFile() {
      mutations.closeContextMenus();
      const source = state.req?.source || state.sources.current || "";
      const path = state.req?.path || "/";
      this.$router.push({
        path: "/tools/fileWatcher",
        query: {
          path: path,
          source: source,
        },
      });
    },
    async edit() {
      window.location.hash = "#edit";
    },
    async save() {
      const button = "save";
      buttons.loading("save");
      try {
        // Call the editor save handler directly and await completion
        if (state.editorSaveHandler) {
          await state.editorSaveHandler();
        } else {
          throw new Error("Editor save handler not found");
        }
        buttons.success(button);
        notify.showSuccessToast(this.$t("editor.fileSaved"));
      } catch (e) {
        // Don't show error notification here - API layer already showed it
        buttons.done(button);
      }
      mutations.closeContextMenus();
    },
    showUploadHover() {
      mutations.closeContextMenus();
      let targetPath = state.req.path;
      let targetSource = state.req.source;
      const selectedItem = this.firstSelected;
      if (selectedItem && selectedItem.isDir) {
        targetPath = selectedItem.path;
        targetSource = selectedItem.source;
      }
      mutations.showHover({
        name: "upload",
        props: {
          targetPath: targetPath,
          targetSource: targetSource,
        },
      });
    },
    openParentFolder() {
      const item = this.firstSelected;
      const parentPath = url.removeLastDir(item.path) || "/";
      url.goToItem(item.source, parentPath, {}, this.isDuplicateFinder);
    },
    selectAllItems() {
      if (state.req && state.req.items && state.req.items.length > 0) {
        // Clear current selection first
        mutations.resetSelected();
        // Add all items from current directory to selection by their indices
        state.req.items.forEach((item, index) => {
          mutations.addSelected(index);
        });
        // Close the context menu
        mutations.closeContextMenus();
      }
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
  border-radius: 1em;
  cursor: unset;
}

.context-menu-header > .action i {
  padding: 0.25em;
}

#context-menu .action {
  display: flex;
  align-items: center;
  justify-content: flex-start;
}

#context-menu > div,
#context-menu > button {
  width: 100%;
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
