<template>
  <div v-if="isMobile" class="card item clickable settings-card" @click="closeSettings">
    <span>
      <span class="material-symbols-outlined">close</span> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      {{ $t("general.exit") }}
    </span>
  </div>
  <div v-for="setting in settings" :key="setting.id + '-sidebar'" :id="setting.id + '-sidebar'" class="card item clickable settings-card"
    @click="setView(setting.id + '-main')" :class="{
      hidden: !shouldShow(setting),
      'active-settings': active(setting.id + '-main'),
    }">
    <span v-if="shouldShow(setting)" >{{ $t(setting.label) }}</span>
  </div>
</template>

<script>
import { state, getters, mutations } from "@/store";
import { settings } from "@/utils/constants";
import { router } from "@/router";

export default {
  name: "SidebarSettings",
  data() {
    return {
      settings, // Initialize the settings array in data
    };
  },
  computed: {
    currentHash: () => getters.currentHash(),
    isMobile: () => getters.isMobile(),
  },
  methods: {
    closeSettings() {
      router.go(-1);
    },
    shouldShow(setting) {
      const perm = setting?.permissions || {};
      // Check if all keys in setting.perm exist in state.user.perm and have truthy values
      return Object.keys(perm).every((key) => state.user.permissions[key]);
    },
    active: (view) => state.activeSettingsView === view,
    setView(view) {
      mutations.closeHovers();
      if (state.route.path != "/settings") {
        router.push({ path: "/settings", hash: "#" + view }, () => {});
      } else {
        mutations.setActiveSettingsView(view);
      }
    },
  },
};
</script>
<style>
.active-settings {
  background: var(--primaryColor) !important;
  color: white !important;
}

.settings-card {
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: unset !important;
  padding: 1em;
}
</style>
