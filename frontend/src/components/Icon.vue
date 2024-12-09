<template>
    <i v-if="isMaterialIcon" :class="classes" class="material-icons">{{ materialIcon }} </i>
    <i v-else :class="classes"></i>
  </template>
  
  <script>
  import { getIconForType } from "@/utils/mimetype-filetypes";
  
  export default {
    name: "Icon",
    props: {
      mimetype: {
        type: String,
        required: true,
      },
    },
    data() {
      return {
        classes: "",
        materialIcon: "",
      };
    },
    computed: {
      isMaterialIcon() {
        return this.materialIcon !== "";
      },
    },
    watch: {
      mimetype: {
        immediate: true,
        handler(newMimetype) {
          const result = getIconForType(newMimetype);
          this.classes = result.classes;
          this.materialIcon = result.materialIcon;
        },
      },
    },
  };
  </script>
  
  <style scoped>
  /* Add any custom styling for icons here */
  </style>
  