<template>
  <div class="card floating" id="download">
    <div class="card-title">
      <h2>{{ $t("prompts.download") }}</h2>
    </div>
    <div class="card-content">
      <p>{{ $t("prompts.downloadMessage") }}</p>

      <button
        v-for="(ext, format) in formats"
        :key="format"
        class="button button--block"
        @click="currentPrompt.confirm(format)"
        v-focus
      >
        {{ ext }}
      </button>
    </div>
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
        tar: "tar",
        targz: "tar.gz",
        tarbz2: "tar.bz2",
        tarxz: "tar.xz",
        tarlz4: "tar.lz4",
        tarsz: "tar.sz",
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
