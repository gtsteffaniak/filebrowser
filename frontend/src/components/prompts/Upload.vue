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
        @click="uploadFile"
        @keypress.enter="uploadFile"
        class="action"
        id="focus-prompt"
        tabindex="1"
      >
        <i class="material-icons">insert_drive_file</i>
        <div class="title">{{ $t("buttons.file") }}</div>
      </div>
      <div
        @click="uploadFolder"
        @keypress.enter="uploadFolder"
        class="action"
        tabindex="2"
      >
        <i class="material-icons">folder</i>
        <div class="title">{{ $t("buttons.folder") }}</div>
      </div>
      <input ref="fileInput" @change="onFilePicked" type="file" style="display: none" />
      <input
        ref="folderInput"
        @change="onFolderPicked"
        type="file"
        webkitdirectory
        directory
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
      fileInput.value.click();
    };

    const triggerFolderPicker = () => {
      folderInput.value.click();
    };

    const onFilePicked = (event) => {
      handleFiles(event);
    };

    const onFolderPicked = (event) => {
      handleFiles(event);
    };

    const handleFiles = async (event) => {
      mutations.closeHovers();
      const files = event.target.files;
      if (!files) return;

      const folderUpload = !!files[0].webkitRelativePath;
      const uploadFiles = [];

      for (let i = 0; i < files.length; i++) {
        const file = files[i];
        const fullPath = folderUpload ? file.webkitRelativePath : undefined;
        uploadFiles.push({
          file, // File object directly
          name: file.name,
          size: file.size,
          isDir: false,
          fullPath,
        });
      }

      const path = getters.routePath();
      const conflict = upload.checkConflict(uploadFiles, state.req.items);

      if (conflict) {
        mutations.showHover({
          name: "replace",
          action: async (event) => {
            event.preventDefault();
            mutations.closeHovers();
            await upload.handleFiles(uploadFiles, path, false);
          },
          confirm: async (event) => {
            event.preventDefault();
            mutations.closeHovers();
            await upload.handleFiles(uploadFiles, path, true);
          },
        });
      } else {
        await upload.handleFiles(uploadFiles, path, true);
      }
      mutations.setReload(true);
    };

    const openUpload = (isFolder) => {
      const input = document.createElement("input");
      input.type = "file";
      input.multiple = true;
      input.webkitdirectory = isFolder;
      input.addEventListener("change", handleFiles);
      input.click();
    };

    const uploadFile = () => {
      openUpload(false);
    };

    const uploadFolder = () => {
      openUpload(true);
    };

    return {
      triggerFilePicker,
      triggerFolderPicker,
      uploadFile,
      uploadFolder,
      onFilePicked,
      onFolderPicked,
    };
  },
};
</script>
