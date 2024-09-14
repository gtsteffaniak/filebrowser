<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card">
    <div class="card-title">
      <h2>{{ $t("settings.userDefaults") }}</h2>
    </div>

    <div class="card-content">
      <p class="small">{{ $t("settings.defaultUserDescription") }}</p>

      <user-form
        :isNew="false"
        :isDefault="true"
        :user="settings.defaults"
        @update:user="updateUser"
      />
    </div>

    <div class="card-action">
      <input class="button button--flat" type="submit" :value="$t('buttons.update')" />
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { state, getters } from "@/store";
import { settings as api } from "@/api";
import { enableExec } from "@/utils/constants";
import UserForm from "@/components/settings/UserForm.vue";
//import Rules from "@/components/settings/Rules.vue";
import Errors from "@/views/Errors.vue";

export default {
  name: "settings",
  components: {
    UserForm,
    //Rules,
    Errors,
  },
  data: function () {
    return {
      error: null,
      originalSettings: null,
    };
  },
  computed: {
    settings() {
      return state.settings;
    },
    loading() {
      return getters.isLoading();
    },
    user() {
      return state.user;
    },
    isExecEnabled: () => enableExec,
  },
  methods: {
    updateRules(updatedRules) {
      state.settings.rules = updatedRules;
    },
    updateUser(updatedUser) {
      state.settings.defaults = updatedUser;
    },
    capitalize(name, where = "_") {
      if (where === "caps") where = /(?=[A-Z])/;
      let splitted = name.split(where);
      name = "";

      for (let i = 0; i < splitted.length; i++) {
        name += splitted[i].charAt(0).toUpperCase() + splitted[i].slice(1) + " ";
      }

      return name.slice(0, -1);
    },
    async save() {
      try {
        await api.update(state.settings);
        notify.showSuccess(this.$t("settings.settingsUpdated"));
      } catch (e) {
        notify.showError(e);
      }
    },
  },
};
</script>
