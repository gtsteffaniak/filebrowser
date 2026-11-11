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

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import ProgressBar from "@/components/ProgressBar.vue";
import type { QuotaSnapshot } from "@/store/types.ts";

type BarStatus = "disk" | "error" | "warning" | "normal";

const props = withDefaults(
  defineProps<{
    source: string;
    path: string;
    enabled?: boolean;
    limitBytes?: number;
    snapshot?: QuotaSnapshot;
  }>(),
  {
    enabled: false,
    limitBytes: 0,
    snapshot: () => ({}),
  }
);

const { t } = useI18n();

const barVal = computed(() => (props.enabled ? props.snapshot.usedBytes || 0 : 0));
const barMax = computed(() => {
  if (!props.enabled) return 1;
  return props.limitBytes > 0 ? props.limitBytes : 1;
});
const barValText = computed(() => (props.enabled ? null : t("general.disabled")));
const barStatus = computed<BarStatus>(() => {
  if (!props.enabled) return "disk";
  const used = (props.snapshot.usedBytes || 0) + (props.snapshot.reservedBytes || 0);
  const max = barMax.value;
  if (used / max >= 0.95) return "error";
  if (used / max >= 0.8) return "warning";
  return "normal";
});
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
  margin: 0.35rem 0 0.5rem;
}
</style>
