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
        @click="currentPrompt.action"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button
        class="button button--flat button--red"
        @click="currentPrompt.confirm"
        :aria-label="$t('buttons.replace')"
        :title="$t('buttons.replace')"
      >
        {{ $t("buttons.replace") }}
      </button>
    </div>
  </div>
</template>
<script>
import { state, getters } from "@/store"; // Import your custom store

export default {
  name: "replace",
  computed: {
    currentPrompt() {
      return getters.currentPrompt(); // Access the getter directly from the store
    },
  },
};
</script>
