<template>
  <div class="card-content prompt-panel user-edit-preferences-prompt">
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
    <div class="card-actions">
      <button type="button" class="button button--flat" @click="closeTopPrompt">
        {{ $t("general.close") }}
      </button>
    </div>
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
    };
  },
  mounted() {
    this.session = getUserEditSession();
    this.sessionUnsubscribe = subscribeUserEditSession((session) => {
      this.session = session;
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
    editAccount() {
      const user = this.session?.user || {};
      const permissions = user.permissions || {};
      return {
        lockPassword: !!user.lockPassword,
        disableSettings: !!user.disableSettings,
        disableUpdateNotifications: !!user.disableUpdateNotifications,
        permissions: {
          admin: !!permissions.admin,
          share: !!permissions.share,
          api: !!permissions.api,
          realtime: !!permissions.realtime,
        },
      };
    },
  },
  methods: {
    onEditAccountChange(account) {
      if (!this.session?.user) {
        return;
      }
      const user = {
        ...this.session.user,
        lockPassword: account.lockPassword,
        disableSettings: account.disableSettings,
        disableUpdateNotifications: account.disableUpdateNotifications,
        permissions: {
          ...(this.session.user.permissions || {}),
          ...account.permissions,
        },
      };
      updateUserEditSession({ user });
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

<style scoped>
.user-edit-preferences-prompt .card-actions {
  margin-top: 1rem;
}
</style>
