<template>
  <div class="card-content">
    <!-- Loading spinner overlay -->
    <div v-show="creating" class="loading-content">
      <LoadingSpinner size="small" />
      <p class="loading-text">{{ $t("prompts.operationInProgress") }}</p>
    </div>
    <div v-show="!creating">
      <p>{{ $t("prompts.newFileMessage") }}</p>
      <div v-if="usesTemplate" class="filename-inputs">
        <i class="file-type-icon" :class="typeInfo.classes" aria-hidden="true">{{ typeInfo.materialSymbol }}</i>
        <input ref="filenameInput" class="input" aria-label="FileName Field" v-focus type="text" @keyup.enter="submit"
          v-model.trim="fileName" @input="updateFullName" />
        <span class="extension-separator">.</span> <!--eslint-disable-line @intlify/vue-i18n/no-raw-text-->
        <input class="input extension-input" type="text" @keyup.enter="submit" v-model.trim="fileExtension" @input="updateFullName" />
      </div>
      <div v-else class="new-file-input-row">
        <i class="file-type-icon" :class="typeInfo.classes" aria-hidden="true">{{ typeInfo.materialSymbol }}</i>
        <input class="input" aria-label="FileName Field" v-focus type="text" @keyup.enter="submit"
          v-model.trim="name" />
      </div>
    </div>
  </div>

  <div class="card-actions">
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
      @click="submit" :aria-label="$t('general.create')"
      :title="$t('general.create')">
      {{ $t("general.create") }}
    </button>
  </div>
</template>
<script>
import { state } from "@/store";
import { resourcesApi } from "@/api";
import { getters, mutations } from "@/store"; // Import your custom store
import { notify } from "@/notify";
import { url } from "@/utils";
import { goToItemNotificationButton } from "@/utils/notificationActions";
import { getTypeInfoFromExt } from "@/utils/mimetype";
import { getFileExtension, removePrefix } from '@/utils/files.js';
import LoadingSpinner from "@/components/LoadingSpinner.vue";
export default {
  name: "new-file",
  components: {
    LoadingSpinner,
  },
  props: {
    base: {
      type: [String, Object, null],
      default: null,
    },
    initialName: {
      type: String,
      default: "",
    },
  },
  data() {
    const usesTemplate = !!(this.initialName && this.initialName.trim() !== "");
    let fileName = "";
    let fileExtension = "";
    if (usesTemplate) {
      const ext = getFileExtension(this.initialName);
      fileName = this.initialName.substring(0, this.initialName.length - ext.length);
      fileExtension = removePrefix(ext, ".");
    }
    return {
      usesTemplate,
      fileName,
      fileExtension,
      name: this.initialName || "",
      creating: false,
    };
  },
  computed: {
    isFiles() {
      return getters.isFiles();
    },
    isListing() {
      return getters.isListing();
    },
    closeTopPrompt() {
      return mutations.closeTopPrompt();
    },
    // Determine parent path and source based on prop
    parentInfo() {
      if (this.base) {
        if (typeof this.base === 'string') {
          return {
            path: this.base,
            source: state.req?.source || null,
          };
        } else if (typeof this.base === 'object' && this.base.path) {
          return {
            path: this.base.path,
            source: this.base.source || state.req?.source || null,
          };
        }
      }
      // Fallback to current path
      return {
        path: state.req?.path || '/',
        source: state.req?.source || null,
      };
    },
    currentPromptName() {
      return getters.currentPromptName();
    },
    typeInfo() {
      return getTypeInfoFromExt(this.name);
    },
  },
  watch: {
    currentPromptName(newName, oldName) {
      if (this.creating && oldName === "replace-rename" && newName !== "new-file") {
        this.creating = false;
      }
    },
  },
  mounted() {
    if (this.usesTemplate) {
      this.$nextTick(() => {
        this.$refs.filenameInput?.select();
      });
    }
  },
  methods: {
    updateFullName() {
      if (this.usesTemplate) {
        this.name = this.fileExtension
          ? `${this.fileName}.${this.fileExtension}`
          : this.fileName;
      }
    },
    async submit(event) {
      try {
        event.preventDefault();
        if (this.name === "") return;
        await this.createFile(false);
      } catch (error) {
        console.error(error);
      }
    },

    showNotification(source, newPath) {
      const buttonProps = {
        icon: "insert_drive_file",
        buttons: [
          goToItemNotificationButton(
            this.$t("buttons.goToItem"),
            source,
            newPath,
            getters.isShare()
          ),
        ],
      };
      notify.showSuccess(this.$t("prompts.newFileSuccess"), buttonProps);
    },

    async createFile(overwrite = false) {
      this.creating = true;
      try {
        const parentPath = this.parentInfo.path;
        const source = this.parentInfo.source;
        const newPath = url.joinPath(parentPath, this.name);

        if (getters.isShare()) {
          await resourcesApi.postPublic(state.shareInfo?.hash, newPath, "", overwrite);
          mutations.setReload(true);
          mutations.closeTopPrompt();
          this.creating = false;
          // Show notification for shares
          this.showNotification(state.shareInfo?.hash, newPath);
          return;
        }
        await resourcesApi.post(source, newPath, "", overwrite);
        mutations.setReload(true);
        mutations.closeTopPrompt();

        // Show notification
        this.showNotification(source, newPath);
        this.creating = false;
      } catch (error) {
        if (error.message === "conflict") {
          // Show replace-rename prompt for file/folder conflicts
          mutations.showPrompt({
            name: "replace-rename",
            pinned: true,
            confirm: async (event, option) => {
              event.preventDefault();
              try {
                if (option === "overwrite") {
                  await this.createFile(true); // Retry with overwrite
                } else if (option === "rename") {
                  // Add a suffix to make the name unique (max 100 attempts)
                  const originalName = this.name;
                  const maxAttempts = 100;
                  let success = false;
                  for (let counter = 1; counter <= maxAttempts && !success; counter++) {
                    try {
                      const newName = counter === 1 ? `${originalName} (1)` : `${originalName} (${counter})`;
                      const parentPath = this.parentInfo.path;
                      const source = this.parentInfo.source;
                      const newPath = url.joinPath(parentPath, newName);

                      if (getters.isShare()) {
                        await resourcesApi.postPublic(state.shareInfo?.hash, newPath, "", false);
                        mutations.setReload(true);
                        mutations.closeTopPrompt(); // close conflict prompt
                        mutations.closeTopPrompt(); // close new file prompt
                        success = true;
                        // Show notification for shares after rename
                        this.showNotification(state.shareInfo?.hash, newPath);
                        return;
                      }
                      await resourcesApi.post(source, newPath, "", false);
                      mutations.setReload(true);
                      mutations.closeTopPrompt(); // close conflict prompt
                      mutations.closeTopPrompt(); // close new file prompt
                      success = true;

                      // Show notification
                      this.showNotification(source, newPath);
                    } catch (renameError) {
                      if (renameError.message !== "conflict") {
                        throw renameError;
                      }
                    }
                  }
                  if (!success) {
                    throw new Error(`Could not find a unique name after ${maxAttempts} attempts`);
                  }
                }
              } catch (retryError) {
                notify.showError(retryError);
              } finally {
                this.creating = false;
              }
            },
          });
        } else {
          this.creating = false;
          throw error;
        }
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
  gap: 16px;
  padding-top: 2em;
}

.loading-text {
  padding: 1em;
  margin: 0;
  font-size: 1em;
  font-weight: 500;
}

.card-content {
  position: relative;
}

.new-file-input-row {
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.new-file-input-row .input {
  flex: 1;
}

.filename-inputs {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.extension-input {
  width: 6em;
}

.extension-separator {
  font-weight: bold;
}

.file-type-icon {
  flex-shrink: 0;
}
</style>
