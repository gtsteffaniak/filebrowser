<template>
  <div v-if="!hasDownloads && currentPrompt && currentPrompt.confirm" class="card-content">
    <p>{{ $t("prompts.downloadMessage") }}</p>

    <button
      v-for="(ext, format) in formats"
      :key="format"
      class="button button--block"
      :aria-label="`Download as ${format}`"
      @click="handleFormatSelect(format)"
      v-focus
    >
      {{ ext }}
    </button>
  </div>
  <div v-if="!hasDownloads && (!currentPrompt || !currentPrompt.confirm)" class="card-content lonely-message">
    <span>{{ $t("files.lonely") }}</span>
  </div>
  <div v-if="hasDownloads" class="card-content">
    <div class="download-list">
      <div v-for="download in downloads" :key="download.id" class="download-item">
        <i class="material-icons file-icon">insert_drive_file</i> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        <div class="file-info">
          <p class="file-name">{{ download.name }}</p>
          <progress-bar
            :val="download.status === 'completed'
              ? $t('prompts.completed')
              : download.status === 'error'
                ? $t('prompts.error')
                : download.loaded
            "
            :unit="download.status === 'completed' || download.status === 'error' ? '' : 'bytes'"
            :max="download.size"
            :status="download.status"
            text-position="inside"
            size="20"
            :help-text="download.errorDetails || ''">
          </progress-bar>
        </div>
        <div class="file-actions">
          <button v-if="download.status === 'error'" @click="retryDownload(download.id)" class="action"
            :aria-label="$t('general.retry')" :title="$t('general.retry')">
            <i class="material-icons">replay</i>
          </button>
          <button @click="cancelDownload(download.id)" class="action" :aria-label="$t('general.cancel')"
            :title="$t('general.cancel')">
            <i class="material-icons">close</i>
          </button>
        </div>
      </div>
    </div>
  </div>
  <div class="card-actions">
    <div v-if="hasDownloads" class="spacer"></div>
    <button v-if="hasDownloads && hasClearable" @click="clearCompleted" class="button button--flat" :disabled="!hasClearable"
      :aria-label="$t('buttons.clearCompleted')" :title="$t('buttons.clearCompleted')">
      {{ $t("buttons.clearCompleted") }}
    </button>
  </div>
</template>

<script>
import { getters, mutations } from "@/store";
import { downloadManager } from "@/utils/downloadManager";
import { resourcesApi } from "@/api";
import ProgressBar from "@/components/ProgressBar.vue";

export default {
  name: "download",
  components: {
    ProgressBar,
  },
  data: function () {
    return {
      formats: {
        zip: "zip",
        targz: "tar.gz",
      },
    };
  },
  computed: {
    currentPrompt() {
      return getters.currentPrompt();
    },
    downloads() {
      return downloadManager?.queue || [];
    },
    hasDownloads() {
      return (downloadManager?.queue?.length || 0) > 0;
    },
    hasClearable() {
      if (!downloadManager?.queue) {
        return false;
      }
      return downloadManager.queue.some((download) => 
        download.status === "completed" || download.status === "error" || download.status === "cancelled"
      );
    },
  },
  methods: {
    handleFormatSelect(format) {
      if (this.currentPrompt && this.currentPrompt.confirm) {
        this.currentPrompt.confirm(format);
      }
    },
    cancelDownload(id) {
      if (downloadManager) {
        downloadManager.cancel(id);
      }
    },
    async retryDownload(id) {
      if (!downloadManager) return;
      const download = downloadManager.findById(id);
      if (download) {
        // Re-trigger download by calling the download function again
        downloadManager.remove(id);
        try {
          await resourcesApi.download(null, [download.file], download.shareHash);
        } catch (err) {
          console.error('Retry download failed:', err);
        }
      }
    },
    clearCompleted() {
      if (downloadManager) {
        downloadManager.clearCompleted();
      }
    },
    close() {
      // Only close if no active downloads
      if (!downloadManager || !downloadManager.hasActive()) {
        const prompt = getters.currentPrompt();
        if (prompt && prompt.name === 'download') {
          mutations.closeTopHover();
        } else {
          prompt?.cancel?.();
        }
      }
    },
  },
};
</script>

<style scoped>
.download-list {
  overflow-y: auto;
  padding-right: 0.5em;
  flex-grow: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.download-item {
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
