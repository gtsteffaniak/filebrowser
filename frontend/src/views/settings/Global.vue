<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="row" v-else-if="!loading">
    <div class="column">
      <form class="card" @submit.prevent="save">
        <div class="card-title">
          <h2>{{ $t("settings.globalSettings") }}</h2>
        </div>

        <div class="card-content">
          <p>
            <input type="checkbox" v-model="settings.signup" />
            {{ $t("settings.allowSignup") }}
          </p>

          <p>
            <input type="checkbox" v-model="settings.createUserDir" />
            {{ $t("settings.createUserDir") }}
          </p>

          <div>
            <p class="small">{{ $t("settings.userHomeBasePath") }}</p>
            <input
              class="input input--block"
              type="text"
              v-model="settings.userHomeBasePath"
            />
          </div>

          <h3>{{ $t("settings.rules") }}</h3>
          <p class="small">{{ $t("settings.globalRules") }}</p>
          <rules :rules="settings.rules" @update:rules="updateRules" />

          <div v-if="isExecEnabled">
            <h3>{{ $t("settings.executeOnShell") }}</h3>
            <p class="small">{{ $t("settings.executeOnShellDescription") }}</p>
            <input
              class="input input--block"
              type="text"
              placeholder="bash -c, cmd /c, ..."
              v-model="settings.shell"
            />
          </div>

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
              v-model="settings.frontend.disableExternal"
              id="branding-links"
            />
            {{ $t("settings.disableExternalLinks") }}
          </p>

          <p>
            <input
              type="checkbox"
              v-model="settings.frontend.disableUsedPercentage"
              id="branding-links"
            />
            {{ $t("settings.disableUsedDiskPercentage") }}
          </p>

          <p>
            <label for="branding-name">{{ $t("settings.instanceName") }}</label>
            <input
              class="input input--block"
              type="text"
              v-model="settings.frontend.name"
              id="branding-name"
            />
          </p>

          <p>
            <label for="branding-files">{{ $t("settings.brandingDirectoryPath") }}</label>
            <input
              class="input input--block"
              type="text"
              v-model="settings.frontend.files"
              id="branding-files"
            />
          </p>
        </div>

        <div class="card-action">
          <input
            class="button button--flat"
            type="submit"
            :value="$t('buttons.update')"
          />
        </div>
      </form>
    </div>

    <div class="column">
      <form class="card" @submit.prevent="save">
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
          <input
            class="button button--flat"
            type="submit"
            :value="$t('buttons.update')"
          />
        </div>
      </form>
    </div>

    <div class="column">
      <form v-if="isExecEnabled" class="card" @submit.prevent="save">
        <div class="card-title">
          <h2>{{ $t("settings.commandRunner") }}</h2>
        </div>

        <div class="card-content">
          <i18n path="settings.commandRunnerHelp" tag="p" class="small">
            <code>FILE</code>
            <code>SCOPE</code>
            <a
              class="link"
              target="_blank"
              href="https://filebrowser.org/configuration/command-runner"
              >{{ $t("settings.documentation") }}</a
            >
          </i18n>

          <div
            v-for="(command, index) in settings.commands"
            :key="index"
            class="collapsible"
          >
            <input :id="command.name" type="checkbox" />
            <label :for="command.name">
              <p>{{ capitalize(command.name) }}</p>
              <i class="material-icons">arrow_drop_down</i>
            </label>
            <div class="collapse">
              <textarea
                class="input input--block input--textarea"
                v-model.trim="command.value"
              ></textarea>
            </div>
          </div>
        </div>

        <div class="card-action">
          <input
            class="button button--flat"
            type="submit"
            :value="$t('buttons.update')"
          />
        </div>
      </form>
    </div>
  </div>
</template>

<script>
import { showSuccess } from "@/notify";
import { state, mutations } from "@/store";
import { settings as api } from "@/api";
import { enableExec } from "@/utils/constants";
import UserForm from "@/components/settings/UserForm.vue";
import Rules from "@/components/settings/Rules.vue";
import Errors from "@/views/Errors.vue";

export default {
  name: "settings",
  components: {
    UserForm,
    Rules,
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
