<template>
  <div class="card floating create-api__prompt__card" id="create-api">
    <div class="card-title">
      <h2>Create API Key</h2>
    </div>

    <div class="card-content">
      <!-- API Key Name Input -->
      <p>{{ $t("settings.apiName") }}</p>
      <input
        class="input input--block"
        type="text"
        v-model.trim="apiName"
        :placeholder="$t('settings.apiNamePlaceholder')"
      />

      <!-- Duration Input -->
      <p>API valid duration in Days</p>
      <div class="input-group">
        <input type="number" min="1" v-model.number="duration" />
        <select v-model="unit">
          <option value="days">days</option>
          <option value="months">months</option>
        </select>
      </div>

      <!-- Permissions Input -->
      <p>{{ $t("settings.apiPermissions") }}</p>
      <div>
        <label v-for="(isEnabled, perm) in availablePermissions" :key="perm">
          <input type="checkbox" v-model="permissions[perm]" />{{ perm }}
        </label>
      </div>
    </div>

    <div class="card-action">
      <button
        @click="closeHovers"
        class="button button--flat button--grey"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button
        class="button button--flat button--blue"
        @click="createAPIKey"
        :title="$t('buttons.create')"
      >
        {{ $t("buttons.create") }}
      </button>
    </div>
  </div>
</template>

<script>
import { state } from "@/store";
import { notify } from "@/notify";
import { usersApi } from "@/api";

export default {
  name: "CreateAPIKey",
  data() {
    return {
      apiName: "",
      duration: 1,
      unit: "days",
      permissions: {},
    };
  },
  computed: {
    availablePermissions() {
      return state.user.perm;
    },
    durationInDays() {
      // Calculate duration based on unit
      return this.unit === "days"
        ? this.duration
        : this.duration * 30; // assuming 30 days per month
    },
  },
  created() {
    // Initialize permissions with the same structure as availablePermissions
    this.permissions = Object.fromEntries(
      Object.keys(this.availablePermissions).map((perm) => [perm, false])
    );
  },
  methods: {
    async createAPIKey() {
      try {
        // Filter to get keys of permissions set to true and join them as a comma-separated string
        const permissionsString = Object.keys(this.permissions)
          .filter((key) => this.permissions[key])
          .join(",");

        const params = {
          name: this.apiName,
          days: this.durationInDays,
          permissions: permissionsString,
        };

        // Call the API to create the key
        usersApi.createApiKey(params);
        notify.showSuccess("successfully created!");
      } catch (error) {
        notify.showError(this.$t("errors.createKeyFailed"));
      }
    },
  },
};
</script>

<style scoped>
.input-group {
  display: flex;
  align-items: center;
}
.input-group input {
  flex: 1;
}
</style>
