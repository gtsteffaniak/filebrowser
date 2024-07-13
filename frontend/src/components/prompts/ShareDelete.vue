<template>
  <div class="card floating">
    <div class="card-content">
      <p>{{ $t("prompts.deleteMessageShare", { path: "" }) }}</p>
    </div>
    <div class="card-action">
      <button
        @click="$store.commit('closeHovers')"
        class="button button--flat button--grey"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button
        @click="submit"
        class="button button--flat button--red"
        :aria-label="$t('buttons.delete')"
        :title="$t('buttons.delete')"
      >
        {{ $t("buttons.delete") }}
      </button>
    </div>
  </div>
</template>
<script>
import { getters } from "@/store"; // Import your custom store

export default {
  name: "share-delete",
  computed: {
    currentPrompt() {
      return getters.currentPrompt(); // Access getter directly from the store
    },
  },
  methods: {
    submit() {
      this.currentPrompt?.confirm();
    },
  },
};
</script>
