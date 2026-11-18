<template>
  <div class="card-content">
    <div v-if="converting" class="loading-content">
      <LoadingSpinner size="small" mode="placeholder" />
      <p class="loading-text">{{ statusMessage }}</p>
    </div>
    <div v-else>
      <p class="file-summary">
        <i class="material-symbols-outlined file-icon">category</i>
        <span class="file-name">{{ item?.name }}</span>
      </p>

      <p class="prompts-label">{{ $t("cad.selectFormat") }}</p>
      <div class="target-options" role="radiogroup" :aria-label="$t('cad.selectFormat')">
        <button
          v-for="target in availableTargets"
          :key="target.format"
          type="button"
          role="radio"
          class="target-card"
          :aria-checked="selectedFormat === target.format"
          :class="{ selected: selectedFormat === target.format }"
          @click="selectedFormat = target.format"
        >
          <span class="target-card-header">
            <i class="material-symbols-outlined target-icon">{{ target.icon }}</i>
            <span class="target-label">{{ target.label }}</span>
          </span>
          <span class="target-desc">{{ target.description }}</span>
        </button>
      </div>

      <p class="prompts-label">{{ $t("cad.outputDestination") }}</p>
      <div class="destination-options">
        <label class="radio-option">
          <input type="radio" value="download" v-model="destination" />
          <span>{{ $t("cad.downloadLocally") }}</span>
        </label>
        <label class="radio-option" v-if="canSaveToServer">
          <input type="radio" value="server" v-model="destination" />
          <span>{{ $t("cad.saveToFilebrowser") }}</span>
        </label>
      </div>

      <p v-if="error" class="error-text">{{ error }}</p>
    </div>
  </div>

  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--grey"
      @click="close"
      :disabled="converting"
    >
      {{ $t("general.cancel") }}
    </button>
    <button
      type="button"
      class="button button--flat"
      @click="startConversion"
      :disabled="converting || !selectedFormat"
    >
      {{ $t("cad.convertAction") }}
    </button>
  </div>
</template>

<script>
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { getCadConversionTargets, convertCadFile } from "@/utils/cadService.ts";
import { resourcesApi } from "@/api";
import { getters, mutations, state } from "@/store";
import { ensureViewToken } from "@/api/viewToken.js";
import { removeLastDir } from "@/utils/url.js";
import { notify } from "@/notify";

export default {
  name: "CadConvert",
  components: { LoadingSpinner },
  props: {
    item: { type: Object, required: true },
  },
  data() {
    const targets = getCadConversionTargets(this.item?.name || "");
    return {
      availableTargets: targets,
      selectedFormat: targets[0]?.format || "",
      destination: "download",
      converting: false,
      statusMessage: "Starting conversion...",
      error: null,
    };
  },
  computed: {
    canSaveToServer() {
      // The save posts to the item's own source, not whatever is currently
      // listed in the request.
      return !getters.isShare() && getters.sourcePermissions(this.item?.source)?.create;
    },
  },
  methods: {
    close() {
      mutations.closeTopPrompt();
    },

    async startConversion() {
      this.converting = true;
      this.error = null;
      this.statusMessage = "Fetching CAD file...";

      try {
        let viewToken = this.item.viewToken;
        if (!viewToken && !getters.isShare()) {
          viewToken = await ensureViewToken(this.item.source);
        }

        let url;
        if (getters.isShare()) {
          url = resourcesApi.getDownloadURLPublic(
            this.item.path,
            state.shareInfo.hash,
            null,
            false,
            false,
            viewToken
          );
        } else {
          url = resourcesApi.getViewURL(this.item.source, this.item.path, viewToken, null, false);
        }

        const res = await fetch(url);
        if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);
        const buffer = await res.arrayBuffer();

        this.statusMessage = "Converting in browser using WASM...";
        const result = await convertCadFile(
          buffer,
          this.item.name,
          this.selectedFormat,
          (msg) => {
            this.statusMessage = msg;
          }
        );

        if (this.destination === "download") {
          const blob = new Blob([result.data], { type: "application/octet-stream" });
          const blobUrl = URL.createObjectURL(blob);
          const a = document.createElement("a");
          a.href = blobUrl;
          a.download = result.filename;
          document.body.appendChild(a);
          a.click();
          a.remove();
          setTimeout(() => URL.revokeObjectURL(blobUrl), 10000);
          notify.success(this.$t("cad.conversionSuccess"));
        } else {
          this.statusMessage = "Saving converted file to current folder...";
          const parentDir = removeLastDir(this.item.path) || "/";
          const targetPath = `${parentDir.replace(/\/$/, "")}/${result.filename}`;
          const blob = new Blob([result.data], { type: "application/octet-stream" });
          // Never replace an existing file silently: confirm first, and only
          // enable overwrite when the user accepted.
          let overwrite = false;
          try {
            const listing = await resourcesApi.fetchFiles(this.item.source, parentDir, false, false, true);
            const exists = (listing?.items || []).some((i) => i.name === result.filename);
            if (exists) {
              overwrite = window.confirm(
                this.$t("prompts.confirmOverwrite", { file: result.filename })
              );
              if (!overwrite) {
                this.error = this.$t("prompts.fileExists");
                this.converting = false;
                return;
              }
            }
          } catch {
            // A listing failure (permissions, races) must not block the save;
            // post without overwrite and let a real conflict surface as 409.
          }
          await resourcesApi.post(this.item.source, targetPath, blob, overwrite);
          notify.success(this.$t("cad.conversionSaved", { file: result.filename }));
          mutations.setReload(true);
        }

        this.close();
      } catch (err) {
        this.error = err.message || "Conversion failed";
        this.converting = false;
      }
    },
  },
};
</script>

<style scoped>
.file-summary {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1rem;
  background-color: var(--surface-secondary, #f1f3f4);
  border-radius: 6px;
  font-weight: 500;
  margin-bottom: 1rem;
}

.file-icon {
  color: var(--primary, #1a73e8);
}

.file-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.target-options {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  margin-bottom: 1.25rem;
}

.target-card {
  padding: 0.75rem 1rem;
  border-radius: 6px;
  border: 1px solid var(--border-color, #dadce0);
  background-color: var(--surface-primary, #fff);
  cursor: pointer;
  transition: all 0.2s ease;
}

.target-card:hover {
  background-color: var(--surface-hover, rgb(0 0 0 / 3%));
  border-color: var(--primary, #1a73e8);
}

.target-card.selected {
  border-color: var(--primary, #1a73e8);
  background-color: rgb(26 115 232 / 8%);
}

.target-card-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.25rem;
}

.target-icon {
  font-size: 1.2rem;
  color: var(--primary, #1a73e8);
}

.target-label {
  font-weight: 600;
  font-size: 0.95rem;
}

.target-desc {
  font-size: 0.8rem;
  color: var(--text-secondary, #5f6368);
  display: block;
  margin-left: 1.7rem;
}

.destination-options {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.radio-option {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  cursor: pointer;
  font-size: 0.9rem;
}

.error-text {
  color: #d93025;
  font-size: 0.85rem;
  margin-top: 0.5rem;
}

.loading-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2rem 1rem;
  text-align: center;
}

.loading-text {
  margin-top: 1rem;
  font-size: 0.9rem;
  color: var(--text-secondary, #5f6368);
}
</style>
