<template>
  <div class="card-content share-picker-content">
    <div v-if="loading" class="share-picker-loading">
      <i class="material-symbols spin">progress_activity</i>
    </div>
    <errors v-else-if="error" :errorCode="error.status" />
    <settings-table
      v-else
      :columns="columns"
      :items="shares"
      item-key="hash"
      default-sort-key="path"
      :aria-label="$t('tools.activityViewer.chooseShare')"
      row-clickable
      :lonely-message-key="shares.length === 0 ? 'tools.activityViewer.noShares' : undefined"
      @row-click="selectShare"
    >
      <template #cell-path="{ row }">
        <span class="share-path">{{ row.path }}</span>
        <span v-if="row.title" class="share-title">{{ row.title }}</span>
      </template>
      <template #cell-hash="{ row }">
        <code class="share-hash">{{ row.hash }}</code>
      </template>
      <template #cell-shareType="{ row }">
        {{ row.shareType || "normal" }}
      </template>
    </settings-table>
  </div>

  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--grey"
      @click="onCancel"
      :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')"
    >
      {{ $t("general.cancel") }}
    </button>
    <button
      type="button"
      class="button button--flat"
      :disabled="!selectedHash"
      @click="confirmSelection"
      :aria-label="$t('general.select')"
      :title="$t('general.select')"
    >
      {{ $t("general.select") }}
    </button>
  </div>
</template>

<script>
import { shareApi } from "@/api";
import SettingsTable from "@/components/settings/Table.vue";
import Errors from "@/views/Errors.vue";
import { mutations } from "@/store";
import { eventBus } from "@/store/eventBus";

export default {
  name: "share-picker",
  components: {
    SettingsTable,
    Errors,
  },
  props: {
    currentHash: {
      type: String,
      default: "",
    },
    selectionContextId: {
      type: String,
      default: null,
    },
  },
  data() {
    return {
      shares: [],
      loading: true,
      error: null,
      selectedHash: "",
      selectionFinished: false,
    };
  },
  computed: {
    columns() {
      return [
        { key: "path", label: this.$t("general.path"), sortable: true },
        { key: "hash", label: this.$t("general.hash"), sortable: true, narrow: true },
        { key: "shareType", label: this.$t("general.type"), sortable: true, narrow: true },
      ];
    },
  },
  async mounted() {
    this.selectedHash = this.currentHash || "";
    await this.loadShares();
  },
  beforeUnmount() {
    if (this.selectionContextId && !this.selectionFinished) {
      eventBus.emit("sharePickerCancelled", {
        selectionContextId: this.selectionContextId,
      });
    }
  },
  methods: {
    async loadShares() {
      this.loading = true;
      this.error = null;
      try {
        this.shares = await shareApi.list();
      } catch (e) {
        this.error = e;
        this.shares = [];
      } finally {
        this.loading = false;
      }
    },
    selectShare(row) {
      if (row?.hash) {
        this.selectedHash = row.hash;
      }
    },
    onCancel() {
      this.selectionFinished = true;
      if (this.selectionContextId) {
        eventBus.emit("sharePickerCancelled", {
          selectionContextId: this.selectionContextId,
        });
      }
      mutations.closeTopPrompt();
    },
    confirmSelection() {
      if (!this.selectedHash) {
        return;
      }
      this.selectionFinished = true;
      const share = this.shares.find((s) => s.hash === this.selectedHash);
      eventBus.emit("shareSelected", {
        selectionContextId: this.selectionContextId,
        hash: this.selectedHash,
        path: share?.path || "",
        title: share?.title || "",
      });
      mutations.closeTopPrompt();
    },
  },
};
</script>

<style scoped>
.share-picker-content {
  min-height: 12rem;
  max-height: min(70vh, 32rem);
  overflow: auto;
}

.share-picker-loading {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 10rem;
}

.share-path {
  display: block;
  font-weight: 500;
}

.share-title {
  display: block;
  font-size: 0.85em;
  color: var(--textSecondary);
  margin-top: 0.15rem;
}

.share-hash {
  font-size: 0.8em;
  word-break: break-all;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
