<template>
  <div class="card-title">
    <h2>{{ $t("settings.profileSettings") }}</h2>
  </div>
  <div class="card-content">
    <form>
      <div class="card-content">
        <SettingsItem aria-label="listingOptions" :title="$t('settings.listingOptions')" :collapsable="true"
          :force-collapsed="isSectionCollapsed('listingOptions')" @toggle="handleSectionToggle('listingOptions')">
          <div class="settings-items">
            <ToggleSwitch class="item" v-model="localuser.deleteWithoutConfirming" @change="updateSettings"
              :name="$t('profileSettings.deleteWithoutConfirming')"
              :description="$t('profileSettings.deleteWithoutConfirmingDescription')" />
            <ToggleSwitch class="item" v-model="localuser.dateFormat" @change="updateSettings"
              :name="$t('profileSettings.setDateFormat')" />
            <ToggleSwitch class="item" v-model="localuser.showHidden" @change="updateSettings"
              :name="$t('profileSettings.showHiddenFiles')"
              :description="$t('profileSettings.showHiddenFilesDescription')" />
            <ToggleSwitch class="item" v-model="localuser.quickDownload" @change="updateSettings"
              :name="$t('profileSettings.showQuickDownload')"
              :description="$t('profileSettings.showQuickDownloadDescription')" />
            <ToggleSwitch class="item" v-model="localuser.showSelectMultiple" @change="updateSettings"
              :name="$t('profileSettings.showSelectMultiple')"
              :description="$t('profileSettings.showSelectMultipleDescription')" />
          </div>
        </SettingsItem>
        <SettingsItem aria-label="thumbnailOptions" :title="$t('profileSettings.thumbnailOptions')" :collapsable="true"
          :start-collapsed="true" :force-collapsed="isSectionCollapsed('thumbnailOptions')"
          @toggle="handleSectionToggle('thumbnailOptions')">
          <div class="settings-items">
            <ToggleSwitch class="item" v-model="showThumbnailsForPreviews" @change="updateSettings"
              :name="$t('profileSettings.showThumbnails')"
              :description="$t('profileSettings.showThumbnailsDescription')" />
            <template v-if="showThumbnailsForPreviews">
              <ToggleSwitch class="item" v-model="localuser.preview.image" @change="updateSettings"
                :name="$t('general.images')" :description="$t('profileSettings.previewImagesDescription')" />
              <ToggleSwitch v-if="mediaEnabled" class="item" v-model="localuser.preview.video" @change="updateSettings"
                :name="$t('general.videos')" :description="$t('profileSettings.previewVideosDescription')" />
              <ToggleSwitch class="item" v-model="localuser.preview.office" @change="updateSettings"
                :name="$t('general.office')" :description="$t('profileSettings.previewOfficeDescription')" />
              <ToggleSwitch class="item" v-model="localuser.preview.folder" @change="updateSettings"
                :name="$t('general.folders')" :description="$t('profileSettings.previewFolderDescription')" />
              <ToggleSwitch class="item" v-model="localuser.preview.popup" @change="updateSettings"
                :name="$t('profileSettings.popupPreview')"
                :description="$t('profileSettings.popupPreviewDescription')" />
              <ToggleSwitch v-if="localuser.preview.popup && ((mediaEnabled && localuser.preview.video) || localuser.preview.folder)" class="item"
                v-model="localuser.preview.motionPreview" @change="updateSettings"
                :name="$t('profileSettings.previewMotion')"
                :description="$t('profileSettings.previewMotionVideosDescription')" />
            </template>
          </div>
        </SettingsItem>
        <SettingsItem aria-label="sidebarOptions" :title="$t('profileSettings.sidebarOptions')" :collapsable="true"
          :start-collapsed="true" :force-collapsed="isSectionCollapsed('sidebarOptions')"
          @toggle="handleSectionToggle('sidebarOptions')">
          <div class="settings-items">
            <ToggleSwitch class="item" v-model="localuser.disableQuickToggles" @change="updateSettings"
              :name="$t('profileSettings.disableQuickToggles')"
              :description="$t('profileSettings.disableQuickTogglesDescription')" />
            <ToggleSwitch class="item" v-model="localuser.preview.disableHideSidebar" @change="updateSettings"
              :name="$t('profileSettings.disableHideSidebar')"
              :description="$t('profileSettings.disableHideSidebarDescription')" />
            <ToggleSwitch class="item" v-model="localuser.hideSidebarFileActions" @change="updateSettings"
              :name="$t('profileSettings.hideSidebarFileActions')" />
          </div>
        </SettingsItem>
        <SettingsItem aria-label="searchOptions" :title="$t('settings.searchOptions')" :collapsable="true"
          :start-collapsed="true" :force-collapsed="isSectionCollapsed('searchOptions')"
          @toggle="handleSectionToggle('searchOptions')">
          <div class="settings-items">
            <ToggleSwitch class="item" v-model="localuser.disableSearchOptions" @change="updateSettings"
              :name="$t('profileSettings.disableSearchOptions')"
              :description="$t('profileSettings.disableSearchOptionsDescription')" />
          </div>
        </SettingsItem>
        <SettingsItem aria-label="fileViewerOptions" :title="$t('profileSettings.fileViewerOptions')"
          :collapsable="true" :start-collapsed="true" :force-collapsed="isSectionCollapsed('fileViewerOptions')"
          @toggle="handleSectionToggle('fileViewerOptions')">
          <div class="settings-items">
            <ToggleSwitch class="item" v-model="localuser.preview.defaultMediaPlayer" @change="updateSettings"
              :name="$t('profileSettings.defaultMediaPlayer')"
              :description="$t('profileSettings.defaultMediaPlayerDescription')" />
            <ToggleSwitch class="item" v-model="localuser.preview.autoplayMedia" @change="updateSettings"
              :name="$t('profileSettings.autoplayMedia')"
              :description="$t('profileSettings.autoplayMediaDescription')" />
            <ToggleSwitch class="item" v-model="localuser.editorQuickSave" @change="updateSettings"
              :name="$t('profileSettings.editorQuickSave')"
              :description="$t('profileSettings.editorQuickSaveDescription')" />
          </div>
          <div>
            <div class="centered-with-tooltip">
              <h3>{{ $t("profileSettings.disableThumbnailPreviews") }}</h3>
              <i class="no-select material-symbols-outlined tooltip-info-icon"
                @mouseenter="showTooltip($event, $t('profileSettings.disableThumbnailPreviewsDescription'))"
                @mouseleave="hideTooltip">
                help
              </i>
            </div>
            <div class="form-flex-group">
              <input class="input form-form flat-right disable-viewing"
                :class="{ 'form-invalid': !validateExtensions(formDisablePreviews) }" type="text"
                :placeholder="$t('profileSettings.disableFileExtensions')" v-model="formDisablePreviews" />
              <button type="button" class="button form-button flat-left" @click="submitDisablePreviewsChange">
                {{ $t("general.save") }}
              </button>
            </div>
          </div>
          <div>
            <div class="centered-with-tooltip">
              <h3>{{ $t("profileSettings.disableViewingFiles") }}</h3>
              <i class="no-select material-symbols-outlined tooltip-info-icon"
                @mouseenter="showTooltip($event, $t('profileSettings.disableViewingFilesDescription'))"
                @mouseleave="hideTooltip">
                help
              </i>
            </div>
            <div class="form-flex-group">
              <input class="input form-form flat-right disable-viewing"
                :class="{ 'form-invalid': !validateExtensions(formDisabledViewing) }" type="text"
                :placeholder="$t('profileSettings.disableFileExtensions')" v-model="formDisabledViewing" />
              <button type="button" class="button form-button flat-left" @click="submitDisabledViewingChange">
                {{ $t("general.save") }}
              </button>
            </div>
          </div>
          <div v-if="onlyOfficeAvailable">
            <div class="centered-with-tooltip">
              <h3>{{ $t("profileSettings.disableOfficeEditor") }}</h3>
              <i class="no-select material-symbols-outlined tooltip-info-icon"
                @mouseenter="showTooltip($event, $t('profileSettings.disableOfficeEditorDescription'))"
                @mouseleave="hideTooltip">
                help
              </i>
            </div>
            <div class="form-flex-group">
              <input class="input form-form flat-right"
                :class="{ 'form-invalid': !validateExtensions(formDisableOfficeViewing) }" type="text"
                :placeholder="$t('profileSettings.disableFileExtensions')" id="disableOfficeViewing"
                v-model="formDisableOfficeViewing" />
              <button type="button" class="button form-button flat-left" @click="submitDisableOfficeViewingChange">
                {{ $t("general.save") }}
              </button>
            </div>
            <div class="settings-items">
              <br />
              <ToggleSwitch class="item" v-model="localuser.debugOffice" @change="updateSettings"
                :name="$t('profileSettings.debugOfficeEditor')"
                :description="$t('profileSettings.debugOfficeEditorDescription')" />
            </div>

          </div>
        </SettingsItem>
        <SettingsItem aria-label="themeLanguage" :title="$t('profileSettings.themeAndLanguage')" :collapsable="true"
          :start-collapsed="true" :force-collapsed="isSectionCollapsed('themeLanguage')"
          @toggle="handleSectionToggle('themeLanguage')">
          <h4>{{ $t('settings.themeColor') }}</h4>
          <ButtonGroup :buttons="colorChoices" @button-clicked="setColor" :initialActive="localuser.themeColor" />

          <h4 v-if="Object.keys(availableThemes).length > 0">{{ $t('profileSettings.customTheme') }}</h4>
          <div v-if="Object.keys(availableThemes).length > 0" class="form-flex-group">
            <select class="input" v-model="selectedTheme" @change="updateSettings" aria-label="themeOptions">
              <option v-for="(theme, key) in availableThemes" :key="key" :value="key">
                {{ String(key) === 'default' ? $t('profileSettings.defaultThemeDescription') : `${key} -
                ${theme.description}` }}
              </option>
            </select>
          </div>

          <h4>{{ $t('general.language') }}</h4>
          <Languages class="input" :locale="localuser.locale" @update:locale="updateLocale"></Languages>
        </SettingsItem>
      </div>
    </form>
    <br />
  </div>
</template>

<script>
import { notify } from "@/notify";
import { globalVars } from "@/utils/constants.js";
import { state, mutations, getters } from "@/store";
import { usersApi } from "@/api";
import Languages from "@/components/settings/Languages.vue";
import ButtonGroup from "@/components/ButtonGroup.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import SettingsItem from "@/components/settings/SettingsItem.vue";

export default {
  name: "settings",
  components: {
    Languages,
    ButtonGroup,
    ToggleSwitch,
    SettingsItem,
  },
  data() {
    return {
      localuser: { preview: {}, permissions: {} }, // Initialize localuser with empty objects to avoid undefined errors
      formDisablePreviews: "", // holds temporary input before saving
      formDisabledViewing: "", // holds temporary input before saving
      formDisableOfficeViewing: "", // holds temporary input before saving
      expandedSection: 'listingOptions', // Track which section is currently expanded for accordion behavior
    };
  },
  computed: {
    colorChoices() {
      return [
        { label: this.$t("colors.blue"), value: "var(--blue)" },
        { label: this.$t("colors.red"), value: "var(--red)" },
        { label: this.$t("colors.green"), value: "var(--icon-green)" },
        { label: this.$t("colors.violet"), value: "var(--icon-violet)" },
        { label: this.$t("colors.yellow"), value: "var(--icon-yellow)" },
        { label: this.$t("colors.orange"), value: "var(--icon-orange)" },
      ];
    },
    availableThemes() {
      return globalVars.userSelectableThemes || {};
    },
    onlyOfficeAvailable() {
      return globalVars.onlyOfficeUrl !== "";
    },
    user() {
      return state.user;
    },
    muPdfAvailable() {
      return globalVars.muPdfAvailable;
    },
    mediaEnabled() {
      return globalVars.mediaAvailable;
    },
    settings() {
      return state.settings;
    },
    active() {
      return state.activeSettingsView === "profile-main";
    },
    selectedTheme: {
      get() {
        return this.localuser.customTheme || "default";
      },
      set(value) {
        this.localuser.customTheme = value;
      }
    },
    /** Pseudo-toggle: on if any thumbnail option is on; turning on enables only images by default, turning off disables all. */
    showThumbnailsForPreviews: {
      get() {
        const p = this.localuser.preview || {};
        return !!(p.image || p.video || p.motionVideoPreview || p.office || p.popup || p.folder);
      },
      set(enabled) {
        if (!this.localuser.preview) {
          this.localuser.preview = {};
        }
        if (enabled) {
          this.localuser.preview.image = true;
        } else {
          this.localuser.preview.image = false;
          this.localuser.preview.video = false;
          this.localuser.preview.motionVideoPreview = false;
          this.localuser.preview.office = false;
          this.localuser.preview.popup = false;
          this.localuser.preview.folder = false;
        }
      },
    },
  },
  mounted() {
    this.localuser = { ...state.user };
    if (getters.eventTheme() === "halloween" && !state.disableEventThemes) {
      this.localuser.themeColor = "";
    }
    this.formDisablePreviews = this.localuser.disablePreviewExt;
    this.formDisabledViewing = this.localuser.disableViewingExt;
    this.formDisableOfficeViewing = this.localuser.disableOnlyOfficeExt;
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
      if (value === "" || value === "*") {
        return true;
      }
      const regex = /^\.\w+(?: \.\w+)*$/;
      return regex.test(value);
    },
    submitDisablePreviewsChange() {
      if (!this.validateExtensions(this.formDisablePreviews)) {
        notify.showError("Invalid input, does not match requirement.");
        return;
      }
      this.localuser.disablePreviewExt = this.formDisablePreviews;
      this.updateSettings();
    },
    submitDisabledViewingChange() {
      if (!this.validateExtensions(this.formDisabledViewing)) {
        notify.showError("Invalid input, does not match requirement.");
        return;
      }
      this.localuser.disableViewingExt = this.formDisabledViewing;
      this.updateSettings();
    },
    submitDisableOfficeViewingChange() {
      if (!this.validateExtensions(this.formDisableOfficeViewing)) {
        notify.showError("Invalid input, does not match requirement.");
        return;
      }
      this.localuser.disableOnlyOfficeExt = this.formDisableOfficeViewing;
      this.updateSettings();
    },
    setColor(string) {
      if (getters.eventTheme() === "halloween" && !state.disableEventThemes) {
        mutations.disableEventThemes();
      }
      this.localuser.themeColor = string;
      this.updateSettings();
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
        const themeChanged = state.user.customTheme != this.localuser.customTheme;
        mutations.updateCurrentUser(data);
        await usersApi.update(data, [
          "locale",
          "showHidden",
          "dateFormat",
          "themeColor",
          "customTheme",
          "quickDownload",
          "disablePreviewExt",
          "disableViewingExt",
          "disableOnlyOfficeExt",
          "deleteWithoutConfirming",
          "preview",
          "disableQuickToggles",
          "disableSearchOptions",
          "hideSidebarFileActions",
          "editorQuickSave",
          "showSelectMultiple",
          "debugOffice",
        ]);
        if (themeChanged) {
          window.location.reload();
        }
        notify.showSuccessToast(
          this.$t('settings.settingsUpdated')
        );

      } catch (e) {
        console.error(e);
      }
    },
    updateLocale(updatedLocale) {
      this.localuser.locale = updatedLocale;
      this.updateSettings();
    },
    handleSectionToggle(sectionTitle) {
      // Accordion logic: if clicking the same section, collapse it, otherwise expand the new one
      if (this.expandedSection === sectionTitle) {
        this.expandedSection = null; // Collapse current section
      } else {
        this.expandedSection = sectionTitle; // Expand new section
      }
    },
    isSectionCollapsed(sectionTitle) {
      return this.expandedSection !== sectionTitle;
    },
  },
};
</script>

<style scoped>
.disable-viewing {
  width: 80%;
}

.centered-with-tooltip {
  display: flex;
  justify-content: center;
  align-items: center;
}
</style>
