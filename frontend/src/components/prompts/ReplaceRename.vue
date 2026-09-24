<template>
  <div class="card-content">
    <template v-if="allowSkip">
      <p>{{ $t("prompts.uploadConflictMessage") }}</p>
      <p>
        <strong>{{ $t("prompts.optionLabel", { name: $t("prompts.uploadMissing") }) }}</strong>
        {{ $t("prompts.uploadMissingInfo") }}
      </p>
      <p v-if="canReplace">
        <strong>{{ $t("prompts.optionLabel", { name: $t("general.replace") }) }}</strong>
        {{ $t("prompts.uploadReplaceInfo") }}
      </p>
      <p v-if="allowRename">
        <strong>{{ $t("prompts.optionLabel", { name: $t("general.rename") }) }}</strong>
        {{ $t("prompts.uploadRenameInfo") }}
      </p>
    </template>
    <p v-else>{{ $t("prompts.replaceMessage") }}</p>
  </div>
  <div class="card-actions">
    <button
      v-if="allowRename"
      type="button"
      class="button button--flat button--blue"
      :id="canReplace ? undefined : 'focus-prompt'"
      @click="(event) => currentPrompt.confirm(event, 'rename')"
      :aria-label="$t('general.rename')"
      :title="$t('general.rename')"
    >
      {{ $t("general.rename") }}
    </button>
    <button
      v-if="allowSkip"
      type="button"
      class="button button--flat button--blue"
      @click="(event) => currentPrompt.confirm(event, 'skip')"
      :aria-label="$t('prompts.uploadMissing')"
      :title="$t('prompts.uploadMissing')"
    >
      {{ $t("prompts.uploadMissing") }}
    </button>
    <button
      v-if="canReplace"
      type="button"
      id="focus-prompt"
      class="button button--flat button--red"
      :disabled="isSameFile"
      @click="(event) => currentPrompt.confirm(event, 'overwrite')"
      :aria-label="$t('general.replace')"
      :title="$t('general.replace')">
      {{ $t("general.replace") }}
    </button>
  </div>
</template>

<script>
import { getters, state } from "@/store";

export default {
  name: "replace-rename",
  computed: {
    currentPrompt() {
      return getters.currentPrompt();
    },
    allowRename() {
      return this.currentPrompt.props?.allowRename !== false;
    },
    allowSkip() {
      return this.currentPrompt.props?.allowSkip === true;
    },
    isSameFile() {
      return this.currentPrompt.props?.isSameFile === true;
    },
    canReplace() {
      const canModify = !!getters.sourcePermissions().modify;
      if (getters.isShare()) {
        return canModify && !!state.shareInfo?.allowReplacements;
      }
      return canModify;
    },
  },
  methods: {},
};
</script>
