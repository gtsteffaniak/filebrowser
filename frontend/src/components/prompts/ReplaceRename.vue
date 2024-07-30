<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ $t("prompts.replace") }}</h2>
    </div>

    <div class="card-content">
      <p>{{ $t("prompts.replaceMessage") }}</p>
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="(event) => currentPrompt.confirm(event, 'rename')"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button
        class="button button--flat button--blue"
        @click="(event) => currentPrompt.confirm(event, 'overwrite')"
        :aria-label="$t('buttons.rename')"
        :title="$t('buttons.rename')"
      >
        {{ $t("buttons.rename") }}
      </button>
      <button
        class="button button--flat button--red"
        @click="(event) => showConfirm(event, 'overwrite')"
        :aria-label="$t('buttons.replace')"
        :title="$t('buttons.replace')"
      >
        {{ $t("buttons.replace") }}
      </button>
    </div>
  </div>
</template>

<script>
import { getters } from "@/store"; // Import your custom store

export default {
  name: "replace-rename",
  computed: {
    currentPrompt() {
      return getters.currentPrompt(); // Access the getter directly from the store
    },
  },
};
</script>
