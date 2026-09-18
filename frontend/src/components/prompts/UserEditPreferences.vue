<template>
  <div class="card-content user-edit-preferences-prompt">
    <UserDefaultsAccountSection
      v-if="session"
      :enforceable="false"
      :start-collapsed="false"
      :account="editAccount"
      :enforced="enforcedAccount"
      :enforced-permissions="enforcedAccountPermissions"
      respect-enforced-policy
      @account-change="onEditAccountChange"
    />
    <UserProfilePreferences
      v-if="session"
      v-model="profileSections"
      :enforced="enforcedPreferences"
      :default-expanded-section="null"
      respect-enforced-policy
      show-extension-inputs
      :show-thumbnail-master="false"
      @change="onPreferenceChange"
    />
  </div>
  <div class="card-actions">
    <button
      type="button"
      class="button button--flat"
      :aria-label="$t('general.close')"
      :title="$t('general.close')"
      @click="closeTopPrompt"
    >
      {{ $t("general.close") }}
    </button>
  </div>
</template>

<script>
import { mutations, state } from "@/store";
import UserDefaultsAccountSection from "@/components/settings/UserDefaultsAccountSection.vue";
import UserProfilePreferences from "@/components/settings/UserProfilePreferences.vue";
import {
  getUserEditSession,
  subscribeUserEditSession,
  updateUserEditSession,
  cloneUserEditSessionValue,
} from "@/utils/userEditSession";
import {
  sectionsFromFlatUser,
  applySectionsToFlatUser,
} from "@/utils/userProfileSections.js";

export default {
  name: "user-edit-preferences",
  components: {
    UserDefaultsAccountSection,
    UserProfilePreferences,
  },
  data() {
    return {
      session: null,
      sessionUnsubscribe: null,
      editAccount: {
        lockPassword: false,
        disableSettings: false,
        disableUpdateNotifications: false,
        showAdvancedProfile: false,
        permissions: {
          admin: false,
          share: false,
          api: false,
          realtime: false,
        },
      },
    };
  },
  mounted() {
    this.session = getUserEditSession();
    this.syncEditAccountForm();
    this.sessionUnsubscribe = subscribeUserEditSession((session) => {
      this.session = session;
      this.syncEditAccountForm();
    });
  },
  beforeUnmount() {
    if (this.sessionUnsubscribe) {
      this.sessionUnsubscribe();
      this.sessionUnsubscribe = null;
    }
  },
  computed: {
    profileSections: {
      get() {
        return sectionsFromFlatUser(this.session?.profileUser || {});
      },
      set(sections) {
        const profileUser = cloneUserEditSessionValue(this.session?.profileUser || {});
        applySectionsToFlatUser(profileUser, sections);
        updateUserEditSession({ profileUser });
      },
    },
    enforcedPreferences() {
      return state.enforcedUserDefaults || {};
    },
    enforcedAccount() {
      return this.enforcedPreferences.account || {};
    },
    enforcedAccountPermissions() {
      return this.enforcedAccount.permissions || {};
    },
  },
  methods: {
    syncEditAccountForm() {
      const user = this.session?.user || {};
      const permissions = user.permissions || {};
      this.editAccount.lockPassword = !!user.lockPassword;
      this.editAccount.disableSettings = !!user.disableSettings;
      this.editAccount.disableUpdateNotifications = !!user.disableUpdateNotifications;
      this.editAccount.showAdvancedProfile = !!user.showAdvancedProfile;
      this.editAccount.permissions = {
        admin: !!permissions.admin,
        share: !!permissions.share,
        api: !!permissions.api,
        realtime: !!permissions.realtime,
      };
    },
    persistAccountEdits() {
      if (!this.session?.user) {
        return;
      }
      const user = {
        ...this.session.user,
        lockPassword: this.editAccount.lockPassword,
        disableSettings: this.editAccount.disableSettings,
        disableUpdateNotifications: this.editAccount.disableUpdateNotifications,
        showAdvancedProfile: this.editAccount.showAdvancedProfile,
        permissions: {
          ...(this.session.user.permissions || {}),
          ...this.editAccount.permissions,
        },
      };
      const profileUser = cloneUserEditSessionValue(this.session.profileUser || {});
      const sections = sectionsFromFlatUser(profileUser);
      sections.account = {
        ...(sections.account || {}),
        lockPassword: this.editAccount.lockPassword,
        disableSettings: this.editAccount.disableSettings,
        disableUpdateNotifications: this.editAccount.disableUpdateNotifications,
        showAdvancedProfile: this.editAccount.showAdvancedProfile,
        permissions: { ...this.editAccount.permissions },
      };
      applySectionsToFlatUser(profileUser, sections);
      updateUserEditSession({ user, profileUser });
    },
    onEditAccountChange() {
      this.persistAccountEdits();
    },
    onPreferenceChange() {
      // profileSections setter persists changes
    },
    closeTopPrompt() {
      mutations.closeTopPrompt();
    },
  },
};
</script>
