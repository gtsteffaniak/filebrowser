<template>
  <div class="card-content">
    <errors v-if="error" :errorCode="error.status" />
    <h2
      class="message"
      v-if="user.loginMethod != 'password' && !stateUser.permissions.admin"
    >
      <i class="material-icons">sentiment_dissatisfied</i>
      <span>{{ $t("files.lonely") }}</span>
    </h2>
    <div v-if="user.loginMethod == 'password' && globalVars.passwordAvailable && !isNew">
      <label for="password">{{ $t("general.password") }}</label>
      <div class="form-flex-group">
        <input
          class="input form-form"
          :class="{ 'form-invalid': invalidPassword }"
          aria-label="Password1"
          type="password"
          autocomplete="new-password"
          :placeholder="$t('settings.enterPassword')"
          v-model="passwordRef"
        />
      </div>
      <div class="form-flex-group">
        <input
          class="input form-form"
          :class="{ 'flat-right': !isNew, 'form-invalid': invalidPassword }"
          aria-label="Password2"
          type="password"
          autocomplete="new-password"
          :placeholder="$t('settings.enterPasswordAgain')"
          v-model="user.password"
          id="password"
        />
        <button
          v-if="!isNew"
          type="button"
          class="button form-button flat-left"
          @click="submitUpdatePassword"
        >
          {{ $t("general.update") }}
        </button>
      </div>
      <div
        style="display: flex; flex-direction: column"
      >
        <div class="settings-items">
          <ToggleSwitch class="item" v-model="user.otpEnabled" :name="$t('otp.name')" />
        </div>
        <button class="button" type="button" v-if="user.otpEnabled" @click="newOTP" aria-label="Generate Code">
          {{ $t("buttons.generateNewOtp") }}
        </button>
      </div>
      <hr />
    </div>
    <div v-if="stateUser.permissions.admin">
      <p v-if="isNew">
        <label for="username">{{ $t("general.username") }}</label>
        <input
          class="input"
          type="text"
          v-model="user.username"
          id="username"
          @input="emitUpdate"
        />
      </p>

      <div v-if="user.loginMethod == 'password' && globalVars.passwordAvailable && isNew">
        <label for="password">{{ $t("general.password") }}</label>
        <div class="form-flex-group">
          <input
            class="input form-form"
            :class="{ 'form-invalid': invalidPassword }"
            aria-label="Password1"
            type="password"
            :placeholder="$t('settings.enterPassword')"
            v-model="passwordRef"
          />
        </div>
        <div class="form-flex-group">
          <input
            class="input form-form"
            :class="{ 'flat-right': !isNew, 'form-invalid': invalidPassword }"
            type="password"
            :placeholder="$t('settings.enterPasswordAgain')"
            aria-label="Password2"
            v-model="user.password"
            autocomplete="new-password"
            id="password"
          />
          <button
            v-if="!isNew"
            type="button"
            class="button form-button flat-left"
            @click="submitUpdatePassword"
          >
            {{ $t("general.update") }}
          </button>
        </div>
      </div>

      <div
        v-if="user.loginMethod == 'password' && globalVars.passwordAvailable"
        class="settings-items"
      >
        <ToggleSwitch
          v-if="user.loginMethod === 'password' && stateUser.permissions?.admin"
          class="item"
          :modelValue="user.lockPassword"
          @update:modelValue="(val) => updateUserField('lockPassword', val)"
          :name="$t('settings.lockPassword')"
        />
      </div>

      <div style="padding-bottom: 1em" v-if="stateUser.permissions.admin">
        <label for="scopes">{{ $t("settings.scopes") }}</label>
        <div
          class="scope-list"
          :class="{ 'form-invalid': duplicateSources.includes(source.name) }"
          v-for="(source, index) in selectedSources"
          :key="index"
        >
          <select
            @change="handleSourceChange(source, $event, source.name)"
            class="input flat-right"
            v-model="source.name"
          >
            <option v-for="s in sourceList" :key="s.name" :value="s.name">
              {{ s.name }}
            </option>
          </select>

          <input
            class="input flat-left scope-input"
            :placeholder="$t('settings.newUserHintSubFolder')"
            @input="updateParent({ source: source, input: $event })"
            :value="source.scope"
            :class="{ 'flat-right': selectedSources.length > 1 }"
          />
          <button
            v-if="selectedSources.length > 1"
            class="button flat-left no-height"
            @click="removeScope(index)"
          >
            <i class="material-icons material-size">delete</i>
          </button>
        </div>
      </div>

      <button v-if="hasMoreSources" @click="addNewScopeSource" class="button no-height">
        <i class="material-icons material-size">add</i>
      </button>

      <div class="settings-items">
        <ToggleSwitch
          v-if="displayHomeDirectoryCheckbox"
          class="item"
          v-model="createUserDir"
          :name="$t('settings.createUserHomeDirectory')"
        />
      </div>

      <p v-if="stateUser.username !== user.username">
        <label for="locale">{{ $t("general.language") }}</label>
        <languages
          class="input"
          id="locale"
          v-model:locale="user.locale"
          @input="emitUpdate"
        ></languages>
      </p>
      <div v-if="stateUser.permissions.admin">
        <label for="loginMethod">{{ $t("settings.loginMethodDescription") }}</label>
        <select v-model="user.loginMethod" class="input" id="loginMethod">
          <option v-if="globalVars.passwordAvailable" value="password">{{ $t("settings.loginMethods.password") }}</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <option v-if="globalVars.oidcAvailable" value="oidc">{{ $t("settings.loginMethods.oidc") }}</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          <option v-if="globalVars.proxyAvailable" value="proxy">{{ $t("settings.loginMethods.proxy") }}</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        </select>
      </div>
      <permissions v-if="stateUser.permissions.admin" :permissions="user.permissions" />
    </div>
  </div>

  <div class="card-action">
    <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button v-if="!isNew" @click.prevent="deletePrompt" type="button" class="button button--flat button--red"
      aria-label="Delete User" :title="$t('general.delete')">
      {{ $t("general.delete") }}
    </button>
    <button @click="save" class="button button--flat" :aria-label="$t('general.save')" :title="$t('general.save')">
      {{ $t("general.save") }}
    </button>
  </div>
</template>

<script>
import { mutations, state } from "@/store";
import { usersApi, settingsApi } from "@/api";
import Languages from "@/components/settings/Languages.vue";
import Permissions from "@/components/settings/Permissions.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import Errors from "@/views/Errors.vue";
import { notify } from "@/notify";
import { globalVars } from "@/utils/constants";
import { eventBus } from "@/store/eventBus";

export default {
  name: "user-edit",
  components: {
    Languages,
    Permissions,
    ToggleSwitch,
    Errors,
  },
  props: {
    userId: {
      type: [String, Number],
      required: false,
    },
  },
  data() {
    return {
      error: null,
      originalUser: null,
      user: {
        scopes: [],
        username: "",
        password: "",
        permissions: { admin: false },
        otpEnabled: false,
        loginMethod: null, // Will be set based on available methods
      },
      showDelete: false,
      createUserDir: false,
      loaded: false,
      originalUserScope: ".",
      sourceList: [],
      availableSources: [],
      selectedSources: [],
      passwordRef: "",
    };
  },
  async created() {
    await this.fetchData();
    await this.initializeForm();
  },
  computed: {
    actor() {
      return state.user;
    },
    settings() {
      return state.settings;
    },
    isNew() {
      return !this.userId;
    },
    stateUser() {
      return state.user;
    },
    invalidPassword() {
      const matching =
        this.user.password != this.passwordRef && this.user.password.length > 0;
      return matching;
    },
    passwordAvailable: () => globalVars.passwordAvailable,
    globalVars: () => globalVars,
    duplicateSources() {
      const names = this.selectedSources.map((s) => s.name);
      return names.filter((name, idx) => names.indexOf(name) !== idx);
    },
    hasMoreSources() {
      return this.selectedSources.length < this.sourceList.length;
    },
    passwordPlaceholder() {
      return this.isNew ? "" : this.$t("settings.avoidChanges");
    },
    displayHomeDirectoryCheckbox() {
      return this.isNew && this.createUserDir;
    },
    firstAvailableLoginMethod() {
      if (this.globalVars.passwordAvailable) return "password";
      if (this.globalVars.oidcAvailable) return "oidc";
      if (this.globalVars.proxyAvailable) return "proxy";
      return "password"; // fallback
    },
  },
  watch: {
    createUserDir(newVal) {
      this.user.scopes = newVal ? { default: "" } : this.originalUserScope;
      this.emitUserUpdate();
    },
    stateUser() {
      this.user.otpEnabled = state.user.otpEnabled;
      this.emitUserUpdate();
    },
    globalVars: {
      handler() {
        // Set loginMethod when globalVars becomes available
        if (this.isNew) {
          this.setDefaultLoginMethod();
        }
      },
      immediate: true
    },
  },
  methods: {
    closeHovers() {
      mutations.closeHovers();
    },
    async fetchData() {
      mutations.setLoading("users", true);
      try {
        if (this.isNew) {
          let defaults = await settingsApi.get("userDefaults");
          this.user = defaults;
          this.user.password = "";
          // Ensure loginMethod is valid, set to first available method if not set or invalid
          const validMethods = [];
          if (this.globalVars.passwordAvailable) validMethods.push("password");
          if (this.globalVars.oidcAvailable) validMethods.push("oidc");
          if (this.globalVars.proxyAvailable) validMethods.push("proxy");

          if (!this.user.loginMethod || !validMethods.includes(this.user.loginMethod)) {
            this.user.loginMethod = this.firstAvailableLoginMethod;
          }
        } else {
          const id = this.userId;
          if (id === undefined) {
            return;
          }
          this.user = { ...(await usersApi.get(id)) };
          this.user.password = "";
          // Normalize scopes to ensure they're in {name, scope} format only
          if (this.user.scopes && Array.isArray(this.user.scopes)) {
            this.user.scopes = this.user.scopes.map(scope => {
              // If it's already in the correct format, use it
              if (scope.name !== undefined && scope.scope !== undefined) {
                return { name: scope.name, scope: scope.scope || "" };
              }
              // If it's a full source object, extract just name and scope
              if (scope.name && typeof scope.name === 'string') {
                return { name: scope.name, scope: scope.scope || "" };
              }
              // Fallback: try to extract from any object structure
              return { name: "", scope: "" };
            });
          }
          // Ensure loginMethod is valid, set to first available method if not set or invalid
          const validMethods = [];
          if (this.globalVars.passwordAvailable) validMethods.push("password");
          if (this.globalVars.oidcAvailable) validMethods.push("oidc");
          if (this.globalVars.proxyAvailable) validMethods.push("proxy");

          if (!this.user.loginMethod || !validMethods.includes(this.user.loginMethod)) {
            this.user.loginMethod = this.firstAvailableLoginMethod;
          }
        }
      } catch (e) {
        this.error = e;
      } finally {
        mutations.setLoading("users", false);
        this.loaded = true;
      }
    },
    async initializeForm() {
      if (!this.stateUser.permissions.admin) {
        this.sourceList = this.user.scopes || [];
      } else {
        this.sourceList = await settingsApi.get("sources");
      }

      this.user.password = this.user.password || "";
      // Set default login method
      this.setDefaultLoginMethod();
      this.selectedSources = this.user.scopes || [];
      this.availableSources = this.sourceList.filter(
        (s) => !this.selectedSources.some((sel) => sel.name === s.name)
      );

      if (this.isNew && this.availableSources.length) {
        const newSource = this.availableSources.shift();
        if (newSource) {
          // Only store {name, scope} format, not the full source config
          this.selectedSources.push({ 
            name: newSource.name || "", 
            scope: "" // Empty scope - backend will handle defaults
          });
          this.emitUserUpdate();
        }
      }
    },
    deletePrompt() {
      mutations.showHover({ name: "deleteUser", props: { user: this.user } });
    },
    async save(event) {
      event.preventDefault();
      try {
        let fields = ["all"];
        // Transform selectedSources to only include {name, scope} format
        // Empty scope strings should be passed as "" for backend to handle defaults
        const scopesToSend = this.selectedSources.map(source => ({
          name: source.name || "",
          scope: source.scope || ""
        }));
        
        if (this.isNew) {
          if (!state.user.permissions.admin) {
            notify.showError(this.$t("settings.userNotAdmin"));
            return;
          }
          await usersApi.create({ ...this.user, scopes: scopesToSend });
          // Emit event to refresh user list
          eventBus.emit('usersChanged');
          // Close the modal
          mutations.closeHovers();
        } else {
          await usersApi.update({ ...this.user, scopes: scopesToSend }, fields);
          // Only emit usersChanged for admin user management, not profile updates
          if (state.user.permissions.admin && this.user.id !== state.user.id) {
            eventBus.emit('usersChanged');
          }
          notify.showSuccessToast(this.$t("settings.userUpdated"));
          mutations.closeHovers();
        }
      } catch (e) {
        console.error(e);
      }
    },
    newOTP() {
      mutations.showHover({
        name: "totp",
        props: {
          generate: true,
          username: this.user.username,
          password: this.passwordRef || this.user.password || "",
        },
      });
    },
    async submitUpdatePassword() {
      event.preventDefault();
      if (this.invalidPassword) {
        notify.showError(this.$t("settings.passwordsDoNotMatch"));
        return;
      }
      try {
        await usersApi.update(this.user, ["password"]);
        // Only emit usersChanged for admin user management, not profile updates
        if (state.user.permissions.admin && this.user.id !== state.user.id) {
          eventBus.emit('usersChanged');
        }
        notify.showSuccessToast(this.$t("settings.userUpdated"));
      } catch (e) {
        notify.showError(e);
      }
    },
    emitUserUpdate() {
      // Update the user object with current scopes
      this.user = { ...this.user, scopes: this.selectedSources };
      // Ensure loginMethod is preserved
      if (!this.user.loginMethod) {
        this.user.loginMethod = this.firstAvailableLoginMethod;
      }
    },
    emitUpdate() {
      // Update the user object
      this.user = { ...this.user };
      // Ensure loginMethod is preserved
      if (!this.user.loginMethod) {
        this.user.loginMethod = this.firstAvailableLoginMethod;
      }
    },
    setUpdatePassword() {
      // This method is kept for compatibility but not used in the new structure
    },
    updateParent(input) {
      const updatedScopes = this.selectedSources.map((source) =>
        source.name === input.source.name
          ? { ...source, scope: input.input.target.value }
          : source
      );
      this.selectedSources = updatedScopes;
      this.emitUserUpdate();
    },
    addNewScopeSource(event) {
      event.preventDefault();
      if (this.hasMoreSources) {
        this.selectedSources.push({ name: "", scope: "" });
        this.emitUserUpdate();
      }
    },
    removeScope(index) {
      const removed = this.selectedSources.splice(index, 1)[0];
      this.availableSources.push({ name: removed.name });
      this.emitUserUpdate();
    },
    handleSourceChange(source, event, oldName) {
      const newName = event.target.value;
      this.availableSources = this.availableSources.filter((s) => s.name !== newName);
      if (oldName && !this.availableSources.find((s) => s.name === oldName)) {
        this.availableSources.push({ name: oldName });
      }
      source.name = newName;
      this.emitUserUpdate();
    },
    updateUserField(field, value) {
      this.user[field] = value;
      this.emitUserUpdate();
    },
    setDefaultLoginMethod() {
      // Set loginMethod to first available method if not already set or if current value is invalid
      const validMethods = [];
      if (this.globalVars.passwordAvailable) validMethods.push("password");
      if (this.globalVars.oidcAvailable) validMethods.push("oidc");
      if (this.globalVars.proxyAvailable) validMethods.push("proxy");

      const isValidMethod = validMethods.includes(this.user.loginMethod);

      if (!this.user.loginMethod || this.user.loginMethod === null || !isValidMethod) {
        this.user.loginMethod = this.firstAvailableLoginMethod;
      }
    },
  },
};
</script>

<style scoped>
.scope-list {
  display: flex;
}

.scope-input {
  width: 100%;
}
.no-height {
  height: unset;
}
.material-size {
  font-size: 1em !important;
}
</style>
