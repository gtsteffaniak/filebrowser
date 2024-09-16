<template>
  <div class="dashboard" style="padding-bottom: 30vh">
    <div v-if="isRootSettings" class="settings-views">
      <div
        v-for="setting in settings"
        :key="setting.id + '-main'"
        :id="setting.id + '-main'"
        :class="{
          active: active(setting.id + '-main'),
          clickable: !active(setting.id + '-main'),
        }"
        @click="!active(setting.id + '-main') && setView(setting.id + '-main')"
      >
        <!-- Dynamically render the component based on the setting -->
        <component v-if="shouldShow(setting)" :is="setting.component"></component>
      </div>
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
import UserSettings from "@/views/settings/User.vue";

export default {
  name: "settings",
  components: {
    UserManagement,
    UserSettings,
    GlobalSettings,
    ProfileSettings,
    SharesSettings,
  },
  data() {
    return {
      settings, // Initialize the settings array in data
    };
  },
  computed: {
    isRootSettings() {
      return state.route.path == "/settings";
    },
    newUserPage() {
      return state.route.path == "/settings/users/new";
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
  },
  mounted() {
    mutations.setActiveSettingsView(getters.currentHash());
  },
  methods: {
    shouldShow(setting) {
      if (state.isMobile) {
        const perm = setting?.perm || {};
        // Check if all keys in setting.perm exist in state.user.perm and have truthy values
        return Object.keys(perm).every((key) => state.user.perm[key]);
      }
      return this.active(setting.id + "-main");
    },
    active(id) {
      return state.activeSettingsView === id;
    },
    setView(view) {
      if (state.activeSettingsView === view) return;
      mutations.setActiveSettingsView(view);
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
  padding-bottom: 35vh;
  width: 100%;
}
.settings-views > .active > .card {
  border-style: solid;
  opacity: 1;
}
.settings-views .card {
  opacity: 0.3;
}
</style>
