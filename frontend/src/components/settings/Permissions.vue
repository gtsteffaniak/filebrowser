<template>
  <div class="settings-items">
    <h3>{{ $t("settings.permissions-name") }}</h3>
    <p class="small">{{ $t("settings.permissionsHelp") }}</p>
    <ToggleSwitch class="item" v-model="permissions.download" :name="$t('settings.permissions.download')" />
    <ToggleSwitch class="item" v-model="permissions.modify" :name="$t('settings.permissions.modify')" />
    <ToggleSwitch class="item" v-model="permissions.create" :name="$t('settings.permissions.create')" />
    <ToggleSwitch class="item" v-model="permissions.delete" :name="$t('settings.permissions.delete')" />
    <ToggleSwitch class="item" v-model="permissions.admin" :name="$t('settings.permissions.admin')" />
    <ToggleSwitch class="item" v-model="permissions.share" :name="$t('settings.permissions.share')" />
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
};
</script>
