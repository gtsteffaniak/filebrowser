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
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.preview.disableHideSidebar"
              :name="$t('profileSettings.disableHideSidebar')"
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
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.quickDownload"
              :name="$t('profileSettings.showQuickDownload')"
            />
                        <ToggleSwitch
              class="item"
              v-model="localuser.preview.image"
              :name="$t('profileSettings.previewImages')"
            />
            <ToggleSwitch
              v-if="mediaEnabled"
              class="item"
              v-model="localuser.preview.video"
              :name="$t('profileSettings.previewVideos')"
            />
            <ToggleSwitch
              v-if="mediaEnabled"
              class="item"
              v-model="localuser.preview.motionVideoPreview"
              :name="$t('profileSettings.previewMotionVideos')"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.preview.highQuality"
              :name="$t('profileSettings.highQualityPreview')"
            />
            <ToggleSwitch
              v-if="hasOnlyOfficeEnabled"
              class="item"
              v-model="localuser.preview.office"
              :name="$t('profileSettings.previewOffice')"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.preview.popup"
              :name="$t('profileSettings.popupPreview')"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.preview.autoplayMedia"
              :name="$t('profileSettings.autoplayMedia')"
            />
          </div>
          <h3>{{ $t("settings.searchOptions") }}</h3>
          <div>
            <ToggleSwitch
              class="item"
              v-model="localuser.disableSearchOptions"
              :name="$t('profileSettings.disableSearchOptions')"
            />
           </div>
           <h3 v-if="user.permissions.admin">{{ $t("settings.adminOptions") }}</h3>
          <div v-if="user.permissions.admin" class="settings-items">
            <ToggleSwitch
                v-if="localuser.permissions?.admin"
                class="item"
                v-model="localuser.disableUpdateNotifications"
                :name="$t('profileSettings.disableUpdateNotifications')"
              />
          </div>
          <div v-if="hasOnlyOfficeEnabled">
            <h3>{{ $t("settings.disableOfficePreview") }}</h3>
            <p>
              {{ $t("settings.disableOfficePreviewDescription") }}
            </p>
            <div class="form-group">
              <input
                class="input input--block form-form flat-right"
                :class="{ 'invalid-form': !formValidation() }"
                type="text"
                placeholder="enter file extensions"
                id="onlyofficeExt"
                v-model="formOnlyOfficeExt"
              />
              <button
                type="button"
                class="button form-button"
                @click="submitOnlyOfficeChange"
              >
                {{ $t("buttons.save") }}
              </button>
            </div>
          </div>

          <div v-if="muPdfAvailable">
            <h3>{{ $t("settings.disableOfficePreviews") }}</h3>
            <p>
              {{ $t("settings.disableOfficePreviewsDescription") }}
            </p>
            <div class="form-group">
              <input
                class="input input--block form-form flat-right"
                :class="{ 'invalid-form': !formValidationOfficePreviews() }"
                type="text"
                placeholder="enter file extensions"
                id="officePreviewExt"
                v-model="formOfficePreviewExt"
              />
              <button
                type="button"
                class="button form-button"
                @click="submitOfficePreviewsChange"
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
      </form>
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { onlyOfficeUrl, mediaAvailable, muPdfAvailable } from "@/utils/constants.js";
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
      formOnlyOfficeExt: "", // holds temporary input before saving
      formOfficePreviewExt: "", // holds temporary input before saving
      colorChoices: [
        { label: "blue", value: "var(--blue)" },
        { label: "red", value: "var(--red)" },
        { label: "green", value: "var(--icon-green)" },
        { label: "violet", value: "var(--icon-violet)" },
        { label: "yellow", value: "var(--icon-yellow)" },
        { label: "orange", value: "var(--icon-orange)" },
      ],
    };
  },
  watch: {
    localuser: {
      handler: function () {
        if (this.initialized) {
          this.updateSettings(); // Ensure updateSettings() is called when localuser changes
        }
        this.initialized = true;
      },
      deep: true, // Watch nested properties of localuser
    },
  },
  computed: {
    user() {
      return state.user;
    },
    muPdfAvailable() {
      return muPdfAvailable;
    },
    hasOnlyOfficeEnabled() {
      return onlyOfficeUrl != "";
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
    this.localuser = { ...state.user };
    this.formOnlyOfficeExt = this.localuser.disableOnlyOfficeExt;
    this.formOfficePreviewExt = this.localuser.disableOfficePreviewExt;
  },
  methods: {
    formValidation() {
      if (this.formOnlyOfficeExt == "") {
        return true;
      }
      let regex = /^\.\w+(?: \.\w+)*$/;
      return regex.test(this.formOnlyOfficeExt);
    },
    submitOnlyOfficeChange() {
      if (!this.formValidation()) {
        notify.showError("Invalid input, does not match requirement.");
        return;
      }
      this.localuser.disableOnlyOfficeExt = this.formOnlyOfficeExt;
    },
    formValidationOfficePreviews() {
      if (this.formOfficePreviewExt == "") {
        return true;
      }
      let regex = /^\.\w+(?: \.\w+)*$/;
      return regex.test(this.formOfficePreviewExt);
    },
    submitOfficePreviewsChange() {
      if (!this.formValidationOfficePreviews()) {
        notify.showError("Invalid input, does not match requirement.");
        return;
      }
      this.localuser.disableOfficePreviewExt = this.formOfficePreviewExt;
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
          "disableOnlyOfficeExt",
          "disableOfficePreviewExt",
          "deleteWithoutConfirming",
          "preview",
          "disableQuickToggles",
          "disableSearchOptions",
          "hideSidebarFileActions",
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
#officePreviewExt {
  width: 80%;
}
</style>
