<template>
  <div v-if="selectedCount > 0" id="file-selection" :class="{ 'dark-mode': isDarkMode }">
    <span>{{ selectedCount }} selected</span>
    <div>
      <action
        v-if="headerButtons.select"
        icon="info"
        :label="$t('buttons.info')"
        show="info"
      />
      <action
        v-if="headerButtons.select"
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
  </div>
</template>

<script>
import downloadFiles from "@/utils/download";
import { state, getters, mutations } from "@/store"; // Import your custom store
import Action from "@/components/header/Action.vue";

export default {
  name: "fileSelection",
  components: {
    Action,
  },
  computed: {
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
<style>
@media (min-width: 800px) {
  #file-selection {
    bottom: 4em;
  }
}

#file-selection .action {
  border-radius: 50%;
  width: auto;
}

#file-selection > span {
  display: inline-block;
  margin-left: 1em;
  color: #6f6f6f;
  margin-right: auto;
}

#file-selection .action span {
  display: none;
}

/* File Selection */
#file-selection {
  box-shadow: rgba(0, 0, 0, 0.3) 0px 2em 50px 10px;
  position: fixed;
  bottom: 4em;
  left: 50%;
  transform: translateX(-50%);
  align-items: center;
  background: #fff;
  max-width: 30em;
  z-index: 3;
  border-radius: 1em;
  display: flex;
  width: 90%;
}
/* File selection */
#file-selection.dark-mode {
  background: var(--surfaceSecondary) !important;
}

#file-selection.dark-mode span {
  color: var(--textPrimary) !important;
}
</style>
