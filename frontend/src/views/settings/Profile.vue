<template>
  <div class="card-title">
    <h2>{{ profileSettingsLabel() }}</h2>
  </div>
  <div class="card-content">
    <form>
      <div class="card-content">
        <UserProfilePreferences
          v-model="profileSections"
          :enforced="enforcedPreferences"
          show-extension-inputs
          show-thumbnail-master
          @change="onPreferenceChange"
          @theme-color="onThemeColor"
          @locale-change="onLocaleChange"
        />
      </div>
    </form>
    <br />
  </div>
</template>

<script>
import { notify } from "@/notify";
import { mutations, state, getters } from "@/store";
import UserProfilePreferences from "@/components/settings/UserProfilePreferences.vue";
import {
  sectionsFromFlatUser,
  applySectionsToFlatUser,
} from "@/utils/userProfileSections.js";

function cloneUser(user) {
  return JSON.parse(
    JSON.stringify(user ?? { preview: {}, permissions: {} })
  );
}

export default {
  name: "settings",
  components: {
    UserProfilePreferences,
  },
  data() {
    return {
      localuser: { preview: {}, permissions: {} },
    };
  },
  computed: {
    user() {
      return state.user;
    },
    active() {
      return state.activeSettingsView === "profile-main";
    },
    profileSections: {
      get() {
        return sectionsFromFlatUser(this.localuser);
      },
      set(sections) {
        applySectionsToFlatUser(this.localuser, sections);
      },
    },
    enforcedPreferences() {
      return state.enforcedUserDefaults || {};
    },
  },
  mounted() {
    this.localuser = cloneUser(state.user);
    void mutations.syncEnforcedUserDefaults();
    if (getters.eventTheme() === "halloween" && !state.disableEventThemes) {
      this.localuser.themeColor = "";
    }
    if (typeof this.localuser.showToolsInSidebar !== "boolean") {
      this.localuser.showToolsInSidebar = true;
    }
  },
  methods: {
    profileSettingsLabel() {
      return this.$t("general.profileSettings");
    },
    onThemeColor(color) {
      if (color !== "") {
        document.documentElement.style.setProperty("--primaryColor", color);
      }
    },
    onPreferenceChange() {
      void this.updateSettings();
    },
    onLocaleChange() {
      void this.updateSettings();
    },
    async updateSettings(event) {
      if (typeof event?.preventDefault === "function") {
        event.preventDefault();
      }
      if (this.localuser.themeColor !== "") {
        document.documentElement.style.setProperty(
          "--primaryColor",
          this.localuser.themeColor
        );
      }
      try {
        const themeChanged = state.user.customTheme !== this.localuser.customTheme;
        await mutations.updateCurrentUser(this.localuser);
        this.localuser = cloneUser(state.user);
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
        if (themeChanged) {
          setTimeout(() => {
            window.location.reload();
          }, 1000);
        }
      } catch (e) {
        this.localuser = cloneUser(state.user);
        if (state.user.preview) {
          this.localuser.preview = { ...state.user.preview };
        }
        notify.showError(e?.message || e);
      }
    },
  },
};
</script>

<style scoped>
.card-content :deep(.settings-group) {
  margin-bottom: 0.75rem;
}
.settings-group {
  padding-top: 0.5em;
}
</style>
