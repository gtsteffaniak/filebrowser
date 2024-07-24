<template>
  <div
    v-if="filesInUploadCount > 0"
    class="upload-files"
    v-bind:class="{ closed: !open }"
  >
    <div class="card floating">
      <div class="card-title">
        <h2>{{ $t("prompts.uploadFiles", { files: filesInUploadCount }) }}</h2>

        <button
          class="action"
          @click="toggle"
          aria-label="Toggle file upload list"
          title="Toggle file upload list"
        >
          <i class="material-icons">{{
            open ? "keyboard_arrow_down" : "keyboard_arrow_up"
          }}</i>
        </button>
      </div>

      <div class="card-content file-icons">
        <div
          class="file"
          v-for="file in filesInUpload"
          :key="file.id"
          :data-dir="file.isDir"
          :data-type="file.type"
          :aria-label="file.name"
        >
          <div class="file-name"><i class="material-icons"></i> {{ file.name }}</div>
          <div class="file-progress">
            <div v-bind:style="{ width: file.progress + '%' }"></div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
<script>
import { getters } from "@/store"; // Import your custom store

export default {
  name: "uploadFiles",
  data() {
    return {
      open: false,
    };
  },
  computed: {
    filesInUpload() {
      return getters.filesInUpload(); // Access the getter directly from the store
    },
    filesInUploadCount() {
      return getters.filesInUploadCount(); // Access the getter directly from the store
    },
  },
  methods: {
    toggle() {
      this.open = !this.open;
    },
  },
};
</script>
