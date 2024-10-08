<template>
  <div
    id="context-menu"
    ref="contextMenu"
    v-show="showContext"
    :style="{
      top: `${top}px`,
      left: `${left}px`,
    }"
    class="button"
    :class="{ 'dark-mode': isDarkMode, mobile: isMobile }"
  >
    <div v-if="selectedCount > 0" class="button selected-count-header">
      <span>{{ selectedCount }} selected</span>
    </div>

    <action
      v-if="!headerButtons.select"
      icon="create_new_folder"
      :label="$t('sidebar.newFolder')"
      @action="showHover('newDir')"
    />
    <action
      v-if="!headerButtons.select"
      icon="note_add"
      :label="$t('sidebar.newFile')"
      @action="showHover('newFile')"
    />
    <action
      v-if="!headerButtons.select"
      icon="file_upload"
      :label="$t('buttons.upload')"
      @action="uploadFunc"
    />

    <action
      v-if="headerButtons.select"
      icon="info"
      :label="$t('buttons.info')"
      show="info"
    />
    <action
      v-if="!isMultiple"
      icon="check_circle"
      :label="$t('buttons.selectMultiple')"
      @action="toggleMultipleSelection"
    />
    <action
      v-if="headerButtons.download"
      icon="file_download"
      :label="$t('buttons.download')"
      @action="startDownload"
      :counter="selectedCount"
    />
    <action
      v-if="headerButtons.share"
      icon="share"
      :label="$t('buttons.share')"
      show="share"
    />
    <action
      v-if="headerButtons.rename"
      icon="mode_edit"
      :label="$t('buttons.rename')"
      show="rename"
    />
    <action
      v-if="headerButtons.copy"
      icon="content_copy"
      :label="$t('buttons.copyFile')"
      show="copy"
    />
    <action
      v-if="headerButtons.move"
      icon="forward"
      :label="$t('buttons.moveFile')"
      show="move"
    />
    <action
      v-if="headerButtons.delete"
      icon="delete"
      :label="$t('buttons.delete')"
      show="delete"
    />
  </div>
</template>

<script>
import downloadFiles from "@/utils/download";
import { state, getters, mutations } from "@/store"; // Import your custom store
import Action from "@/components/Action.vue";

export default {
  name: "ContextMenu",
  components: {
    Action,
  },
  data() {
    return {
      posX: 0,
      posY: 0,
    };
  },
  computed: {
    isMultiple() {
      return state.multiple;
    },
    user() {
      return state.user;
    },
    isMobile() {
      return getters.isMobile();
    },
    showContext() {
      if (getters.currentPromptName() == "ContextMenu" && state.prompts != []) {
        this.setPositions();
        return true;
      }
      return false;
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
    headerButtons() {
      return {
        select: state.selected.length > 0,
        upload: state.user.perm?.create && state.selected.length > 0,
        download: state.user.perm.download && state.selected.length > 0,
        delete: state.selected.length > 0 && state.user.perm.delete,
        rename: state.selected.length === 1 && state.user.perm.rename,
        share: state.selected.length === 1 && state.user.perm.share,
        move: state.selected.length > 0 && state.user.perm.rename,
        copy: state.selected.length > 0 && state.user.perm?.create,
      };
    },
    selectedCount() {
      return getters.selectedCount();
    },
  },
  methods: {
    uploadFunc() {
      mutations.showHover("upload");
    },
    showHover(value) {
      return mutations.showHover(value);
    },
    setPositions() {
      const contextProps = getters.currentPrompt().props;
      this.posX = contextProps.posX;
      this.posY = contextProps.posY;
    },
    toggleMultipleSelection() {
      mutations.setMultiple(!state.multiple);
      mutations.closeHovers();
    },
    startDownload() {
      downloadFiles();
    },
  },
};
</script>

<style scoped>
#context-menu {
  position: absolute;
  z-index: 1000;
  background-color: white;
  max-width: 20em;
  min-width: 15em;
  min-height: 4em;
  height: auto;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

#context-menu.mobile {
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
  color: #6f6f6f;
  margin-right: auto;
}

#context-menu .action span {
  display: none;
}

/* File selection */
#context-menu.dark-mode {
  background: var(--surfaceSecondary) !important;
}

#context-menu.dark-mode span {
  color: var(--textPrimary) !important;
}
</style>
