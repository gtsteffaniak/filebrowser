<template>
  <div class="settings-items">
    <h3>{{ $t("general.permissions") }}</h3>
    <p class="small">{{ $t("settings.permissionsHelp") }}</p>
    <ToggleSwitch class="item" v-model="permissions.download" :name="downloadPermissionName" />
    <ToggleSwitch class="item" v-model="permissions.modify" :name="modifyPermissionName" />
    <ToggleSwitch class="item" v-model="permissions.create" :name="createPermissionName" />
    <ToggleSwitch class="item" v-model="permissions.delete" :name="deletePermissionName" />
    <ToggleSwitch class="item" v-model="permissions.admin" :name="$t('settings.permissions.admin')" />
    <ToggleSwitch class="item" v-model="permissions.share" :name="sharePermissionName" />
    <ToggleSwitch class="item" v-model="permissions.api" :name="$t('settings.permissions.api')" />
    <ToggleSwitch class="item" v-model="permissions.realtime" :name="$t('settings.permissions.realtime')" />
  </div>
</template>

<script>
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "permissions",
  props: {
    permissions: {
      type: Object,
      required: true,
    },
  },
  components: {
    ToggleSwitch,
  },
  computed: {
    downloadPermissionName() {
      return this.$t("general.downloadFiles");
    },
    modifyPermissionName() {
      return this.$t("general.editFiles");
    },
    createPermissionName() {
      return this.$t("general.createFiles");
    },
    deletePermissionName() {
      return this.$t("general.deleteFiles");
    },
    sharePermissionName() {
      return this.$t("general.shareFiles");
    },
  },
};
</script>
