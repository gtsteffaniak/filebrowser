<template>
  <SettingsItem :title="$t('settings.accountDefaults')" :collapsable="true" :start-collapsed="startCollapsed">
    <div class="settings-items">
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforced.lockPassword"
        v-model="account.lockPassword"
        @change="$emit('account-change', 'lockPassword')"
        @update:enforced="(v) => emitEnforced('lockPassword', v)"
        :disabled="isFieldDisabled('lockPassword')"
        :value-tooltip="fieldDisabledTooltip('lockPassword')"
        :name="$t('settings.lockPassword')"
      />
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforced.disableSettings"
        v-model="account.disableSettings"
        @change="$emit('account-change', 'disableSettings')"
        @update:enforced="(v) => emitEnforced('disableSettings', v)"
        :disabled="isFieldDisabled('disableSettings')"
        :value-tooltip="fieldDisabledTooltip('disableSettings')"
        :name="$t('settings.disableUserSettings')"
      />
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforced.disableUpdateNotifications"
        v-model="account.disableUpdateNotifications"
        @change="$emit('account-change', 'disableUpdateNotifications')"
        @update:enforced="(v) => emitEnforced('disableUpdateNotifications', v)"
        :disabled="isFieldDisabled('disableUpdateNotifications')"
        :value-tooltip="fieldDisabledTooltip('disableUpdateNotifications')"
        :name="$t('profileSettings.disableUpdateNotifications')"
        :description="$t('profileSettings.disableUpdateNotificationsDescription')"
      />
    </div>
    <div class="settings-items">
      <h3>{{ $t("general.permissions") }}</h3>
      <p class="small">{{ $t("settings.permissionsHelp") }}</p>
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforcedPermissions.admin"
        v-model="account.permissions.admin"
        @change="$emit('account-change', 'permissions.admin')"
        @update:enforced="(v) => emitEnforcedPermission('admin', v)"
        :disabled="isPermissionDisabled('admin')"
        :value-tooltip="permissionDisabledTooltip('admin')"
        :name="$t('settings.permissions.admin')"
      />
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforcedPermissions.share"
        v-model="account.permissions.share"
        @change="$emit('account-change', 'permissions.share')"
        @update:enforced="(v) => emitEnforcedPermission('share', v)"
        :disabled="isPermissionDisabled('share')"
        :value-tooltip="permissionDisabledTooltip('share')"
        :name="$t('general.shareFiles')"
      />
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforcedPermissions.api"
        v-model="account.permissions.api"
        @change="$emit('account-change', 'permissions.api')"
        @update:enforced="(v) => emitEnforcedPermission('api', v)"
        :disabled="isPermissionDisabled('api')"
        :value-tooltip="permissionDisabledTooltip('api')"
        :name="$t('settings.permissions.api')"
      />
      <ToggleSwitch
        class="item"
        :enforceable="enforceable"
        :enforced="!!enforcedPermissions.realtime"
        v-model="account.permissions.realtime"
        @change="$emit('account-change', 'permissions.realtime')"
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
    emitEnforced(field, value) {
      this.$emit("enforced-change", field, value);
    },
    emitEnforcedPermission(field, value) {
      this.$emit("enforced-permission-change", field, value);
    },
  },
};
</script>
