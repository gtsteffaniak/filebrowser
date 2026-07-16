<template>
  <div class="settings-items source-file-permissions">
    <ToggleSwitch class="item" v-model="permissions.view" :name="viewPermissionName" />
    <ToggleSwitch class="item" v-model="permissions.download" :name="downloadPermissionName" />
    <ToggleSwitch class="item" v-model="permissions.modify" :name="modifyPermissionName" />
    <ToggleSwitch class="item" v-model="permissions.create" :name="createPermissionName" />
    <ToggleSwitch class="item" v-model="permissions.delete" :name="deletePermissionName" />
  </div>
</template>

<script>
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "source-file-permissions",
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
    viewPermissionName() {
      return this.$t("general.viewFiles");
    },
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
  },
};
</script>

<style scoped>
.source-file-permissions {
  margin-top: 0.5em;
  padding-left: 0.5em;
  border-left: 2px solid var(--borderPrimary);
}
</style>
