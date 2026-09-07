<template>
  <div class="card-content no-buttons prompt-panel profile-advanced-prompt">
    <div class="profile-advanced-scroll">
      <UserProfilePreferences
        v-model="profileSections"
        :enforced="enforcedPreferences"
        show-extension-inputs
        show-thumbnail-master
        :default-expanded-section="null"
        @change="onPreferenceChange"
        @theme-color="onThemeColor"
        @locale-change="onLocaleChange"
      />
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { mutations, state } from "@/store";
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
  name: "profile-advanced",
  components: {
    UserProfilePreferences,
  },
  data() {
    return {
      localuser: { preview: {}, permissions: {} },
    };
  },
  computed: {
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
  },
  methods: {
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
    async updateSettings() {
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
        notify.showError(e?.message || e);
      }
    },
  },
};
</script>

<style scoped>
.profile-advanced-prompt {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

.profile-advanced-scroll {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.profile-advanced-prompt :deep(.settings-group) {
  margin-bottom: 0.75rem;
}
</style>
