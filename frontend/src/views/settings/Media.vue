<template>
  <div class="card-title">
    <h2>{{ t("general.media") }}</h2>
  </div>
  <div class="card-content">
    <Errors v-if="error" :errorCode="errorStatus" />
    <div v-else-if="loading" class="loading-hint">
      <LoadingSpinner size="medium" />
    </div>
    <template v-else>
      <div class="settings-items">
        <div class="settings-select-input item">
          <div class="no-padding">
            <label for="accelDevice">{{ t("media.accelDevice") }}</label>
            <HelpTooltipIcon :text="accelDeviceTooltip" />
          </div>
          <ExpandDropdown
            input-id="accelDevice"
            :model-value="accelDeviceSelectValue"
            :options="accelDeviceSelectOptions"
            :aria-label="t('media.accelDevice')"
            :disabled="isLocked('hardwareAcceleration') || isLocked('accelDevice')"
            @update:model-value="onAccelDeviceChange" />
        </div>
        <ToggleSwitch class="item" v-model="media.alwaysUseFfmpeg"
          :disabled="isLocked('alwaysUseFfmpeg')"
          :value-tooltip="fieldTooltip('alwaysUseFfmpeg')"
          :name="t('media.alwaysUseFfmpeg')"
          :description="t('media.alwaysUseFfmpegDescription')"
          @update:model-value="save" />
        <ToggleSwitch class="item" v-model="media.debug" :name="t('media.debug')"
          :disabled="isLocked('debug')"
          :value-tooltip="fieldTooltip('debug')"
          :description="t('media.debugDescription')"
          @update:model-value="save" />
        <SettingsButton class="item"
          :name="t('media.imageVideoConversionFormats')"
          :description="t('media.imageVideoConversionFormatsDescription')"
          @activate="openConversionFormats" />
      </div>

      <div class="settings-items">
        <h3>{{ t("media.transcode") }}</h3>
        <ToggleSwitch class="item" v-model="media.transcode.enabled"
          :disabled="isLocked('transcode.enabled')"
          :value-tooltip="fieldTooltip('transcode.enabled')"
          :name="t('media.transcodeEnabled')"
          :description="t('media.transcodeEnabledDescription')"
          @update:model-value="save" />
        <SettingsButton class="item"
          :name="t('media.transcodeSettings')"
          :description="t('media.transcodeSettingsDescription')"
          :disabled="!media.transcode.enabled"
          @activate="openTranscodeSettings" />
      </div>

      <div v-if="isAdmin" class="settings-items">
        <h3>{{ t("media.shadowRegressions") }}</h3>
        <SettingsButton class="item"
          :name="t('media.shadowRegressionsView')"
          :description="t('media.shadowRegressionsViewDescription')"
          @activate="openShadowRegressions" />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { getMediaSettings } from "@/api/settings.ts";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import SettingsButton from "@/components/settings/SettingsButton.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { useI18n } from "@/i18n/index.ts";
import { notify } from "@/notify/index.ts";
import { httpStatusOf } from "@/player/playerErrors.ts";
import { eventBus } from "@/store/eventBus.ts";
import { getters, mutations } from "@/store/index.ts";
import type { AccelDeviceOption, MediaSettings } from "@/types/media.ts";
import {
  MEDIA_SETTINGS_CHANGED_EVENT,
  emptyMediaSettings,
  mergeMediaSettings,
  saveMediaSettingsPatch,
} from "@/utils/config/mediaSettings.ts";
import Errors from "@/views/Errors.vue";

defineOptions({ name: "mediaSettings" });

const { t } = useI18n();

const media = ref<MediaSettings>(emptyMediaSettings());
const error = ref<Error | null>(null);
const loading = ref(true);
/** Media settings pinned in the config file cannot be changed here. */
const lockedFromConfigPaths = ref<string[]>([]);
const lockMessage = ref("");
/** Selectable acceleration devices reported by the backend (no "auto" entry). */
const accelDeviceOptions = ref<AccelDeviceOption[]>([]);
/** Device id that accelDevice "auto" resolves to on this host ("" = software only). */
const autoAccelDevice = ref("");

const errorStatus = computed(() => httpStatusOf(error.value));
const isAdmin = computed(() => getters.isAdmin());

function isLocked(path: string): boolean {
  return lockedFromConfigPaths.value.includes(path);
}

function fieldTooltip(path: string, fallback = ""): string {
  return isLocked(path) ? t("settings.lockedFromConfig") : fallback;
}

const accelDeviceTooltip = computed(() =>
  isLocked("hardwareAcceleration") || isLocked("accelDevice")
    ? t("settings.lockedFromConfig")
    : t("media.accelDeviceDescription"),
);

const accelDeviceSelectOptions = computed(() =>
  accelDeviceOptions.value.map((option) => ({
    value: option.id,
    label: option.label || option.id,
  })),
);

/** "auto" is never listed; show the device it resolves to instead. */
const accelDeviceSelectValue = computed(() => {
  if (media.value.accelDevice === "auto") {
    return autoAccelDevice.value || "software";
  }
  return media.value.accelDevice;
});

async function load({ silent = false } = {}): Promise<void> {
  if (!silent) {
    loading.value = true;
  }
  try {
    const data = await getMediaSettings();
    media.value = mergeMediaSettings(data);
    accelDeviceOptions.value = data.accelDeviceOptions ?? [];
    autoAccelDevice.value = data.autoAccelDevice ?? "";
    lockedFromConfigPaths.value = data.lockedFromConfigPaths ?? [];
    lockMessage.value = data.lockMessage ?? "";
    error.value = null;
  } catch (e) {
    error.value = e instanceof Error ? e : new Error(String(e));
  } finally {
    loading.value = false;
  }
}

function onMediaSettingsChanged(): void {
  void load({ silent: true });
  // Keep the shared live view of media settings in sync after a save.
  void mutations.syncMediaSettings();
}

onMounted(() => {
  eventBus.on(MEDIA_SETTINGS_CHANGED_EVENT, onMediaSettingsChanged);
  void load();
});

onBeforeUnmount(() => {
  eventBus.off(MEDIA_SETTINGS_CHANGED_EVENT, onMediaSettingsChanged);
});

/** Only patch the fields owned by this section so prompt edits are not clobbered. */
async function save(): Promise<void> {
  try {
    const result = await saveMediaSettingsPatch({
      accelDevice: media.value.accelDevice,
      alwaysUseFfmpeg: media.value.alwaysUseFfmpeg,
      debug: media.value.debug,
      transcode: { enabled: media.value.transcode.enabled },
    }, lockedFromConfigPaths.value);
    if (result.skipped) {
      notify.showSuccessToast(lockMessage.value || t("settings.lockedFromConfig"));
      return;
    }
    media.value = result.media;
    void mutations.syncMediaSettings();
    notify.showSuccessToast(t("settings.settingsUpdated"));
  } catch (e) {
    notify.showError((e instanceof Error && e.message) || t("media.updateFailed"));
  }
}

function onAccelDeviceChange(value: string): void {
  // Picking the device auto resolves to keeps the stored "auto" so the
  // fallback chain (iGPU -> dGPU -> software) still applies.
  const isAutoDevice = autoAccelDevice.value && value === autoAccelDevice.value;
  media.value.accelDevice = isAutoDevice ? "auto" : value;
  void save();
}

function openConversionFormats(): void {
  mutations.showPrompt({
    name: "media-conversion-formats",
    props: {
      title: t("media.imageVideoConversionFormats"),
      lockedFromConfigPaths: lockedFromConfigPaths.value,
      lockMessage: lockMessage.value,
    },
  });
}

function openShadowRegressions(): void {
  mutations.showPrompt({
    name: "shadow-regressions",
    props: {
      title: t("media.shadowRegressionsView"),
    },
  });
}

function openTranscodeSettings(): void {
  mutations.showPrompt({
    name: "media-transcode-settings",
    props: {
      title: t("media.transcodeSettings"),
      lockedFromConfigPaths: lockedFromConfigPaths.value,
      lockMessage: lockMessage.value,
    },
  });
}
</script>

<style scoped>
.card-content h3 {
  text-align: center;
}

.item {
  padding: 1em;
}

.settings-select-input {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.settings-select-input .no-padding {
  display: flex;
  padding: 0;
  align-items: center;
}

.settings-select-input .expand-dropdown {
  max-width: 20em;
}

.loading-hint {
  display: flex;
  justify-content: center;
  padding: 2em;
}
</style>
