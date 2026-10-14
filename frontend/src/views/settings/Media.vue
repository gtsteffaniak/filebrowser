<template>
  <div class="card-title">
    <h2>{{ $t("settings.media") }}</h2>
  </div>
  <div class="card-content">
    <errors v-if="error" :errorCode="error.status" />
    <div v-else-if="loading" class="loading-hint">
      <LoadingSpinner size="medium" />
    </div>
    <template v-else>
      <div class="settings-items">
        <ToggleSwitch class="item" v-model="media.hardwareAcceleration"
          :name="$t('settings.hardwareAcceleration')"
          :description="$t('settings.hardwareAccelerationDescription')" />
        <ToggleSwitch class="item" v-model="media.extractEmbeddedSubtitles"
          :name="$t('settings.extractEmbeddedSubtitles')"
          :description="$t('settings.extractEmbeddedSubtitlesDescription')" />
        <ToggleSwitch class="item" v-model="media.debug" :name="$t('settings.mediaDebug')"
          :description="$t('settings.mediaDebugDescription')" />
      </div>

      <div v-if="hasImagePreview" class="settings-items">
        <h3>{{ $t("settings.imagePreview") }}</h3>
        <ToggleSwitch v-for="(value, key) in media.convert.imagePreview" :key="`img-${key}`" class="item"
          v-model="media.convert.imagePreview[key]" :name="String(key)" />
      </div>

      <div v-if="hasVideoPreview" class="settings-items">
        <h3>{{ $t("settings.videoPreview") }}</h3>
        <ToggleSwitch v-for="(value, key) in media.convert.videoPreview" :key="`vid-${key}`" class="item"
          v-model="media.convert.videoPreview[key]" :name="String(key)" />
      </div>

      <div class="settings-items">
        <h3>{{ $t("settings.transcode") }}</h3>
        <ToggleSwitch class="item" v-model="media.transcode.enabled"
          :name="$t('settings.transcodeEnabled')"
          :description="$t('settings.transcodeEnabledDescription')" />
        <div class="settings-number-input item">
          <div class="no-padding">
            <label for="transcodeMaxConcurrent">{{ $t("settings.transcodeMaxConcurrent") }}</label>
            <HelpTooltipIcon :text="$t('settings.transcodeMaxConcurrentHelp')" />
          </div>
          <div class="no-padding">
            <input id="transcodeMaxConcurrent" class="sizeInput input" v-model.number="media.transcode.maxConcurrent" type="number" min="1" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label for="transcodeMaxConcurrentPerUser">{{ $t("settings.transcodeMaxConcurrentPerUser") }}</label>
            <HelpTooltipIcon :text="$t('settings.transcodeMaxConcurrentPerUserHelp')" />
          </div>
          <div class="no-padding">
            <input id="transcodeMaxConcurrentPerUser" class="sizeInput input" v-model.number="media.transcode.maxConcurrentPerUser" type="number" min="1" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label for="transcodeCacheMaxSizeMB">{{ $t("settings.transcodeCacheMaxSizeMB") }}</label>
            <HelpTooltipIcon :text="$t('settings.transcodeCacheMaxSizeMBHelp')" />
          </div>
          <div class="no-padding">
            <input id="transcodeCacheMaxSizeMB" class="sizeInput input" v-model.number="media.transcode.cacheMaxSizeMB" type="number" min="0" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label for="transcodeCacheRetentionH">{{ $t("settings.transcodeCacheRetentionH") }}</label>
            <HelpTooltipIcon :text="$t('settings.transcodeCacheRetentionHHelp')" />
          </div>
          <div class="no-padding">
            <input id="transcodeCacheRetentionH" class="sizeInput input" v-model.number="media.transcode.cacheRetentionH" type="number" min="1" />
          </div>
        </div>
      </div>

      <div v-for="profile in profiles" :key="profile.id" class="settings-items">
        <ToggleSwitch class="item" v-model="media.transcode.profiles[profile.id].enabled"
          :name="$t(profile.label)"
          :description="$t('settings.transcodeProfileEnabledDescription')" />
        <div class="settings-number-input item">
          <div class="no-padding">
            <label :for="`profile-${profile.id}-maxResolution`">{{ $t("settings.transcodeProfileMaxResolution") }}</label>
            <HelpTooltipIcon :text="$t('settings.transcodeProfileMaxResolutionHelp')" />
          </div>
          <div class="no-padding">
            <input :id="`profile-${profile.id}-maxResolution`" class="sizeInput input"
              v-model.number="media.transcode.profiles[profile.id].maxResolution"
              type="number" min="0" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label :for="`profile-${profile.id}-avgBitrate`">{{ $t("settings.transcodeProfileAvgBitrate") }}</label>
            <HelpTooltipIcon :text="$t('settings.transcodeProfileAvgBitrateHelp')" />
          </div>
          <div class="no-padding">
            <input :id="`profile-${profile.id}-avgBitrate`" class="sizeInput input"
              v-model.number="media.transcode.profiles[profile.id].avgBitrateKbps"
              type="number" min="0" />
          </div>
        </div>
      </div>

      <div class="card-actions">
        <button type="button" class="button button--flat" @click="save">
          {{ $t("general.save") }}
        </button>
      </div>
    </template>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { getMediaSettings, patchMediaSettings } from "@/api/settings";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import Errors from "@/views/Errors.vue";

const PROFILES = [
  { id: "quality", label: "general.quality" },
  { id: "balanced", label: "general.balanced" },
  { id: "datasaver", label: "general.dataSaver" },
];

function defaultProfile(maxResolution) {
  return { enabled: true, maxResolution, avgBitrateKbps: 0 };
}

function defaultMedia() {
  return {
    convert: { imagePreview: {}, videoPreview: {} },
    debug: false,
    extractEmbeddedSubtitles: false,
    hardwareAcceleration: false,
    transcode: {
      enabled: false,
      maxConcurrent: 2,
      maxConcurrentPerUser: 1,
      cacheMaxSizeMB: 10240,
      cacheRetentionH: 24,
      profiles: {
        quality: defaultProfile(0),
        balanced: defaultProfile(1080),
        datasaver: defaultProfile(720),
      },
    },
  };
}

function mergeMedia(loaded) {
  const base = defaultMedia();
  const value = loaded || {};
  return {
    ...base,
    ...value,
    convert: {
      imagePreview: { ...base.convert.imagePreview, ...(value.convert?.imagePreview || {}) },
      videoPreview: { ...base.convert.videoPreview, ...(value.convert?.videoPreview || {}) },
    },
    transcode: {
      ...base.transcode,
      ...(value.transcode || {}),
      profiles: {
        quality: { ...base.transcode.profiles.quality, ...(value.transcode?.profiles?.quality || {}) },
        balanced: { ...base.transcode.profiles.balanced, ...(value.transcode?.profiles?.balanced || {}) },
        datasaver: { ...base.transcode.profiles.datasaver, ...(value.transcode?.profiles?.datasaver || {}) },
      },
    },
  };
}

export default {
  name: "mediaSettings",
  components: {
    HelpTooltipIcon,
    ToggleSwitch,
    LoadingSpinner,
    Errors,
  },
  data() {
    return {
      media: defaultMedia(),
      profiles: PROFILES,
      error: null,
      loading: true,
    };
  },
  computed: {
    hasImagePreview() {
      return Object.keys(this.media.convert.imagePreview || {}).length > 0;
    },
    hasVideoPreview() {
      return Object.keys(this.media.convert.videoPreview || {}).length > 0;
    },
  },
  async mounted() {
    try {
      this.media = mergeMedia(await getMediaSettings());
    } catch (e) {
      this.error = e;
    } finally {
      this.loading = false;
    }
  },
  methods: {
    async save() {
      try {
        this.media = mergeMedia(await patchMediaSettings(this.media));
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
      } catch (e) {
        notify.showError(e?.message || this.$t("settings.settingsUpdateFailed"));
      }
    },
  },
};
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

.item {
  padding: 1em;
}

.card-actions {
  margin-top: 1em;
}

.loading-hint {
  display: flex;
  justify-content: center;
  padding: 2em;
}
</style>
