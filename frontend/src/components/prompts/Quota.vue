<template>
  <div class="card-content quota-prompt-content">
    <QuotaFolderBar
      class="quota-usage-bar"
      :source="source"
      :path="displayPath"
      :enabled="enabled"
      :limit-bytes="limitBytes"
      :snapshot="snapshot"
    />
    <p class="quota-path">{{ displayPath }}</p>
    <p class="quota-source">{{ source }}</p>
    <ActivityViewerButton :href="activityViewerHref" />

    <ToggleSwitch
      class="item"
      v-model="enabled"
      :name="$t('general.limit')"
      :description="$t('quotas.folderDescription')"
    />

    <div v-if="enabled">
      <ExpandDropdown
        v-model="meter"
        :options="meterOptions"
        :aria-label="$t('quotas.usageCounting')"
      />
      <p v-if="indexingDisabled" class="quota-help">{{ $t("quotas.indexingDisabledMeterHint") }}</p>
      <p v-else-if="measurementHelp" class="quota-help">{{ measurementHelp }}</p>

      <p>{{ $t("general.limit") }}</p>
      <div class="quota-custom-row">
        <QuotaCustomLimitInput
          :amount="customAmount"
          :unit="customUnit"
          :aria-label="$t('general.limit')"
          @update:amount="customAmount = $event"
          @update:unit="customUnit = $event"
        />
      </div>
    </div>
  </div>
  <div class="card-actions">
    <button type="button" class="button button--flat" @click="close">{{ $t("general.cancel") }}</button>
    <button v-if="quotaId" type="button" class="button button--flat button--red" @click="removeQuota">
      {{ $t("general.remove") }}
    </button>
    <button type="button" class="button button--flat button--blue" @click="save">{{ $t("general.save") }}</button>
  </div>
</template>

<script>
import { mutations, state } from "@/store";
import { quotasApi } from "@/api";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import QuotaFolderBar from "@/components/prompts/QuotaFolderBar.vue";
import ActivityViewerButton from "@/components/settings/ActivityViewerButton.vue";
import QuotaCustomLimitInput from "@/components/settings/QuotaCustomLimitInput.vue";
import { activityViewerPresets } from "@/utils/activityViewerLink";
import { notify } from "@/notify";
import {
  bytesFromCustomAmount,
  customAmountFromBytes,
} from "@/utils/quotaUnits";

export default {
  name: "Quota",
  components: { ToggleSwitch, ExpandDropdown, QuotaFolderBar, ActivityViewerButton, QuotaCustomLimitInput },
  props: {
    item: { type: Object, required: true },
    source: { type: String, required: true },
    path: { type: String, default: "" },
  },
  data() {
    return {
      enabled: false,
      customAmount: 10,
      customUnit: "gb",
      meter: "index_size",
      quotaId: "",
      snapshot: {},
    };
  },
  computed: {
    indexingDisabled() {
      return Boolean(state.sources.info?.[this.source]?.indexingDisabled);
    },
    meterOptions() {
      const opts = [
        { value: "index_size", label: this.$t("quotas.meterIndexScope") },
        { value: "accounted", label: this.$t("quotas.meterAccounted") },
      ];
      if (this.indexingDisabled) {
        return opts.filter((o) => o.value === "accounted");
      }
      return opts;
    },
    measurementHelp() {
      if (this.meter === "accounted") {
        return this.$t("quotas.meterAccountedHint");
      }
      return this.$t("quotas.meterIndexHint");
    },
    displayPath() {
      return this.path || this.item?.path || "/";
    },
    limitBytes() {
      if (!this.enabled) return 0;
      return bytesFromCustomAmount(this.customAmount, this.customUnit);
    },
    activityViewerHref() {
      return activityViewerPresets.quotas(this.source, this.displayPath);
    },
  },
  async mounted() {
    if (this.indexingDisabled) {
      this.meter = "accounted";
    }
    await this.load();
  },
  methods: {
    close() {
      mutations.closeTopPrompt();
    },
    applySnapshot(data) {
      if (!data) return;
      this.snapshot = {
        ...this.snapshot,
        usedBytes: data.usedBytes,
        reservedBytes: data.reservedBytes,
        measurementStatus: data.measurementStatus,
        meter: data.effectiveMeter || data.meter,
        configuredMeter: data.configuredMeter,
        effectiveMeter: data.effectiveMeter,
        limitBytes: data.limitBytes,
      };
    },
    async load() {
      try {
        const raw = await quotasApi.get(this.source, this.displayPath);
        const data = Array.isArray(raw) ? raw[0] : raw;
        if (!data) return;
        this.applySnapshot(data);
        if (data.id) {
          this.quotaId = data.id;
          this.enabled = data.limitBytes > 0;
          this.meter = data.configuredMeter || data.meter || "index_size";
          if (this.indexingDisabled) {
            this.meter = "accounted";
          }
          this.syncLimitFromBytes(data.limitBytes);
        }
      } catch {
        // no quota yet
      }
    },
    syncLimitFromBytes(bytes) {
      const { amount, unit } = customAmountFromBytes(bytes);
      this.customAmount = amount;
      this.customUnit = unit;
    },
    async save() {
      try {
        if (!this.enabled) {
          if (this.quotaId) await quotasApi.remove(this.quotaId);
          notify.showSuccess(this.$t("general.saved"));
          this.close();
          return;
        }
        const meter = this.indexingDisabled ? "accounted" : this.meter;
        const body = {
          source: this.source,
          path: this.displayPath,
          limitBytes: this.limitBytes,
          meter,
        };
        if (this.quotaId) {
          const updated = await quotasApi.update(this.quotaId, { limitBytes: this.limitBytes, meter });
          this.applySnapshot(updated);
        } else {
          const created = await quotasApi.create(body);
          this.quotaId = created.id;
          this.applySnapshot(created);
        }
        notify.showSuccess(this.$t("general.saved"));
        this.close();
      } catch (/** @type {any} */ err) {
        notify.showError(err.message || this.$t("prompts.operationFailed"));
      }
    },
    async removeQuota() {
      if (!this.quotaId) return;
      try {
        await quotasApi.remove(this.quotaId);
        notify.showSuccess(this.$t("general.removed"));
        this.close();
      } catch (/** @type {any} */ err) {
        notify.showError(err.message || this.$t("prompts.operationFailed"));
      }
    },
  },
};
</script>

<style scoped>
.quota-prompt-content {
  padding-left: 1em;
  padding-right: 1em;
}
.quota-path {
  font-weight: 600;
}
.quota-source {
  opacity: 0.8;
  margin-bottom: 1rem;
}
.quota-usage-bar {
  margin: 1em 0 1.5rem 0;
}
.quota-custom-row {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  margin-bottom: 1rem;
}
.quota-help {
  font-size: 0.85rem;
  opacity: 0.85;
  margin-bottom: 0.75rem;
}
</style>
