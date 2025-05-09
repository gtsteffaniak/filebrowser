<template>
  <div class="card" :class="{ active: active }">
    <div class="card-title">
      <h2>{{ $t("settings.profileSettings") }}</h2>
    </div>
    <div class="card-content">
      <form>
        <div class="card-content">
          <h3>Listing options</h3>
          <div class="settings-items">
            <ToggleSwitch
              class="item"
              v-model="localuser.dateFormat"
              :name="$t('settings.setDateFormat')"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.showHidden"
              :name="`Show hidden files`"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.quickDownload"
              :name="`Always show download icon for quick access`"
            />
          </div>
          <h3>File preview options</h3>
          <div class="settings-items">
            <ToggleSwitch
              class="item"
              v-model="localuser.preview.image"
              name="Preview images"
            />
            <ToggleSwitch
              v-if="mediaEnabled"
              class="item"
              v-model="localuser.preview.video"
              name="Preview videos"
            />
            <ToggleSwitch
              v-if="mediaEnabled"
              class="item"
              v-model="localuser.preview.motionVideoPreview"
              name="Motion previews for videos"
            />
            <ToggleSwitch
              v-if="mediaEnabled"
              class="item"
              v-model="localuser.preview.livePhotoPreview"
              name="Motion previews for live photos"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.preview.highQuality"
              name="Enable higher quality previews"
            />
            <ToggleSwitch
              v-if="hasOnlyOfficeEnabled"
              class="item"
              v-model="localuser.preview.office"
              name="Preview office files"
            />
            <ToggleSwitch
              class="item"
              v-model="localuser.preview.popup"
              name="Enable popup previewer"
            />
          </div>
          <div v-if="hasOnlyOfficeEnabled">
            <h3>Disable onlyoffice viewer for certain file extensions</h3>
            <p>
              A space-separated list of file extensions to disable the OnlyOffice viewer
              for. (e.g., <code>.txt .html</code>)
            </p>
            <div class="onlyoffice-group">
              <input
                class="input input--block onlyoffice-form"
                :class="{ 'invalid-form': !formValidation() }"
                type="text"
                placeholder="enter file extensions"
                id="onlyofficeExt"
                v-model="formOnlyOfficeExt"
              />
              <button type="button" class="button onlyoffice-button" @click="submitOnlyOfficeChange">
                save
              </button>
            </div>
          </div>

          <h3>Theme Color</h3>
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
import { onlyOfficeUrl, mediaAvailable } from "@/utils/constants.js";
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
      localuser: { preview: {} },
      initialized: false,
      formOnlyOfficeExt: "", // holds temporary input before saving
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
          "preview",
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

.onlyoffice-group {
  display: flex;
  flex-direction: row;
  align-items: flex-start;
}
.onlyoffice-button {
  margin-left: 1em;
}
.onlyoffice-form {
  height: 3em;
}
.invalid-form {
  border-color: red !important;
}
</style>
