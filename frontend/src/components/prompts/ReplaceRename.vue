<template>
  <div class="card-content">
    <p v-if="conflictDetails.count > 0" class="conflict-target">
      <strong>{{ conflictDetails.names.join(", ") }}</strong>
      <span v-if="conflictDetails.count > conflictDetails.names.length">+{{ conflictDetails.count - conflictDetails.names.length }}<!--eslint-disable-line @intlify/vue-i18n/no-raw-text--></span>
    </p>
    <p v-if="allowSkip">{{ $t("prompts.uploadConflictMessage") }}</p>
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
    <button
      type="button"
      class="button button--flat button--blue"
      @click="(event) => currentPrompt.confirm(event, 'cancel')"
      :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')"
    >
      {{ $t("general.cancel") }}
    </button>
  </div>
</template>

<script>
import { getters, state } from "@/store";
import { uploadManager } from "@/utils/upload";

export default {
  name: "replace-rename",
  props: {
    promptId: {
      type: [String, Number],
      default: null,
    },
  },
  computed: {
    conflictDetails() {
      return uploadManager.getConflictDetails();
    },
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
      // Replacing changes the destination, which for copy/move is not the source being browsed
      const canModify = !!getters.sourcePermissions(this.currentPrompt.props?.destSource).modify;
      if (getters.isShare()) {
        return canModify && !!state.shareInfo?.allowReplacements;
      }
      return canModify;
    },
  },
  methods: {},
};
</script>
