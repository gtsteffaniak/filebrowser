<template>
  <div class="card-content">
    <p>{{ $t("prompts.renameMessage") }}</p>
    <input
      ref="nameInput"
      v-focus
      v-model.trim="name"
      class="input"
      type="text"
      aria-label="New Name"
      @keyup.enter="submit"
    />
  </div>
  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--blue"
      @click="(event) => currentPrompt.confirm(event, null)"
      :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')"
    >
      {{ $t("general.cancel") }}
    </button>
    <button
      type="button"
      id="focus-prompt"
      class="button button--flat button--red"
      :disabled="!name"
      @click="submit"
      :aria-label="$t('general.rename')"
      :title="$t('general.rename')"
    >
      {{ $t("general.rename") }}
    </button>
  </div>
</template>

<script>
import { getters } from "@/store";

/**
 * Asks for the name the folder being uploaded should take when a folder with the
 * same name is already there. Nothing on the server is touched here: the answer
 * only renames the incoming upload, so the existing folder stays as it is.
 */
export default {
  name: "upload-rename",
  props: {
    promptId: {
      type: [String, Number],
      default: null,
    },
    initialName: {
      type: String,
      required: true,
    },
  },
  data() {
    return { name: this.initialName };
  },
  computed: {
    currentPrompt() {
      return getters.currentPrompt();
    },
  },
  methods: {
    submit() {
      if (!this.name) {
        return;
      }
      this.currentPrompt.confirm(undefined, this.name);
    },
  },
};
</script>
