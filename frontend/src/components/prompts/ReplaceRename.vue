<template>
  <div class="card-content">
    <p>{{ $t("prompts.replaceMessage") }}</p>
  </div>
  <div class="card-actions">
    <button class="button button--flat button--blue" @click="(event) => currentPrompt.confirm(event, 'rename')"
      :aria-label="$t('general.rename')" :title="$t('general.rename')" tabindex="2">
      {{ $t("general.rename") }}
    </button>
    <button id="focus-prompt" class="button button--flat button--red"
      :disabled="isSameFile"
      @click="(event) => currentPrompt.confirm(event, 'overwrite')" :aria-label="$t('general.replace')"
      :title="$t('general.replace')" tabindex="1">
      {{ $t("general.replace") }}
    </button>
  </div>
</template>

<script>
import { getters, mutations } from "@/store"; // Import your custom store

export default {
  name: "replace-rename",
  computed: {
    currentPrompt() {
      return getters.currentPrompt(); // Access the getter directly from the store
    },
    isSameFile() {
      // Check if the current prompt has props indicating same file
      return this.currentPrompt?.props?.isSameFile === true;
    },
  },
  methods: {},
};
</script>
