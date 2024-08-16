<template>
  <div class="dashboard">
    <div class="settings-views">
      <UserColumnSettings></UserColumnSettings>
      <ProfileSettings></ProfileSettings>
      <ShareSettings></ShareSettings>
      <GlobalSettings></GlobalSettings>
      <UserDefaultSettings></UserDefaultSettings>
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
import { state, mutations } from "@/store";
import GlobalSettings from "@/views/settings/Global.vue";
import UserDefaultSettings from "@/views/settings/UserDefaults.vue";
import UserColumnSettings from "@/views/settings/UserColumn.vue";
import ProfileSettings from "@/views/settings/Profile.vue";
import ShareSettings from "@/views/settings/Shares.vue";
import UserSettings from "@/views/settings/User.vue";
import UsersSettings from "@/views/settings/Users.vue";

export default {
  name: "settings",
  components: {
    GlobalSettings,
    UserDefaultSettings,
    UserColumnSettings,
    ProfileSettings,
    ShareSettings,
    UsersSettings,
  },
  computed: {
    loading() {
      return state.loading;
    },
    user() {
      return state.user;
    },
    settingsEnabled() {
      return state.user.disableSettings == false;
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
}
.settings-views .card.active {
  border-style: solid;
  opacity: 1;
}
.settings-views .card {
  opacity: 0.3;
}
</style>