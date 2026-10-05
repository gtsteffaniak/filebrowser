<template>
  <SettingsItem :title="$t('settings.accountDefaults')" :collapsable="true" :start-collapsed="startCollapsed">
    <div class="settings-items">
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforced.lockPassword"
        :model-value="account.lockPassword"
        @update:model-value="(v) => emitAccountChange('lockPassword', v)"
        @update:enforced="(v) => emitEnforced('lockPassword', v)"
        :disabled="isFieldDisabled('lockPassword')"
        :value-tooltip="fieldDisabledTooltip('lockPassword')"
        :name="$t('settings.lockPassword')"
      />
      <ToggleSwitch
        v-if="showRequirePasswordChange"
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforced.requirePasswordChange"
        :model-value="account.requirePasswordChange"
        @update:model-value="(v) => emitAccountChange('requirePasswordChange', v)"
        @update:enforced="(v) => emitEnforced('requirePasswordChange', v)"
        :disabled="isFieldDisabled('requirePasswordChange')"
        :value-tooltip="fieldDisabledTooltip('requirePasswordChange')"
        :name="$t('settings.requirePasswordChange')"
      />
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforced.disableSettings"
        :model-value="account.disableSettings"
        @update:model-value="(v) => emitAccountChange('disableSettings', v)"
        @update:enforced="(v) => emitEnforced('disableSettings', v)"
        :disabled="isFieldDisabled('disableSettings')"
        :value-tooltip="fieldDisabledTooltip('disableSettings')"
        :name="$t('settings.disableUserSettings')"
      />
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforced.disableUpdateNotifications"
        :model-value="account.disableUpdateNotifications"
        @update:model-value="(v) => emitAccountChange('disableUpdateNotifications', v)"
        @update:enforced="(v) => emitEnforced('disableUpdateNotifications', v)"
        :disabled="isFieldDisabled('disableUpdateNotifications')"
        :value-tooltip="fieldDisabledTooltip('disableUpdateNotifications')"
        :name="$t('profileSettings.disableUpdateNotifications')"
        :description="$t('profileSettings.disableUpdateNotificationsDescription')"
      />
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforced.showAdvancedProfile"
        :model-value="account.showAdvancedProfile"
        @update:model-value="(v) => emitAccountChange('showAdvancedProfile', v)"
        @update:enforced="(v) => emitEnforced('showAdvancedProfile', v)"
        :disabled="isFieldDisabled('showAdvancedProfile')"
        :value-tooltip="fieldDisabledTooltip('showAdvancedProfile')"
        :name="$t('profileSettings.showAdvancedProfile')"
        :description="$t('profileSettings.showAdvancedProfileDescription')"
      />
    </div>
    <div class="settings-items">
      <h3>{{ $t("general.permissions") }}</h3>
      <p class="small">{{ $t("settings.permissionsHelp") }}</p>
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforcedPermissions.admin"
        :model-value="account.permissions.admin"
        @update:model-value="(v) => emitAccountChange('permissions.admin', v)"
        @update:enforced="(v) => emitEnforcedPermission('admin', v)"
        :disabled="isPermissionDisabled('admin')"
        :value-tooltip="permissionDisabledTooltip('admin')"
        :name="$t('settings.permissions.admin')"
      />
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforcedPermissions.share"
        :model-value="account.permissions.share"
        @update:model-value="(v) => emitAccountChange('permissions.share', v)"
        @update:enforced="(v) => emitEnforcedPermission('share', v)"
        :disabled="isPermissionDisabled('share')"
        :value-tooltip="permissionDisabledTooltip('share')"
        :name="$t('general.shareFiles')"
      />
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforcedPermissions.api"
        :model-value="account.permissions.api"
        @update:model-value="(v) => emitAccountChange('permissions.api', v)"
        @update:enforced="(v) => emitEnforcedPermission('api', v)"
        :disabled="isPermissionDisabled('api')"
        :value-tooltip="permissionDisabledTooltip('api')"
        :name="$t('settings.permissions.api')"
      />
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforcedPermissions.realtime"
        :model-value="account.permissions.realtime"
        @update:model-value="(v) => emitAccountChange('permissions.realtime', v)"
        @update:enforced="(v) => emitEnforcedPermission('realtime', v)"
        :disabled="isPermissionDisabled('realtime')"
        :value-tooltip="permissionDisabledTooltip('realtime')"
        :name="$t('settings.permissions.realtime')"
      />
    </div>
  </SettingsItem>
</template>

<script>
import SettingsItem from "@/components/settings/SettingsItem.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { getObjectProperty } from "@/utils/object.js";

export default {
  name: "UserDefaultsAccountSection",
  components: {
    SettingsItem,
    ToggleSwitch,
  },
  props: {
    startCollapsed: {
      type: Boolean,
      default: true,
    },
    enforceable: {
      type: Boolean,
      default: true,
    },
    account: {
      type: Object,
      required: true,
    },
    enforced: {
      type: Object,
      default: () => ({}),
    },
    enforcedPermissions: {
      type: Object,
      default: () => ({}),
    },
    configLockedPaths: {
      type: Array,
      default: () => [],
    },
    respectEnforcedPolicy: {
      type: Boolean,
      default: false,
    },
    showRequirePasswordChange: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["account-change", "enforced-change", "enforced-permission-change"],
  methods: {
    isFieldLocked(field) {
      return this.configLockedPaths.includes(`account.${field}`);
    },
    isPermissionLocked(field) {
      return this.configLockedPaths.includes(`account.permissions.${field}`);
    },
    isFieldDisabled(field) {
      if (this.isFieldLocked(field)) {
        return true;
      }
      return this.respectEnforcedPolicy && !!getObjectProperty(this.enforced, field);
    },
    isPermissionDisabled(field) {
      if (this.isPermissionLocked(field)) {
        return true;
      }
      return this.respectEnforcedPolicy && !!getObjectProperty(this.enforcedPermissions, field);
    },
    fieldDisabledTooltip(field) {
      if (this.respectEnforcedPolicy && getObjectProperty(this.enforced, field)) {
        return this.$t("profileSettings.enforcedByAdmin");
      }
      return this.configLockTooltipForField(field);
    },
    permissionDisabledTooltip(field) {
      if (this.respectEnforcedPolicy && getObjectProperty(this.enforcedPermissions, field)) {
        return this.$t("profileSettings.enforcedByAdmin");
      }
      return this.configLockTooltipForPermission(field);
    },
    configLockTooltipForField(field) {
      if (this.isFieldLocked(field)) {
        return this.$t("settings.userDefaultFieldLockedFromConfig");
      }
      return "";
    },
    configLockTooltipForPermission(field) {
      if (this.isPermissionLocked(field)) {
        return this.$t("settings.userDefaultFieldLockedFromConfig");
      }
      return "";
    },
    emitAccountChange(field, value) {
      this.$emit("account-change", field, value);
    },
    emitEnforced(field, value) {
      this.$emit("enforced-change", field, value);
    },
    emitEnforcedPermission(field, value) {
      this.$emit("enforced-permission-change", field, value);
    },
  },
};
</script>
