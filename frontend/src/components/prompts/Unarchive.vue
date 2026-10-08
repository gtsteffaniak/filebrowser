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
          :sortable="true"
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
        <section v-if="isZip" class="encoding-options" aria-live="polite">
          <label for="zip-filename-encoding" class="prompts-label">{{ $t("prompts.zipFilenameEncoding") }}</label>
          <p v-if="encodingLoading">{{ $t("prompts.zipEncodingLoading") }}</p>
          <template v-else-if="encodingError">
            <p role="alert">{{ $t("prompts.zipEncodingError") }}</p>
            <button type="button" class="button button--flat" @click="loadEncodingPreview">
              {{ $t("prompts.zipEncodingRetry") }}
            </button>
          </template>
          <template v-else>
            <select id="zip-filename-encoding" class="input" v-model="filenameEncoding">
              <option disabled value="">{{ $t("prompts.zipEncodingChoose") }}</option>
              <option v-for="candidate in encodingCandidates" :key="candidate.encoding" :value="candidate.encoding">
                {{ candidate.encoding }}{{ candidate.encoding === suggestedEncoding ? ` (${$t("prompts.zipEncodingSuggested")})` : "" }}
              </option>
            </select>
            <p>{{ $t("prompts.zipEncodingHint") }}</p>
            <template v-if="filenamePreview.length">
              <p class="prompts-label">{{ $t("prompts.zipFilenamePreview") }}</p>
              <ul class="filename-preview">
                <li v-for="(name, index) in filenamePreview" :key="index">{{ name }}</li>
              </ul>
            </template>
            <p v-else-if="!encodingCandidates.length" role="alert">{{ $t("prompts.zipEncodingUnavailable") }}</p>
          </template>
        </section>
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
        class="button button--flat button--grey"
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
        :placeholder="$t('files.newFolderMessage')"
        @keydown.enter="handleEnter"
      />
      <button
        type="button"
        v-if="!showNewDirInput"
        class="button button--flat"
        @click="showFileList = false"
        :aria-label="$t('general.select')"
        :title="$t('general.select')"
      >
        {{ $t("general.select") }}
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
        :disabled="!destPath || !isDirSelection || isLoading || !encodingReady"
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
    promptId: {
      type: [String, Number],
      default: null,
    },
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
      encodingLoading: true,
      encodingError: false,
      encodingCandidates: [],
      filenameEncoding: "",
      suggestedEncoding: "",
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
    if (this.isZip) void this.loadEncodingPreview();
  },
  computed: {
    isZip() {
      return (this.itemPath || "").toLowerCase().endsWith(".zip");
    },
    encodingReady() {
      return !this.isZip || (!this.encodingLoading && !this.encodingError && !!this.filenameEncoding);
    },
    filenamePreview() {
      return this.encodingCandidates.find((candidate) => candidate.encoding === this.filenameEncoding)?.names || [];
    },
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
      return getters.canCreateInSource(this.destSource || this.itemSource);
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
    async loadEncodingPreview() {
      this.encodingLoading = true;
      this.encodingError = false;
      this.filenameEncoding = "";
      try {
        const result = await resourcesApi.unarchive({
          fromSource: this.itemSource,
          toSource: this.destSource || this.itemSource,
          path: this.itemPath,
          destination: this.destPath,
          preview: true,
        });
        this.encodingCandidates = result.candidates;
        this.suggestedEncoding = result.suggested;
        this.filenameEncoding = result.suggested;
      } catch {
        this.encodingError = true;
      } finally {
        this.encodingLoading = false;
      }
    },
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
    async createNewDir() {
      this.showNewDirInput = true;
      this.newDirName = this.defaultNewDirName;
      await this.$nextTick();
      this.$refs.newDirInput?.focus();
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
      if (!this.destPath || !this.isDirSelection || !this.encodingReady || this.isLoading) return;
      this.isLoading = true;
      try {
        const toSource = this.destSource || this.itemSource;
        await resourcesApi.unarchive({
          fromSource: this.itemSource,
          toSource: toSource !== this.itemSource ? toSource : undefined,
          path: this.itemPath,
          destination: this.destPath,
          deleteAfter: this.deleteAfter,
          ...(this.isZip && { filenameEncoding: this.filenameEncoding }),
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
.encoding-options {
  margin-top: 1em;
}

.encoding-options label {
  display: block;
}

.filename-preview {
  max-height: 10em;
  overflow: auto;
  overflow-wrap: anywhere;
}

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
