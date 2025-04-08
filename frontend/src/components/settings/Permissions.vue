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
      {{ $t("settings.perm.modify") }}
    </p>
    <p>
      <input type="checkbox" :disabled="admin" v-model="perm.share" />
      {{ $t("settings.perm.share") }}
    </p>
    <p>
      <input type="checkbox" v-model="perm.api" />
      {{ $t("settings.perm.api") }}
    </p>
  </div>
</template>

<script>
export default {
  name: "permissions",
  props: ["perm"],
  computed: {
    admin: {
      get() {
        return this.perm.admin;
      },
      set(value) {
        if (value) {
          for (const key in this.perm) {
            this.perm[key] = true;
          }
        }

        this.perm.admin = value;
      },
    },
  },
};
</script>
