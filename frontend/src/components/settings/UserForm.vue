<template>
  <h2
    class="message"
    v-if="user.loginMethod != 'password' && !stateUser.permissions.admin"
  >
    <i class="material-icons">sentiment_dissatisfied</i>
    <span>{{ $t("files.lonely") }}</span>
  </h2>
  <div v-if="user.loginMethod == 'password' && globalVars.passwordAvailable && !isNew">
    <label for="password">{{ $t("settings.password") }}</label>
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
        {{ $t("buttons.update") }}
      </button>
    </div>
    <div
      style="display: flex; flex-direction: column"
    >
      <div class="settings-items">
        <ToggleSwitch class="item" v-model="user.otpEnabled" :name="$t('otp.name')" />
      </div>
      <button class="button" type="button" v-if="user.otpEnabled" :onclick="newOTP" aria-label="Generate Code">
        {{ $t("buttons.generateNewOtp") }}
      </button>
    </div>
    <hr />
  </div>
  <div v-if="stateUser.permissions.admin">
    <p v-if="isNew">
      <label for="username">{{ $t("settings.username") }}</label>
      <input
        class="input"
        type="text"
        v-model="user.username"
        id="username"
        @input="emitUpdate"
      />
    </p>

    <div v-if="user.loginMethod == 'password' && globalVars.passwordAvailable && isNew">
      <label for="password">{{ $t("settings.password") }}</label>
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
          {{ $t("buttons.update") }}
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
          placeholder="scope eg. '/subfolder', leave blank for default path"
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
      <label for="locale">{{ $t("settings.language") }}</label>
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
        <option value="password">Password</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        <option value="oidc">OIDC</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        <option value="proxy">Proxy</option> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      </select>
    </div>
    <permissions v-if="stateUser.permissions.admin" :permissions="user.permissions" />
  </div>
</template>

<script>
import Languages from "./Languages.vue";
import Permissions from "./Permissions.vue";
import { mutations, state } from "@/store";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { notify } from "@/notify";
import { usersApi, settingsApi } from "@/api";
import { globalVars } from "@/utils/constants";

export default {
  name: "UserForm",
  components: {
    Permissions,
    Languages,
    ToggleSwitch,
  },
  props: {
    user: Object,
    isNew: Boolean,
  },
  data() {
    return {
      createUserDir: false,
      originalUserScope: ".",
      sourceList: [],
      availableSources: [],
      selectedSources: [],
      passwordRef: "",
    };
  },
  async mounted() {
    if (!this.stateUser.permissions.admin) {
      this.sourceList = this.user.scopes || [];
    } else {
      this.sourceList = await settingsApi.get("sources");
    }

    this.user.password = this.user.password || "";
    this.selectedSources = this.user.scopes || [];
    this.availableSources = this.sourceList.filter(
      (s) => !this.selectedSources.some((sel) => sel.name === s.name)
    );

    if (this.isNew && this.availableSources.length) {
      const newSource = this.availableSources.shift();
      if (newSource) {
        this.selectedSources.push(newSource);
        this.emitUserUpdate();
      }
    }
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
  },
  computed: {
    invalidPassword() {
      const matching =
        this.user.password != this.passwordRef && this.user.password.length > 0;
      return matching;
    },
    passwordAvailable: () => globalVars.passwordAvailable,
    duplicateSources() {
      const names = this.selectedSources.map((s) => s.name);
      return names.filter((name, idx) => names.indexOf(name) !== idx);
    },
    hasMoreSources() {
      return this.selectedSources.length < this.sourceList.length;
    },
    stateUser() {
      return state.user;
    },
    passwordPlaceholder() {
      return this.isNew ? "" : this.$t("settings.avoidChanges");
    },
    displayHomeDirectoryCheckbox() {
      return this.isNew && this.createUserDir;
    },
  },
  methods: {
    newOTP() {
      mutations.showHover({
        name: "totp",
        props: {
          generate: true,
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
        notify.showSuccess(this.$t("settings.userUpdated"));
      } catch (e) {
        notify.showError(e);
      }
    },
    emitUserUpdate() {
      this.$emit("update:user", { ...this.user, scopes: this.selectedSources });
    },
    emitUpdate() {
      this.$emit("update:user", { ...this.user });
    },
    setUpdatePassword() {
      this.$emit("update:updatePassword", true);
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
  },
};
</script>

<style>
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
