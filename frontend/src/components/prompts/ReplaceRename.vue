<template>
  <div class="card-content">
    <p v-if="conflictCount > 0" class="conflict-target">
      <strong>{{ conflictNames.join(", ") }}</strong>
      <span v-if="conflictCount > conflictNames.length">+{{ conflictCount - conflictNames.length }}<!--eslint-disable-line @intlify/vue-i18n/no-raw-text--></span>
    </p>
    <p v-if="allowSkip">{{ $t("prompts.uploadConflictMessage") }}</p>
    <p v-else>{{ $t("prompts.replaceMessage") }}</p>
  </div>
  <div class="card-actions">
    <button
      v-if="canCancel"
      type="button"
      class="button button--flat button--blue"
      @click="cancelPrompt"
      :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')"
    >
      {{ $t("general.cancel") }}
    </button>
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
import { getters, mutations, state } from "@/store";

export default {
  name: "replace-rename",
  props: {
    promptId: {
      type: [String, Number],
      default: null,
    },
    // Only the upload flow names the items that are in the way.
    conflictNames: {
      type: Array,
      default: () => [],
    },
    conflictCount: {
      type: Number,
      default: 0,
    },
  },
  computed: {
    canCancel() {
      // Other callers settle their own promise only from confirm: a Cancel button
      // there would close the dialog and leave them waiting forever.
      return typeof this.currentPrompt?.cancel === "function";
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
  methods: {
    // Cancel means "close and do nothing" for every caller of this dialog, not
    // an option their confirm handlers have to know about.
    cancelPrompt() {
      const prompt = this.currentPrompt;
      mutations.closeTopPrompt(undefined, { skipCancel: true });
      prompt?.cancel?.();
    },
  },
};
</script>
