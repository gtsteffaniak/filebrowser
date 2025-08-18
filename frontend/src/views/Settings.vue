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
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
        <span>{{ $t("files.loading") }}</span>
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
  mounted() {
    mutations.closeHovers();
    mutations.setSearch(false);
  },
  methods: {
    shouldShow(setting) {
      const perm = setting?.permissions || {};
      return Object.keys(perm).every((key) => state.user.permissions[key]);
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
  background: var(--surfacePrimary);
  padding: 0.5em;
  border-radius: 1em;
  color: white;
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
