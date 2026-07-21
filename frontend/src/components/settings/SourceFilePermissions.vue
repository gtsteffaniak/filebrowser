<template>
  <div class="settings-items source-file-permissions">
    <ToggleSwitch
      class="item"
      :modelValue="permissions.view"
      @update:modelValue="(v) => setPermission('view', v)"
      :enforceable="enforceable"
      :enforced="!!enforcedPermissions.view"
      @update:enforced="(v) => $emit('enforced-change', 'view', v)"
      :name="viewPermissionName"
    />
    <ToggleSwitch
      class="item"
      :modelValue="permissions.download"
      @update:modelValue="(v) => setPermission('download', v)"
      :enforceable="enforceable"
      :enforced="!!enforcedPermissions.download"
      @update:enforced="(v) => $emit('enforced-change', 'download', v)"
      :name="downloadPermissionName"
    />
    <ToggleSwitch
      class="item"
      :modelValue="permissions.modify"
      @update:modelValue="(v) => setPermission('modify', v)"
      :enforceable="enforceable"
      :enforced="!!enforcedPermissions.modify"
      @update:enforced="(v) => $emit('enforced-change', 'modify', v)"
      :name="modifyPermissionName"
    />
    <ToggleSwitch
      class="item"
      :modelValue="permissions.create"
      @update:modelValue="(v) => setPermission('create', v)"
      :enforceable="enforceable"
      :enforced="!!enforcedPermissions.create"
      @update:enforced="(v) => $emit('enforced-change', 'create', v)"
      :name="createPermissionName"
    />
    <ToggleSwitch
      class="item"
      :modelValue="permissions.delete"
      @update:modelValue="(v) => setPermission('delete', v)"
      :enforceable="enforceable"
      :enforced="!!enforcedPermissions.delete"
      @update:enforced="(v) => $emit('enforced-change', 'delete', v)"
      :name="deletePermissionName"
    />
  </div>
</template>

<script>
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "source-file-permissions",
  emits: ["changed", "enforced-change"],
  props: {
    permissions: {
      type: Object,
      required: true,
    },
    enforceable: {
      type: Boolean,
      default: false,
    },
    enforcedPermissions: {
      type: Object,
      default: () => ({}),
    },
  },
  components: {
    ToggleSwitch,
  },
  data() {
    return {
      emitChanges: false,
    };
  },
  mounted() {
    this.$nextTick(() => {
      this.emitChanges = true;
    });
  },
  methods: {
    setPermission(key, value) {
      if (this.permissions[key] === value) {
        return;
      }
      this.permissions[key] = value;
      if (this.emitChanges) {
        this.$emit("changed");
      }
    },
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
