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
import { showSuccess } from "@/notify";
import { state, mutations } from "@/store";
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
      settings: null,
    };
  },
  computed: {
    loading() {
      return state.loading;
    },
    user() {
      return state.user;
    },
    isExecEnabled: () => enableExec,
  },
  async created() {
    try {
      mutations.setLoading(true);
      const original = await api.get();
      let settings = { ...original, commands: [] };

      for (const key in original.commands) {
        settings.commands.push({
          name: key,
          value: original.commands[key].join("\n"),
        });
      }
      settings.shell = settings.shell.join(" ");
      this.originalSettings = original;
      this.settings = settings;
    } catch (e) {
      this.error = e;
    } finally {
      mutations.setLoading(false);
    }
  },
  methods: {
    updateRules(updatedRules) {
      this.settings.rules = updatedRules;
    },
    updateUser(updatedUser) {
      this.settings.defaults = updatedUser;
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
      let settings = {
        ...this.settings,
        shell: this.settings.shell
          .trim()
          .split(" ")
          .filter((s) => s !== ""),
        commands: {},
      };

      for (const { name, value } of this.settings.commands) {
        settings.commands[name] = value.split("\n").filter((cmd) => cmd !== "");
      }

      try {
        await api.update(settings);
        showSuccess(this.$t("settings.settingsUpdated"));
      } catch (e) {
        showError(e);
      }
    },
  },
};
</script>
