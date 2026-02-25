<template>
  <div class="card-content">
    <!-- Loading spinner overlay -->
    <div v-show="renaming" class="loading-content">
      <LoadingSpinner size="small" />
      <p class="loading-text">{{ $t("prompts.operationInProgress") }}</p>
    </div>
    <div v-show="!renaming">
      <p>{{ $t("prompts.renameMessage") }}</p>

      <div v-if="item.type !== 'directory'" class="filename-inputs">
      <input ref="filenameInput" aria-label="New Name" class="input" :class="{ 'form-invalid': !validation.valid }" v-focus type="text" @keydown="onKeydown" @keyup="onKeyup" v-model.trim="fileName" @input="updateFullName" />
      <span class="extension-separator">.</span> <!--eslint-disable-line @intlify/vue-i18n/no-raw-text-->
      <input class="input extension-input" type="text" @keydown="onKeydown" @keyup="onKeyup" v-model.trim="fileExtension" @input="updateFullName" />
    </div>

    <input v-else ref="directoryInput" class="input" aria-label="New Name" :class="{ 'form-invalid': !validation.valid }" v-focus type="text" @keydown="onKeydown" @keyup="onKeyup" v-model.trim="name" />
    <p v-if="!validation.valid && name.length > 0" class="validation-error">
      <span v-if="validation.reason === 'conflict'">
        {{ $t("prompts.renameMessageConflict", { filename: name }) }}
      </span>
      <span v-else>
        {{ $t("prompts.renameMessageInvalid") }}
      </span>
    </p>
    </div>
  </div>

  <div class="card-actions">
    <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button @click="submit" class="button button--flat" :disabled="!canRename"
      type="submit" :aria-label="$t('general.submit')" :title="$t('general.submit')">
      {{ $t("general.rename") }}
    </button>
  </div>
</template>
<script>
import { resourcesApi } from "@/api";
import { mutations, state, getters } from "@/store";
import { notify } from "@/notify";
import { getFileExtension, removePrefix } from '@/utils/files.js';
import { url } from "@/utils";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
export default {
  name: "rename",
  components: {
    LoadingSpinner,
  },
  props: {
    item: {
      type: Object,
      required: true,
      default: () => ({ source: "", path: "", name: "" })
    },
    parentItems: {  // Parent items for comparison of filenames in a preview
      type: Array,
      default: () => []
    }
  },
  data() {
    const itemName = this.item.name;
    if (this.item.type !== 'directory') {
      const ext = getFileExtension(this.item.name);
      const filenamePrefix = this.item.name.substring(0, this.item.name.length - ext.length);
      return {
        fileName: filenamePrefix,
        fileExtension: removePrefix(ext, "."),
        name: itemName, // Initialize name for non-directory items
        renaming: false,
      };
    }
    return {
      fileName: "",
      fileExtension: "",
      name: itemName,
      renaming: false,
    };
  },
  computed: {
    closeHovers() {
      return mutations.closeTopHover();
    },
    validation() {
      return this.validateFileName(this.name);
    },
    canRename() {
      // Enable the button only if the name is different from the original, and if the filename field is not empty
      const hasValidFileName = this.item.type === 'directory'
        ? this.name.trim() !== ''
        : this.fileName.trim() !== '';
      return this.validation.valid &&
             this.name !== this.item.name &&
             hasValidFileName;
    },
    isPreviewView() {
      return getters.isPreviewView();
    },
  },
  mounted() {
    this.$nextTick(() => {
      // Auto-focus filename input field
      if (this.item.type !== 'directory') {
        const filenameInput = this.$refs.filenameInput;
        filenameInput?.select();
      } else {
        const directoryInput = this.$refs.directoryInput;
        directoryInput?.select();
      }
    });
  },
  methods: {
    onKeydown(event) {
      // Allow "esc" key to close prompt and block other shortcuts
      // e.g. in plyrViewer we have "P" and "L", without this we can't type those letters
    if (event.key !== "Escape") {
        event.stopPropagation();
      }
    },
    onKeyup(event) {
    if (event.key !== "Escape") {
        event.stopPropagation();
      }
      if (event.key === 'Enter') {
        this.submit();
      }
    },
    updateFullName() {
      // Combine filename and extension
      if (this.item.type !== 'directory') {
        this.name = this.fileExtension 
          ? `${this.fileName}.${this.fileExtension}`
          : this.fileName;
      }
    },
    /**
     * @param {string} value
     */
    validateFileName(value) {
      // Handle undefined, null, or empty values
      if (!value || value === "") {
        return { valid: true };
      }

      const isFolder = this.item.type === "directory";

      // Check for forbidden characters in file names
      if (!isFolder) {
        const forbiddenChars = /[/\\]/;
        if (forbiddenChars.test(value)) {
          return { valid: false, reason: 'invalidChar' };
        }
      }

      // Get the current item's name for comparison
      const currentItemName = this.item.name.toLowerCase();
      const newName = value.toLowerCase();

      // If renaming to the same name (case-insensitive), allow it
      if (currentItemName === newName) {
        return { valid: true };
      }

      // Use parentItems if we are in a preview, otherwise use state.req.items
      const items = this.parentItems.length > 0 ? this.parentItems : (state.req?.items || []);
      for (const item of items) {
        if (!item.name) {
          continue;
        }
        const itemName = item.name.toLowerCase();
        // Skip the current item by comparing names (case-insensitive)
        // This is more reliable than comparing paths as before
        if (itemName === currentItemName) {
          continue;
        }
        if (itemName === newName) {
          return { valid: false, reason: 'conflict' };
        }
      }

      return { valid: true };
    },
    async submit() {
      // remove trailing slashes
      if (this.name.endsWith("/") || this.name.endsWith("\\")) {
        this.name = this.name.substring(0, this.name.length - 1);
      }
      if (!this.validation.valid || !this.canRename) return;
      // Remove trailing slashes from the source path before calculating parent directory
      let sourcePath = this.item.path;
      while (sourcePath.endsWith("/") || sourcePath.endsWith("\\")) {
        sourcePath = sourcePath.substring(0, sourcePath.length - 1);
      }

      let newPath = sourcePath.substring(0, sourcePath.lastIndexOf("/"));
      newPath = `${newPath}/${this.name}`;
      this.renaming = true;
      try {
        const items = [{
          from: this.item.path,
          fromSource: this.item.source,
          to: newPath,
          toSource: this.item.source,
        }];

        if (getters.isShare()) {
          await resourcesApi.moveCopyPublic(state.shareInfo.hash, items, "move");
        } else {
          await resourcesApi.moveCopy(items, "move");
        }
        notify.showSuccessToast(this.$t("prompts.renameSuccess"));
        mutations.closeTopHover();

        if (this.isPreviewView) {
          url.goToItem(this.item.source, newPath, undefined); // When undefined will not create browser history
        } else {
          mutations.setReload(true);
        }
      } catch (error) {
        console.error(error);
        // Parse the error response structure (similar to Delete.vue)
        let errorMessage = this.$t("prompts.renameFailed");
        
        if (error && error.failed && error.failed.length > 0) {
          // Get message from first failed item
          errorMessage = error.failed[0].message || errorMessage;
        } else if (error && error.message) {
          errorMessage = error.message;
        }
        
        notify.showError(errorMessage);
      } finally {
        this.renaming = false;
      }
    },
  },
};
</script>

<style scoped>
.form-invalid {
  border-color: var(--red) !important;
}

.validation-error {
  color: var(--red);
  font-size: 0.9em;
  margin-top: 0.5em;
  margin-bottom: 0;
}

.button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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
</style>
