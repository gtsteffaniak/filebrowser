<template>
  <div class="card-title">
    <h2>{{ $t("prompts.fileInfo") }}</h2>
  </div>

  <div class="card-content info-content">
    <p v-if="selected.length > 1" class="info-description">
      {{ $t("prompts.filesSelected", { count: selected.length }) }}
    </p>

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
        <div class="info-item" v-if="selected.length < 2 && humanTime">
          <strong>{{ $t("prompts.lastModified") }}</strong>
          <span aria-label="info last modified" :title="modTime">{{ humanTime }}</span>
        </div>
        <div class="info-item" v-if="selected.length < 2 && source">
          <strong>{{ $t("general.source", ) }}</strong>
          <span aria-label="info source">{{ source }}</span>
        </div>
        <div class="info-item" v-if="selected.length < 2 && filePath">
          <strong>{{ $t("general.path") }}</strong>
          <span aria-label="info path" class="break-word">{{ filePath }}</span>
        </div>
        <div class="info-item" v-if="hidden !== undefined">
          <strong>{{ $t("prompts.hidden") }}</strong>
          <span aria-label="info hidden">{{ hidden ? "✓" : "✗" }}</span> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        </div>
        <div class="info-item" v-if="hasPreview !== undefined">
          <strong>{{ $t("prompts.hasPreview") }}</strong>
          <span aria-label="info has preview">{{ hasPreview ? "✓" : "✗" }}</span> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        </div>
      </div>
      <!-- Directory Information Section -->
      <div class="info-section" v-if="dir && selected.length === 0">
        <h3 class="section-title">{{ $t("prompts.directoryInfo") }}</h3>
        <div class="info-item">
          <strong>{{ $t("prompts.numberFiles") }}</strong>
          <span>{{ req.numFiles }}</span>
        </div>
        <div class="info-item">
          <strong>{{ $t("prompts.numberDirs") }}</strong>
          <span>{{ req.numDirs }}</span>
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
              <button
                type="button"
                class="button form-button flat-left"
                @click="generateHash"
                :title="$t('prompts.generate')"
                :aria-label="$t('prompts.generate')"
              >
                {{ $t("prompts.generate") }}
              </button>
            </div>
          </div>

          <div class="hash-result">
            <label for="hash-result">{{ $t("prompts.hashValue") }}</label>
            <div class="form-flex-group">
              <input
                style="height: 100%; padding: 0.75em;"
                id="hash-result"
                class="input form-form flat-right"
                type="text"
                :value="hashResult"
                readonly
                :placeholder="$t('prompts.selectHashAlgorithm')"
              />
              <button
                class="button form-button flat-left"
                @click="copyToClipboard"
                :disabled="!hashResult"
                :title="$t('buttons.copyToClipboard')"
                :aria-label="$t('buttons.copyToClipboard')"
              >
              <i class="material-icons" style="font-size: 16px;">content_copy</i>
              </button>
            </div>
          </div>
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
import { state, getters, mutations } from "@/store";
import { notify } from "@/notify";

export default {
  name: "info",
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
    req() {
      return state.req;
    },
    selected() {
      return state.selected;
    },
    selectedCount() {
      return getters.selectedCount();
    },
    isListing() {
      return getters.isListing();
    },
    humanSize() {
      if (state.isSearchActive) {
        return getHumanReadableFilesize(state.selected[0].size);
      }
      if (getters.selectedCount() === 0 || !this.isListing) {
        return getHumanReadableFilesize(state.req.size);
      }

      let sum = 0;

      for (let selected of this.selected) {
        const item = typeof(selected) === 'number' ? state.req.items[selected] : selected;
        sum += item.size;
      }

      return getHumanReadableFilesize(sum);
    },
    humanTime() {
      if (state.isSearchActive) {
        return "";
      }
      const modifiedDate = getters.selectedCount() === 0
        ? state.req.modified
        : getters.getFirstSelected()?.modified;

      if (!modifiedDate) {
        return "";
      }

      return formatTimestamp(modifiedDate, state.user.locale);
    },
    modTime() {
      if (state.isSearchActive) {
        return "";
      }
      const modifiedDate = getters.selectedCount() === 0
        ? state.req.modified
        : getters.getFirstSelected()?.modified;

      if (!modifiedDate) {
        return "";
      }

      return new Date(Date.parse(modifiedDate)).toLocaleString();
    },
    name() {
      if (state.isSearchActive) {
        return state.selected[0].name;
      }
      return getters.selectedCount() === 0
        ? state.req.name
        : getters.getFirstSelected().name;
    },
    type() {
      if (state.isSearchActive) {
        return state.selected[0].type;
      }
      return getters.selectedCount() === 0
        ? state.req.type
        : getters.getFirstSelected().type;
    },
    displayName() {
      if (this.selected.length > 1) {
        return this.$t("prompts.fileInfo");
      }
      return this.name;
    },
    dir() {
      if (state.isSearchActive) {
        return state.selected[0].type === "directory";
      }
      return (
        getters.selectedCount() > 1 ||
        (getters.selectedCount() === 0
          ? state.req.type == "directory"
          : getters.getFirstSelected().type == "directory")
      );
    },
    source() {
      if (state.isSearchActive) {
        return state.selected[0].source;
      }
      const currentSource = state.sources.current;
      if (!currentSource || currentSource === "") {
        return "";
      }
      return currentSource;
    },
    filePath() {
      if (state.isSearchActive) {
        return state.selected[0].path;
      }
      if (getters.selectedCount() === 0) {
        return state.route.path;
      }
      return getters.getFirstSelected()?.path || "";
    },
    hidden() {
      if (state.isSearchActive) {
        return state.selected[0].hidden;
      }
      if (getters.selectedCount() === 0) {
        return state.req.hidden;
      }
      return getters.getFirstSelected()?.hidden;
    },
    hasPreview() {
      if (state.isSearchActive) {
        return state.selected[0].hasPreview;
      }
      if (getters.selectedCount() === 0) {
        return state.req.hasPreview;
      }
      return getters.getFirstSelected()?.hasPreview;
    },
    additionalInfo() {
      const info = [];

      // Add more info fields here if needed
      if (state.req.token) {
        info.push({ key: "token", label: this.$t("prompts.token"), value: state.req.token });
      }
      if (state.req.hash) {
        info.push({ key: "hash", label: this.$t("general.hash"), value: state.req.hash });
      }
      if (state.req.onlyOfficeId) {
        info.push({ key: "onlyOfficeId", label: this.$t("prompts.onlyOfficeId"), value: state.req.onlyOfficeId });
      }

      return info;
    },
  },
  methods: {
    async generateHash() {
      if (this.generatingHash) return;

      this.hashResult = "";
      this.generatingHash = true;

      try {
        let source, path;

        if (state.isSearchActive) {
          source = state.selected[0].source;
          path = state.selected[0].path;
        } else if (getters.selectedCount()) {
          source = state.sources.current;
          path = getters.getFirstSelected().path;
        } else {
          source = state.sources.current;
          path = state.route.path;
        }

        const hash = await filesApi.checksum(source, path, this.selectedHashAlgo);
        this.hashResult = hash;
      } catch (err) {
        this.hashResult = this.$t("prompts.errorGeneratingHash");
        notify.showError(err.message || "Error generating hash");
      } finally {
        this.generatingHash = false;
      }
    },
    async copyToClipboard() {
      if (!this.hashResult) return;

      try {
        await navigator.clipboard.writeText(this.hashResult);
        notify.showSuccess(this.$t("prompts.hashCopied"));
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
          notify.showSuccess(this.$t("prompts.hashCopied"));
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
