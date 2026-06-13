<template>
  <div class="card-content">
    <div v-show="isLoading" class="loading-content">
      <LoadingSpinner size="small" mode="placeholder" />
      <p class="loading-text">{{ $t("prompts.operationInProgress") }}</p>
    </div>
    <div v-show="!isLoading">
      <template v-if="showFileList">
        <file-list
          ref="fileList"
          :browse-path="parentPath"
          :browse-source="itemSource"
          :show-folders="true"
          :show-files="false"
          @update:selected="updateDestination"
        />
      </template>
      <template v-else>
        <p>{{ $t("prompts.unarchiveMessage") }}</p>
        <p class="prompts-label">{{ $t("prompts.unarchiveDestination") }}</p>
        <div
          aria-label="unarchive-destination"
          class="searchContext clickable button"
          @click="showFileList = true"
        >
          {{ $t("general.path", { suffix: ":" }) }} {{ destPath }}{{ destSource ? ` (${destSource})` : "" }}
        </div>
        <div class="unarchive-options settings-items">
          <ToggleSwitch class="item" v-model="deleteAfter" 
            :name="$t('profileSettings.deleteAfterArchive')"
            :description="$t('profileSettings.deleteAfterArchiveDescription')" />
        </div>
      </template>
    </div>
  </div>
  <div class="card-actions" :class="{ 'split-buttons': showFileList }">
    <template v-if="showFileList">
      <button
        type="button"
        v-if="!showNewDirInput"
        class="button button--flat button--grey"
        @click="showFileList = false"
        :aria-label="$t('general.cancel')"
        :title="$t('general.cancel')"
      >
        {{ $t("general.cancel") }}
      </button>
      <button
        type="button"
        v-if="canCreateFolder && showNewDirInput"
        class="button button--flat"
        @click="cancelNewDir"
        :aria-label="$t('general.cancel')"
        :title="$t('general.cancel')"
      >
        {{ $t("general.cancel") }}
      </button>
      <button
        type="button"
        v-if="canCreateFolder && !showNewDirInput"
        class="button button--flat"
        @click="createNewDir"
        :aria-label="$t('files.newFolder')"
        :title="$t('files.newFolder')"
      >
        <span>{{ $t("files.newFolder") }}</span>
      </button>
      <input
        v-if="showNewDirInput"
        ref="newDirInput"
        class="input new-dir-input"
        :class="{ 'form-invalid': !isDirNameValid }"
        v-model.trim="newDirName"
        :placeholder="$t('prompts.newDirMessage')"
        @keydown.enter="handleEnter"
      />
      <button
        type="button"
        v-if="!showNewDirInput"
        class="button button--flat"
        @click="showFileList = false"
        :aria-label="$t('general.select', { suffix: '' })"
        :title="$t('general.select', { suffix: '' })"
      >
        {{ $t("general.select", { suffix: "" }) }}
      </button>
      <button
        type="button"
        v-if="showNewDirInput"
        class="button button--flat"
        @click="createDirectory"
        :disabled="!newDirName || !isDirNameValid"
      >
        {{ $t("general.create") }}
      </button>
    </template>
    <template v-else>
      <button
        type="button"
        class="button button--flat button--grey"
        @click="closeTopPrompt"
        :aria-label="$t('general.cancel')"
        :title="$t('general.cancel')"
      >
        {{ $t("general.cancel") }}
      </button>
      <button
        type="button"
        class="button button--flat"
        :disabled="!destPath || !isDirSelection || isLoading"
        :aria-label="$t('prompts.unarchive')"
        :title="$t('prompts.unarchive')"
        @click="submit"
      >
        {{ $t("prompts.unarchive") }}
      </button>
    </template>
  </div>
</template>

<script>
import { state, mutations, getters } from "@/store";
import { url } from "@/utils";
import { notify } from "@/notify";
import { resourcesApi } from "@/api";
import { goToItemNotificationButton } from "@/utils/notificationActions";
import FileList from "@/components/files/FileList.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "unarchive",
  components: { FileList, LoadingSpinner, ToggleSwitch },
  props: {
    item: {
      type: Object,
      required: true,
    },
  },
  data() {
    return {
      destPath: "/",
      destSource: null,
      destType: null,
      deleteAfter: state.user?.deleteAfterArchive === true,
      isLoading: false,
      showFileList: false,
      showNewDirInput: false,
      newDirName: "",
    };
  },
  watch: {
    deleteAfter(newVal) {
      // Update the user preference in real time
      void mutations.updateCurrentUser({ deleteAfterArchive: newVal });
    },
    showFileList(newVal) {
      if (!newVal) {
        this.showNewDirInput = false;
        this.newDirName = "";
      }
    },
  },
  mounted() {
    this.destPath = this.parentPath || "/";
    this.destSource = this.itemSource;
  },
  computed: {
    itemSource() {
      return this.item.source || this.item.fromSource;
    },
    itemPath() {
      return this.item.path || this.item.from;
    },
    parentPath() {
      if (!this.itemPath) return "/";
      return `${url.removeLastDir(this.itemPath)}/`;
    },
    isDirSelection() {
      return this.destType === "directory" || !this.destType;
    },
    canCreateFolder() {
      const perms = getters.permissions();
      return !!perms?.create;
    },
    isDirNameValid() {
      return this.validateDirName(this.newDirName);
    },
    defaultNewDirName() {
      const name = this.item?.name || "";
      const lower = name.toLowerCase();
      if (lower.endsWith(".tar.gz")) return name.slice(0, -7);
      if (lower.endsWith(".tgz")) return name.slice(0, -4);
      if (lower.endsWith(".zip")) return name.slice(0, -4);
      return name;
    },
  },
  methods: {
    closeTopPrompt() {
      mutations.closeTopPrompt();
    },
    updateDestination(pathOrData) {
      if (typeof pathOrData === "string") {
        this.destPath = pathOrData;
      } else if (pathOrData?.path) {
        this.destPath = pathOrData.path;
        this.destSource = pathOrData.source;
        this.destType = pathOrData.type;
      }
    },
    createNewDir() {
      this.showNewDirInput = true;
      this.newDirName = this.defaultNewDirName;
      this.$nextTick(() => {
        this.$refs.newDirInput?.focus();
      });
    },
    validateDirName(value) {
      if (this.$refs.fileList?.items) {
        const currentItems = this.$refs.fileList.items.filter((item) => item.name !== "..");
        return !currentItems.some((item) => item.name.toLowerCase() === value.toLowerCase());
      }
      return true;
    },
    cancelNewDir() {
      this.showNewDirInput = false;
      this.newDirName = "";
    },
    handleEnter(event) {
      event.stopPropagation();
      event.preventDefault();
      if (this.newDirName && this.isDirNameValid) {
        void this.createDirectory();
      }
    },
    async createDirectory() {
      if (!this.newDirName || !this.isDirNameValid) return;
      try {
        this.isLoading = true;
        const currentPath = this.$refs.fileList.path;
        const currentSource = this.$refs.fileList.source;
        const fullPath = currentPath.endsWith("/")
          ? `${currentPath + this.newDirName}/`
          : `${currentPath}/${this.newDirName}/`;
        if (getters.isShare()) {
          await resourcesApi.postPublic(state.shareInfo?.hash, fullPath, "", false, undefined, {}, true);
        } else {
          await resourcesApi.post(currentSource, fullPath, "", false, undefined, {}, true);
        }
        if (getters.isShare()) {
          await resourcesApi.fetchFilesPublic(currentPath, state.shareInfo.hash)
            .then((req) => this.$refs.fileList.fillOptions(req, true));
        } else {
          await resourcesApi.fetchFiles(currentSource, currentPath)
            .then((req) => this.$refs.fileList.fillOptions(req, true));
        }
        mutations.setReload(true);
        this.showNewDirInput = false;
        this.newDirName = "";
      } catch (error) {
        console.error("Error creating directory:", error);
      } finally {
        this.isLoading = false;
      }
    },
    async submit() {
      if (!this.destPath || !this.isDirSelection) return;
      this.isLoading = true;
      try {
        const toSource = this.destSource || this.itemSource;
        await resourcesApi.unarchive({
          fromSource: this.itemSource,
          toSource: toSource !== this.itemSource ? toSource : undefined,
          path: this.itemPath,
          destination: this.destPath,
          deleteAfter: this.deleteAfter,
        });
        mutations.setReload(true);
        mutations.closeTopPrompt();

        const destPath = this.destPath;
        const destSource = toSource;
        notify.showSuccess(this.$t("prompts.unarchiveSuccess"), {
          icon: "folder",
          buttons: destPath
            ? [
                goToItemNotificationButton(
                  this.$t("buttons.goToItem"),
                  destSource || state.shareInfo?.hash,
                  destPath,
                  getters.isShare()
                ),
              ]
            : undefined,
        });
      } catch (err) {
        console.error(err);
      } finally {
        this.isLoading = false;
      }
    },
  },
};
</script>

<style scoped>
.loading-content {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding-top: 2em;
  min-height: 200px;
}
.loading-text {
  padding: 1em;
  margin: 0;
  font-size: 1em;
  font-weight: 500;
}
.prompts-label {
  margin-top: 1em;
  margin-bottom: 0.25em;
  font-weight: 500;
}
.unarchive-options {
  margin-top: 1em;
}
.checkbox-label {
  display: flex;
  align-items: center;
  gap: 0.5em;
  cursor: pointer;
}
.card-content {
  position: relative;
}

.new-dir-input {
  justify-self: left;
}

.split-buttons {
  justify-content: space-between;
}
</style>
