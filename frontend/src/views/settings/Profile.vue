<template>
  <div class="card" :class="{ active: active }">
    <div class="card-title">
      <h2>{{ $t("settings.profileSettings") }}</h2>
    </div>
    <div class="card-content">
      <form>
        <div class="card-content">
          <p>
            <input type="checkbox" v-model="dateFormat" />
            {{ $t("settings.setDateFormat") }}
          </p>
          <p>
            <input type="checkbox" v-model="showHidden" />
            show hidden files
          </p>
          <h3>Theme Color</h3>
          <ButtonGroup :buttons="colorChoices" @button-clicked="setColor" :initialActive="color" />
          <h3>{{ $t("settings.language") }}</h3>
          <Languages class="input input--block" :locale="locale" @update:locale="updateLocale"></Languages>
        </div>
      </form>
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { state, mutations } from "@/store";
import { usersApi } from "@/api";
import Languages from "@/components/settings/Languages.vue";
import i18n, { rtlLanguages } from "@/i18n";
import ButtonGroup from "@/components/ButtonGroup.vue";

export default {
  name: "settings",
  components: {
    Languages,
    ButtonGroup,
  },
  data() {
    return {
      dateFormat: false,
      initialized: false,
      locale: "",
      color: "",
      showHidden: false,
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
    dateFormat: function () {
      if (this.initialized) {
        this.updateSettings(); // Only run if initialized
      }
    },
  },
  computed: {
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
  },
  mounted() {
    this.initialized = true;
  },
  methods: {
    setColor(string) {
      this.color = string
      this.updateSettings()
    },
    async updateSettings(event) {
      if (event !== undefined) {
        event.preventDefault();
      }
      if (this.color != "") {
        document.documentElement.style.setProperty('--primaryColor', this.color);
      }
      try {
        const data = {
          id: state.user.id,
          locale: this.locale,
          showHidden: this.showHidden,
          dateFormat: this.dateFormat,
          themeColor: this.color,
        };
        const shouldReload =
          rtlLanguages.includes(data.locale) !== rtlLanguages.includes(i18n.locale);
        await usersApi.update(data, [
          "locale",
          "showHidden",
          "dateFormat",
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
