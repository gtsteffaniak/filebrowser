<template>
  <FileList
    :fileList="fileList"
    :title="title"
    :showFiles="showFiles"
    :showFolders="showFolders"
  />
</template>

<script>
import FileList from "@/components/files/FileList.vue";

export default {
  name: "file-list",
  components: { FileList },
  props: {
    fileList: {
      type: Array,
      required: false
    },
    title: {
      type: String,
      default: null
    },
    showFiles: {
      type: Boolean,
      default: true
    },
    showFolders: {
      type: Boolean,
      default: false
    },
    // It should accept any other prop too without specify it here with the defaults from the component
  },
};
</script>