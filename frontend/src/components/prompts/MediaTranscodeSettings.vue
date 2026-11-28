<template>
  <div class="card-content">
    <div v-if="loading" class="loading-hint">
      <LoadingSpinner size="medium" />
    </div>
    <template v-else>
      <div class="settings-items">
        <div class="settings-number-input item">
          <div class="no-padding">
            <label for="transcodeMaxConcurrent">{{ t("media.transcodeMaxConcurrent") }}</label>
            <HelpTooltipIcon :text="isLocked('transcode.maxConcurrent') ? lockMessageText : t('media.transcodeMaxConcurrentHelp')" />
          </div>
          <div class="no-padding">
            <input id="transcodeMaxConcurrent" class="sizeInput input" :disabled="isLocked('transcode.maxConcurrent')" :title="fieldTooltip('transcode.maxConcurrent')" v-model.number="media.transcode.maxConcurrent" type="number" min="1" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label for="transcodeMaxConcurrentPerUser">{{ t("media.transcodeMaxConcurrentPerUser") }}</label>
            <HelpTooltipIcon :text="isLocked('transcode.maxConcurrentPerUser') ? lockMessageText : t('media.transcodeMaxConcurrentPerUserHelp')" />
          </div>
          <div class="no-padding">
            <input id="transcodeMaxConcurrentPerUser" class="sizeInput input" :disabled="isLocked('transcode.maxConcurrentPerUser')" :title="fieldTooltip('transcode.maxConcurrentPerUser')" v-model.number="media.transcode.maxConcurrentPerUser" type="number" min="1" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label for="transcodeCacheMaxSizeMB">{{ t("media.transcodeCacheMaxSizeMB") }}</label>
            <HelpTooltipIcon :text="isLocked('transcode.cacheMaxSizeMB') ? lockMessageText : t('media.transcodeCacheMaxSizeMBHelp')" />
          </div>
          <div class="no-padding">
            <input id="transcodeCacheMaxSizeMB" class="sizeInput input" :disabled="isLocked('transcode.cacheMaxSizeMB')" :title="fieldTooltip('transcode.cacheMaxSizeMB')" v-model.number="media.transcode.cacheMaxSizeMB" type="number" min="0" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label for="transcodeCacheRetentionH">{{ t("media.transcodeCacheRetentionH") }}</label>
            <HelpTooltipIcon :text="isLocked('transcode.cacheRetentionH') ? lockMessageText : t('media.transcodeCacheRetentionHHelp')" />
          </div>
          <div class="no-padding">
            <input id="transcodeCacheRetentionH" class="sizeInput input" :disabled="isLocked('transcode.cacheRetentionH')" :title="fieldTooltip('transcode.cacheRetentionH')" v-model.number="media.transcode.cacheRetentionH" type="number" min="1" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label for="transcodeReadRate">{{ t("media.transcodeReadRate") }}</label>
            <HelpTooltipIcon :text="isLocked('transcode.readRate') ? lockMessageText : t('media.transcodeReadRateHelp')" />
          </div>
          <div class="no-padding">
            <input id="transcodeReadRate" class="sizeInput input" :disabled="isLocked('transcode.readRate')" :title="fieldTooltip('transcode.readRate')" v-model.number="media.transcode.readRate" type="number" min="1" step="0.5" />
          </div>
        </div>
      </div>

      <div v-for="profile in profiles" :key="profile.id" class="settings-items">
        <h3>{{ profile.label() }}</h3>
        <ToggleSwitch
          class="item"
          v-model="media.transcode.profiles[profile.id].enabled"
          :disabled="isLocked(`transcode.profiles.${profile.id}.enabled`)"
          :value-tooltip="fieldTooltip(`transcode.profiles.${profile.id}.enabled`)"
          :name="t('media.transcodeProfileEnabled')"
          :description="t('media.transcodeProfileEnabledDescription')"
        />
        <div class="settings-number-input item">
          <div class="no-padding">
            <label :for="`profile-${profile.id}-maxResolution`">{{ t("media.transcodeProfileMaxResolution") }}</label>
            <HelpTooltipIcon :text="isLocked(`transcode.profiles.${profile.id}.maxResolution`) ? lockMessageText : t('media.transcodeProfileMaxResolutionHelp')" />
          </div>
          <div class="no-padding">
            <input :id="`profile-${profile.id}-maxResolution`" class="sizeInput input"
              :disabled="isLocked(`transcode.profiles.${profile.id}.maxResolution`)"
              :title="fieldTooltip(`transcode.profiles.${profile.id}.maxResolution`)"
              v-model.number="media.transcode.profiles[profile.id].maxResolution" type="number" min="0" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label :for="`profile-${profile.id}-avgBitrate`">{{ t("media.transcodeProfileAvgBitrate") }}</label>
            <HelpTooltipIcon :text="isLocked(`transcode.profiles.${profile.id}.avgBitrateKbps`) ? lockMessageText : t('media.transcodeProfileAvgBitrateHelp')" />
          </div>
          <div class="no-padding">
            <input :id="`profile-${profile.id}-avgBitrate`" class="sizeInput input"
              :disabled="isLocked(`transcode.profiles.${profile.id}.avgBitrateKbps`)"
              :title="fieldTooltip(`transcode.profiles.${profile.id}.avgBitrateKbps`)"
              v-model.number="media.transcode.profiles[profile.id].avgBitrateKbps" type="number" min="0" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label :for="`profile-${profile.id}-encoderBackend`">{{ t("media.encoderBackend") }}</label>
            <HelpTooltipIcon :text="isLocked(`transcode.profiles.${profile.id}.encoderBackend`) ? lockMessageText : t('media.transcodeProfileEncoderBackendHelp')" />
          </div>
          <ExpandDropdown
            :input-id="`profile-${profile.id}-encoderBackend`"
            :model-value="profileEncoderBackendValue(profile.id)"
            :options="backendSelectOptions"
            :aria-label="t('media.encoderBackend')"
            :disabled="isLocked(`transcode.profiles.${profile.id}.encoderBackend`)"
            @update:model-value="onProfileBackendChange(profile.id, $event)" />
        </div>
      </div>
    </template>
  </div>

  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--grey"
      :disabled="saving"
      @click="close"
      :aria-label="t('general.cancel')"
      :title="t('general.cancel')"
    >
      {{ t("general.cancel") }}
    </button>
    <button
      type="button"
      class="button button--flat button--blue"
      :disabled="saving || loading"
      @click="save"
      :aria-label="t('general.save')"
      :title="t('general.save')"
    >
      {{ t("general.save") }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useI18n } from "@/i18n/index.ts";
import { notify } from "@/notify/index.ts";
import { mutations } from "@/store/index.ts";
import { eventBus } from "@/store/eventBus.ts";
import { getMediaSettings } from "@/api/settings.ts";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import type { EncoderBackendOption, MediaSettings } from "@/types/media.ts";
import type { TranscodeProfile } from "@/types/transcode.ts";
import {
  MEDIA_SETTINGS_CHANGED_EVENT,
  MEDIA_TRANSCODE_PROFILES as profiles,
  emptyMediaSettings,
  mergeMediaSettings,
  saveMediaSettingsPatch,
} from "@/utils/config/mediaSettings.ts";

const props = withDefaults(
  defineProps<{
    promptId?: string | number | null;
    /** Media settings pinned in the config file cannot be changed here. */
    lockedFromConfigPaths?: string[];
    lockMessage?: string;
  }>(),
  { promptId: null, lockedFromConfigPaths: () => [], lockMessage: "" },
);

const { t } = useI18n();

const media = ref<MediaSettings>(emptyMediaSettings());
// H.264 encoder backends reported by the backend (hardware ones only when usable)
const encoderBackendOptions = ref<EncoderBackendOption[]>([]);
const loading = ref(true);
const saving = ref(false);

const lockMessageText = computed(() => t("settings.lockedFromConfig"));
const backendSelectOptions = computed(() => [
  { value: "", label: t("general.default") },
  ...encoderBackendOptions.value.map((option) => ({
    value: option.id,
    label: option.label || option.id,
  })),
]);

function isLocked(path: string): boolean {
  return props.lockedFromConfigPaths.includes(path);
}

function fieldTooltip(path: string): string {
  return isLocked(path) ? lockMessageText.value : "";
}

function profileEncoderBackendValue(profileId: TranscodeProfile): string {
  return media.value.transcode.profiles[profileId].encoderBackend;
}

function onProfileBackendChange(profileId: TranscodeProfile, value: string | null): void {
  media.value.transcode.profiles[profileId].encoderBackend = value ?? "";
}

function close(): void {
  mutations.closeTopPrompt();
}

async function save(): Promise<void> {
  if (saving.value || loading.value) {
    return;
  }
  saving.value = true;
  try {
    // `transcode.enabled` belongs to the media section, so only send the
    // transcode object this prompt owns (locked paths are stripped for us).
    const result = await saveMediaSettingsPatch({ transcode: media.value.transcode }, props.lockedFromConfigPaths);
    if (!result.skipped) {
      media.value = result.media;
    }
    void mutations.syncMediaSettings();
    notify.showSuccess(t("general.saved"));
    eventBus.emit(MEDIA_SETTINGS_CHANGED_EVENT);
    close();
  } catch (e) {
    notify.showError(e);
    console.error(e);
  } finally {
    saving.value = false;
  }
}

onMounted(async () => {
  try {
    const data = await getMediaSettings();
    media.value = mergeMediaSettings(data);
    encoderBackendOptions.value = data.encoderBackendOptions ?? [];
  } catch (e) {
    notify.showError(e);
    console.error(e);
  } finally {
    loading.value = false;
  }
});
</script>

<style scoped>
.card-content h3 {
  text-align: center;
}

.settings-number-input {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 1em;
}

.settings-number-input div {
  display: flex;
  padding: 0.5em;
  align-items: center;
}

.settings-number-input .expand-dropdown {
  max-width: 16em;
}

.item {
  padding: 1em;
}

.loading-hint {
  display: flex;
  justify-content: center;
  padding: 2em;
}

/* Settings pinned in the config file cannot be edited here. */
input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
