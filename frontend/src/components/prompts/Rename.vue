<template>
  <div class="card-title">
    <h2>{{ $t("prompts.rename") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("prompts.renameMessage", { filename: oldName }) }}</p>
    <input class="input" :class="{ 'form-invalid': !validateFileName(name) }" v-focus type="text"
      @keyup.enter="submit" v-model.trim="name" />
    <p v-if="!validateFileName(name) && name.length > 0" class="validation-error">
      {{ $t("prompts.invalidName") }}
    </p>
  </div>

  <div class="card-action">
    <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('buttons.cancel')"
      :title="$t('buttons.cancel')">
      {{ $t("buttons.cancel") }}
    </button>
    <button @click="submit" class="button button--flat" :disabled="!validateFileName(name) || name.length === 0"
      type="submit" :aria-label="$t('buttons.rename')" :title="$t('buttons.rename')">
      {{ $t("buttons.rename") }}
    </button>
  </div>
</template>
<script>
import { filesApi } from "@/api";
import { mutations } from "@/store";
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
  },
  methods: {
    /**
     * @param {string} value
     */
    validateFileName(value) {
      if (value === "") {
        return false;
      }
      // Check for forbidden characters: forward slash and backslash
      const forbiddenChars = /[/\\]/;
      return !forbiddenChars.test(value);
    },
    async submit() {
      // Validate before submitting
      if (!this.validateFileName(this.name) || this.name.length === 0) {
        notify.showError(this.$t("prompts.invalidName"));
        return;
      }

      try {
        const items = [{
          from: this.item.path,
          fromSource: this.item.source,
          to: this.item.path.replace(/[^/]+$/, this.name),
          toSource: this.item.source,
        }];

        await filesApi.moveCopy(items, "move");
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
