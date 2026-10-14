<template>
  <div class="card-content">
    <div v-if="loading" class="loading-hint">
      <LoadingSpinner size="medium" />
    </div>
    <template v-else>
      <div class="settings-items">
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
        <h3>{{ $t(profile.label) }}</h3>
        <ToggleSwitch
          class="item"
          v-model="media.transcode.profiles[profile.id].enabled"
          :name="$t('settings.transcodeProfileEnabled')"
          :description="$t('settings.transcodeProfileEnabledDescription')"
        />
        <div class="settings-number-input item">
          <div class="no-padding">
            <label :for="`profile-${profile.id}-maxResolution`">{{ $t("settings.transcodeProfileMaxResolution") }}</label>
            <HelpTooltipIcon :text="$t('settings.transcodeProfileMaxResolutionHelp')" />
          </div>
          <div class="no-padding">
            <input :id="`profile-${profile.id}-maxResolution`" class="sizeInput input"
              v-model.number="media.transcode.profiles[profile.id].maxResolution" type="number" min="0" />
          </div>
        </div>
        <div class="settings-number-input item">
          <div class="no-padding">
            <label :for="`profile-${profile.id}-avgBitrate`">{{ $t("settings.transcodeProfileAvgBitrate") }}</label>
            <HelpTooltipIcon :text="$t('settings.transcodeProfileAvgBitrateHelp')" />
          </div>
          <div class="no-padding">
            <input :id="`profile-${profile.id}-avgBitrate`" class="sizeInput input"
              v-model.number="media.transcode.profiles[profile.id].avgBitrateKbps" type="number" min="0" />
          </div>
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
import { getMediaSettings, patchMediaSettings } from "@/api/settings";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import {
  MEDIA_SETTINGS_CHANGED_EVENT,
  MEDIA_TRANSCODE_PROFILES,
  emptyMediaSettings,
  mergeMediaSettings,
} from "@/utils/mediaSettings";

export default {
  name: "MediaTranscodeSettings",
  components: {
    ToggleSwitch,
    HelpTooltipIcon,
    LoadingSpinner,
  },
  props: {
    promptId: {
      type: [String, Number],
      default: null,
    },
  },
  data() {
    return {
      media: emptyMediaSettings(),
      profiles: MEDIA_TRANSCODE_PROFILES,
      loading: true,
      saving: false,
    };
  },
  async mounted() {
    try {
      this.media = mergeMediaSettings(await getMediaSettings());
    } catch (e) {
      notify.showError(e);
      console.error(e);
    } finally {
      this.loading = false;
    }
  },
  methods: {
    close() {
      mutations.closeTopPrompt();
    },
    async save() {
      if (this.saving || this.loading) {
        return;
      }
      this.saving = true;
      try {
        await patchMediaSettings({ transcode: this.media.transcode });
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

.item {
  padding: 1em;
}

.loading-hint {
  display: flex;
  justify-content: center;
  padding: 2em;
}
</style>
