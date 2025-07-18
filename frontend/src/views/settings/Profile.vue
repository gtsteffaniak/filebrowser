<template>
  <div class="card" :class="{ active: active }">
    <div class="card-title">
      <h2>{{ $t("settings.profileSettings") }}</h2>
    </div>
    <div class="card-content">
      <form>
        <div class="card-content">
          <h3>{{ $t("profileSettings.sidebarOptions") }}</h3>
          <div class="settings-items">
            <ToggleSwitch
              class="item"
              v-model="localuser.disableQuickToggles"
              :name="$t('profileSettings.disableQuickToggles')"
              :description="$t('profileSettings.disableQuickTogglesDescription')"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.preview.disableHideSidebar"
              :name="$t('profileSettings.disableHideSidebar')"
              :description="$t('profileSettings.disableHideSidebarDescription')"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.hideSidebarFileActions"
              :name="$t('profileSettings.hideSidebarFileActions')"
            />

          </div>
        </div>
        <div class="card-content">
          <h3>{{ $t("settings.listingOptions") }}</h3>
          <div class="settings-items">
            <ToggleSwitch
              class="item"
              v-model="localuser.deleteWithoutConfirming"
              :name="$t('profileSettings.deleteWithoutConfirming')"
              :description="$t('profileSettings.deleteWithoutConfirmingDescription')"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.dateFormat"
              :name="$t('profileSettings.setDateFormat')"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.showHidden"
              :name="$t('profileSettings.showHiddenFiles')"
              :description="$t('profileSettings.showHiddenFilesDescription')"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.quickDownload"
              :name="$t('profileSettings.showQuickDownload')"
              :description="$t('profileSettings.showQuickDownloadDescription')"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.preview.image"
              :name="$t('profileSettings.previewImages')"
              :description="$t('profileSettings.previewImagesDescription')"
            />
            <ToggleSwitch
              v-if="mediaEnabled"
              class="item"
              v-model="localuser.preview.video"
              :name="$t('profileSettings.previewVideos')"
              :description="$t('profileSettings.previewVideosDescription')"
            />
            <ToggleSwitch
              v-if="mediaEnabled"
              class="item"
              v-model="localuser.preview.motionVideoPreview"
              :name="$t('profileSettings.previewMotionVideos')"
              :description="$t('profileSettings.previewMotionVideosDescription')"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.preview.highQuality"
              :name="$t('profileSettings.highQualityPreview')"
              :description="$t('profileSettings.highQualityPreviewDescription')"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.preview.office"
              :name="$t('profileSettings.previewOffice')"
              :description="$t('profileSettings.previewOfficeDescription')"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.preview.popup"
              :name="$t('profileSettings.popupPreview')"
              :description="$t('profileSettings.popupPreviewDescription')"
            />
          </div>
          <h3>{{ $t("profileSettings.editorViewerOptions") }}</h3>
          <div class="settings-items">
            <ToggleSwitch
                class="item"
                v-model="localuser.preview.autoplayMedia"
                :name="$t('profileSettings.autoplayMedia')"
                :description="$t('profileSettings.autoplayMediaDescription')"
              />
              <ToggleSwitch
                class="item"
                v-model="localuser.editorQuickSave"
                :name="$t('profileSettings.editorQuickSave')"
                :description="$t('profileSettings.editorQuickSaveDescription')"
              />
          </div>
          <h3>{{ $t("settings.searchOptions") }}</h3>
          <div class="settings-items">
            <ToggleSwitch
              class="item"
              v-model="localuser.disableSearchOptions"
              :name="$t('profileSettings.disableSearchOptions')"
              :description="$t('profileSettings.disableSearchOptionsDescription')"
            />
           </div>
           <h3 v-if="user.permissions.admin">{{ $t("settings.adminOptions") }}</h3>
          <div v-if="user.permissions.admin" class="settings-items">
            <ToggleSwitch
                v-if="localuser.permissions?.admin"
                class="item"
                v-model="localuser.disableUpdateNotifications"
                :name="$t('profileSettings.disableUpdateNotifications')"
                :description="$t('profileSettings.disableUpdateNotificationsDescription')"
              />
          </div>
          <div>
            <div class="centered-with-tooltip">
              <h3>{{ $t("profileSettings.disableThumbnailPreviews") }}</h3>
              <i class="no-select material-symbols-outlined tooltip-info-icon"
                @mouseenter="showTooltip($event, $t('profileSettings.disableThumbnailPreviewsDescription'))" @mouseleave="hideTooltip">
                help
              </i>
            </div>
            <div class="form-group">
              <input
                class="input input--block form-form flat-right"
                :class="{ 'invalid-form': !validateExtensions(localuser.disablePreviewExt) }"
                type="text"
                placeholder="enter file extensions"
                id="disablePreviews"
                v-model="localuser.disablePreviewExt"
              />
              <button
                type="button"
                class="button form-button"
                @click="updateSettings"
              >
                {{ $t("buttons.save") }}
              </button>
            </div>
          </div>
          <div>
            <div class="centered-with-tooltip">
              <h3>{{ $t("profileSettings.disableViewingFiles") }}</h3>
              <i class="no-select material-symbols-outlined tooltip-info-icon"
                @mouseenter="showTooltip($event, $t('profileSettings.disableViewingFilesDescription'))" @mouseleave="hideTooltip">
                help
              </i>
            </div>
            <div class="form-group">
              <input
                class="input input--block form-form flat-right"
                :class="{ 'invalid-form': !validateExtensions(localuser.disabledViewingExt) }"
                type="text"
                placeholder="enter file extensions"
                id="disableViewing"
                v-model="localuser.disabledViewingExt"
              />
              <button
                type="button"
                class="button form-button"
                @click="updateSettings"
              >
                {{ $t("buttons.save") }}
              </button>
            </div>
          </div>
          <h3>{{ $t("settings.themeColor") }}</h3>
          <ButtonGroup
            :buttons="colorChoices"
            @button-clicked="setColor"
            :initialActive="localuser.themeColor"
          />
          <h3>{{ $t("settings.language") }}</h3>
          <Languages
            class="input input--block"
            :locale="localuser.locale"
            @update:locale="updateLocale"
          ></Languages>
        </div>
        <div class="card-action">
          <button class="button button--flat" @click="updateSettings">{{ $t("buttons.save") }}</button>
        </div>
      </form>
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { mediaAvailable, muPdfAvailable } from "@/utils/constants.js";
import { state, mutations } from "@/store";
import { usersApi } from "@/api";
import Languages from "@/components/settings/Languages.vue";
import ButtonGroup from "@/components/ButtonGroup.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "settings",
  components: {
    Languages,
    ButtonGroup,
    ToggleSwitch,
  },
  data() {
    return {
      localuser: { preview: {}, permissions: {} }, // Initialize localuser with empty objects to avoid undefined errors
      initialized: false,
      colorChoices: [
        { label: this.$t("colors.blue"), value: "var(--blue)" },
        { label: this.$t("colors.red"), value: "var(--red)" },
        { label: this.$t("colors.green"), value: "var(--icon-green)" },
        { label: this.$t("colors.violet"), value: "var(--icon-violet)" },
        { label: this.$t("colors.yellow"), value: "var(--icon-yellow)" },
        { label: this.$t("colors.orange"), value: "var(--icon-orange)" },
      ],
    };
  },
  computed: {
    user() {
      return state.user;
    },
    muPdfAvailable() {
      return muPdfAvailable;
    },
    mediaEnabled() {
      return mediaAvailable;
    },
    settings() {
      return state.settings;
    },
    active() {
      return state.activeSettingsView === "profile-main";
    },
  },
  mounted() {
    this.localuser = JSON.parse(JSON.stringify(state.user));
  },
  methods: {
    showTooltip(event, text) {
      mutations.showTooltip({
        content: text,
        x: event.clientX,
        y: event.clientY,
      });
    },
    hideTooltip() {
      mutations.hideTooltip();
    },
    validateExtensions(value) {
      if (value === "") {
        return true;
      }
      const regex = /^\.\w+(?: \.\w+)*$/;
      return regex.test(value);
    },
    setColor(string) {
      this.localuser.themeColor = string;
    },
    async updateSettings(event) {
      if (event !== undefined) {
        event.preventDefault();
      }
      if (this.localuser.themeColor != "") {
        document.documentElement.style.setProperty(
          "--primaryColor",
          this.localuser.themeColor
        );
      }
      try {
        const data = this.localuser;
        mutations.updateCurrentUser(data);
        await usersApi.update(data, [
          "locale",
          "showHidden",
          "dateFormat",
          "themeColor",
          "quickDownload",
          "disablePreviewExt",
          "disabledViewingExt",
          "deleteWithoutConfirming",
          "preview",
          "disableQuickToggles",
          "disableSearchOptions",
          "hideSidebarFileActions",
          "editorQuickSave",
        ]);
        notify.showSuccess(this.$t("settings.settingsUpdated"));
      } catch (e) {
        notify.showError(e);
      }
    },
    updateLocale(updatedLocale) {
      this.localuser.locale = updatedLocale;
    },
  },
};
</script>

<style scoped>
.card-content h3 {
  text-align: center;
}
#disablePreviews,
#disableViewing {
  width: 80%;
}

.centered-with-tooltip {
  display: flex;
  justify-content: center;
  align-items: center;
}
</style>
