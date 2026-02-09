<template>
  <div class="card-content">
    <p>{{ $t("prompts.deleteMessageShare", { path }) }}</p>
  </div>
  <div class="card-actions">
    <button @click="closeHovers" class="button button--flat button--grey" :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button @click="submit" class="button button--flat button--red" :aria-label="$t('general.delete')"
      :title="$t('general.delete')">
      {{ $t("general.delete") }}
    </button>
  </div>
</template>
<script>
import { getters, mutations } from "@/store"; // Import your custom store

export default {
  name: "share-delete",
  props: {
    path: String,
  },
  computed: {
    closeHovers() {
      return mutations.closeHovers();
    },
    currentPrompt() {
      return getters.currentPrompt();
    },
  },
  methods: {
    submit() {
      this.currentPrompt?.confirm();
    },
  },
};
</script>
