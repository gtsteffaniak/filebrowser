<template>
  <div>
    <h3>{{ $t("settings.permissions") }}</h3>
    <p class="small">{{ $t("settings.permissionsHelp") }}</p>
    <p>
      <input type="checkbox" v-model="admin" />
      {{ $t("settings.administrator") }}
    </p>
    <p>
      <input type="checkbox" :disabled="admin" v-model="perm.modify" />
      {{ $t("settings.permissions.modify") }}
    </p>
    <p>
      <input type="checkbox" :disabled="admin" v-model="perm.share" />
      {{ $t("settings.permissions.share") }}
    </p>
    <p>
      <input type="checkbox" v-model="perm.api" />
      {{ $t("settings.permissions.api") }}
    </p>
    <p>
      <input type="checkbox" v-model="perm.realtime" />
      realtime
    </p>
  </div>
</template>

<script>
export default {
  name: "permissions",
  props: ["permissions"],
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
