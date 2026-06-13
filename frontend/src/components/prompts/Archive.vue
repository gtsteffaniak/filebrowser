<template>
  <div class="card-content">
    <div v-show="creating" class="loading-content">
      <LoadingSpinner size="small" mode="placeholder" />
      <p class="loading-text">{{ $t("prompts.operationInProgress") }}</p>
    </div>
    <div v-show="!creating">
      <template v-if="showFileList">
        <file-list
          ref="fileList"
          :browse-path="currentPath"
          :show-folders="true"
          :show-files="false"
          @update:selected="updateDestination"
        />
      </template>
      <template v-else>
        <p>{{ $t("prompts.archiveMessage") }}</p>
        <p class="prompts-label">{{ $t("prompts.archiveDestination") }}</p>
        <div
          aria-label="archive-destination"
          class="searchContext clickable button"
          @click="showFileList = true"
        >
          {{ $t("general.path", { suffix: ":" }) }} {{ destPath }}{{ destSource ? ` (${destSource})` : "" }}
        </div>
        <p class="prompts-label">{{ $t("prompts.archiveName") }}</p>
        <input
          v-model.trim="archiveName"
          class="input"
          type="text"
          :placeholder="defaultArchiveName"
        />
        <p class="prompts-label">{{ $t("general.format", { suffix: ":" }) }}</p>
        <select v-model="format" class="input">
          <option value="zip">zip</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <option value="tar.gz">tar.gz</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        </select>
        <p v-if="format === 'tar.gz'" class="prompts-label">{{ $t("prompts.archiveCompression") }}</p>
        <input
          v-if="format === 'tar.gz'"
          v-model.number="compression"
          class="input"
          type="number"
          min="0"
          max="9"
        />
        <div class="archive-options settings-items">
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
        :disabled="!canSubmit || creating"
        :aria-label="$t('prompts.archive')"
        :title="$t('prompts.archive')"
        @click="submit"
      >
        {{ $t("prompts.archive") }}
      </button>
    </template>
  </div>
</template>

<script>
import { state, mutations, getters } from "@/store";
import { notify } from "@/notify";
import { resourcesApi } from "@/api";
import { goToItemNotificationButton } from "@/utils/notificationActions";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import FileList from "@/components/files/FileList.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "archive",
  components: { LoadingSpinner, FileList, ToggleSwitch },
  props: {
    items: {
      type: Array,
      required: true,
    },
    source: {
      type: String,
      required: true,
    },
    currentPath: {
      type: String,
      default: "/",
    },
  },
  data() {
    return {
      destPath: "/",
      destSource: null,
      archiveName: "",
      format: "zip",
      compression: 0,
      creating: false,
      showFileList: false,
      showNewDirInput: false,
      newDirName: "",
      deleteAfter: state.user?.deleteAfterArchive === true,
    };
  },
  watch: {
    deleteAfter(newVal) {
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
    if (this.currentPath) {
      this.destPath = this.currentPath.replace(/\/+$/, "") || "/";
      if (!this.destPath.startsWith("/")) this.destPath = `/${this.destPath}`;
    }
  },
  computed: {
    defaultArchiveName() {
      return this.format === "tar.gz" ? "archive.tar.gz" : "archive.zip";
    },
    canSubmit() {
      const name = this.archiveName || this.defaultArchiveName;
      return name.length > 0 && this.destPath;
    },
    canCreateFolder() {
      const perms = getters.permissions();
      return !!perms?.create;
    },
    isDirNameValid() {
      return this.validateDirName(this.newDirName);
    },
    fullDestination() {
      const name = this.archiveName || this.defaultArchiveName;
      const base = (this.destPath || "/").replace(/\/+$/, "");
      const path = base === "" ? `/${name}` : `${base}/${name}`;
      return path.replace(/\/+/g, "/");
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
      }
    },
    createNewDir() {
      this.showNewDirInput = true;
      this.newDirName = "";
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
        this.creating = true;
        const currentPath = this.$refs.fileList.path;
        const currentSource = this.$refs.fileList.source;
        const fullPath = currentPath.endsWith("/")
          ? `${currentPath + this.newDirName}/`
          : `${currentPath}/${this.newDirName}/`;
        if (getters.isShare()) {
          await resourcesApi.postPublic(state.shareInfo.hash, fullPath, "", false, undefined, {}, true);
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
        this.creating = false;
      }
    },
    async submit() {
      if (!this.canSubmit) return;
      this.creating = true;
      try {
        let dest = this.fullDestination;
        const ext = this.format === "tar.gz" ? ".tar.gz" : ".zip";
        if (!dest.endsWith(".zip") && !dest.endsWith(".tar.gz") && !dest.endsWith(".tgz")) {
          dest = dest + ext;
        }
        const payload = {
          fromSource: this.source,
          paths: this.items.map((it) => (typeof it === "string" ? it : it.path)),
          destination: dest,
          format: this.format,
          compression: this.compression,
        };
        if (this.destSource && this.destSource !== this.source) {
          payload.toSource = this.destSource;
        }
        if (this.deleteAfter) {
          payload.deleteAfter = true;
        }
        await resourcesApi.createArchive(payload);
        mutations.setReload(true);
        mutations.closeTopPrompt();

        const destSource = this.destSource || this.source;
        const archivePath = dest;
        notify.showSuccess(this.$t("prompts.archiveSuccess"), {
          icon: "folder",
          buttons: archivePath
            ? [
                goToItemNotificationButton(
                  this.$t("buttons.goToItem"),
                  destSource,
                  archivePath,
                  getters.isShare()
                ),
              ]
            : undefined,
        });
      } catch (err) {
        console.error(err);
      } finally {
        this.creating = false;
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
.archive-options {
  margin-top: 1em;
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
