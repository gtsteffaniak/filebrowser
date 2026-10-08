<template>
  <div class="settings-items">
    <h3>{{ $t("general.permissions") }}</h3>
    <p class="small">{{ $t("settings.permissionsHelp") }}</p>
    <ToggleSwitch class="item" :model-value="permissions.download" @update:model-value="(v) => updatePermission('download', v)" :name="downloadPermissionName" />
    <ToggleSwitch class="item" :model-value="permissions.modify" @update:model-value="(v) => updatePermission('modify', v)" :name="modifyPermissionName" />
    <ToggleSwitch class="item" :model-value="permissions.create" @update:model-value="(v) => updatePermission('create', v)" :name="createPermissionName" />
    <ToggleSwitch class="item" :model-value="permissions.delete" @update:model-value="(v) => updatePermission('delete', v)" :name="deletePermissionName" />
    <ToggleSwitch class="item" :model-value="permissions.admin" @update:model-value="(v) => updatePermission('admin', v)" :name="$t('settings.permissions.admin')" />
    <ToggleSwitch class="item" :model-value="permissions.share" @update:model-value="(v) => updatePermission('share', v)" :name="sharePermissionName" />
    <ToggleSwitch class="item" :model-value="permissions.api" @update:model-value="(v) => updatePermission('api', v)" :name="$t('settings.permissions.api')" />
    <ToggleSwitch class="item" :model-value="permissions.realtime" @update:model-value="(v) => updatePermission('realtime', v)" :name="$t('settings.permissions.realtime')" />
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
  emits: ["update:permissions"],
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
  methods: {
    updatePermission(key, value) {
      this.$emit("update:permissions", { ...this.permissions, [key]: value });
    },
  },
};
</script>
