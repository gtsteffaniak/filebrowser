<template>
  <div class="card-content">
    <file-list ref="fileList" @update:selected="updateSelection" :browseSource="currentSource"
      :browse-path="initialBrowsePath" :hide-destination-source="hideDestinationSource" :showFiles="showFiles"
      :showFolders="showFolders">
    </file-list>
  </div>

  <div class="card-actions">
    <button class="button button--flat button--grey" type="button" @click="onCancel" :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button type="button" class="button button--flat" @click="confirmSelection" :aria-label="$t('general.select')"
      :title="$t('general.select')">
      {{ $t("general.select") }}
    </button>
  </div>
</template>

<script>
import { mutations } from "@/store";
import FileList from "../files/FileList.vue";
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
    showFiles: {
      type: Boolean,
      default: false,
    },
    showFolders: {
      type: Boolean,
      default: true,
    },
    hideDestinationSource: {
      type: Boolean,
      default: false,
    },
    /** When set, included on pathSelected so listeners can tell this pick apart from other pickers. */
    selectionContextId: {
      type: String,
      default: null,
    },
  },
  data() {
    return {
      selectedPath: "/",
      selectedSource: "",
      /** True after confirm or explicit cancel — used to avoid duplicate cancel events. */
      selectionFinished: false,
    };
  },
  computed: {
    initialBrowsePath() {
      const p = this.currentPath;
      if (p && typeof p === "string" && p.length > 0) {
        return p;
      }
      return "/";
    },
  },
  mounted() {
    // Initialize with current values
    this.selectedPath = this.currentPath || "/";
    this.selectedSource = this.currentSource || "";
  },
  beforeUnmount() {
    if (
      this.selectionContextId &&
      !this.selectionFinished
    ) {
      eventBus.emit("pathPickerCancelled", {
        selectionContextId: this.selectionContextId,
      });
    }
  },
  methods: {
    onCancel() {
      this.selectionFinished = true;
      if (this.selectionContextId) {
        eventBus.emit("pathPickerCancelled", {
          selectionContextId: this.selectionContextId,
        });
      }
      mutations.closeTopPrompt();
    },
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
      this.selectionFinished = true;
      const payload = {
        path: this.selectedPath,
        source: this.selectedSource,
      };
      if (this.selectionContextId) {
        payload.selectionContextId = this.selectionContextId;
      }
      eventBus.emit("pathSelected", payload);
      mutations.closeTopPrompt();
    },
  },
};
</script>

<style scoped>
.card-content {
  min-height: 300px;
}
</style>

