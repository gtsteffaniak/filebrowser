<template>
  <div class="card-content">
    <div class="settings-items">
      <ToggleSwitch class="item" v-model="localuser.disableUpdateNotifications" @change="updateSettings"
        :name="$t('profileSettings.disableUpdateNotifications')"
        :description="$t('profileSettings.disableUpdateNotificationsDescription')" />
      <ToggleSwitch class="item" v-model="analyticsEnabled" :disabled="analyticsLoading || !publishSupported"
        @update:model-value="updateAnalytics" :name="$t('settings.analyticsEnabled')"
        :description="$t('settings.analyticsEnabledDescription')" />
      <SettingsButton class="item" :name="$t('settings.analyticsView')"
        :description="$t('settings.analyticsViewDescription')" @activate="openAnalyticsPrompt" />
      <SettingsButton class="item" :name="$t('settings.configViewerOpen')"
        :description="$t('settings.configViewerDescription')" @activate="openConfigViewerPrompt" />
      <ToggleSwitch class="item" v-model="media.alwaysUseOnlyOffice"
        :disabled="isLocked('alwaysUseOnlyOffice')"
        :value-tooltip="isLocked('alwaysUseOnlyOffice') ? $t('settings.lockedFromConfig') : ''"
        :name="$t('media.alwaysUseOffice')"
        :description="$t('media.alwaysUseOfficeDescription')"
        @update:model-value="saveAlwaysUseOnlyOffice" />
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify/index.ts";
import { state, mutations } from "@/store/index.ts";
import { getAnalytics, updateAnalytics, getMediaSettings } from "@/api/settings.js";
import { emptyMediaSettings, mergeMediaSettings, saveMediaSettingsPatch } from "@/utils/mediaSettings.js";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import SettingsButton from "@/components/settings/SettingsButton.vue";

export default {
  name: "systemAdmin",
  components: {
    ToggleSwitch,
    SettingsButton,
  },
  data() {
    return {
      localuser: { disableUpdateNotifications: false },
      analyticsEnabled: false,
      publishSupported: false,
      analyticsLoading: false,
      media: emptyMediaSettings(),
      /** Media settings pinned in the config file cannot be changed here. */
      lockedFromConfigPaths: [],
      lockMessage: "",
    };
  },
  async mounted() {
    this.localuser = {
      disableUpdateNotifications: false,
      ...state.user,
    };
    await this.loadAnalytics();
    await this.loadMedia();
  },
  methods: {
    async loadMedia() {
      try {
        const data = await getMediaSettings();
        this.media = mergeMediaSettings(data);
        this.lockedFromConfigPaths = Array.isArray(data?.lockedFromConfigPaths)
          ? data.lockedFromConfigPaths
          : [];
        this.lockMessage = typeof data?.lockMessage === "string" ? data.lockMessage : "";
      } catch (e) {
        console.error(e);
        notify.showErrorToast(this.$t("media.updateFailed"));
      }
    },
    isLocked(path) {
      return this.lockedFromConfigPaths.includes(path);
    },
    async saveAlwaysUseOnlyOffice() {
      try {
        const { skipped, media } = await saveMediaSettingsPatch(
          { alwaysUseOnlyOffice: this.media.alwaysUseOnlyOffice },
          this.lockedFromConfigPaths,
        );
        if (skipped) {
          notify.showSuccessToast(this.lockMessage || this.$t("settings.lockedFromConfig"));
          return;
        }
        this.media = media;
        void mutations.syncMediaSettings();
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
      } catch (e) {
        notify.showError(e?.message || this.$t("media.updateFailed"));
      }
    },
    async updateSettings(event) {
      if (event && typeof event.preventDefault === "function") {
        event.preventDefault();
      }
      try {
        void mutations.updateCurrentUser(this.localuser);
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
      } catch (e) {
        console.error(e);
      }
    },
    async loadAnalytics() {
      try {
        const status = await getAnalytics();
        this.analyticsEnabled = status.enabled;
        this.publishSupported = status.publishSupported;
      } catch (e) {
        console.error(e);
        notify.showErrorToast(e?.message || this.$t("settings.analyticsLoadFailed"));
      }
    },
    async updateAnalytics() {
      if (!this.publishSupported) {
        this.analyticsEnabled = false;
        return;
      }
      this.analyticsLoading = true;
      try {
        const status = await updateAnalytics({ enabled: this.analyticsEnabled });
        this.analyticsEnabled = status.enabled;
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
      } catch (e) {
        console.error(e);
        notify.showErrorToast(e?.message || this.$t("settings.analyticsUpdateFailed"));
        await this.loadAnalytics();
      } finally {
        this.analyticsLoading = false;
      }
    },
    openAnalyticsPrompt() {
      mutations.showPrompt({
        name: "analytics-diagnostic",
        props: {
          title: this.$t("settings.analyticsView"),
        },
      });
    },
    openConfigViewerPrompt() {
      mutations.showPrompt({
        name: "config-viewer",
        props: {
          title: this.$t("settings.configViewer"),
        },
      });
    },
  },
};
</script>

<style scoped>
.card-content {
  margin-top: 1em;
}
</style>
