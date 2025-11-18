<template>
  <div class="card-title">
    <h2>{{ $t("prompts.rename") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("prompts.renameMessage") }}</p>

    <div v-if="item.type !== 'directory'" class="filename-inputs">
      <input class="input" :class="{ 'form-invalid': !validation.valid }" v-focus type="text" @keyup.enter="submit"
        v-model.trim="fileName" @input="updateFullName" />
      <span class="extension-separator">.</span> <!--eslint-disable-line @intlify/vue-i18n/no-raw-text-->
      <input class="input extension-input" type="text" @keyup.enter="submit" v-model.trim="fileExtension"
        @input="updateFullName" />
    </div>

    <input v-else class="input" :class="{ 'form-invalid': !validation.valid }" v-focus type="text" @keyup.enter="submit"
      v-model.trim="name" />
    <p v-if="!validation.valid && name.length > 0" class="validation-error">
      <span v-if="validation.reason === 'conflict'">
        {{ $t("prompts.renameMessageConflict", { filename: name }) }}
      </span>
      <span v-else>
        {{ $t("prompts.renameMessageInvalid") }}
      </span>
    </p>
  </div>

  <div class="card-action">
    <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button @click="submit" class="button button--flat" :disabled="!canRename"
      type="submit" :aria-label="$t('general.rename')" :title="$t('general.rename')">
      {{ $t("general.rename") }}
    </button>
  </div>
</template>
<script>
import { filesApi, publicApi } from "@/api";
import { mutations, state, getters } from "@/store";
import { notify } from "@/notify";
export default {
  name: "rename",
  props: {
    item: {
      type: Object,
      required: true,
      default: () => ({ source: "", path: "", name: "" })
    }
  },
  data() {
    // Separate filename and extension for avoid renmame mistakenly the extension too
    const itemName = this.item.name;

    if (this.item.type !== 'directory') {
      const lastDotIndex = this.item.name.lastIndexOf('.');
      const hasExtension = lastDotIndex > 0;
      return {
        fileName: hasExtension ? itemName.substring(0, lastDotIndex) : itemName,
        fileExtension: hasExtension ? itemName.substring(lastDotIndex + 1) : "",
        name: itemName,
      };
    }
    return {
      fileName: "",
      fileExtension: "",
      name: itemName,
    };
  },
  computed: {
    closeHovers() {
      return mutations.closeHovers;
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
    }
  },
  mounted() {
    this.$nextTick(() => {
      // Auto-focus filename input field
      if (this.item.type !== 'directory') {
        const filenameInput = this.$el.querySelector('.filename-input');
        filenameInput?.select();
      }
    });
  },
  methods: {
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
      if (value === "") {
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

      // Check if the item already exists
      for (const item of state.req.items) {
        if (item.path === this.item.path) continue;
        if (item.name.toLowerCase() === value.toLowerCase()) {
          if (isFolder === (item.type === "directory")) {
            return { valid: false, reason: 'conflict' };
          }
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
      try {
        const items = [{
          from: this.item.path,
          fromSource: this.item.source,
          to: newPath,
          toSource: this.item.source,
        }];

        if (getters.isShare()) {
          await publicApi.moveCopy(state.shareInfo.hash, items, "move");
        } else {
          await filesApi.moveCopy(items, "move");
        }
        notify.showSuccess(this.$t("prompts.renameSuccess"));
        mutations.setReload(true);
        mutations.closeHovers();
      } catch (error) {
        console.error(error);
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
  width: 80px;
}

.extension-separator {
  font-weight: bold;
}
</style>
