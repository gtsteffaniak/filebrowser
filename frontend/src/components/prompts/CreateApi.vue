<template>
  <div class="card floating create-api__prompt__card" id="create-api">
    <div class="card-title">
      <h2>{{ $t("buttons.createAPIKey") }}</h2>
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
      <p>{{ $t("settings.apiDuration") }}</p>
      <div class="input-group">
        <input type="number" min="1" v-model.number="duration" />
        <select v-model="unit">
          <option value="days">{{ $t("time.days") }}</option>
          <option value="months">{{ $t("time.months") }}</option>
        </select>
      </div>

      <!-- Permissions Input -->
      <p>{{ $t("settings.apiPermissions") }}</p>
      <div>
        <label v-for="perm in availablePermissions" :key="perm">
          <input type="checkbox" :value="perm" v-model="permissions" />
          {{ perm }}
        </label>
      </div>
    </div>

    <div class="card-action">
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
import { notify } from "@/notify";

export default {
  name: "CreateAPIKey",
  data() {
    return {
      apiName: "",
      duration: "",
      unit: "hours",
      permissions: [],
      availablePermissions: ["read", "write", "delete"], // Define all possible permissions here
    };
  },
  methods: {
    async createAPIKey() {
      try {
        const durationInSeconds = this.calculateDurationInSeconds();
        const params = {
          name: this.apiName,
          duration: durationInSeconds,
          permissions: this.permissions.join(","),
        };
        console.log(durationInSeconds, params);
      } catch (error) {
        notify.showError(this.$t("errors.createKeyFailed"));
      }
    },
    calculateDurationInSeconds() {
      const timeUnits = {
        days: 86400,
        months: 86400 * 30,
      };
      return this.duration * timeUnits[this.unit];
    },
    resetForm() {
      this.apiName = "";
      this.duration = "";
      this.unit = "hours";
      this.permissions = [];
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
