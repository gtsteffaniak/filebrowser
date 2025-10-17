<template>
  <div class="card-title">
    <h2>{{ $t("prompts.rename") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("prompts.renameMessage") }}</p>
    <input class="input" :class="{ 'form-invalid': !validation.valid }" v-focus type="text" @keyup.enter="submit"
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
    <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('buttons.cancel')"
      :title="$t('buttons.cancel')">
      {{ $t("buttons.cancel") }}
    </button>
    <button @click="submit" class="button button--flat" :disabled="!validation.valid || name.length === 0"
      type="submit" :aria-label="$t('buttons.rename')" :title="$t('buttons.rename')">
      {{ $t("buttons.rename") }}
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
    return {
      name: this.item?.name || "",
    };
  },
  computed: {
    closeHovers() {
      return mutations.closeHovers;
    },
    oldName() {
      return this.item?.name || "";
    },
    validation() {
      return this.validateFileName(this.name);
    }
  },
  methods: {
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
      if (!this.validation.valid) {
        return;
      }

      // Remove trailing slashes from the source path before calculating parent directory
      let sourcePath = this.item.path;
      while (sourcePath.endsWith("/") || sourcePath.endsWith("\\")) {
        sourcePath = sourcePath.substring(0, sourcePath.length - 1);
      }

      let newPath = sourcePath.substring(0, sourcePath.lastIndexOf("/"));
      newPath = `${newPath}/${this.name}`;
      try {
        if (this.name === this.item.name) {
          mutations.closeHovers();
          return;
        }

        const items = [{
          from: this.item.path,
          fromSource: this.item.source,
          to: newPath,
          toSource: this.item.source,
        }];

        if (getters.isShare()) {
          await publicApi.moveCopy(items, "move");
        } else {
          await filesApi.moveCopy(items, "move");
        }
        mutations.closeHovers();
      } catch (error) {
        notify.showError(error);
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
</style>
