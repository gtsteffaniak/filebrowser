<template>
  <div class="card-content">
    <errors v-if="error" :errorCode="error.status" />
    <h2 class="message" v-if="user.loginMethod !== 'password' && !stateUser.permissions.admin">
      <i class="material-symbols-outlined">sentiment_dissatisfied</i>
      <span>{{ $t("files.lonely") }}</span>
    </h2>
    <div v-if="showPasswordChangeSection">
      <label for="password">{{ $t("general.password") }}</label>
      <div class="form-flex-group">
        <input class="input form-form" :class="{ 'form-invalid': invalidPassword }" aria-label="Password1"
          type="password" autocomplete="new-password" :placeholder="$t('settings.enterPassword')"
          v-model="passwordRef" />
      </div>
      <div class="form-flex-group">
        <input class="input form-form" :class="{ 'flat-right': !isNew, 'form-invalid': invalidPassword }"
          aria-label="Password2" type="password" autocomplete="new-password"
          :placeholder="$t('settings.enterPasswordAgain')" v-model="user.password" id="password" />
        <button
          v-if="!isNew"
          type="button"
          class="button form-button flat-left"
          :disabled="!canUpdatePassword"
          @click="submitUpdatePassword"
        >
          {{ $t("general.update") }}
        </button>
      </div>
      <div style="display: flex; flex-direction: column">
        <div class="settings-items">
          <ToggleSwitch class="item" v-model="user.otpEnabled" :name="$t('otp.name')" />
        </div>
        <button class="button" type="button" v-if="user.otpEnabled" @click="newOTP" aria-label="Generate Code">
          {{ $t("buttons.generateNewOtp") }}
        </button>
      </div>
      <hr />
    </div>
    <div v-if="globalVars.passkeyAvailable" style="margin-top: 0.5em;">
      <label>{{ $t("profileSettings.passkeys") }}</label>
      <div v-if="user.passkeyCredentials && user.passkeyCredentials.length > 0" class="passkey-list">
        <div v-for="pk in user.passkeyCredentials" :key="pk.id" class="passkey-item">
          <div class="passkey-info">
            <span class="passkey-name">{{ pk.name || $t("profileSettings.passkeyDefaultName") }}</span>
            <span class="passkey-meta">
              {{ $t("profileSettings.created") }} {{ formatDate(pk.createdAt) }}<span v-if="pk.lastUsedAt" class="passkey-last-used">{{ $t("profileSettings.lastUsed") }} {{ formatDate(pk.lastUsedAt) }}</span>
            </span>
          </div>
          <button type="button" class="button button--flat button--red" @click="deletePasskey(pk.id)">
            {{ $t("general.delete") }}
          </button>
        </div>
      </div>
      <div v-else class="passkey-empty">
        {{ $t("profileSettings.noPasskeys") }}
      </div>
      <button type="button" class="button" style="margin-top: 0.5em;" :disabled="addingPasskey" @click="addPasskey">
        {{ addingPasskey ? $t("profileSettings.addingPasskey") : $t("profileSettings.addPasskey") }}
      </button>
    </div>
    <div v-if="stateUser.permissions.admin">
      <p v-if="isNew">
        <label for="username">{{ $t("general.username") }}</label>
        <input class="input" type="text" v-model="user.username" id="username" @input="emitUpdate" />
      </p>

      <div v-if="user.loginMethod === 'password' && globalVars.passwordAvailable && isNew">
        <label for="password">{{ $t("general.password") }}</label>
        <div class="form-flex-group">
          <input class="input form-form" :class="{ 'form-invalid': invalidPassword }" aria-label="Password1"
            type="password" :placeholder="$t('settings.enterPassword')" v-model="passwordRef" />
        </div>
        <div class="form-flex-group">
          <input class="input form-form" :class="{ 'flat-right': !isNew, 'form-invalid': invalidPassword }"
            type="password" :placeholder="$t('settings.enterPasswordAgain')" aria-label="Password2"
            v-model="user.password" autocomplete="new-password" id="password" />
          <button
            v-if="!isNew"
            type="button"
            class="button form-button flat-left"
            :disabled="!canUpdatePassword"
            @click="submitUpdatePassword"
          >
            {{ $t("general.update") }}
          </button>
        </div>
      </div>

      <div style="padding-bottom: 1em" v-if="stateUser.permissions.admin">
        <label for="scopes">{{ $t("settings.scopes") }}</label>
        <p class="small">{{ $t("settings.sourcePermissionsHelp") }}</p>
        <ExpandDropdown
          v-model="selectedSourceNames"
          class="source-dropdown-select"
          :options="allSourceOptions"
          allow-multiple
          multi-summary-mode="count"
          :default-placeholder-if-empty="noSourcesPlaceholder"
          :aria-label="$t('settings.scopes')"
        />
        <div class="scope-blocks">
          <div class="scope-block" v-for="source in selectedSources" :key="source.name">
            <SettingsItem
              :title="sourceBlockTitle(source)"
              :collapsable="true"
              :force-collapsed="expandedSourceName !== source.name"
              @toggle="onSourceExpandToggle(source.name)"
            >
              <div class="scope-path-row">
                <label class="scope-path-label">{{ $t("settings.scopePath") }}</label>
                <button
                  type="button"
                  :aria-label="`user-edit-scope-path-${source.name}`"
                  class="clickable button scope-path-display"
                  @click="onScopePathRowClick(source)"
                >{{ scopePathDisplay(source) }}
                </button>
              </div>
              <source-file-permissions
                :permissions="sourcePermissionsFor(source.name)"
                @changed="markScopePermissionsExplicit(source.name)"
              />
            </SettingsItem>
          </div>
        </div>
      </div>

      <p v-if="stateUser.username !== user.username">
        <label for="locale">{{ $t("general.language") }}</label>
        <languages id="locale" v-model:locale="user.locale" @input="emitUpdate"></languages>
      </p>
      <div v-if="stateUser.permissions.admin">
        <label for="loginMethod">{{ $t("settings.loginMethod") }}</label>
        <ExpandDropdown
          v-model="user.loginMethod"
          input-id="loginMethod"
          :options="loginMethodOptions"
          :aria-label="$t('settings.loginMethod')"
          @update:model-value="emitUpdate"
        />
      </div>

      <UserDefaultsAccountSection
        v-if="stateUser.permissions.admin"
        :enforceable="false"
        :start-collapsed="false"
        :account="editAccount"
        @account-change="onEditAccountChange"
      />
    </div>
  </div>

  <div class="card-actions">
    <button type="button" class="button button--flat button--grey" @click="closeTopPrompt" :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button v-if="!isNew" @click.prevent="deletePrompt" type="button" class="button button--flat button--red"
      aria-label="Delete User" :title="$t('general.delete')">
      {{ $t("general.delete") }}
    </button>
    <button @click="save" type="button" class="button button--flat" :aria-label="$t('general.save')" :title="$t('general.save')">
      {{ $t("general.save") }}
    </button>
  </div>
</template>

<script>
import { mutations, state } from "@/store";
import { usersApi, settingsApi, authApi } from "@/api";
import Languages from "@/components/settings/Languages.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import SourceFilePermissions from "@/components/settings/SourceFilePermissions.vue";
import SettingsItem from "@/components/settings/SettingsItem.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import UserDefaultsAccountSection from "@/components/settings/UserDefaultsAccountSection.vue";
import Errors from "@/views/Errors.vue";
import { notify } from "@/notify";
import { globalVars } from "@/utils/constants";
import { eventBus } from "@/store/eventBus";
import { setObjectProperty } from '@/utils/object.js';

export default {
  name: "user-edit",
  components: {
    Languages,
    ExpandDropdown,
    SourceFilePermissions,
    SettingsItem,
    ToggleSwitch,
    UserDefaultsAccountSection,
    Errors,
  },
  props: {
    /** Login name of the user being edited (omit for “new user”). */
    targetUsername: {
      type: String,
      required: false,
    },
    promptId: {
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
      selectedSources: [],
      expandedSourceName: null,
      passwordRef: "",
      pendingScopeSelectionContextId: null,
      pendingScopeSourceName: null,
      addingPasskey: false,
      sourceFilePermissionDefaults: null,
      editAccount: {
        lockPassword: false,
        disableSettings: false,
        disableUpdateNotifications: false,
        permissions: {
          admin: false,
          share: false,
          api: false,
          realtime: false,
        },
      },
    };
  },
  async created() {
    await this.fetchData();
    await this.initializeForm();
  },
  mounted() {
    eventBus.on("pathSelected", this.onPathSelectedFromPicker);
    eventBus.on("pathPickerCancelled", this.onPathPickerCancelled);
  },
  beforeUnmount() {
    eventBus.off("pathSelected", this.onPathSelectedFromPicker);
    eventBus.off("pathPickerCancelled", this.onPathPickerCancelled);
  },
  computed: {
    actor() {
      return state.user;
    },
    settings() {
      return state.settings;
    },
    isNew() {
      return !this.targetUsername;
    },
    stateUser() {
      return state.user;
    },
    noSourcesPlaceholder() {
      return this.$t("general.sources", {
        prefix: this.$t("general.no", { suffix: " " }),
      });
    },
    invalidPassword() {
      const matching =
        this.user.password !== this.passwordRef && this.user.password.length > 0;
      return matching;
    },
    /** Update is allowed only when both password fields are non-empty (trimmed) and match. */
    canUpdatePassword() {
      const a = String(this.passwordRef ?? "").trim();
      const b = String(this.user.password ?? "").trim();
      if (a.length === 0 || b.length === 0) {
        return false;
      }
      return !this.invalidPassword;
    },
    passwordAvailable: () => globalVars.passwordAvailable,
    globalVars: () => globalVars,
    allSourceOptions() {
      return (this.sourceList || []).map((source) => ({
        value: source.name,
        label: source.name,
      }));
    },
    selectedSourceNames: {
      get() {
        return this.selectedSources.map((source) => source.name).filter(Boolean);
      },
      set(names) {
        const selected = Array.isArray(names) ? names.filter(Boolean) : [];
        const previousByName = new Map(
          this.selectedSources
            .filter((source) => source.name)
            .map((source) => [source.name, source])
        );
        const previousNames = this.selectedSourceNames;
        for (const name of previousNames) {
          if (!selected.includes(name)) {
            const idx = this.selectedSources.findIndex((source) => source.name === name);
            if (idx >= 0) {
              this.selectedSources.splice(idx, 1);
            }
          }
        }
        this.selectedSources = selected.map((name) => {
          if (previousByName.has(name)) {
            return previousByName.get(name);
          }
          return {
            name,
            scope: "/",
            permissions: undefined,
            permissionsExplicit: false,
          };
        });
        if (
          this.expandedSourceName !== null
          && !selected.includes(this.expandedSourceName)
        ) {
          this.expandedSourceName = null;
        }
        this.emitUserUpdate();
      },
    },
    loginMethodOptions() {
      const options = [];
      if (this.globalVars.passwordAvailable) {
        options.push({ value: "password", label: this.$t("settings.loginMethods.password") });
      }
      if (this.globalVars.oidcAvailable) {
        options.push({ value: "oidc", label: "OIDC" });
      }
      if (this.globalVars.proxyAvailable) {
        options.push({ value: "proxy", label: "Proxy" });
      }
      if (this.globalVars.ldapAvailable) {
        options.push({ value: "ldap", label: "LDAP" });
      }
      if (this.globalVars.jwtAvailable) {
        options.push({ value: "jwt", label: "JWT" });
      }
      return options;
    },
    passwordPlaceholder() {
      return this.isNew ? "" : this.$t("settings.avoidChanges");
    },
    /** Password change (existing user): self-service requires password login; admins editing another user always see it. */
    showPasswordChangeSection() {
      if (this.isNew || !this.globalVars.passwordAvailable) {
        return false;
      }
      if (this.stateUser.permissions?.admin && this.stateUser.username !== this.user.username) {
        return true;
      }
      if (this.user.loginMethod !== "password") {
        return false;
      }
      return this.stateUser.loginMethod === "password";
    },
    firstAvailableLoginMethod() {
      if (this.globalVars.passwordAvailable) return "password";
      if (this.globalVars.oidcAvailable) return "oidc";
      if (this.globalVars.proxyAvailable) return "proxy";
      if (this.globalVars.ldapAvailable) return "ldap";
      return "password"; // fallback
    },
  },
  watch: {
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
    defaultPermissions() {
      return {
        admin: false,
        api: false,
        share: false,
        realtime: false,
      };
    },
    defaultSourceFilePermissions() {
      if (this.sourceFilePermissionDefaults) {
        return { ...this.sourceFilePermissionDefaults };
      }
      return {
        view: true,
        download: true,
        modify: false,
        create: false,
        delete: false,
      };
    },
    async loadSourceFilePermissionDefaults() {
      if (this.sourceFilePermissionDefaults) {
        return;
      }
      try {
        const settings = await settingsApi.getSourceSettings();
        const defaults = settings?.defaultPermissions ?? {};
        this.sourceFilePermissionDefaults = {
          view: defaults.view !== false,
          download: defaults.download !== false,
          modify: !!defaults.modify,
          create: !!defaults.create,
          delete: !!defaults.delete,
        };
      } catch (e) {
        console.error(e);
        this.sourceFilePermissionDefaults = {
          view: true,
          download: true,
          modify: false,
          create: false,
          delete: false,
        };
      }
    },
    markScopePermissionsExplicit(sourceName) {
      const scope = this.selectedSources.find((entry) => entry.name === sourceName);
      if (scope) {
        scope.permissionsExplicit = true;
      }
    },
    sourcePermissionsFor(sourceName) {
      const scope = this.selectedSources.find((entry) => entry.name === sourceName);
      if (!scope) {
        return this.defaultSourceFilePermissions();
      }
      if (!scope.permissions) {
        scope.permissions = { ...this.defaultSourceFilePermissions() };
      }
      return scope.permissions;
    },
    toggleSourceExpanded(sourceName) {
      this.expandedSourceName = this.expandedSourceName === sourceName ? null : sourceName;
    },
    onSourceExpandToggle(sourceName) {
      this.toggleSourceExpanded(sourceName);
    },
    sourceBlockTitle(source) {
      return source?.name || "";
    },
    normalizeFormUser(raw) {
      const user = { ...(raw ?? {}) };
      if ((user.permissions === null || user.permissions === undefined) && user.account?.permissions !== null && user.account?.permissions !== undefined) {
        user.permissions = { ...user.account.permissions };
        if (user.permissions.download === null || user.permissions.download === undefined) {
          user.permissions.download = true;
        }
      }
      if (user.permissions) {
        delete user.permissions.modify;
        delete user.permissions.create;
        delete user.permissions.delete;
        delete user.permissions.download;
        delete user.permissions.view;
      }
      if (user.permissions === null || user.permissions === undefined) {
        user.permissions = this.defaultPermissions();
      }
      user.scopes = this.normalizeScopesForForm(user.scopes, user.sourcePermissions);
      delete user.sourcePermissions;
      return user;
    },
    normalizeScopesForForm(scopes, legacySourcePermissions) {
      const normalized = Array.isArray(scopes) ? scopes.map((scope) => ({
        name: scope?.name || "",
        scope: this.normalizeScopeForApi(scope?.scope),
        permissions: scope?.permissions
          ? { ...scope.permissions }
          : undefined,
        permissionsExplicit: !!scope?.permissions,
      })) : [];
      if (legacySourcePermissions && typeof legacySourcePermissions === "object") {
        for (const entry of normalized) {
          if (!entry.permissions && legacySourcePermissions[entry.name]) {
            entry.permissions = { ...legacySourcePermissions[entry.name] };
          }
        }
      }
      for (const entry of normalized) {
        if (!entry.permissions) {
          entry.permissionsExplicit = false;
        }
      }
      return normalized;
    },
    /** Scope path sent to the API: trimmed, or "/" when empty (matches backend root). */
    normalizeScopeForApi(scope) {
      const t = String(scope ?? "").trim();
      return t.length > 0 ? t : "/";
    },
    closeTopPrompt() {
      mutations.closeTopPrompt();
    },
    async fetchData() {
      mutations.setLoading("users", true);
      try {
        if (this.isNew) {
          const defaults = await settingsApi.get("userDefaults");
          this.user = this.normalizeFormUser(defaults);
          this.user.password = "";
          // Ensure loginMethod is valid, set to first available method if not set or invalid
          const validMethods = [];
          if (this.globalVars.passwordAvailable) validMethods.push("password");
          if (this.globalVars.oidcAvailable) validMethods.push("oidc");
          if (this.globalVars.proxyAvailable) validMethods.push("proxy");
          if (this.globalVars.ldapAvailable) validMethods.push("ldap");
          if (this.globalVars.jwtAvailable) validMethods.push("jwt");

          if (!this.user.loginMethod || !validMethods.includes(this.user.loginMethod)) {
            this.user.loginMethod = this.firstAvailableLoginMethod;
          }
        } else {
          const uname = this.targetUsername;
          if (!uname) {
            return;
          }
          this.user = this.normalizeFormUser(await usersApi.get(uname));
          this.user.password = "";
          // Normalize scopes to ensure they're in {name, scope} format only
          if (this.user.scopes && Array.isArray(this.user.scopes)) {
            this.user.scopes = this.normalizeScopesForForm(this.user.scopes);
          }
          // Ensure loginMethod is valid, set to first available method if not set or invalid
          const validMethods = [];
          if (this.globalVars.passwordAvailable) validMethods.push("password");
          if (this.globalVars.oidcAvailable) validMethods.push("oidc");
          if (this.globalVars.proxyAvailable) validMethods.push("proxy");
          if (this.globalVars.ldapAvailable) validMethods.push("ldap");
          if (this.globalVars.jwtAvailable) validMethods.push("jwt");

          if (!this.user.loginMethod || !validMethods.includes(this.user.loginMethod)) {
            this.user.loginMethod = this.firstAvailableLoginMethod;
          }
        }
      } catch (e) {
        this.error = e;
      } finally {
        mutations.setLoading("users", false);
        this.loaded = true;
        // Update prompt name after user data is loaded
        this.updatePromptTitle();
      }
    },
    async initializeForm() {
      await this.loadSourceFilePermissionDefaults();
      if (!this.stateUser.permissions.admin) {
        this.sourceList = this.user.scopes || [];
      } else {
        this.sourceList = await settingsApi.get("sources");
      }

      this.user.password = this.user.password || "";
      // Set default login method
      this.setDefaultLoginMethod();
      const catalogueNames = new Set((this.sourceList || []).map((source) => source.name));
      this.selectedSources = (this.user.scopes || [])
        .filter((scope) => scope?.name && catalogueNames.has(scope.name))
        .map((scope) => ({
        name: scope.name || "",
        scope: this.normalizeScopeForApi(scope.scope),
        permissions: scope.permissions
          ? { ...scope.permissions }
          : undefined,
        permissionsExplicit: !!scope.permissions,
      }));

      if (this.isNew && this.selectedSources.length === 0 && this.sourceList.length > 0) {
        this.selectedSourceNames = [this.sourceList[0].name];
      }
      this.syncEditAccountForm();
    },
    syncEditAccountForm() {
      const p = this.user.permissions || {};
      this.editAccount.lockPassword = !!this.user.lockPassword;
      this.editAccount.disableSettings = !!this.user.disableSettings;
      this.editAccount.disableUpdateNotifications = !!this.user.disableUpdateNotifications;
      this.editAccount.permissions = {
        admin: !!p.admin,
        share: !!p.share,
        api: !!p.api,
        realtime: !!p.realtime,
      };
    },
    applyEditAccountToUser() {
      this.user.lockPassword = this.editAccount.lockPassword;
      this.user.disableSettings = this.editAccount.disableSettings;
      this.user.disableUpdateNotifications = this.editAccount.disableUpdateNotifications;
      if (!this.user.permissions) {
        this.user.permissions = this.defaultPermissions();
      }
      this.user.permissions.admin = this.editAccount.permissions.admin;
      this.user.permissions.share = this.editAccount.permissions.share;
      this.user.permissions.api = this.editAccount.permissions.api;
      this.user.permissions.realtime = this.editAccount.permissions.realtime;
    },
    onEditAccountChange() {
      this.applyEditAccountToUser();
      this.emitUpdate();
    },
    deletePrompt() {
      mutations.showPrompt({
        name: "generic",
        props: {
          title: this.$t("general.delete"),
          body: this.$t("prompts.deleteUserMessage", { username: this.user.username }),
          buttons: [
            {
              label: this.$t("general.delete"),
              action: async () => {
                try {
                  await usersApi.deleteUser(this.user.username, {
                    actorPasswordPromptI18nKey: "prompts.confirmPasswordToSaveUser",
                  });
                  notify.showSuccessToast(this.$t("settings.userDeleted"));
                  eventBus.emit('usersChanged');
                  mutations.closeTopPrompt(); // close delete user prompt confirmation
                  mutations.closeTopPrompt(); // close user prompt since user doens't exist anymore
                } catch (e) {
                  console.error(e);
                  notify.showError(e);
                }
              },
            },
          ],
        },
      });
    },
    async save(event) {
      event.preventDefault();
      try {
        const fields = ["all"];
        // Transform selectedSources to only include {name, scope} format
        // Empty scope strings should be passed as "" for backend to handle defaults
        const scopesToSend = this.selectedSources.map((source) => {
          const entry = {
            name: source.name || "",
            scope: this.normalizeScopeForApi(source.scope),
          };
          if (source.permissionsExplicit) {
            entry.permissions = { ...this.sourcePermissionsFor(source.name) };
          }
          return entry;
        });
        const payload = {
          ...this.user,
          scopes: scopesToSend,
        };
        delete payload.sourcePermissions;

        if (this.isNew) {
          if (!state.user.permissions.admin) {
            notify.showError(this.$t("settings.userNotAdmin"));
            return;
          }
          await usersApi.create(
            payload,
            {
              actorPasswordPromptI18nKey: "prompts.confirmPasswordToSaveUser",
            }
          );
          // Emit event to refresh user list
          eventBus.emit('usersChanged');
          // Close the prompt
          mutations.closeTopPrompt();
        } else {
          await usersApi.update(payload, fields);
          eventBus.emit('usersChanged');
          notify.showSuccessToast(this.$t("settings.userUpdated"));
          mutations.closeTopPrompt();
        }
      } catch (e) {
        notify.showError(e);
      }
    },
    newOTP() {
      mutations.showPrompt({
        name: "password",
        props: {
          infoText: this.$t("prompts.confirmPasswordToSaveUser"),
          submitLabel: this.$t("general.confirm"),
          submitCallback: (accountPassword) => {
            mutations.showPrompt({
              name: "totp",
              props: {
                generate: true,
                username: this.user.username,
                password: accountPassword,
              },
            });
          },
        },
      });
    },
    async submitUpdatePassword() {
      event.preventDefault();
      if (!this.canUpdatePassword) {
        return;
      }
      try {
        await usersApi.update(this.user, ["password"], {
          actorPasswordPromptI18nKey: "prompts.confirmPasswordToSaveUser",
        });
        eventBus.emit("usersChanged");
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
    onScopePathRowClick(source) {
      if (!source?.name) {
        return;
      }
      this.openScopePicker(source);
    },
    scopePathDisplay(source) {
      const s = source?.scope;
      if (s !== undefined && s !== null && String(s).length > 0) {
        return s;
      }
      return "/";
    },
    openScopePicker(source) {
      if (!source?.name) {
        return;
      }
      const selectionContextId = `user-scope-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
      this.pendingScopeSelectionContextId = selectionContextId;
      this.pendingScopeSourceName = source.name;
      const initialPath =
        source.scope && typeof source.scope === "string" && source.scope.length > 0
          ? source.scope
          : "/";
      mutations.showPrompt({
        name: "pathPicker",
        props: {
          currentPath: initialPath,
          currentSource: source.name,
          hideDestinationSource: true,
          selectionContextId,
        },
      });
    },
    onPathPickerCancelled(data) {
      if (!this.pendingScopeSelectionContextId || !data) {
        return;
      }
      if (data.selectionContextId !== this.pendingScopeSelectionContextId) {
        return;
      }
      this.pendingScopeSelectionContextId = null;
      this.pendingScopeSourceName = null;
    },
    onPathSelectedFromPicker(data) {
      if (!this.pendingScopeSelectionContextId) {
        return;
      }
      if (!data || data.selectionContextId !== this.pendingScopeSelectionContextId) {
        return;
      }
      this.pendingScopeSelectionContextId = null;
      const sourceName = this.pendingScopeSourceName;
      this.pendingScopeSourceName = null;
      if (!sourceName || typeof data.path !== "string") {
        return;
      }
      const path = data.path;
      this.selectedSources = this.selectedSources.map((source) =>
        source.name === sourceName ? { ...source, scope: path } : source
      );
      this.emitUserUpdate();
    },
    updateUserField(field, value) {
      this.user = setObjectProperty(this.user, field, value);
      this.emitUserUpdate();
    },
    setDefaultLoginMethod() {
      // Set loginMethod to first available method if not already set or if current value is invalid
      const validMethods = [];
      if (this.globalVars.passwordAvailable) validMethods.push("password");
      if (this.globalVars.oidcAvailable) validMethods.push("oidc");
      if (this.globalVars.proxyAvailable) validMethods.push("proxy");
      if (this.globalVars.ldapAvailable) validMethods.push("ldap");
      if (this.globalVars.jwtAvailable) validMethods.push("jwt");

      const isValidMethod = validMethods.includes(this.user.loginMethod);

      if (!this.user.loginMethod || this.user.loginMethod === null || !isValidMethod) {
        this.user.loginMethod = this.firstAvailableLoginMethod;
      }
    },
    updatePromptTitle() {
      // Update the prompt display name to show the username
      // This allows the title to show the actual username instead of just the generic "user-edit" title
      const displayName = this.isNew
        ? this.$t("general.newUser")
        : this.$t("settings.modifyOtherUserTitle", { username: this.user.username });
      mutations.updatePromptTitle(this.promptId, displayName);
    },
    async addPasskey() {
      this.addingPasskey = true;
      try {
        await authApi.beginPasskeyRegistration();
        notify.showSuccessToast(this.$t("profileSettings.passkeyAdded"));
        setTimeout(() => { window.location.reload(); }, 500);
      } catch (err) {
        notify.showError(err.message || this.$t("profileSettings.passkeyAddFailed"));
      } finally {
        this.addingPasskey = false;
      }
    },
    async deletePasskey(id) {
      try {
        await authApi.deletePasskeyCredential(id);
        notify.showSuccessToast(this.$t("profileSettings.passkeyDeleted"));
        setTimeout(() => { window.location.reload(); }, 500);
      } catch (err) {
        notify.showError(err.message || this.$t("profileSettings.passkeyDeleteFailed"));
      }
    },
    formatDate(timestamp) {
      if (!timestamp) return "";
      const date = new Date(timestamp * 1000);
      return date.toLocaleDateString();
    },
  },
};
</script>

<style scoped>
.scope-blocks {
  display: flex;
  flex-direction: column;
  gap: 0.75em;
  margin-top: 0.75em;
}

.source-dropdown-select {
  width: 100%;
}

.scope-path-row {
  display: flex;
  flex-direction: column;
  gap: 0.35em;
  margin-bottom: 0.75em;
}

.scope-path-label {
  font-size: 0.9em;
  color: var(--textSecondary, #888);
}

.scope-block :deep(.settings-group) {
  margin-top: 0.35em;
}

.scope-block :deep(.settings-group-title) {
  padding: 0.5em 0.75em;
  border: 1px solid var(--borderColor, #ddd);
  border-radius: var(--borderRadius, 4px);
}

.scope-path-display {
  width: 100%;
}

.passkey-list {
  margin-top: 0.3em;
}

.passkey-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.3em 0;
  border-bottom: 1px solid var(--borderColor, #ddd);
}

.passkey-name {
  font-weight: 500;
}

.passkey-meta {
  font-size: 0.8em;
  color: var(--textSecondary, #888);
}

.passkey-last-used::before {
  content: " · ";
}

.passkey-info {
  display: flex;
  flex-direction: column;
}

.passkey-empty {
  padding: 0.3em 0;
  color: var(--textSecondary, #888);
  font-size: 0.8em;
}
</style>
