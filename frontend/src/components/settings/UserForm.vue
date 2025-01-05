<template>
  <div v-if="!user.perm.admin">
    <label for="password">{{ $t("settings.password") }}</label>
      <input
        class="input input--block"
        type="password"
        placeholder="enter new password"
        v-model="user.password"
        id="password"
        @input="emitUpdate"
      />
  </div>
  <div v-else>
    <p v-if="!isDefault">
      <label for="username">{{ $t("settings.username") }}</label>
      <input
        class="input input--block"
        type="text"
        v-model="user.username"
        id="username"
        @input="emitUpdate"
      />
    </p>

    <p v-if="!isDefault">
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

    <p>
      <label for="scope">{{ $t("settings.scope") }}</label>
      <input
        :disabled="createUserDir"
        :placeholder="scopePlaceholder"
        class="input input--block"
        type="text"
        v-model="user.scope"
        id="scope"
        @input="emitUpdate"
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
        v-model:locale="user.locale"
        @input="emitUpdate"
      ></languages>
    </p>

    <p v-if="!isDefault">
      <input
        type="checkbox"
        :disabled="user.perm?.admin"
        v-model="user.lockPassword"
        @input="emitUpdate"
      />
      {{ $t("settings.lockPassword") }}
    </p>

    <permissions :perm="localUser.perm" />
    <commands v-if="isExecEnabled" v-model:commands="user.commands" />
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
    Commands,
  },
  data() {
    return {
      createUserDir: false,
      originalUserScope: ".",
      localUser: { ...this.user },
    };
  },
  props: {
    user: Object, // Define user as a prop
    isDefault: Boolean,
    isNew: Boolean,
  },
  watch: {
    user: {
      handler(newUser) {
        this.localUser = { ...newUser };  // Watch for changes in the parent and update the local copy
      },
      immediate: true,
      deep: true,
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
