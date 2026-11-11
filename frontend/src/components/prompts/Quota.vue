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

    <div class="settings-items">
      <ActivityViewerButton :href="activityViewerHref" />
    </div>
    <div class="settings-items">
      <ToggleSwitch
        class="item"
        v-model="enabled"
        :name="$t('general.limit')"
        :description="$t('quotas.folderDescription')"
      />
    </div>
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
    <button
      type="button"
      class="button button--flat button--grey"
      :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')"
      @click="close"
    >
      {{ $t("general.cancel") }}
    </button>
    <button
      v-if="quotaExists"
      type="button"
      class="button button--flat button--red"
      :aria-label="$t('general.remove')"
      :title="$t('general.remove')"
      @click="removeQuota"
    >
      {{ $t("general.remove") }}
    </button>
    <button
      type="button"
      class="button button--flat button--blue"
      :disabled="!dirty"
      :aria-label="$t('general.save')"
      :title="$t('general.save')"
      @click="save"
    >
      {{ $t("general.save") }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { mutations, state } from "@/store";
import { quotasApi } from "@/api";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import QuotaFolderBar from "@/components/prompts/QuotaFolderBar.vue";
import ActivityViewerButton from "@/components/settings/ActivityViewerButton.vue";
import QuotaCustomLimitInput from "@/components/settings/QuotaCustomLimitInput.vue";
import { activityViewerPresets } from "@/utils/activityViewerLink";
import { notify } from "@/notify";
import { bytesFromCustomAmount, customAmountFromBytes } from "@/utils/quotaUnits";
import type { FileListItem, FolderQuota, QuotaMeter, QuotaSnapshot, QuotaUnit } from "@/store/types.ts";

type QuotaResponse = FolderQuota | FolderQuota[];

interface QuotaBody {
  limitBytes: number;
  meter: QuotaMeter;
}

const props = withDefaults(
  defineProps<{
    promptId?: number | null;
    item: Pick<FileListItem, "path">;
    source: string;
    path?: string;
  }>(),
  {
    promptId: null,
    path: "",
  }
);

const { t } = useI18n();

const enabled = ref(false);
const customAmount = ref(10);
const customUnit = ref<QuotaUnit>("gb");
const meter = ref<QuotaMeter>("index_size");
const quotaExists = ref(false);
const snapshot = ref<QuotaSnapshot>({});
// Limit and meter as loaded, to tell whether there is something to save
const saved = ref<QuotaBody | null>(null);

const indexingDisabled = computed(() => Boolean(state.sources.info?.[props.source]?.indexingDisabled));
const meterOptions = computed(() => {
  const opts: { value: QuotaMeter; label: string }[] = [
    { value: "index_size", label: t("quotas.meterIndexScope") },
    { value: "accounted", label: t("quotas.meterAccounted") },
  ];
  if (indexingDisabled.value) {
    return opts.filter((o) => o.value === "accounted");
  }
  return opts;
});
const measurementHelp = computed(() =>
  meter.value === "accounted" ? t("quotas.meterAccountedHint") : t("quotas.meterIndexHint")
);
const displayPath = computed(() => props.path || props.item.path || "/");
const limitBytes = computed(() => {
  if (!enabled.value) return 0;
  return bytesFromCustomAmount(customAmount.value, customUnit.value);
});
const dirty = computed(() => {
  if (!enabled.value) return quotaExists.value;
  if (!saved.value) return true;
  return limitBytes.value !== saved.value.limitBytes || meter.value !== saved.value.meter;
});
const activityViewerHref = computed(() => activityViewerPresets.quotas(props.source, displayPath.value));

onMounted(async () => {
  if (indexingDisabled.value) {
    meter.value = "accounted";
  }
  await load();
});

function close() {
  mutations.closeTopPrompt();
}

// The quotas endpoints answer with a list
function firstQuota(raw: QuotaResponse) {
  return Array.isArray(raw) ? raw[0] : raw;
}

function errorMessage(err: unknown) {
  return (err instanceof Error && err.message) || t("prompts.operationFailed");
}

function applySnapshot(data?: QuotaSnapshot) {
  if (!data) return;
  snapshot.value = {
    usedBytes: data.usedBytes,
    reservedBytes: data.reservedBytes,
    measurementStatus: data.measurementStatus,
  };
}

async function load() {
  try {
    const data = firstQuota((await quotasApi.get(props.source, displayPath.value)) as QuotaResponse);
    if (!data) return;
    applySnapshot(data);
    if ((data.limitBytes || 0) > 0) {
      quotaExists.value = true;
      enabled.value = true;
      meter.value = data.configuredMeter || data.meter || "index_size";
      if (indexingDisabled.value) {
        meter.value = "accounted";
      }
      syncLimitFromBytes(data.limitBytes);
      saved.value = { limitBytes: bytesFromCustomAmount(customAmount.value, customUnit.value), meter: meter.value };
    }
  } catch {
    // no quota yet
  }
}

function syncLimitFromBytes(bytes: number) {
  const { amount, unit } = customAmountFromBytes(bytes);
  customAmount.value = amount;
  customUnit.value = unit;
}

async function save() {
  try {
    if (!enabled.value) {
      if (quotaExists.value) {
        await quotasApi.remove(props.source, displayPath.value);
      }
      notify.showSuccess(t("general.saved"));
      close();
      return;
    }
    const body: QuotaBody = {
      limitBytes: limitBytes.value,
      meter: indexingDisabled.value ? "accounted" : meter.value,
    };
    if (quotaExists.value) {
      const updated = await quotasApi.update(props.source, displayPath.value, body);
      applySnapshot(firstQuota(updated as QuotaResponse));
    } else {
      const created = await quotasApi.create({
        source: props.source,
        path: displayPath.value,
        ...body,
      });
      quotaExists.value = true;
      applySnapshot(firstQuota(created as QuotaResponse));
    }
    notify.showSuccess(t("general.saved"));
    close();
  } catch (err) {
    notify.showError(errorMessage(err));
  }
}

async function removeQuota() {
  if (!quotaExists.value) return;
  try {
    await quotasApi.remove(props.source, displayPath.value);
    notify.showSuccess(t("general.removed"));
    close();
  } catch (err) {
    notify.showError(errorMessage(err));
  }
}
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
  margin: 1em 0 1.5rem;
}

.quota-custom-row {
  width: 100%;
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
