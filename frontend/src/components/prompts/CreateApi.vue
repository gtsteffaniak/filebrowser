<template>
  <div class="card floating create-api__prompt__card" id="create-api">
    <div class="card-title">
      <h2>Create API Key</h2>
    </div>

    <div class="card-content">
      <!-- API Key Name Input -->
      <p>API Key Name</p>
      <input
        class="input input--block"
        type="text"
        v-model.trim="apiName"
        placeholder="enter a uinque api key name"
      />

      <!-- Duration Input -->
      <p>Token Duration</p>
      <div class="inputWrapper">
        <input
          class="sizeInput roundedInputLeft input"
          v-model.number="duration"
          type="number"
          min="1"
          placeholder="number"
        />
        <select v-model="unit" class="roundedInputRight input">
          <option value="days">days</option>
          <option value="months">months</option>
        </select>
      </div>

      <!-- Permissions Input -->
      <p>
        Choose at least one permission for the key. Your User must also have the
        permission.
      </p>
      <div>
        <p v-for="(isEnabled, perm) in availablePermissions" :key="permissions">
          <input type="checkbox" v-model="permissions[perm]" />
          {{ perm }}
        </p>
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
import { mutations, state } from "@/store";
import { notify } from "@/notify";
import { usersApi } from "@/api";

export default {
  name: "CreateAPI",
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
      return state.user.permissions;
    },
    durationInDays() {
      // Calculate duration based on unit
      return this.unit === "days" ? this.duration : this.duration * 30; // assuming 30 days per month
    },
  },
  created() {
    // Initialize permissions with the same structure as availablePermissions
    this.permissions = Object.fromEntries(
      Object.keys(this.availablePermissions).map((perm) => [perm, false])
    );
  },
  methods: {
    closeHovers() {
      mutations.closeHovers();
    },
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

        await usersApi.createApiKey(params);
        notify.showSuccess("successfully created!");
        window.location.reload();
      } catch (error) {
        notify.showError(this.$t("errors.createKeyFailed"));
      }
    },
  },
};
</script>
