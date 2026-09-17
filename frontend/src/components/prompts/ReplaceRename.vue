<template>
  <div class="card-content">
    <p>{{ $t("prompts.replaceMessage") }}</p>
  </div>
  <div class="card-actions">
    <button
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
