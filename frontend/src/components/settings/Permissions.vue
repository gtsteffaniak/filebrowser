<template>
  <div class="settings-items">
    <h3>{{ $t("settings.permissions-name") }}</h3>
    <p class="small">{{ $t("settings.permissionsHelp") }}</p>
    <ToggleSwitch class="item" v-model="admin" :name="$t('settings.permissions.admin')" />
    <ToggleSwitch
      class="item"
      v-model="modify"
      :name="$t('settings.permissions.modify')"
    />
    <ToggleSwitch class="item" v-model="share" :name="$t('settings.permissions.share')" />
    <ToggleSwitch class="item" v-model="api" :name="$t('settings.permissions.api')" />
    <ToggleSwitch
      class="item"
      v-model="realtime"
      :name="$t('settings.permissions.realtime')"
    />
  </div>
</template>

<script>
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "permissions",
  props: ["permissions"],
  components: {
    ToggleSwitch,
  },
  computed: {
    admin: {
      get() {
        return this.permissions.admin;
      },
      set(value) {
        if (value) {
          for (const key in this.permissions) {
            this.permissions[key] = true;
          }
        }

        this.permissions.admin = value;
      },
    },
  },
};
</script>
