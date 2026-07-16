<template>
  <div class="card-content">
    <div class="settings-items">
      <ToggleSwitch class="item" v-model="localuser.disableUpdateNotifications" @change="updateSettings"
        :name="$t('profileSettings.disableUpdateNotifications')"
        :description="$t('profileSettings.disableUpdateNotificationsDescription')" />
      <ToggleSwitch class="item" v-model="analyticsEnabled" :disabled="analyticsLoading || !publishSupported"
        @update:modelValue="updateAnalytics" :name="$t('settings.analyticsEnabled')"
        :description="$t('settings.analyticsEnabledDescription')" />
      <SettingsButton class="item" :name="$t('settings.analyticsView')"
        :description="$t('settings.analyticsViewDescription')" @click="openAnalyticsPrompt" />
      <SettingsButton class="item" :name="$t('settings.configViewerOpen')"
        :description="$t('settings.configViewerDescription')" @click="openConfigViewerPrompt" />
    </div>
  </div>
</template>

<script>
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import { notify } from "@/notify";
import { state, mutations } from "@/store";
import * as settingsApi from "@/api/settings";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import SettingsButton from "@/components/settings/SettingsButton.vue";

export default {
  name: "systemAdmin",
  components: {
    ToggleSwitch,
    SettingsButton,
    Editor: createAsyncComponent(() => import('@/views/files/Editor.vue')),
  },
  data() {
    return {
      localuser: { disableUpdateNotifications: false },
      analyticsEnabled: false,
      publishSupported: false,
      analyticsLoading: false,
    };
  },
  async mounted() {
    this.localuser = {
      disableUpdateNotifications: false,
      ...state.user,
    };
    await this.loadAnalytics();
  },
  methods: {
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
        const status = await settingsApi.getAnalytics();
        this.analyticsEnabled = status.enabled;
        this.publishSupported = status.publishSupported;
      } catch (e) {
        console.error(e);
        notify.showErrorToast(this.$t("settings.analyticsLoadFailed"));
      }
    },
    async updateAnalytics() {
      if (!this.publishSupported) {
        this.analyticsEnabled = false;
        return;
      }
      this.analyticsLoading = true;
      try {
        const status = await settingsApi.patchAnalytics({ enabled: this.analyticsEnabled });
        this.analyticsEnabled = status.enabled;
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
      } catch (e) {
        console.error(e);
        notify.showErrorToast(this.$t("settings.analyticsUpdateFailed"));
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
