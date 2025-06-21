<template>
  <div
    id="context-menu"
    ref="contextMenu"
    v-if="showContext"
    :style="{
      top: `${top}px`,
      left: `${left}px`,
    }"
    class="button no-select"
    :class="{ 'dark-mode': isDarkMode, centered: centered }"
  >
    <div v-if="selectedCount > 0" class="button selected-count-header">
      <span>{{ selectedCount }} {{ $t("prompts.selected") }} </span>
    </div>

    <action
      v-if="!showCreate && !isSearchActive && userPerms.modify"
      icon="add"
      label="New"
      @action="startShowCreate"
    />

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
      v-if="!isMultiple && !isSearchActive"
      icon="check_circle"
      :label="$t('buttons.selectMultiple')"
      @action="toggleMultipleSelection"
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
      show="share"
    />
    <action
      v-if="!showCreate && selectedCount == 1 && userPerms.modify && !isSearchActive"
      icon="mode_edit"
      :label="$t('buttons.rename')"
      show="rename"
    />
    <action
      v-if="!showCreate && selectedCount > 0 && userPerms.modify"
      icon="content_copy"
      :label="$t('buttons.copyFile')"
      show="copy"
    />
    <action
      v-if="!showCreate && selectedCount > 0 && userPerms.modify"
      icon="forward"
      :label="$t('buttons.moveFile')"
      show="move"
    />
    <action
      v-if="!showCreate && selectedCount > 0 && userPerms.modify"
      icon="delete"
      :label="$t('buttons.delete')"
      show="delete"
    />
  </div>
  <div
    id="context-menu"
    ref="contextMenu"
    v-else-if="showOverflow"
    :style="{
      top: '3em',
      right: '1em',
    }"
    class="button no-select"
    :class="{ 'dark-mode': isDarkMode }"
  >
    <action v-if="showGoToRaw" icon="open_in_new" :label="$t('buttons.openFile')" @action="goToRaw()" />
    <action v-if="isPreview" icon="file_download" :label="$t('buttons.download')" @action="startDownload" />
    <action v-if="showEdit" icon="edit" :label="$t('buttons.edit')" @action="edit()" />
    <action v-if="showSave" icon="save" :label="$t('buttons.save')" @action="save()" />
    <action v-if="showDelete" icon="delete" :label="$t('buttons.delete')" show="delete" />
  </div>
</template>

<script>
import downloadFiles from "@/utils/download";
import { state, getters, mutations } from "@/store";
import Action from "@/components/Action.vue";
import { onlyOfficeUrl } from "@/utils/constants.js";
import buttons from "@/utils/buttons";
import { notify } from "@/notify";
import { eventBus } from "@/store/eventBus";
import { filesApi } from "@/api";
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
    };
  },
  computed: {
    noItems() {
      return !this.showEdit && !this.showSave && !this.showDelete;
    },
    showGoToRaw() {
      return getters.currentView() == "preview" || 
        getters.currentView() == "markdownViewer"
    },
    showEdit() {
      return getters.currentView() == "markdownViewer" && state.user.permissions.modify;
    },
    showDelete() {
      return state.user.permissions.modify && this.isPreview;
    },
    isPreview() {
      const cv = getters.currentView();
      return (
        cv == "preview" ||
        cv == "onlyOfficeEditor" ||
        cv == "markdownViewer" ||
        cv == "epubViewer" ||
        cv == "docViewer"
      );
    },
    showSave() {
      return getters.currentView() == "editor" && state.user.permissions.modify;
    },
    showOverflow() {
      return getters.currentPromptName() == "OverflowMenu";
    },
    showShare() {
      return (
        state.user?.permissions &&
        state.user?.permissions.share &&
        state.user.username != "publicUser" &&
        getters.currentView() != "share"
      );
    },
    showContext() {
      if (getters.currentPromptName() == "ContextMenu") {
        this.setPositions();
        return true;
      }
      return false;
    },
    onlyofficeEnabled() {
      return onlyOfficeUrl !== "";
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
      return getters.isMobile() || !this.posX || !this.posY;
    },
    top() {
      // Ensure the context menu stays within the viewport
      return Math.min(
        this.posY,
        window.innerHeight - (this.$refs.contextMenu?.clientHeight ?? 0)
      );
    },
    left() {
      return Math.min(
        this.posX,
        window.innerWidth - (this.$refs.contextMenu?.clientWidth ?? 0)
      );
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    selectedCount() {
      return getters.selectedCount();
    },
    userPerms() {
      return {
        upload: state.user.permissions?.modify && state.selected.length > 0,
        share: state.user.permissions.share,
        modify: state.user.permissions.modify,
      };
    },
  },
  methods: {
    startShowCreate() {
      this.showCreate = true;
    },
    uploadFunc() {
      mutations.showHover("upload");
    },
    showHover(value) {
      return mutations.showHover(value);
    },
    setPositions() {
      if (state.selected.length > 0) {
        this.showCreate = false;
      } else {
        this.showCreate = true;
      }
      const contextProps = getters.currentPrompt().props;
      let tempX = contextProps.posX;
      let tempY = contextProps.posY;
      // Assuming the screen width and height (adjust values based on your context)
      const screenWidth = window.innerWidth; // or any fixed width depending on your app's layout
      const screenHeight = window.innerHeight; // or any fixed height depending on your app's layout

      // if x is too close to the right edge, move it to the left by 400px
      if (tempX > screenWidth - 200) {
        tempX -= 200;
      }

      // if y is too close to the bottom edge, move it up by 400px
      if (tempY > screenHeight - 400) {
        tempY -= 200;
      }

      this.posX = tempX;
      this.posY = tempY;
    },
    toggleMultipleSelection() {
      mutations.setMultiple(!state.multiple);
      mutations.closeHovers();
    },
    startDownload() {
      downloadFiles();
    },
    goToRaw() {
      const downloadUrl = filesApi.getDownloadURL(
          state.req.source,
          state.req.path,
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
        notify.showError("Error saving file: ", e);
      }

      mutations.closeHovers();
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
</style>
