<template>
  <div class="card-title">
    <h2>{{ $t("prompts.saveBeforeExit") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("prompts.saveBeforeExitMessage") }}</p>
  </div>

  <div class="card-action">
    <button
      class="button button--flat button--grey"
      @click="keepEditing"
      :aria-label="$t('buttons.keepEditing')"
      :title="$t('buttons.keepEditing')">
      {{ $t("buttons.keepEditing") }}
    </button>
    <button
      class="button button--flat button--blue"
      @click="discardAndExit"
      :aria-label="$t('buttons.discardAndExit')"
      :title="$t('buttons.discardAndExit')">
      {{ $t("buttons.discardAndExit") }}
    </button>
    <button
      class="button button--flat"
      @click="saveAndExit"
      :aria-label="$t('buttons.saveAndExit')"
      :title="$t('buttons.saveAndExit')">
      {{ $t("buttons.saveAndExit") }}
    </button>
  </div>
</template>

<script>
import { getters, mutations } from "@/store";

export default {
  name: "saveBeforeExit",
  computed: {
    currentPrompt() {
      return getters.currentPrompt();
    },
  },
  created() {
    // Listen for ESC key to treat it as "Keep Editing"
    this.escKeyHandler = (event) => {
      if (event.keyCode === 27 && getters.currentPromptName() === "SaveBeforeExit") {
        event.stopImmediatePropagation();
        this.keepEditing();
      }
    };
    window.addEventListener("keydown", this.escKeyHandler, true);
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.escKeyHandler, true);
  },
  methods: {
    keepEditing() {
      if (this.currentPrompt && this.currentPrompt.cancel) {
        this.currentPrompt.cancel();
      }
      mutations.closeTopHover();
    },
    async saveAndExit() {
      if (this.currentPrompt && this.currentPrompt.confirm) {
        try {
          await this.currentPrompt.confirm();
          // Only close prompt if save succeeded
          mutations.closeTopHover();
        } catch (error) {
          // If save fails, keep the prompt open so user can try again or choose another option
          console.error("Save failed:", error);
          // The error notification is already shown by the save handler
        }
      }
    },
    discardAndExit() {
      if (this.currentPrompt && this.currentPrompt.discard) {
        this.currentPrompt.discard();
      }
      mutations.closeTopHover();
    },
  },
};
</script>

