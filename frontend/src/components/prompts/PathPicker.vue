<template>
  <div class="card-title">
    <h2>{{ $t("prompts.selectPath") }}</h2>
  </div>

  <div class="card-content">
    <file-list ref="fileList" @update:selected="updateSelection" :browseSource="currentSource">
    </file-list>
  </div>

  <div class="card-action">
    <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button class="button button--flat" @click="confirmSelection" :aria-label="$t('general.select')"
      :title="$t('general.select')">
      {{ $t("general.select") }}
    </button>
  </div>
</template>

<script>
import { mutations } from "@/store";
import FileList from "./FileList.vue";
import { eventBus } from "@/store/eventBus";

export default {
  name: "path-picker",
  components: { FileList },
  props: {
    currentPath: {
      type: String,
      default: "/",
    },
    currentSource: {
      type: String,
      default: "",
    },
  },
  data() {
    return {
      selectedPath: "/",
      selectedSource: "",
    };
  },
  mounted() {
    // Initialize with current values
    this.selectedPath = this.currentPath || "/";
    this.selectedSource = this.currentSource || "";
  },
  computed: {
    closeHovers() {
      return mutations.closeHovers();
    },
  },
  methods: {
    updateSelection(pathOrData) {
      // Handle both old format (just path) and new format (object with path and source)
      if (typeof pathOrData === 'string') {
        this.selectedPath = pathOrData;
      } else if (pathOrData && pathOrData.path) {
        this.selectedPath = pathOrData.path;
        this.selectedSource = pathOrData.source;
      }
    },
    confirmSelection() {
      // Emit the selected path and source via eventBus
      eventBus.emit('pathSelected', {
        path: this.selectedPath,
        source: this.selectedSource
      });
      // Close the modal
      mutations.closeHovers();
    },
  },
};
</script>

<style scoped>
.card-content {
  min-height: 300px;
}
</style>

