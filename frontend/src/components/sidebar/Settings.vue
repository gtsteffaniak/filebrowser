<template>
  <div v-if="isMobile" role="button" class="card item clickable settings-card" @click="closeSettings">
    <span class="settings-item-content">
      <span class="material-symbols-outlined settings-icon">close</span> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      {{ $t("general.exit") }}
    </span>
  </div>
  <template v-for="setting in settings" :key="`${setting.id}-sidebar`">
    <div
      v-if="setting.id === 'profile'"
      class="card item settings-card-collapsible"
      :class="{ hidden: !shouldShow(setting) }"
    >
      <div
        role="button"
        class="settings-card-collapsible-header settings-card clickable"
        :class="{ 'active-settings': profileActive }"
        @click="setView('profile-main')"
      >
        <span class="settings-item-content">
          <span class="material-symbols-outlined settings-icon">{{ setting.icon }}</span>
          {{ settingLabel(setting) }}
        </span>
        <i
          v-if="showAdvancedProfile"
          role="button"
          class="material-symbols-outlined settings-card-collapsible-chevron"
          :class="{ rotated: sectionExpanded }"
          :aria-expanded="sectionExpanded"
          @click.stop="expandSection"
        >
          keyboard_arrow_down
        </i>
      </div>
      <div class="settings-card-sub-item" :class="{ 'settings-card-collapsible--expanded': sectionExpanded }">
        <div class="settings-card-sub-item-inner">
          <div
            v-for="section in profileSections"
            :key="section.id"
            role="button"
            class="settings-card-collapsible-sub-item settings-card clickable"
            :class="{ 'active-settings': active(`profile-${section.id}`) }"
            @click.stop="setView(`profile-${section.id}`)"
          >
            <span class="settings-item-content">
              <span class="material-symbols-outlined settings-icon">{{ section.icon }}</span>
              {{ $t(section.label) }}
            </span>
          </div>
        </div>
      </div>
    </div>
    <div
      v-else
      :id="`${setting.id}-sidebar`"
      role="button"
      class="card item clickable settings-card"
      @click="setView(`${setting.id}-main`)"
      :class="{
        hidden: !shouldShow(setting),
        'active-settings': active(`${setting.id}-main`),
      }">
      <span v-if="shouldShow(setting)" class="settings-item-content">
        <span class="material-symbols-outlined settings-icon">{{ setting.icon }}</span>
        {{ settingLabel(setting) }}
      </span>
    </div>
  </template>
</template>

<script>
import { state, getters, mutations } from "@/store";
import { getObjectProperty } from '@/utils/object.js';
import { settings } from "@/utils/constants";
import { router } from "@/router";

export default {
  name: "SidebarSettings",
  data() {
    return {
      settings, // Initialize the settings array in data
      sectionExpanded: false,
    };
  },
  computed: {
    currentHash: () => getters.currentHash(),
    isMobile: () => getters.isMobile(),
    isProfileSectionActive() {
      const hash = state.activeSettingsView || "";
      return hash.startsWith("profile-");
    },
    profileActive() {
      return this.isProfileSectionActive;
    },
    isProfileSubSectionActive() {
      const hash = state.activeSettingsView || "";
      return hash.startsWith("profile-") && hash !== "profile-main";
    },
    profileSections() {
      return this.settings.find((setting) => setting.id === 'profile')?.sections || [];
    },
    showAdvancedProfile() {
      return !!state.user?.showAdvancedProfile;
    },
  },
  watch: {
    isProfileSectionActive(val) {
      if (!val) this.sectionExpanded = false;
    },
    isProfileSubSectionActive(val) {
      if (val) this.sectionExpanded = true;
    },
    showAdvancedProfile(val) {
      if (val) this.sectionExpanded = true;
    },
  },
  mounted() {
    if (this.isProfileSubSectionActive || this.showAdvancedProfile) {
      requestAnimationFrame(() => { this.sectionExpanded = true; });
    }
  },
  methods: {
    expandSection() {
      this.sectionExpanded = !this.sectionExpanded;
    },
    closeSettings() {
      router.go(-1);
    },
    shouldShow(setting) {
      const perm = setting?.permissions || {};
      // Check if all keys in setting.perm exist in state.user.perm and have truthy values
      return Object.keys(perm).every((key) => getObjectProperty(state.user.permissions, key));
    },
    active: (view) => state.activeSettingsView === view,
    setView(view) {
      mutations.closeHovers();
      mutations.closeTopPrompt();
      void router.push({ path: "/settings", hash: `#${view}` }, () => {});
    },
    settingLabel(setting) {
      switch (setting.id) {
        case "profile":
          return this.$t("general.profileSettings");
        case "shares":
          return this.$t("general.shareSettings");
        case "users":
          return this.$t("general.userManagement");
        default:
          return this.$t(setting.label);
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

.active-settings .settings-icon,
.settings-card:hover .material-symbols-outlined {
  font-variation-settings: 'FILL' 1;
}

.settings-card {
  display: flex;
  align-items: center;
  overflow: unset !important;
  padding: 1em;
}

.settings-item-content {
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.settings-icon {
  font-size: 1.2em;
}
</style>

<style scoped>
.settings-card-collapsible {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  overflow: hidden;
}

.settings-card-collapsible-chevron {
  margin-left: auto;
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.settings-card-collapsible-chevron.rotated {
  transform: rotate(180deg);
}

.settings-card-sub-item {
  display: grid;
  grid-template-rows: 0fr;
  transition: grid-template-rows 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.settings-card-collapsible--expanded {
  grid-template-rows: 1fr;
}

.settings-card-sub-item-inner {
  overflow: hidden;
}

.settings-card-collapsible--expanded .settings-card-sub-item-inner {
  border-top: 1px solid var(--divider);
}

.settings-card-collapsible-header,
.settings-card-collapsible-sub-item,
.settings-card-collapsible .active-settings {
  margin: 0;
  border-radius: 0;
}

.settings-card-collapsible-sub-item {
  padding: 0.6em 1em 0.6em 2.25em;
}
</style>
