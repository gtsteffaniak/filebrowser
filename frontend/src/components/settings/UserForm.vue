<template>
  <div>
    <p v-if="!isDefault">
      <label for="username">{{ $t("settings.username") }}</label>
      <input
        class="input input--block"
        type="text"
        v-model="userData.username"
        id="username"
      />
    </p>

    <p v-if="!isDefault">
      <label for="password">{{ $t("settings.password") }}</label>
      <input
        class="input input--block"
        type="password"
        :placeholder="passwordPlaceholder"
        v-model="userData.password"
        id="password"
      />
    </p>

    <p>
      <label for="scope">{{ $t("settings.scope") }}</label>
      <input
        :disabled="createUserDir"
        :placeholder="scopePlaceholder"
        class="input input--block"
        type="text"
        v-model="userData.scope"
        id="scope"
      />
    </p>
    <p class="small" v-if="displayHomeDirectoryCheckbox">
      <input type="checkbox" v-model="createUserDir" />
      {{ $t("settings.createUserHomeDirectory") }}
    </p>

    <p>
      <label for="locale">{{ $t("settings.language") }}</label>
      <languages
        class="input input--block"
        id="locale"
        v-model:locale="userData.locale"
      ></languages>
    </p>

    <p v-if="!isDefault">
      <input
        type="checkbox"
        :disabled="userData.perm.admin"
        v-model="userData.lockPassword"
      />
      {{ $t("settings.lockPassword") }}
    </p>

    <permissions :perm="userData.perm" />
    <commands v-if="isExecEnabled" v-model:commands="userData.commands" />

    <div v-if="!isDefault">
      <h3>{{ $t("settings.rules") }}</h3>
      <p class="small">{{ $t("settings.rulesHelp") }}</p>
      <rules v-model:rules="userData.rules" />
    </div>
  </div>
</template>

<script>
import Languages from "./Languages.vue";
import Rules from "./Rules.vue";
import Permissions from "./Permissions.vue";
import Commands from "./Commands.vue";
import { enableExec } from "@/utils/constants";

export default {
  name: "UserForm",
  components: {
    Permissions,
    Languages,
    Rules,
    Commands,
  },
  data() {
    return {
      createUserDir: false,
      originalUserScope: this.user.scope,
      userData: { ...this.user }, // Create a local copy of the user object
    };
  },
  watch: {
    userData: {
      deep: true, // Watch nested changes
      handler(newValue) {
        this.$emit("update:user", newValue); // Emit the updated user data to the parent
      },
    },
    "user.perm.admin": function (newValue) {
      if (newValue) {
        this.user.lockPassword = false;
      }
    },
    createUserDir(newVal) {
      this.user.scope = newVal ? "" : this.originalUserScope;
    },
  },
  computed: {
    passwordPlaceholder() {
      return this.isNew ? "" : this.$t("settings.avoidChanges");
    },
    scopePlaceholder() {
      return this.createUserDir
        ? this.$t("settings.userScopeGenerationPlaceholder")
        : "./";
    },
    displayHomeDirectoryCheckbox() {
      return this.isNew && this.createUserDir;
    },
    isExecEnabled() {
      return enableExec; // Removed arrow function
    },
  },
};
</script>
