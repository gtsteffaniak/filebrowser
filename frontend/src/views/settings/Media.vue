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
        <SettingsButton class="item"
          :name="$t('settings.imageVideoConversionFormats')"
          :description="$t('settings.imageVideoConversionFormatsDescription')"
          @click="openConversionFormats" />
      </div>

      <div class="settings-items">
        <h3>{{ $t("settings.transcode") }}</h3>
        <ToggleSwitch class="item" v-model="media.transcode.enabled"
          :name="$t('settings.transcodeEnabled')"
          :description="$t('settings.transcodeEnabledDescription')" />
        <SettingsButton class="item"
          :name="$t('settings.transcodeSettings')"
          :description="$t('settings.transcodeSettingsDescription')"
          :disabled="!media.transcode.enabled"
          @click="openTranscodeSettings" />
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
import { mutations } from "@/store";
import { eventBus } from "@/store/eventBus";
import { getMediaSettings, patchMediaSettings } from "@/api/settings";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import SettingsButton from "@/components/settings/SettingsButton.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import Errors from "@/views/Errors.vue";
import {
  MEDIA_SETTINGS_CHANGED_EVENT,
  emptyMediaSettings,
  mergeMediaSettings,
} from "@/utils/mediaSettings";

export default {
  name: "mediaSettings",
  components: {
    ToggleSwitch,
    SettingsButton,
    LoadingSpinner,
    Errors,
  },
  data() {
    return {
      media: emptyMediaSettings(),
      error: null,
      loading: true,
      mediaSettingsHandler: null,
    };
  },
  mounted() {
    this.mediaSettingsHandler = () => this.load({ silent: true });
    eventBus.on(MEDIA_SETTINGS_CHANGED_EVENT, this.mediaSettingsHandler);
    void this.load();
  },
  beforeUnmount() {
    if (this.mediaSettingsHandler) {
      eventBus.off(MEDIA_SETTINGS_CHANGED_EVENT, this.mediaSettingsHandler);
      this.mediaSettingsHandler = null;
    }
  },
  methods: {
    async load({ silent = false } = {}) {
      if (!silent) {
        this.loading = true;
      }
      try {
        this.media = mergeMediaSettings(await getMediaSettings());
        this.error = null;
      } catch (e) {
        this.error = e;
      } finally {
        this.loading = false;
      }
    },
    openConversionFormats() {
      mutations.showPrompt({
        name: "media-conversion-formats",
        props: {
          title: this.$t("settings.imageVideoConversionFormats"),
        },
      });
    },
    openTranscodeSettings() {
      mutations.showPrompt({
        name: "media-transcode-settings",
        props: {
          title: this.$t("settings.transcodeSettings"),
        },
      });
    },
    /** Only patch the fields owned by this section so prompt edits are not clobbered. */
    async save() {
      try {
        this.media = mergeMediaSettings(await patchMediaSettings({
          hardwareAcceleration: this.media.hardwareAcceleration,
          extractEmbeddedSubtitles: this.media.extractEmbeddedSubtitles,
          debug: this.media.debug,
          transcode: { enabled: this.media.transcode.enabled },
        }));
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
