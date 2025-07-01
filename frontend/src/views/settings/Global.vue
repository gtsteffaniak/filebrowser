<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card-title">
    <h2>{{ $t("settings.globalSettings") }}</h2>
  </div>

  <div class="card-content"> {{ $t('settings.emptyGlobal') }} </div>

  <div class="card-action">
    <input class="button button--flat" type="submit" :value="$t('buttons.update')" />
  </div>
</template>

<script>
import { notify } from "@/notify";
import { state, mutations, getters } from "@/store";
import { settingsApi } from "@/api";
import Errors from "@/views/Errors.vue";

export default {
  name: "settings",
  components: {
    Errors,
  },
  data: function () {
    return {
      error: null,
      originalSettings: null,
      selectedSettings: state.settings,
    };
  },
  computed: {
    loading() {
      return getters.isLoading();
    },
    user() {
      return state.user;
    },
  },
  async created() {
    mutations.setLoading("settings", true);
    const original = await settingsApi.get();
    mutations.setSettings(original);
    mutations.setLoading("settings", false);
  },
  methods: {
    updateRules(updatedRules) {
      this.selectedSettings = { ...this.selectedSettings, rules: updatedRules };
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
        mutations.setSettings(this.selectedSettings);
        await settingsApi.update(state.settings);
        notify.showSuccess(this.$t("settings.settingsUpdated"));
      } catch (e) {
        notify.showError(e);
      }
    },
  },
};
</script>
