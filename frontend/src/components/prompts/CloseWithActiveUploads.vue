<template>
  <div class="card-title">
    <h2>{{ $t("prompts.closeWithActiveUploads") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("prompts.closeWithActiveUploadsMessage") }}</p>
  </div>

  <div class="card-action">
    <button
      class="button button--flat button--grey"
      @click="stayOnPrompt"
      :aria-label="$t('buttons.stayOnPrompt')"
      :title="$t('buttons.stayOnPrompt')">
      {{ $t("buttons.stayOnPrompt") }}
    </button>
    <button
      class="button button--flat button--red"
      @click="closeAnyway"
      :aria-label="$t('buttons.closeAnyway')"
      :title="$t('buttons.closeAnyway')">
      {{ $t("buttons.closeAnyway") }}
    </button>
  </div>
</template>

<script>
import { getters, mutations } from "@/store";

export default {
  name: "closeWithActiveUploads",
  computed: {
    currentPrompt() {
      return getters.currentPrompt();
    },
  },
  created() {
    // Listen for ESC key to treat it as "Stay on Prompt"
    this.escKeyHandler = (event) => {
      if (event.keyCode === 27 && getters.currentPromptName() === "CloseWithActiveUploads") {
        event.stopImmediatePropagation();
        this.stayOnPrompt();
      }
    };
    window.addEventListener("keydown", this.escKeyHandler, true);
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.escKeyHandler, true);
  },
  methods: {
    stayOnPrompt() {
      if (this.currentPrompt && this.currentPrompt.cancel) {
        this.currentPrompt.cancel();
      }
      // Don't call closeTopHover here - the cancel callback handles it
    },
    closeAnyway() {
      if (this.currentPrompt && this.currentPrompt.confirm) {
        this.currentPrompt.confirm();
      }
      // Don't call closeTopHover here - the confirm callback handles it
    },
  },
};
</script>

