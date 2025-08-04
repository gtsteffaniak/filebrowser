<template>
  <div class="card-title">
    <h2>{{ $t("prompts.rename") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("prompts.renameMessage", { filename: oldName() }) }}</p>
    <input class="input input--block" :class="{ 'invalid-form': !validateFileName(name) }" v-focus type="text"
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
import { state, getters, mutations } from "@/store";
import { notify } from "@/notify";

export default {
  name: "rename",
  data() {
    return {
      name: "",
    };
  },
  created() {
    this.name = this.oldName();
  },
  computed: {
    req() {
      return state.req;
    },
    selected() {
      return state.selected;
    },
    selectedCount() {
      return state.selectedCount;
    },
    isListing() {
      return getters.isListing();
    },
    closeHovers() {
      return mutations.closeHovers;
    },
    currentPrompt() {
      return getters.currentPrompt();
    },
  },
  methods: {
    validateFileName(value) {
      if (value === "") {
        return false;
      }
      // Check for forbidden characters: forward slash and backslash
      const forbiddenChars = /[/\\]/;
      return !forbiddenChars.test(value);
    },
    cancel() {
      mutations.closeHovers();
    },
    oldName() {
      // Check if this is being called from upload context with props
      if (this.currentPrompt && this.currentPrompt.props && this.currentPrompt.props.folderName) {
        return this.currentPrompt.props.folderName;
      }

      if (!this.isListing) {
        return state.req.name;
      }

      if (getters.selectedCount() === 0 || getters.selectedCount() > 1) {
        return "";
      }

      return state.req.items[this.selected[0]].name;
    },
    async submit() {
      // Validate before submitting
      if (!this.validateFileName(this.name) || this.name.length === 0) {
        notify.showError(this.$t("prompts.invalidName"));
        return;
      }

      try {
        // Check if this is being called from upload context with custom confirm handler
        if (this.currentPrompt && this.currentPrompt.confirm) {
          // This is for upload rename - call the custom confirm handler
          this.currentPrompt.confirm(this.name);
          return;
        }

        // Default file rename operation
        const items = [{
          from: state.req.path + "/" + state.req.items[this.selected[0]].name,
          fromSource: state.req.source,
          to: state.req.path + "/" + this.name,
          toSource: state.req.source,
        }];

        await filesApi.moveCopy(items, "move");
        mutations.closeHovers();
        if (!this.isListing) {
          this.$router.push({ path: newLink });
          return;
        }
      } catch (error) {
        notify.showError(error);
      }
    },
  },
};
</script>

<style scoped>
.invalid-form {
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
