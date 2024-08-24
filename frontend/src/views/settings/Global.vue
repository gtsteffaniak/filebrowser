<template>
  <errors v-if="error" :errorCode="error.status" />
  <form class="card" @submit.prevent="save">
    <div class="card-title">
      <h2>{{ $t("settings.globalSettings") }}</h2>
    </div>

    <div class="card-content">
      <p>
        <input type="checkbox" v-model="selectedSettings.signup" />
        {{ $t("settings.allowSignup") }}
      </p>

      <p>
        <input type="checkbox" v-model="selectedSettings.createUserDir" />
        {{ $t("settings.createUserDir") }}
      </p>

      <div>
        <p class="small">{{ $t("settings.userHomeBasePath") }}</p>
        <input
          class="input input--block"
          type="text"
          v-model="selectedSettings.userHomeBasePath"
        />
      </div>

      <h3>{{ $t("settings.rules") }}</h3>
      <p class="small">{{ $t("settings.globalRules") }}</p>
      <rules :rules="selectedSettings.rules" @update:rules="updateRules" />

      <h3>{{ $t("settings.branding") }}</h3>

      <i18n path="settings.brandingHelp" tag="p" class="small">
        <a
          class="link"
          target="_blank"
          href="https://filebrowser.org/configuration/custom-branding"
          >{{ $t("settings.documentation") }}</a
        >
      </i18n>

      <p>
        <input
          type="checkbox"
          v-model="selectedSettings.frontend.disableExternal"
          id="branding-links"
        />
        {{ $t("settings.disableExternalLinks") }}
      </p>

      <p>
        <input
          type="checkbox"
          v-model="selectedSettings.frontend.disableUsedPercentage"
          id="branding-links"
        />
        {{ $t("settings.disableUsedDiskPercentage") }}
      </p>

      <p>
        <label for="branding-name">{{ $t("settings.instanceName") }}</label>
        <input
          class="input input--block"
          type="text"
          v-model="selectedSettings.frontend.name"
          id="branding-name"
        />
      </p>

      <p>
        <label for="branding-files">{{ $t("settings.brandingDirectoryPath") }}</label>
        <input
          class="input input--block"
          type="text"
          v-model="selectedSettings.frontend.files"
          id="branding-files"
        />
      </p>
    </div>

    <div class="card-action">
      <input class="button button--flat" type="submit" :value="$t('buttons.update')" />
    </div>
  </form>
</template>

<script>
import { showSuccess, showError } from "@/notify";
import { state, mutations, getters } from "@/store";
import { settings as api } from "@/api";
import { enableExec } from "@/utils/constants";
import Rules from "@/components/settings/Rules.vue";
import Errors from "@/views/Errors.vue";

export default {
  name: "settings",
  components: {
    Rules,
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
    isExecEnabled: () => enableExec,
  },
  async created() {
    mutations.setLoading("settings", true);
    const original = await api.get();
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
        await api.update(state.settings);
        showSuccess(this.$t("settings.settingsUpdated"));
      } catch (e) {
        showError(e);
      }
    },
  },
};
</script>
