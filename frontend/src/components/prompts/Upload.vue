<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ $t("prompts.upload") }}</h2>
    </div>

    <div class="card-content">
      <p>{{ $t("prompts.uploadMessage") }}</p>
    </div>

    <div class="card-action full">
      <div
        @click="triggerFilePicker"
        @keypress.enter="triggerFilePicker"
        class="action"
        id="focus-prompt"
        tabindex="1"
      >
        <i class="material-icons">insert_drive_file</i>
        <div class="title">{{ $t("buttons.file") }}</div>
      </div>
      <div
        @click="triggerFolderPicker"
        @keypress.enter="triggerFolderPicker"
        class="action"
        tabindex="2"
      >
        <i class="material-icons">folder</i>
        <div class="title">{{ $t("buttons.folder") }}</div>
      </div>
      <input
        ref="fileInput"
        @change="onFilePicked"
        type="file"
        multiple
        style="display: none"
      />
      <input
        ref="folderInput"
        @change="onFolderPicked"
        type="file"
        webkitdirectory
        directory
        multiple
        style="display: none"
      />
    </div>
  </div>
</template>

<script>
import { ref } from "vue";
import * as upload from "@/utils/upload";
import { mutations, state, getters } from "@/store";

export default {
  name: "UploadFiles",
  setup() {
    const fileInput = ref(null);
    const folderInput = ref(null);

    const triggerFilePicker = () => {
      if (fileInput.value) fileInput.value.click();
    };

    const triggerFolderPicker = () => {
      if (folderInput.value) folderInput.value.click();
    };

    const onFilePicked = (event) => {
      handleFiles(event, false);
    };

    const onFolderPicked = (event) => {
      handleFiles(event, true);
    };

    const handleFiles = async (event) => {
      mutations.closeHovers();
      const rawFiles = event.target.files;
      if (!rawFiles || rawFiles.length === 0) return;

      const uploadFiles = [];

      for (let i = 0; i < rawFiles.length; i++) {
        const file = rawFiles[i];
        const fullPath = file.webkitRelativePath || file.name;

        uploadFiles.push({
          file,
          name: file.name,
          path: fullPath,
          fullPath: fullPath,
          source: state.req.source,
          size: file.size,
        });
      }

      const path = getters.routePath();
      const conflict = upload.checkConflict(uploadFiles, state.req.items);

      const doUpload = async () => {
        mutations.closeHovers();
        await upload.handleFiles(uploadFiles, path, true);
        mutations.setReload(true);
      };

      if (conflict) {
        mutations.showHover({
          name: "replace",
          confirm: async (e) => {
            e.preventDefault();
            await doUpload();
          },
        });
      } else {
        await doUpload();
      }
    };

    return {
      triggerFilePicker,
      triggerFolderPicker,
      onFilePicked,
      onFolderPicked,
      fileInput,
      folderInput,
    };
  },
};
</script>
