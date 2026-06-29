<template>
  <div v-if="!hasTransfers" class="card-content lonely-message">
    <span>{{ $t("prompts.noActiveTransfers") }}</span>
  </div>
  <div v-if="hasTransfers" class="card-content">
    <div class="transfer-list">
      <div
        v-for="transfer in transfers"
        :key="transfer.id"
        class="transfer-item-wrapper"
        :class="{ 'has-error': transfer.status === 'failed' }"
      >
        <div class="transfer-item">
          <i class="material-symbols file-icon">{{
            transfer.action === "move" ? "drive_file_move" : "file_copy"
          }}</i>
          <div class="file-info">
            <p class="file-name">
              {{ getTransferName(transfer) }}
              <span v-if="transfer.status === 'calculating'" class="status-badge calculating">
                {{ $t("prompts.calculating") }}
              </span>
            </p>
            <progress-bar
              :val="progressVal(transfer)"
              :unit="progressUnit(transfer)"
              :max="transfer.totalBytes || 100"
              :status="progressStatus(transfer)"
              text-position="inside"
              size="20"
            />
            <p v-if="transfer.currentFile && transfer.status === 'running'" class="current-file">
              {{ transfer.currentFile }}
              <span v-if="transfer.itemsTotal > 1">
                ({{ transfer.itemsCompleted }}/{{ transfer.itemsTotal }})
              </span>
            </p>
          </div>
          <div class="file-actions">
            <button
              type="button"
              v-if="isActive(transfer)"
              @click="cancelTransfer(transfer.id)"
              class="action"
              :aria-label="$t('general.cancel')"
              :title="$t('general.cancel')"
            >
              <i class="material-symbols">close</i>
            </button>
            <button
              type="button"
              v-else
              @click="removeTransfer(transfer.id)"
              class="action"
              :aria-label="$t('general.close')"
              :title="$t('general.close')"
            >
              <i class="material-symbols">close</i>
            </button>
          </div>
        </div>
        <div v-if="transfer.status === 'failed'" class="error-banner" role="alert">
          {{ transfer.error || $t("prompts.transferFailed") }}
        </div>
      </div>
    </div>
  </div>
  <div class="card-actions">
    <div v-if="hasTransfers" class="spacer"></div>
    <button
      type="button"
      v-if="hasTransfers && hasClearable"
      @click="clearCompleted"
      class="button button--flat"
      :aria-label="$t('buttons.clearCompleted')"
      :title="$t('buttons.clearCompleted')"
    >
      {{ $t("buttons.clearCompleted") }}
    </button>
  </div>
</template>

<script>
import { transferManager } from "@/utils/transferManager";
import ProgressBar from "@/components/ProgressBar.vue";

export default {
  name: "transfer",
  components: {
    ProgressBar,
  },
  computed: {
    transfers() {
      return transferManager.queue || [];
    },
    hasTransfers() {
      return (transferManager.queue.length || 0) > 0;
    },
    hasClearable() {
      if (!transferManager.queue) return false;
      return transferManager.queue.some(
        (t) =>
          t.status === "completed" ||
          t.status === "failed" ||
          t.status === "cancelled"
      );
    },
  },
  methods: {
    progressVal(transfer) {
      if (transfer.status === "completed") return this.$t("prompts.completed");
      if (transfer.status === "failed") return this.$t("prompts.error");
      if (transfer.status === "cancelled") return this.$t("prompts.cancelled");
      if (transfer.status === "pending" || transfer.status === "calculating") {
        return this.$t("prompts.calculating");
      }
      return transfer.copiedBytes;
    },
    progressUnit(transfer) {
      if (
        transfer.status === "completed" ||
        transfer.status === "failed" ||
        transfer.status === "cancelled" ||
        transfer.status === "pending" ||
        transfer.status === "calculating"
      ) {
        return "";
      }
      return "bytes";
    },
    progressStatus(transfer) {
      if (transfer.status === "failed") return "error";
      if (transfer.status === "pending" || transfer.status === "calculating") return "indexing";
      return "default";
    },
    getTransferName(transfer) {
      if (transfer.items && transfer.items.length === 1) {
        return transfer.items[0].name || transfer.items[0].from?.split("/").pop() || "Transfer";
      }
      if (transfer.items && transfer.items.length > 1) {
        return `${transfer.items.length} items`;
      }
      return "Transfer";
    },
    isActive(transfer) {
      return (
        transfer.status === "pending" ||
        transfer.status === "calculating" ||
        transfer.status === "running"
      );
    },
    cancelTransfer(id) {
      transferManager.cancel(id);
    },
    removeTransfer(id) {
      transferManager.remove(id);
    },
    clearCompleted() {
      transferManager.clearCompleted();
    },
  },
};
</script>

<style scoped>
.transfer-list {
  overflow-y: auto;
  padding-right: 0.5em;
  flex-grow: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.transfer-item-wrapper {
  margin-bottom: 0.5rem;
}

.transfer-item-wrapper:last-child {
  margin-bottom: 0;
}

.transfer-item-wrapper.has-error {
  border-left: 3px solid var(--errorColor, #f44336);
  padding-left: 0.5rem;
}

.error-banner {
  margin-top: 0.5rem;
  padding: 0.5rem;
  background: var(--errorBackground, rgba(244, 67, 54, 0.1));
  color: var(--errorColor, #f44336);
  border-radius: 4px;
  font-size: 0.875rem;
  word-break: break-word;
}

.transfer-item {
  display: flex;
  align-items: center;
  padding: 0.5em 0;
}

.file-icon {
  margin-right: 0.5em;
  color: #999;
}

.file-info {
  flex-grow: 1;
}

.file-name {
  margin: 0;
  font-size: 0.9em;
}

.current-file {
  margin: 0.2em 0 0 0;
  font-size: 0.75em;
  color: #999;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-badge {
  font-size: 0.75em;
  padding: 0.1em 0.4em;
  border-radius: 3px;
  margin-left: 0.5em;
}

.status-badge.calculating {
  background: #fff8d6;
  color: #9e7600;
}

.file-actions .action {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0.2em;
}

.file-actions .action i {
  font-size: 1.2em;
}

.spacer {
  flex-grow: 1;
}

.lonely-message {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100px;
  text-align: center;
}
</style>
