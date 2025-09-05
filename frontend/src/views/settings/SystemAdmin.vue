<template>
    <div class="card-title">
        <h2>{{ $t("settings.systemAdmin") }}</h2>
    </div>
    <div class="card-content">
        <div class="card-content">
            <div class="settings-items">
                <ToggleSwitch
                    v-model="localuser.disableUpdateNotifications"
                    @change="updateSettings"
                    :name="$t('profileSettings.disableUpdateNotifications')"
                    :description="$t('profileSettings.disableUpdateNotificationsDescription')"
                />
            </div>
        </div>
    </div>
</template>

<script>
import { notify } from "@/notify";
import { state, mutations } from "@/store";
import { usersApi } from "@/api";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "systemAdmin",
  components: {
    ToggleSwitch,
  },
  data() {
    return {
      localuser: { disableUpdateNotifications: false },
    };
  },
  computed: {},
  mounted() {
    this.localuser = { ...state.user };
  },
  methods: {
    async updateSettings(event) {
      if (event !== undefined) {
        event.preventDefault();
      }
      try {
        const data = this.localuser;
        mutations.updateCurrentUser(data);
        await usersApi.update(data, [
          "disableUpdateNotifications",
        ]);
        notify.showSuccess(this.$t("settings.settingsUpdated"));
      } catch (e) {
        notify.showError(e);
      }
    },
  },
  };
</script>
