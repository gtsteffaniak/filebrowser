<template>
  <div class="dashboard">
    <div v-if="isRootSettings && !userPage" class="settings-views">
      <component
        v-if="activeSetting"
        :is="activeSetting.component"
        :id="activeSetting.id + '-main'"
      />
    </div>
    <div v-else class="settings-views">
      <div class="active">
        <UserSettings />
      </div>
    </div>
    <div v-if="loading">
      <h2 class="message delayed">
        <LoadingSpinner size="medium" />
        <span>{{ $t("general.loading", { suffix: "..." }) }}</span>
      </h2>
    </div>
  </div>
</template>

<script>
import { state, getters, mutations } from "@/store";
import { settings } from "@/utils/constants";
import GlobalSettings from "@/views/settings/Global.vue";
import ProfileSettings from "@/views/settings/Profile.vue";
import SharesSettings from "@/views/settings/Shares.vue";
import UserManagement from "@/views/settings/Users.vue";
import AccessSettings from "@/views/settings/Access.vue";
import UserSettings from "@/views/settings/Users.vue";
import FileLoading from "@/views/settings/FileLoading.vue";
import ApiKeys from "@/views/settings/Api.vue";
import SystemAdmin from "@/views/settings/SystemAdmin.vue";
import NotificationsSettings from "@/views/settings/Notifications.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
export default {
  name: "settings",
  components: {
    UserManagement,
    GlobalSettings,
    ProfileSettings,
    SharesSettings,
    ApiKeys,
    AccessSettings,
    FileLoading,
    UserSettings,
    SystemAdmin,
    NotificationsSettings,
    LoadingSpinner,
  },
  data() {
    return {
      settings, // Initialize the settings array in data
    };
  },
  computed: {
    isRootSettings() {
      return getters.currentView() == "settings";
    },
    userPage() {
      return getters.routePath().startsWith(`/settings/users/`);
    },
    loading() {
      return getters.isLoading();
    },
    user() {
      return state.user;
    },
    currentHash() {
      return getters.currentHash();
    },
    activeSetting() {
      // Find the setting that matches the current activeSettingsView
      let active = this.settings.find(
        (setting) => `${setting.id}-main` === state.activeSettingsView && this.shouldShow(setting)
      );
      // Fallback: first allowed setting
      if (!active) {
        active = this.settings.find((setting) => this.shouldShow(setting));
      }
      return active;
    },
  },
  watch: {
    // Watch for route hash changes
    "$route.hash"() {
      this.initializeActiveSettingFromHash();
    }
  },
  mounted() {
    mutations.closeHovers();
    mutations.setSearch(false);
    this.initializeActiveSettingFromHash();
    // Listen for hash changes (browser navigation)
    window.addEventListener('hashchange', this.handleHashChange);
  },
  beforeUnmount() {
    // Clean up event listener
    window.removeEventListener('hashchange', this.handleHashChange);
  },
  methods: {
    /**
     * @param {any} setting
     */
    shouldShow(setting) {
      const perm = setting?.permissions || {};
      const userPermissions = /** @type {Record<string, boolean>} */ (state.user.permissions || {});
      return Object.keys(perm).every((key) => userPermissions[key]);
    },
    handleHashChange() {
      // Handle browser back/forward navigation
      this.initializeActiveSettingFromHash();
    },
    initializeActiveSettingFromHash() {
      // Get the current hash from the URL
      const hash = window.location.hash.replace('#', '');
      
      if (hash) {
        // Check if the hash corresponds to a valid setting
        const validSetting = this.settings.find(
          (setting) => `${setting.id}-main` === hash && this.shouldShow(setting)
        );
        
        if (validSetting) {
          // Set the active settings view to the hash value
          mutations.setActiveSettingsView(hash);
          return;
        }
      }
      
      // Default to profile-main if no hash or invalid hash
      const defaultSetting = this.settings.find(
        (setting) => setting.id === 'profile' && this.shouldShow(setting)
      );
      
      if (defaultSetting) {
        mutations.setActiveSettingsView('profile-main');
      } else {
        // Fallback to first allowed setting if profile is not available
        const firstAllowed = this.settings.find((setting) => this.shouldShow(setting));
        if (firstAllowed) {
          mutations.setActiveSettingsView(`${firstAllowed.id}-main`);
        }
      }
    },
  },
};
</script>

<style>

.dashboard {
  display: flex;
  flex-direction: column;
  height: 100%;
  align-items: center;
}

.settings-views {
  max-width: 1000px;
  width: 100%;
}

.settings-views .card-title {
  color: var(--textPrimary);
}

.settings-views .card {
  border-style: solid;
  opacity: 1;
}

.settings-items > .item {
  padding: 1em;
  border-radius: 1em;
}

.settings-items > .item:hover {
  background-color: var(--surfaceSecondary);
}



</style>
