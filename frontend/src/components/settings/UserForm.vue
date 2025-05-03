<template>
  <div
    v-if="!stateUser.permissions.admin && !isNew && stateUser.loginMethod == 'password'"
  >
    <label for="password">{{ $t("settings.password") }}</label>
    <input
      class="input input--block"
      type="password"
      placeholder="enter new password"
      v-model="user.password"
      id="password"
      @input="setUpdatePassword"
    />
  </div>
  <div v-else>
    <p v-if="isNew">
      <label for="username">{{ $t("settings.username") }}</label>
      <input
        class="input input--block"
        type="text"
        v-model="user.username"
        id="username"
        @input="emitUpdate"
      />
    </p>

    <p v-if="stateUser.loginMethod == 'password'">
      <label for="password">{{ $t("settings.password") }}</label>
      <input
        class="input input--block"
        type="password"
        :placeholder="passwordPlaceholder"
        v-model="user.password"
        id="password"
        @input="emitUpdate"
      />
    </p>

    <div class="settings-items">
      <ToggleSwitch
        class="item"
        v-if="user.loginMethod === 'password'"
        :modelValue="updatePassword"
        @update:modelValue="$emit('update:updatePassword', $event)"
        :name="$t('settings.changePassword')"
      />
      <ToggleSwitch
        v-if="user.loginMethod === 'password' && stateUser.permissions?.admin"
        class="item"
        :modelValue="user.lockPassword"
        @update:modelValue="(val) => updateUserField('lockPassword', val)"
        :name="$t('settings.lockPassword')"
      />
    </div>

    <div v-if="stateUser.permissions.admin">
      <label for="scopes">{{ $t("settings.scopes") }}</label>
      <div
        class="scope-list"
        :class="{ 'invalid-form': duplicateSources.includes(source.name) }"
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
        class="input input--block"
        id="locale"
        v-model:locale="user.locale"
        @input="emitUpdate"
      ></languages>
    </p>

    <permissions v-if="stateUser.permissions.admin" :permissions="user.permissions" />
  </div>
</template>

<script>
import Languages from "./Languages.vue";
import Permissions from "./Permissions.vue";
import { state } from "@/store";
import { settingsApi } from "@/api";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "UserForm",
  components: {
    Permissions,
    Languages,
    ToggleSwitch,
  },
  props: {
    user: Object,
    updatePassword: Boolean,
    isNew: Boolean,
  },
  data() {
    return {
      createUserDir: false,
      originalUserScope: ".",
      sourceList: [],
      availableSources: [],
      selectedSources: [],
    };
  },
  async mounted() {
    if (!this.stateUser.permissions.admin) {
      this.sourceList = this.user.scopes || [];
    } else {
      this.sourceList = await settingsApi.get("sources");
    }

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
  },
  computed: {
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
.flat-right {
  border-top-right-radius: 0 !important;
  border-bottom-right-radius: 0 !important;
}
.flat-left {
  border-top-left-radius: 0 !important;
  border-bottom-left-radius: 0 !important;
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
