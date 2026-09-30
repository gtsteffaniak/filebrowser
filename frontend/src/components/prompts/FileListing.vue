<template>
  <FileList
    v-bind="$attrs"
    :fileList="fileList"
    :title="title"
    :showFiles="showFiles"
    :showFolders="showFolders"
    :filterQuery="filterQuery"
  />
  <div class="card-actions">
    <ListingFilter v-if="filterable" v-model="filterQuery" />
    <button
      type="button"
      class="button button--flat"
      @click="close"
      :aria-label="$t('general.ok')"
      :title="$t('general.ok')"
    >
      {{ $t("general.ok") }}
    </button>
  </div>
</template>

<script>
import FileList from "@/components/files/FileList.vue";
import ListingFilter from "@/components/files/ListingFilter.vue";
import { mutations } from "@/store";

export default {
  name: "file-list",
  components: { FileList, ListingFilter },
  inheritAttrs: false,
  props: {
    promptId: {
      type: [String, Number],
      default: null,
    },
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
    filterable: {
      type: Boolean,
      default: false
    },
  },
  data() {
    return {
      filterQuery: "",
    };
  },
  methods: {
    close() {
      mutations.closeTopPrompt(this.promptId ?? undefined);
    },
  },
};
</script>
