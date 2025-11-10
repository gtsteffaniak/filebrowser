<template>
    <div class="card-title">
      <h2>{{ $t("prompts.download") }}</h2>
    </div>
    <div class="card-content">
      <p>{{ $t("prompts.downloadMessage") }}</p>

      <button
        v-for="(ext, format) in formats"
        :key="format"
        class="button button--block"
        :aria-label="`Download as ${format}`"
        @click="currentPrompt.confirm(format)"
        v-focus
      >
        {{ ext }}
    </button>
  </div>
</template>

<script>
import { getters } from "@/store"; // Import your custom store

export default {
  name: "download",
  data: function () {
    return {
      formats: {
        zip: "zip",
        targz: "tar.gz",
      },
    };
  },
  computed: {
    currentPrompt() {
      return getters.currentPrompt();
    },
  },
};
</script>
