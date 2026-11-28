<template>
  <div class="card-content">
    <div class="settings-items">
      <ToggleSwitch class="item" v-model="disableUpdateNotifications" @change="updateSettings"
        :name="t('profileSettings.disableUpdateNotifications')"
        :description="t('profileSettings.disableUpdateNotificationsDescription')" />
      <ToggleSwitch class="item" v-model="analyticsEnabled" :disabled="analyticsLoading || !publishSupported"
        @update:model-value="saveAnalytics" :name="t('settings.analyticsEnabled')"
        :description="t('settings.analyticsEnabledDescription')" />
      <SettingsButton class="item" :name="t('settings.analyticsView')"
        :description="t('settings.analyticsViewDescription')" @activate="openAnalyticsPrompt" />
      <SettingsButton class="item" :name="t('settings.configViewerOpen')"
        :description="t('settings.configViewerDescription')" @activate="openConfigViewerPrompt" />
      <ToggleSwitch class="item" v-model="media.alwaysUseOnlyOffice"
        :disabled="onlyOfficeDisabled"
        :value-tooltip="isLocked('alwaysUseOnlyOffice') ? t('settings.lockedFromConfig') : ''"
        :name="t('media.alwaysUseOffice')"
        :description="t('media.alwaysUseOfficeDescription')"
        @update:model-value="saveAlwaysUseOnlyOffice" />
      <SettingsButton v-if="mediaLoadFailed" class="item" :name="t('general.retry', { suffix: '' })"
        :description="t('media.updateFailed')" @activate="loadMedia" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useI18n } from "@/i18n/index.ts";
import { notify } from "@/notify/index.ts";
import { state, mutations } from "@/store/index.ts";
import { getAnalytics, updateAnalytics, getMediaSettings } from "@/api/settings.ts";
import { emptyMediaSettings, mergeMediaSettings, saveMediaSettingsPatch } from "@/utils/config/mediaSettings.ts";
import SettingsButton from "@/components/settings/SettingsButton.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import type { MediaSettings } from "@/types/media.ts";

const { t } = useI18n();

const disableUpdateNotifications = ref(state.user.disableUpdateNotifications ?? false);
const analyticsEnabled = ref(false);
const publishSupported = ref(false);
const analyticsLoading = ref(false);
const media = ref<MediaSettings>(emptyMediaSettings());
/** Media settings pinned in the config file cannot be changed here. */
const lockedFromConfigPaths = ref<string[]>([]);
const lockMessage = ref("");
/** True only after media settings and lock metadata both loaded. */
const mediaLoaded = ref(false);
const mediaLoadFailed = ref(false);
/** Last value the server confirmed. A failed or skipped save restores it. */
const confirmedOnlyOffice = ref(false);
const savingOnlyOffice = ref(false);

function isLocked(path: string): boolean {
  return lockedFromConfigPaths.value.includes(path);
}

const onlyOfficeDisabled = computed(
  () => !mediaLoaded.value || savingOnlyOffice.value || isLocked("alwaysUseOnlyOffice"),
);

async function loadMedia(): Promise<void> {
  mediaLoadFailed.value = false;
  try {
    const data = await getMediaSettings();
    media.value = mergeMediaSettings(data);
    lockedFromConfigPaths.value = data.lockedFromConfigPaths ?? [];
    lockMessage.value = data.lockMessage ?? "";
    confirmedOnlyOffice.value = media.value.alwaysUseOnlyOffice;
    mediaLoaded.value = true;
  } catch (e) {
    console.error(e);
    mediaLoaded.value = false;
    mediaLoadFailed.value = true;
    notify.showErrorToast(t("media.updateFailed"));
  }
}

async function saveAlwaysUseOnlyOffice(): Promise<void> {
  // Saves never overlap. A change that arrives during a save is reverted
  if (savingOnlyOffice.value) {
    media.value.alwaysUseOnlyOffice = confirmedOnlyOffice.value;
    return;
  }
  savingOnlyOffice.value = true;
  try {
    const result = await saveMediaSettingsPatch(
      { alwaysUseOnlyOffice: media.value.alwaysUseOnlyOffice },
      lockedFromConfigPaths.value,
    );
    if (result.skipped) {
      media.value.alwaysUseOnlyOffice = confirmedOnlyOffice.value;
      notify.showSuccessToast(lockMessage.value || t("settings.lockedFromConfig"));
      return;
    }
    media.value = result.media;
    confirmedOnlyOffice.value = result.media.alwaysUseOnlyOffice;
    void mutations.syncMediaSettings();
    notify.showSuccessToast(t("settings.settingsUpdated"));
  } catch (e) {
    // v-model already changed the local value. Show the value the server still has.
    media.value.alwaysUseOnlyOffice = confirmedOnlyOffice.value;
    notify.showError((e instanceof Error && e.message) || t("media.updateFailed"));
  } finally {
    savingOnlyOffice.value = false;
  }
}

function updateSettings(): void {
  void mutations.updateCurrentUser({ disableUpdateNotifications: disableUpdateNotifications.value });
  notify.showSuccessToast(t("settings.settingsUpdated"));
}

async function loadAnalytics(): Promise<void> {
  try {
    const status = await getAnalytics();
    analyticsEnabled.value = status.enabled;
    publishSupported.value = status.publishSupported;
  } catch (e) {
    console.error(e);
    notify.showErrorToast((e instanceof Error && e.message) || t("settings.analyticsLoadFailed"));
  }
}

async function saveAnalytics(): Promise<void> {
  if (!publishSupported.value) {
    analyticsEnabled.value = false;
    return;
  }
  analyticsLoading.value = true;
  try {
    const status = await updateAnalytics({ enabled: analyticsEnabled.value });
    analyticsEnabled.value = status.enabled;
    notify.showSuccessToast(t("settings.settingsUpdated"));
  } catch (e) {
    console.error(e);
    notify.showErrorToast((e instanceof Error && e.message) || t("settings.analyticsUpdateFailed"));
    await loadAnalytics();
  } finally {
    analyticsLoading.value = false;
  }
}

function openAnalyticsPrompt(): void {
  mutations.showPrompt({
    name: "analytics-diagnostic",
    props: { title: t("settings.analyticsView") },
  });
}

function openConfigViewerPrompt(): void {
  mutations.showPrompt({
    name: "config-viewer",
    props: { title: t("settings.configViewer") },
  });
}

onMounted(async () => {
  await loadAnalytics();
  await loadMedia();
});
</script>

<style scoped>
.card-content {
  margin-top: 1em;
}
</style>
