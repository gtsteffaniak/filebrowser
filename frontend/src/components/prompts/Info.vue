<template>
  <div class="card-content info-content">
    <ActivityViewerButton
      v-if="source"
      :href="activityViewerHref"
    />
    <div class="info-grid">
      <!-- Basic Information Section -->
      <div class="info-section">
        <h3 class="section-title">{{ $t("prompts.basicInfo") }}</h3>
        <div class="info-item">
          <strong>{{ $t("prompts.displayName") }}</strong>
          <span aria-label="info display name">{{ displayName }}</span>
        </div>
        <div class="info-item">
          <strong>{{ $t("general.size") }}</strong>
          <span aria-label="info size">{{ humanSize }}</span>
        </div>
        <div class="info-item">
          <strong>{{ $t("general.type") }}</strong>
          <span aria-label="info type">{{ type }}</span>
        </div>
        <div class="info-item" v-if="humanTime">
          <strong>{{ $t("files.lastModified") }}</strong>
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

      <!-- Storage quota (admin, folders) -->
      <div v-if="showQuotaSection" class="info-section">
        <h3 class="section-title">{{ $t("quotas.title") }}</h3>
        <SettingsButton
          class="info-manage-link"
          :name="$t('quotas.title')"
          :description="$t('quotas.openDescription')"
          @click="openQuotaPrompt"
        />
        <div class="info-item">
          <strong>{{ $t("general.enabled") }}</strong>
          <span>{{ quotaEnabled ? $t("general.yes") : $t("general.no") }}</span>
        </div>
        <div v-if="quotaEnabled" class="info-quota-usage">
          <QuotaFolderBar
            class="info-quota-bar"
            :source="source"
            :path="filePath"
            :enabled="true"
            :limit-bytes="quotaLimitBytes"
            :snapshot="quotaSnapshot"
          />
        </div>
      </div>

      <!-- Access rules (admin) -->
      <div v-if="showAccessSection" class="info-section">
        <h3 class="section-title">{{ $t("access.rules") }}</h3>
        <SettingsButton
          class="info-manage-link"
          :name="$t('access.accessManagement')"
          :description="$t('access.manageDescription')"
          @click="openAccessPrompt"
        />
        <div class="info-item">
          <strong>{{ $t("access.hasRules") }}</strong>
          <span>{{ hasAccessRules ? $t("general.yes") : $t("general.no") }}</span>
        </div>
        <div v-if="accessRuleEntries.length" class="access-rules-list">
          <div
            v-for="entry in accessRuleEntries"
            :key="accessEntryKey(entry)"
            class="info-item access-rule-entry"
          >
            <strong>{{ entry.allow ? $t("access.allow") : $t("access.deny") }}</strong>
            <span>{{ accessEntryLabel(entry) }}</span>
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
      <!-- Hash Generator Section -->
      <div class="info-section" v-if="!dir">
        <h3 class="section-title">{{ $t("prompts.checksums") }}</h3>
        <div class="hash-generator">
          <div class="hash-select">
            <label for="hash-algo">{{ $t("prompts.hashAlgorithm") }}</label>
            <div class="form-flex-group">
              <ExpandDropdown
                input-id="hash-algo"
                v-model="selectedHashAlgo"
                class="form-form flat-right"
                :options="hashAlgoOptions"
                :aria-label="$t('prompts.hashAlgorithm')"
              />
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
              <button
                type="button"
                class="button form-button flat-left"
                @click="copyToClipboard"
                :disabled="!hashResult"
                :title="$t('buttons.copyToClipboard')"
                :aria-label="$t('buttons.copyToClipboard')"
              >
                <i class="material-symbols-outlined" style="font-size: 16px;">content_copy</i>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

</template>
<script>
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { formatTimestamp } from "@/utils/moment";
import { copyToClipboard } from "@/utils/clipboard";
import { resourcesApi, quotasApi, accessApi } from "@/api";
import { getters, mutations, state } from "@/store";
import { notify } from "@/notify";
import { activityViewerPresets } from "@/utils/activityViewerLink";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import ActivityViewerButton from "@/components/settings/ActivityViewerButton.vue";
import SettingsButton from "@/components/settings/SettingsButton.vue";
import QuotaFolderBar from "@/components/prompts/QuotaFolderBar.vue";

export default {
  name: "info",
  components: {
    ExpandDropdown,
    ActivityViewerButton,
    SettingsButton,
    QuotaFolderBar,
  },
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
      quotaSnapshot: {},
      quotaId: "",
      accessRule: {
        denyAll: false,
        deny: { users: [], groups: [] },
        allow: { users: [], groups: [] },
      },
    };
  },
  async mounted() {
    if (this.showQuotaSection) {
      await this.loadQuota();
    }
    if (this.showAccessSection) {
      await this.loadAccessRules();
    }
  },
  computed: {
    hashAlgoOptions() {
      return [
        { value: "md5", label: "MD5" },
        { value: "sha1", label: "SHA1" },
        { value: "sha256", label: "SHA256" },
        { value: "sha512", label: "SHA512" },
      ];
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
    activityViewerHref() {
      return activityViewerPresets.files(this.source, this.filePath);
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
    isAdmin() {
      return getters.isAdmin();
    },
    showQuotaSection() {
      return this.isAdmin && this.dir && this.source && this.filePath;
    },
    showAccessSection() {
      return this.isAdmin && this.source && this.filePath;
    },
    quotaEnabled() {
      return Boolean(this.quotaId) && (this.quotaSnapshot.limitBytes || 0) > 0;
    },
    quotaLimitBytes() {
      return this.quotaSnapshot.limitBytes || 0;
    },
    accessRuleEntries() {
      /** @type {{allow: boolean, type: "user" | "group" | "all", name: string}[]} */
      const entries = [];
      if (this.accessRule.denyAll) {
        entries.push({ allow: false, type: "all", name: this.$t("access.all") });
      }
      (this.accessRule.deny?.users || []).forEach((name) => {
        entries.push({ allow: false, type: "user", name });
      });
      (this.accessRule.deny?.groups || []).forEach((name) => {
        entries.push({ allow: false, type: "group", name });
      });
      (this.accessRule.allow?.users || []).forEach((name) => {
        entries.push({ allow: true, type: "user", name });
      });
      (this.accessRule.allow?.groups || []).forEach((name) => {
        entries.push({ allow: true, type: "group", name });
      });
      return entries;
    },
    hasAccessRules() {
      return this.accessRuleEntries.length > 0;
    },
  },
  methods: {
    async loadQuota() {
      try {
        const raw = await quotasApi.get(this.source, this.filePath);
        const data = Array.isArray(raw) ? raw[0] : raw;
        if (!data) return;
        this.quotaSnapshot = {
          usedBytes: data.usedBytes,
          reservedBytes: data.reservedBytes,
          measurementStatus: data.measurementStatus,
          limitBytes: data.limitBytes,
        };
        if (data.id) {
          this.quotaId = data.id;
        }
      } catch {
        // no quota or preview unavailable
      }
    },
    async loadAccessRules() {
      try {
        const response = await accessApi.get(this.source, this.filePath);
        this.accessRule = response;
      } catch {
        this.accessRule = {
          denyAll: false,
          deny: { users: [], groups: [] },
          allow: { users: [], groups: [] },
        };
      }
    },
    openQuotaPrompt() {
      mutations.showPrompt({
        name: "Quota",
        props: {
          item: this.item,
          source: this.source,
          path: this.filePath,
        },
      });
    },
    openAccessPrompt() {
      mutations.showPrompt({
        name: "access",
        props: {
          sourceName: this.source,
          path: this.filePath,
        },
      });
    },
    accessEntryKey(entry) {
      return `${entry.type}-${entry.name}-${entry.allow}`;
    },
    accessEntryLabel(entry) {
      if (entry.type === "user") {
        return `${this.$t("general.user")}: ${entry.name}`;
      }
      if (entry.type === "group") {
        return `${this.$t("general.group")}: ${entry.name}`;
      }
      return entry.name;
    },
    async generateHash() {
      if (this.generatingHash || !this.item) return;

      this.hashResult = "";
      this.generatingHash = true;

      try {
        const source = this.item.source;
        const path = this.item.path;

        const hash = await resourcesApi.checksum(source, path, this.selectedHashAlgo);
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
      await copyToClipboard(this.hashResult);
    },
  },
};
</script>

<style scoped>
.info-content {
  height: 100%;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.info-manage-link {
  padding: 0.25em 0.5em;
}

.info-quota-usage {
  padding: 0.5em;
}

.access-rules-list {
  display: flex;
  flex-direction: column;
  gap: 0.25em;
}

.access-rule-entry strong {
  min-width: 72px;
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
  flex: 1;
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
