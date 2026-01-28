<template>
  <div class="card-title">
    <h2>{{ $t("prompts.fileInfo") }}</h2>
  </div>

  <div class="card-content info-content">
    <div class="info-grid">
      <!-- Basic Information Section -->
      <div class="info-section">
        <h3 class="section-title">{{ $t("prompts.basicInfo") }}</h3>
        <div class="info-item">
          <strong>{{ $t("prompts.displayName") }}</strong>
          <span aria-label="info display name">{{ displayName }}</span>
        </div>
        <div class="info-item">
          <strong>{{ $t("prompts.size") }}</strong>
          <span aria-label="info size">{{ humanSize }}</span>
        </div>
        <div class="info-item">
          <strong>{{ $t("prompts.typeName") }}</strong>
          <span aria-label="info type">{{ type }}</span>
        </div>
        <div class="info-item" v-if="humanTime">
          <strong>{{ $t("prompts.lastModified") }}</strong>
          <span aria-label="info last modified" :title="modTime">{{ humanTime }}</span>
        </div>
        <div class="info-item" v-if="source">
          <strong>{{ $t("general.source") }}</strong>
          <span aria-label="info source">{{ source }}</span>
        </div>
        <div class="info-item" v-if="filePath">
          <strong>{{ $t("general.path") }}</strong>
          <span aria-label="info path" class="break-word">{{ filePath }}</span>
        </div>
        <div class="info-item" v-if="hidden !== undefined">
          <strong>{{ $t("prompts.hidden") }}</strong>
          <span aria-label="info hidden">{{ hidden ? "✓" : "✗" }}</span><!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        </div>
        <div class="info-item" v-if="hasPreview !== undefined">
          <strong>{{ $t("prompts.hasPreview") }}</strong>
          <span aria-label="info has preview">{{ hasPreview ? "✓" : "✗" }}</span><!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        </div>
      </div>

      <!-- Additional Information Section -->
      <div class="info-section" v-if="additionalInfo.length > 0">
        <h3 class="section-title">{{ $t("prompts.additionalInfo") }}</h3>
        <div class="info-item" v-for="info in additionalInfo" :key="info.key">
          <strong>{{ info.label }}</strong>
          <span>{{ info.value }}</span>
        </div>
      </div>
      <!-- Hash Generator Section -->
      <div class="info-section" v-if="!dir">
        <h3 class="section-title">{{ $t("prompts.checksums") }}</h3>
        <div class="hash-generator">
          <div class="hash-select">
            <label for="hash-algo">{{ $t("prompts.hashAlgorithm") }}</label>
            <div class="form-flex-group">
              <select id="hash-algo" class="input form-form flat-right" v-model="selectedHashAlgo">
                <option value="md5">MD5</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
                <option value="sha1">SHA1</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
                <option value="sha256">SHA256</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
                <option value="sha512">SHA512</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
              </select>
              <button type="button" class="button form-button flat-left" @click="generateHash"
                :title="$t('prompts.generate')" :aria-label="$t('prompts.generate')">
                {{ $t("prompts.generate") }}
              </button>
            </div>
          </div>

          <div class="hash-result">
            <label for="hash-result">{{ $t("prompts.hashValue") }}</label>
            <div class="form-flex-group">
              <input id="hash-result" class="input form-form flat-right" type="text" :value="hashResult" readonly
                :placeholder="$t('prompts.selectHashAlgorithm')" />
              <button class="button form-button flat-left" @click="copyToClipboard" :disabled="!hashResult"
                :title="$t('buttons.copyToClipboard')" :aria-label="$t('buttons.copyToClipboard')">
                <i class="material-icons" style="font-size: 16px;">content_copy</i>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <div class="card-action">
    <button type="submit" @click="closeHovers" class="button button--flat" :aria-label="$t('general.close')"
      :title="$t('general.close')">
      {{ $t("general.close") }}
    </button>
  </div>
</template>
<script>
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { formatTimestamp } from "@/utils/moment";
import { filesApi } from "@/api";
import { state, mutations } from "@/store";
import { notify } from "@/notify";

export default {
  name: "info",
  props: {
    item: {
      type: Object,
      required: true,
    },
  },
  data() {
    return {
      selectedHashAlgo: "md5",
      hashResult: "",
      generatingHash: false,
    };
  },
  computed: {
    closeHovers() {
      return mutations.closeHovers;
    },
    humanSize() {
      return getHumanReadableFilesize(this.item?.size || 0);
    },
    humanTime() {
      if (!this.item?.modified) return "";
      return formatTimestamp(this.item.modified, state.user.locale);
    },
    modTime() {
      if (!this.item?.modified) return "";
      return new Date(Date.parse(this.item.modified)).toLocaleString();
    },
    name() {
      return this.item?.name || "";
    },
    type() {
      return this.item?.type || "";
    },
    displayName() {
      return this.item?.name || "";
    },
    dir() {
      return this.item?.type === "directory";
    },
    source() {
      return this.item?.source || "";
    },
    filePath() {
      return this.item?.path || "";
    },
    hidden() {
      return this.item?.hidden;
    },
    hasPreview() {
      return this.item?.hasPreview;
    },
    additionalInfo() {
      const info = [];
      
      if (this.item?.token) {
        info.push({ key: "token", label: this.$t("prompts.token"), value: this.item.token });
      }
      if (this.item?.hash) {
        info.push({ key: "hash", label: this.$t("general.hash"), value: this.item.hash });
      }
      if (this.item?.onlyOfficeId) {
        info.push({ key: "onlyOfficeId", label: this.$t("prompts.onlyOfficeId"), value: this.item.onlyOfficeId });
      }

      return info;
    },
  },
  methods: {
    async generateHash() {
      if (this.generatingHash || !this.item) return;

      this.hashResult = "";
      this.generatingHash = true;

      try {
        const source = this.item.source;
        const path = this.item.path;

        const hash = await filesApi.checksum(source, path, this.selectedHashAlgo);
        this.hashResult = hash;
      } catch (err) {
        this.hashResult = this.$t("prompts.errorGeneratingHash");
        const errorMessage = err instanceof Error ? err.message : "Error generating hash";
        notify.showError(errorMessage);
      } finally {
        this.generatingHash = false;
      }
    },
    async copyToClipboard() {
      if (!this.hashResult) return;

      try {
        await navigator.clipboard.writeText(this.hashResult);
        notify.showSuccessToast(this.$t("prompts.hashCopied"));
      } catch (err) {
        // Fallback for older browsers
        const textArea = document.createElement("textarea");
        textArea.value = this.hashResult;
        textArea.style.position = "fixed";
        textArea.style.opacity = "0";
        document.body.appendChild(textArea);
        textArea.select();
        try {
          document.execCommand("copy");
          notify.showSuccessToast(this.$t("prompts.hashCopied"));
        } catch (e) {
          notify.showError(this.$t("prompts.errorCopyingHash"));
        }
        document.body.removeChild(textArea);
      }
    },
  },
};
</script>

<style scoped>
.info-content {
  max-height: 70vh;
  overflow-y: auto;
}

.info-description {
  margin-bottom: 1.5em;
  color: var(--textSecondary);
  line-height: 1.5;
  text-align: center;
}

.info-grid {
  display: grid;
  gap: 1.5em;
}

.info-section {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
}

.section-title {
  font-size: 0.95em;
  font-weight: 600;
  color: var(--textPrimary);
  margin: 0 0 0.75em 0;
  padding-bottom: 0.5em;
  border-bottom: 1px solid var(--divider);
}

.info-item {
  display: flex;
  align-items: flex-start;
  gap: 0.75em;
  padding: 0.5em;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.info-item:hover {
  background-color: var(--surfaceSecondary);
}

.info-item strong {
  min-width: 120px;
  font-weight: 600;
  color: var(--textPrimary);
}

.info-item span {
  flex: 1;
  color: var(--textSecondary);
  word-break: break-word;
}

.break-word {
  word-break: break-word;
}

.hash-generator {
  display: flex;
  flex-direction: column;
  gap: 1em;
}

.hash-select,
.hash-result {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
}

.hash-select label,
.hash-result label {
  font-weight: 600;
  font-size: 0.9em;
  color: var(--textPrimary);
}

#hash-result {
  font-family: monospace;
}

/* Responsive adjustments */
@media (max-width: 768px) {
  .info-grid {
    grid-template-columns: 1fr;
  }

  .info-item strong {
    min-width: 100px;
  }
}
</style>
