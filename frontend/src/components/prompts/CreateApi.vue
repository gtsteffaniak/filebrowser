<template>
  <div class="card-title">
    <h2>{{ $t('api.createTitle') }}</h2>
  </div>

  <div class="card-content">
    <!-- API Key Name Input -->
    <p>{{ $t('general.name') }}</p>
    <input v-focus class="input" type="text" v-model.trim="apiName"
      :placeholder="$t('api.keyNamePlaceholder')" />

    <!-- Duration Input -->
    <p>{{ $t('api.tokenDuration') }}</p>
    <div class="sizeInputWrapper">
      <input class="sizeInput roundedInputLeft input" v-model.number="duration" type="number" min="1"
        :placeholder="$t('api.durationNumberPlaceholder')" />
      <select v-model="unit" class="roundedInputRight input">
        <option value="days">{{ $t('api.days') }}</option>
        <option value="months">{{ $t('api.months') }}</option>
      </select>
    </div>

    <!-- Customize Token Option -->
    <div class="settings-items">
      <ToggleSwitch
        v-model="customizeToken"
        :name="$t('api.customizeToken')"
        class="item"
        :description="$t('api.customizeTokenInfo')"
      />
    </div>

    <!-- Permissions Input (only shown when customizing) -->
    <div v-if="customizeToken">
      <p>{{ $t('api.permissionNote') }}</p>
      <div class="settings-items">
        <ToggleSwitch v-for="(isEnabled, permission) in permissions" :key="permission" class="item"
          v-model="permissions[permission]" :name="permission" :disabled="!userPermissions[permission]" />
      </div>
    </div>
  </div>

  <div class="card-action">
    <button @click="closeHovers" class="button button--flat button--grey" :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button class="button button--flat button--blue" @click="createAPIKey" :title="$t('general.create')">
      {{ $t("general.create") }}
    </button>
  </div>
</template>

<script>
import { mutations } from "@/store";
import { notify } from "@/notify";
import { usersApi } from "@/api";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { eventBus } from "@/store/eventBus";

export default {
  name: "CreateAPI",
  data() {
    return {
      apiName: "",
      duration: 1,
      unit: "days",
      customizeToken: false, // false = minimal token (default), true = customizable/full token
    };
  },
  components: {
    ToggleSwitch,
  },
  props: {
    permissions: {
      type: Object,
      required: true,
    },
    userPermissions: {
      type: Object,
      required: true,
    },
  },
  computed: {
    durationInDays() {
      // Calculate duration based on unit
      return this.unit === "days" ? this.duration : this.duration * 30; // assuming 30 days per month
    },
  },
  methods: {
    closeHovers() {
      mutations.closeHovers();
    },
    async createAPIKey() {
      try {
        const params = {
          name: this.apiName,
          days: this.durationInDays,
          minimal: !this.customizeToken, // minimal = true when NOT customizing
        };

        // Only include permissions when customizing token
        if (this.customizeToken) {
          // Filter to get keys of permissions set to true and join them as a comma-separated string
          const permissionsString = Object.keys(this.permissions)
            .filter((key) => this.permissions[key])
            .join(",");
          params.permissions = permissionsString;
        }

        await usersApi.createApiKey(params);
        // Emit event to refresh API keys list
        eventBus.emit('apiKeysChanged');
        notify.showSuccessToast(this.$t("api.createKeySuccess"));
        mutations.closeHovers();
      } catch (error) {
        notify.showError($t("api.createKeyFailed"));
      }
    },
  },
};
</script>
<style scoped>
.sizeInputWrapper {
  display: flex !important;
}
.description {
  font-size: 0.9em;
  color: #666;
  margin-top: 0.5em;
}
.info-text {
  font-style: italic;
  color: #666;
  margin-top: 0.5em;
}
</style>