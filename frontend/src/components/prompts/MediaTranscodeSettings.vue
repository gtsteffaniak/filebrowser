<template>
  <div class="card-content">
    <div v-if="loading" class="loading-hint">
      <LoadingSpinner size="medium" />
    </div>
    <template v-else>
      <div class="settings-items">
        <div class="settings-number-input item">
          <div class="no-padding">
            <label for="transcodeMaxConcurrent">{{ $t("media.transcodeMaxConcurrent") }}</label>
            <HelpTooltipIcon :text="isLocked('transcode.maxConcurrent') ? lockMessageText : $t('media.transcodeMaxConcurrentHelp')" />
          </div>
          <div class="no-padding">
            <input id="transcodeMaxConcurrent" class="sizeInput input" :disabled="isLocked('transcode.maxConcurrent')" :title="fieldTooltip('transcode.maxConcurrent')" v-model.number="media.transcode.maxConcurrent" type="number" min="1" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label for="transcodeMaxConcurrentPerUser">{{ $t("media.transcodeMaxConcurrentPerUser") }}</label>
            <HelpTooltipIcon :text="isLocked('transcode.maxConcurrentPerUser') ? lockMessageText : $t('media.transcodeMaxConcurrentPerUserHelp')" />
          </div>
          <div class="no-padding">
            <input id="transcodeMaxConcurrentPerUser" class="sizeInput input" :disabled="isLocked('transcode.maxConcurrentPerUser')" :title="fieldTooltip('transcode.maxConcurrentPerUser')" v-model.number="media.transcode.maxConcurrentPerUser" type="number" min="1" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label for="transcodeCacheMaxSizeMB">{{ $t("media.transcodeCacheMaxSizeMB") }}</label>
            <HelpTooltipIcon :text="isLocked('transcode.cacheMaxSizeMB') ? lockMessageText : $t('media.transcodeCacheMaxSizeMBHelp')" />
          </div>
          <div class="no-padding">
            <input id="transcodeCacheMaxSizeMB" class="sizeInput input" :disabled="isLocked('transcode.cacheMaxSizeMB')" :title="fieldTooltip('transcode.cacheMaxSizeMB')" v-model.number="media.transcode.cacheMaxSizeMB" type="number" min="0" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label for="transcodeCacheRetentionH">{{ $t("media.transcodeCacheRetentionH") }}</label>
            <HelpTooltipIcon :text="isLocked('transcode.cacheRetentionH') ? lockMessageText : $t('media.transcodeCacheRetentionHHelp')" />
          </div>
          <div class="no-padding">
            <input id="transcodeCacheRetentionH" class="sizeInput input" :disabled="isLocked('transcode.cacheRetentionH')" :title="fieldTooltip('transcode.cacheRetentionH')" v-model.number="media.transcode.cacheRetentionH" type="number" min="1" />
          </div>
        </div>
      </div>

      <div v-for="profile in profiles" :key="profile.id" class="settings-items">
        <h3>{{ $t(profile.label) }}</h3>
        <ToggleSwitch
          class="item"
          v-model="media.transcode.profiles[profile.id].enabled"
          :disabled="isLocked(`transcode.profiles.${profile.id}.enabled`)"
          :value-tooltip="fieldTooltip(`transcode.profiles.${profile.id}.enabled`)"
          :name="$t('media.transcodeProfileEnabled')"
          :description="$t('media.transcodeProfileEnabledDescription')"
        />
        <div class="settings-number-input item">
          <div class="no-padding">
            <label :for="`profile-${profile.id}-maxResolution`">{{ $t("media.transcodeProfileMaxResolution") }}</label>
            <HelpTooltipIcon :text="isLocked(`transcode.profiles.${profile.id}.maxResolution`) ? lockMessageText : $t('media.transcodeProfileMaxResolutionHelp')" />
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
            <label :for="`profile-${profile.id}-avgBitrate`">{{ $t("media.transcodeProfileAvgBitrate") }}</label>
            <HelpTooltipIcon :text="isLocked(`transcode.profiles.${profile.id}.avgBitrateKbps`) ? lockMessageText : $t('media.transcodeProfileAvgBitrateHelp')" />
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
            <label :for="`profile-${profile.id}-encoderBackend`">{{ $t("media.encoderBackend") }}</label>
            <HelpTooltipIcon :text="isLocked(`transcode.profiles.${profile.id}.encoderBackend`) ? lockMessageText : $t('media.transcodeProfileEncoderBackendHelp')" />
          </div>
          <ExpandDropdown
            :input-id="`profile-${profile.id}-encoderBackend`"
            :model-value="profileEncoderBackendValue(profile.id)"
            :options="backendSelectOptions"
            :aria-label="$t('media.encoderBackend')"
            :disabled="isLocked(`transcode.profiles.${profile.id}.encoderBackend`)"
            @update:modelValue="onProfileBackendChange(profile.id, $event)" />
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
      :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')"
    >
      {{ $t("general.cancel") }}
    </button>
    <button
      type="button"
      class="button button--flat button--blue"
      :disabled="saving || loading"
      @click="save"
      :aria-label="$t('general.save')"
      :title="$t('general.save')"
    >
      {{ $t("general.save") }}
    </button>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { mutations } from "@/store";
import { eventBus } from "@/store/eventBus";
import { getMediaSettings } from "@/api/settings";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import {
  MEDIA_SETTINGS_CHANGED_EVENT,
  MEDIA_TRANSCODE_PROFILES,
  emptyMediaSettings,
  mergeMediaSettings,
  saveMediaSettingsPatch,
} from "@/utils/mediaSettings";

export default {
  name: "MediaTranscodeSettings",
  components: {
    ToggleSwitch,
    ExpandDropdown,
    HelpTooltipIcon,
    LoadingSpinner,
  },
  props: {
    promptId: {
      type: [String, Number],
      default: null,
    },
    /** Media settings pinned in the config file cannot be changed here. */
    lockedFromConfigPaths: {
      type: Array,
      default: () => [],
    },
    lockMessage: {
      type: String,
      default: "",
    },
  },
  data() {
    return {
      media: emptyMediaSettings(),
      profiles: MEDIA_TRANSCODE_PROFILES,
      /** H.264 encoder backends reported by the backend (hardware ones only when usable). */
      encoderBackendOptions: [],
      loading: true,
      saving: false,
    };
  },
  async mounted() {
    try {
      const data = await getMediaSettings();
      this.media = mergeMediaSettings(data);
      this.encoderBackendOptions = Array.isArray(data?.encoderBackendOptions)
        ? data.encoderBackendOptions
        : [];
    } catch (e) {
      notify.showError(e);
      console.error(e);
    } finally {
      this.loading = false;
    }
  },
  computed: {
    lockMessageText() {
      return this.$t("settings.lockedFromConfig");
    },
    backendSelectOptions() {
      return this.encoderBackendOptions.map((option) => ({
        value: option.id,
        label: option.label || option.id,
      }));
    },
    /** The backend ffmpeg's auto selection resolves to (flagged default, else CPU). */
    defaultEncoderBackendId() {
      const flagged = this.encoderBackendOptions.find((option) => option.default);
      return flagged?.id || "cpu";
    },
    /** The resolved transcode-wide backend presets inherit from. */
    resolvedTranscodeBackend() {
      return this.media.transcode.encoderBackend || this.defaultEncoderBackendId;
    },
  },
  methods: {
    isLocked(path) {
      return this.lockedFromConfigPaths.includes(path);
    },
    fieldTooltip(path) {
      return this.isLocked(path) ? this.$t("settings.lockedFromConfig") : "";
    },
    /** A stored '' (inherit) displays as the resolved transcode-wide backend. */
    profileEncoderBackendValue(profileId) {
      return this.media.transcode.profiles[profileId]?.encoderBackend || this.resolvedTranscodeBackend;
    },
    onProfileBackendChange(profileId, value) {
      if (this.media.transcode.profiles[profileId]) {
        this.media.transcode.profiles[profileId].encoderBackend = value;
      }
    },
    close() {
      mutations.closeTopPrompt();
    },
    async save() {
      if (this.saving || this.loading) {
        return;
      }
      this.saving = true;
      try {
        // `transcode.enabled` belongs to the media section, so only send the
        // transcode object this prompt owns (locked paths are stripped for us).
        const { skipped, media } = await saveMediaSettingsPatch(
          { transcode: this.media.transcode },
          this.lockedFromConfigPaths,
        );
        if (!skipped) {
          this.media = media;
        }
        void mutations.syncMediaPlayerSettings();
        notify.showSuccess(this.$t("general.saved"));
        eventBus.emit(MEDIA_SETTINGS_CHANGED_EVENT);
        this.close();
      } catch (e) {
        notify.showError(e);
        console.error(e);
      } finally {
        this.saving = false;
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
