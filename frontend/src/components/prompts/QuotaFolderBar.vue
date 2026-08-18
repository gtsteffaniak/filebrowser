<template>
  <div class="quota-folder-bar">
    <ProgressBar
      class="quota-folder-bar__progress"
      :class="{ 'quota-folder-bar__progress--disabled': !enabled }"
      :val="barVal"
      :val-background="enabled ? (snapshot.reservedBytes || 0) : 0"
      :max="barMax"
      :val-text="barValText"
      unit="bytes"
      :status="barStatus"
      :bg-color="enabled ? '#e8e8e8' : '#d5d5d5'"
      :bar-color="enabled ? 'var(--primaryColor)' : '#d5d5d5'"
      size="huge"
      :font-size="15"
      :bar-border-radius="12"
    />
    <p v-if="enabled && snapshot.measurementStatus === 'accounted_fallback'" class="quota-folder-bar__help">
      {{ $t("quotas.accountedFallback") }}
    </p>
    <p
      v-else-if="enabled && snapshot.measurementStatus && snapshot.measurementStatus !== 'ready'"
      class="quota-folder-bar__help"
    >
      {{ $t("quotas.usageSyncPending") }}
    </p>
  </div>
</template>

<script>
import ProgressBar from "@/components/ProgressBar.vue";

export default {
  name: "QuotaFolderBar",
  components: { ProgressBar },
  props: {
    source: { type: String, required: true },
    path: { type: String, required: true },
    enabled: { type: Boolean, default: false },
    limitBytes: { type: Number, default: 0 },
    snapshot: { type: Object, default: () => ({}) },
  },
  computed: {
    barVal() {
      if (!this.enabled) return 0;
      return this.snapshot.usedBytes || 0;
    },
    barMax() {
      if (!this.enabled) return 1;
      return this.limitBytes > 0 ? this.limitBytes : 1;
    },
    barValText() {
      if (!this.enabled) return this.$t("general.disabled");
      return null;
    },
    barStatus() {
      if (!this.enabled) return "disk";
      const used = (this.snapshot.usedBytes || 0) + (this.snapshot.reservedBytes || 0);
      const max = this.barMax;
      if (used / max >= 0.95) return "error";
      if (used / max >= 0.8) return "warning";
      return "normal";
    },
  },
};
</script>

<style scoped>
.quota-folder-bar__progress {
  margin: 0;
}
.quota-folder-bar__progress :deep(.vue-simple-progress) {
  margin: 0;
  min-height: 36px;
}
.quota-folder-bar__progress :deep(.vue-simple-progress-bar) {
  min-height: 36px;
}
.quota-folder-bar__progress--disabled :deep(.vue-simple-progress-text) {
  color: #5a5a5a;
  font-weight: 600;
  letter-spacing: 0.02em;
}
.quota-folder-bar__help {
  font-size: 0.85rem;
  opacity: 0.85;
  margin: 0.35rem 0 0.5rem 0;
}
</style>
