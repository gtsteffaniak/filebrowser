<template>
  <div class="card-title">
    <h2>{{ $t("prompts.replace") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("prompts.replaceMessage") }}</p>
  </div>

  <div class="card-action">
    <button class="button button--flat button--grey" @click="closeTopHover" :aria-label="$t('buttons.cancel')"
      :title="$t('buttons.cancel')">
      {{ $t("buttons.cancel") }}
    </button>
    <button class="button button--flat button--red" @click="currentPrompt.confirm" :aria-label="$t('buttons.replace')"
      :title="$t('buttons.replace')">
      {{ $t("buttons.replace") }}
    </button>
  </div>
</template>
<script>
import { getters, mutations } from "@/store"; // Import your custom store

export default {
  name: "replace",
  computed: {
    currentPrompt() {
      return getters.currentPrompt(); // Access the getter directly from the store
    },
  },
  methods: {
    closeTopHover() {
      mutations.closeTopHover();
    },
  },
};
</script>
