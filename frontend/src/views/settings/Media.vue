<template>
  <div class="card-title">
    <h2>{{ $t("general.media") }}</h2>
  </div>
  <div class="card-content">
    <errors v-if="error" :errorCode="error.status" />
    <div v-else-if="loading" class="loading-hint">
      <LoadingSpinner size="medium" />
    </div>
    <template v-else>
      <div class="settings-items">
        <ToggleSwitch class="item" v-model="media.hardwareAcceleration"
          :disabled="!hardwareAccelerationAvailable || isLocked('hardwareAcceleration')"
          :value-tooltip="fieldTooltip('hardwareAcceleration', hardwareAccelerationDescription)"
          :name="$t('media.hardwareAcceleration')"
          :description="hardwareAccelerationDescription" />
        <ToggleSwitch class="item" v-model="media.extractEmbeddedSubtitles"
          :disabled="isLocked('extractEmbeddedSubtitles')"
          :value-tooltip="fieldTooltip('extractEmbeddedSubtitles')"
          :name="$t('media.extractEmbeddedSubtitles')"
          :description="$t('media.extractEmbeddedSubtitlesDescription')" />
        <ToggleSwitch class="item" v-model="media.debug" :name="$t('media.debug')"
          :disabled="isLocked('debug')"
          :value-tooltip="fieldTooltip('debug')"
          :description="$t('media.debugDescription')" />
        <SettingsButton class="item"
          :name="$t('media.imageVideoConversionFormats')"
          :description="$t('media.imageVideoConversionFormatsDescription')"
          @click="openConversionFormats" />
      </div>

      <div class="settings-items">
        <h3>{{ $t("media.transcode") }}</h3>
        <ToggleSwitch class="item" v-model="media.transcode.enabled"
          :disabled="isLocked('transcode.enabled')"
          :value-tooltip="fieldTooltip('transcode.enabled')"
          :name="$t('media.transcodeEnabled')"
          :description="$t('media.transcodeEnabledDescription')" />
        <SettingsButton class="item"
          :name="$t('media.transcodeSettings')"
          :description="$t('media.transcodeSettingsDescription')"
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
import { getMediaSettings } from "@/api/settings";
import { globalVars } from "@/utils/constants";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import SettingsButton from "@/components/settings/SettingsButton.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import Errors from "@/views/Errors.vue";
import {
  MEDIA_SETTINGS_CHANGED_EVENT,
  emptyMediaSettings,
  mergeMediaSettings,
  saveMediaSettingsPatch,
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
      /** Media settings pinned in the config file cannot be changed here. */
      lockedFromConfigPaths: [],
      lockMessage: "",
    };
  },
  computed: {
    /** Detection always runs, so this reflects whether any hardware encoder works. */
    hardwareAccelerationAvailable() {
      return globalVars?.hardwareAccelerationAvailable === true;
    },
    hardwareAccelerationDescription() {
      return this.hardwareAccelerationAvailable
        ? this.$t("media.hardwareAccelerationDescription")
        : this.$t("media.hardwareAccelerationUnavailable");
    },
  },
  mounted() {
    this.mediaSettingsHandler = () => {
      void this.load({ silent: true });
      // Keep the player's live view of media settings in sync after a save.
      void mutations.syncMediaPlayerSettings();
    };
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
        const data = await getMediaSettings();
        this.media = mergeMediaSettings(data);
        this.lockedFromConfigPaths = Array.isArray(data?.lockedFromConfigPaths)
          ? data.lockedFromConfigPaths
          : [];
        this.lockMessage = typeof data?.lockMessage === "string" ? data.lockMessage : "";
        this.error = null;
      } catch (e) {
        this.error = e;
      } finally {
        this.loading = false;
      }
    },
    isLocked(path) {
      return this.lockedFromConfigPaths.includes(path);
    },
    fieldTooltip(path, fallback = "") {
      return this.isLocked(path) ? this.$t("settings.lockedFromConfig") : fallback;
    },
    openConversionFormats() {
      mutations.showPrompt({
        name: "media-conversion-formats",
        props: {
          title: this.$t("media.imageVideoConversionFormats"),
          lockedFromConfigPaths: this.lockedFromConfigPaths,
          lockMessage: this.lockMessage,
        },
      });
    },
    openTranscodeSettings() {
      mutations.showPrompt({
        name: "media-transcode-settings",
        props: {
          title: this.$t("media.transcodeSettings"),
          lockedFromConfigPaths: this.lockedFromConfigPaths,
          lockMessage: this.lockMessage,
        },
      });
    },
    /** Only patch the fields owned by this section so prompt edits are not clobbered. */
    async save() {
      try {
        const { skipped, media } = await saveMediaSettingsPatch({
          hardwareAcceleration: this.media.hardwareAcceleration,
          extractEmbeddedSubtitles: this.media.extractEmbeddedSubtitles,
          debug: this.media.debug,
          transcode: { enabled: this.media.transcode.enabled },
        }, this.lockedFromConfigPaths);
        if (skipped) {
          notify.showSuccessToast(this.lockMessage || this.$t("settings.lockedFromConfig"));
          return;
        }
        this.media = media;
        void mutations.syncMediaPlayerSettings();
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
      } catch (e) {
        notify.showError(e?.message || this.$t("media.updateFailed"));
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
