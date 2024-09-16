<template>
  <div class="card" :class="{ active: active }">
    <div class="card-title">
      <h2>{{ $t("settings.profileSettings") }}</h2>
    </div>
    <div class="card-content">
      <form @submit="updateSettings">
        <div class="card-content">
          <p>
            <input type="checkbox" v-model="darkMode" />
            Dark Mode
          </p>
          <p>
            <input type="checkbox" v-model="hideDotfiles" />
            {{ $t("settings.hideDotfiles") }}
          </p>
          <p>
            <input type="checkbox" v-model="singleClick" />
            {{ $t("settings.singleClick") }}
          </p>
          <p>
            <input type="checkbox" v-model="dateFormat" />
            {{ $t("settings.setDateFormat") }}
          </p>
          <h3>Listing View Style</h3>
          <ViewMode
            class="input input--block"
            :viewMode="viewMode"
            @update:viewMode="updateViewMode"
          ></ViewMode>
          <br />
          <h3>Default View Size</h3>
          <p>
            Note: only applicable for normal and gallery views. Changes here will persist
            accross logins.
          </p>
          <div>
            <input
              v-model="gallerySize"
              type="range"
              id="gallary-size"
              name="gallary-size"
              :value="gallerySize"
              min="0"
              max="10"
            />
          </div>
          <h3>{{ $t("settings.language") }}</h3>
          <Languages
            class="input input--block"
            :locale="locale"
            @update:locale="updateLocale"
          ></Languages>
        </div>

        <div class="card-action">
          <input
            class="button button--flat"
            type="submit"
            :value="$t('buttons.update')"
          />
        </div>
      </form>
      <hr />
      <form v-if="!user.lockPassword" @submit="updatePassword">
        <div class="card-title">
          <h2>{{ $t("settings.changePassword") }}</h2>
        </div>

        <div class="card-content">
          <input
            :class="passwordClass"
            type="password"
            :placeholder="$t('settings.newPassword')"
            v-model="password"
            name="password"
          />
          <input
            :class="passwordClass"
            type="password"
            :placeholder="$t('settings.newPasswordConfirm')"
            v-model="passwordConf"
            name="password"
          />
        </div>

        <div class="card-action">
          <input
            class="button button--flat"
            type="submit"
            :value="$t('buttons.update')"
          />
        </div>
      </form>
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { state, mutations } from "@/store";
import { users } from "@/api";
import Languages from "@/components/settings/Languages.vue";
import ViewMode from "@/components/settings/ViewMode.vue";
import i18n, { rtlLanguages } from "@/i18n";

export default {
  name: "settings",
  components: {
    ViewMode,
    Languages,
  },
  data() {
    return {
      password: "",
      passwordConf: "",
      hideDotfiles: false,
      singleClick: false,
      dateFormat: false,
      darkMode: false,
      viewMode: "list",
      locale: "",
      gallerySize: 0,
    };
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
    passwordClass() {
      const baseClass = "input input--block";

      if (this.password === "" && this.passwordConf === "") {
        return baseClass;
      }

      if (this.password === this.passwordConf) {
        return `${baseClass} input--green`;
      }

      return `${baseClass} input--red`;
    },
  },
  created() {
    this.darkMode = state.user.darkMode;
    this.locale = state.user.locale;
    this.viewMode = state.user.viewMode;
    this.hideDotfiles = state.user.hideDotfiles;
    this.singleClick = state.user.singleClick;
    this.dateFormat = state.user.dateFormat;
    this.gallerySize = state.user.gallerySize;
  },
  watch: {
    gallerySize(newValue) {
      this.gallerySize = parseInt(newValue, 0); // Update the user object
    },
  },
  methods: {
    async updatePassword(event) {
      event.preventDefault();

      if (this.password !== this.passwordConf || this.password === "") {
        return;
      }

      try {
        let newUserSettings = state.user;
        newUserSettings.id = state.user.id;
        newUserSettings.password = this.password;
        await users.update(newUserSettings, ["password"]);
        notify.showSuccess(this.$t("settings.passwordUpdated"));
      } catch (e) {
        notify.showError(e);
      }
    },
    async updateSettings(event) {
      event.preventDefault();
      try {
        const data = {
          id: state.user.id,
          locale: this.locale,
          darkMode: this.darkMode,
          viewMode: this.viewMode,
          hideDotfiles: this.hideDotfiles,
          singleClick: this.singleClick,
          dateFormat: this.dateFormat,
          gallerySize: this.gallerySize,
        };
        const shouldReload =
          rtlLanguages.includes(data.locale) !== rtlLanguages.includes(i18n.locale);
        await users.update(data, [
          "locale",
          "darkMode",
          "viewMode",
          "hideDotfiles",
          "singleClick",
          "dateFormat",
          "gallerySize",
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
    updateViewMode(updatedMode) {
      this.viewMode = updatedMode;
    },
    updateLocale(updatedLocale) {
      this.locale = updatedLocale;
    },
  },
};
</script>
