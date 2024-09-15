<template>
  <div>
    <p v-if="!isDefault">
      <label for="username">{{ $t("settings.username") }}</label>
      <input
        class="input input--block"
        type="text"
        v-model="$props.user.username"
        id="username"
      />
    </p>

    <p v-if="!isDefault">
      <label for="password">{{ $t("settings.password") }}</label>
      <input
        class="input input--block"
        type="password"
        :placeholder="passwordPlaceholder"
        v-model="$props.user.password"
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
        v-model="$props.user.scope"
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
        v-model:locale="$props.user.locale"
      ></languages>
    </p>

    <p v-if="!isDefault">
      <input type="checkbox" :disabled="user.perm.admin" v-model="user.lockPassword" />
      {{ $t("settings.lockPassword") }}
    </p>

    <permissions :perm="$props.user.perm" />
    <commands v-if="isExecEnabled" v-model:commands="user.commands" />

    <div v-if="!isDefault">
      <h3>{{ $t("settings.rules") }}</h3>
      <p class="small">{{ $t("settings.rulesHelp") }}</p>
      <rules v-model:rules="$props.user.rules" />
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
  data() {
    return {
      createUserDir: false,
      originalUserScope: this.user.scope, // Store the original scope if needed
    };
  },
  components: {
    Permissions,
    Languages,
    Rules,
    Commands,
  },
  props: {
    isNew: {
      type: Boolean,
      required: true,
    },
    isDefault: {
      type: Boolean,
      required: true,
    },
    user: {
      type: Object,
      required: true,
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
  watch: {
    "user.perm.admin": function (newValue) {
      if (newValue) {
        this.user.lockPassword = false;
      }
    },
    createUserDir(newVal) {
      this.user.scope = newVal ? "" : this.originalUserScope;
    },
  },
};
</script>
