<template>
  <div class="card" :class="{ active: active }">
    <div class="card-title">
      <h2>{{ $t("settings.profileSettings") }}</h2>
    </div>
    <div class="card-content">
      <form>
        <div class="card-content">
          <div class="settings-items">
            <ToggleSwitch
              class="item"
              v-model="dateFormat"
              :name="$t('settings.setDateFormat')"
            />
            <ToggleSwitch class="item" v-model="showHidden" :name="`Show hidden files`" />
            <ToggleSwitch
              class="item"
              v-model="quickDownload"
              :name="`Always show download icon for quick access`"
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
                v-model="disableOnlyOfficeExt"
              />
              <button class="button onlyoffice-button" @click="submitOnlyOfficeChange">
                save
              </button>
            </div>
          </div>

          <h3>Theme Color</h3>
          <ButtonGroup
            :buttons="colorChoices"
            @button-clicked="setColor"
            :initialActive="color"
          />
          <h3>{{ $t("settings.language") }}</h3>
          <Languages
            class="input input--block"
            :locale="locale"
            @update:locale="updateLocale"
          ></Languages>
        </div>
      </form>
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { onlyOfficeUrl } from "@/utils/constants.js";
import { state, mutations } from "@/store";
import { usersApi } from "@/api";
import Languages from "@/components/settings/Languages.vue";
import i18n, { rtlLanguages } from "@/i18n";
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
      dateFormat: false,
      initialized: false,
      locale: "",
      color: "",
      showHidden: false,
      quickDownload: false,
      disableOnlyOfficeExt: "",
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
    showHidden: function () {
      if (this.initialized) {
        this.updateSettings(); // Only run if initialized
      }
    },
    quickDownload: function () {
      if (this.initialized) {
        this.updateSettings(); // Only run if initialized
      }
    },
    dateFormat: function () {
      if (this.initialized) {
        this.updateSettings(); // Only run if initialized
      }
    },
  },
  computed: {
    hasOnlyOfficeEnabled() {
      return onlyOfficeUrl != "";
    },
    settings() {
      return state.settings;
    },
    active() {
      return state.activeSettingsView === "profile-main";
    },
    user() {
      return state.user;
    },
  },
  created() {
    this.locale = state.user.locale;
    this.showHidden = state.user.showHidden;
    this.dateFormat = state.user.dateFormat;
    this.color = state.user.themeColor;
    this.quickDownload = state.user?.quickDownload;
    this.disableOnlyOfficeExt = state.user.disableOnlyOfficeExt;
  },
  mounted() {
    this.initialized = true;
  },
  methods: {
    formValidation() {
      if (this.disableOnlyOfficeExt == "") {
        return true;
      }
      let regex = /^\.\w+(?: \.\w+)*$/;
      return regex.test(this.disableOnlyOfficeExt);
    },
    submitOnlyOfficeChange(event) {
      if (!this.formValidation()) {
        notify.showError("Invalid input, does not match requirement.");
        return;
      }
      this.updateSettings(event);
    },
    setColor(string) {
      this.color = string;
      this.updateSettings();
    },
    async updateSettings(event) {
      if (event !== undefined) {
        event.preventDefault();
      }
      if (this.color != "") {
        document.documentElement.style.setProperty("--primaryColor", this.color);
      }
      try {
        const data = {
          id: state.user.id,
          locale: this.locale,
          showHidden: this.showHidden,
          dateFormat: this.dateFormat,
          themeColor: this.color,
          quickDownload: this.quickDownload,
          disableOnlyOfficeExt: this.disableOnlyOfficeExt,
        };
        const shouldReload =
          rtlLanguages.includes(data.locale) !== rtlLanguages.includes(i18n.locale);
        await usersApi.update(data, [
          "locale",
          "showHidden",
          "dateFormat",
          "themeColor",
          "quickDownload",
          "disableOnlyOfficeExt",
        ]);
        mutations.updateCurrentUser(data);
        if (shouldReload) {
          location.reload();
        }
        notify.showSuccess(this.$t("settings.settingsUpdated"));
      } catch (e) {
        notify.showError(e);
      }
    },
    updateLocale(updatedLocale) {
      this.locale = updatedLocale;
      this.updateSettings();
    },
  },
};
</script>

<style scoped>
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

.item {
  padding: 1em;
  border-radius: 1em;
}

.item:hover {
  background-color: var(--surfaceSecondary);
}
</style>
