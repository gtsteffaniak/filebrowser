<template>
  <div v-if="!stateUser.perm.admin && !isNew">
    <label for="password">{{ $t("settings.password") }}</label>
    <input class="input input--block" type="password" placeholder="enter new password" v-model="user.password"
      id="password" @input="emitUpdate" />
  </div>
  <div v-else>
    <p v-if="!isDefault">
      <label for="username">{{ $t("settings.username") }}</label>
      <input class="input input--block" type="text" v-model="user.username" id="username" @input="emitUpdate" />
    </p>

    <p v-if="!isDefault">
      <label for="password">{{ $t("settings.password") }}</label>
      <input class="input input--block" type="password" :placeholder="passwordPlaceholder" v-model="user.password"
        id="password" @input="emitUpdate" />
    </p>

    <p>
      <label for="scopes">{{ $t("settings.scopes") }}</label>
    <div class="scope-list" v-for="(source, index) in selectedSources" :key="index">
      <!-- Select dropdown -->
      <select class="input flat-right" v-model="source.name">
        <option v-for="(info, name) in sourceInfo" :key="name" :value="name">
          {{ name }}
        </option>
      </select>

      <!-- Input field for scope, bound to the selectedSources array -->
      <input class="input flat-left scope-input"
          placeholder="scope eg. 'subfolder', leave blank for root"
          @input="updateParent({ name: source.name, input: $event })"
          :value="source.scope"
          :class="{ 'flat-right': index != 0 }"
        />
      <!-- Remove button -->
      <button v-if="index != 0" class="button flat-left no-height" @click="removeScope(index)">
        <i class="material-icons material-size">delete</i>
      </button>
    </div>

    <!-- Button to add more sources -->
    <button v-if='hasMoreSources' @click="addNewScopeSource" class="button no-height">
      <i class="material-icons material-size">add</i>
    </button>
    </p>


    <p class="small" v-if="displayHomeDirectoryCheckbox">
      <input type="checkbox" v-model="createUserDir" />
      {{ $t("settings.createUserHomeDirectory") }}
    </p>

    <p>
      <label for="locale">{{ $t("settings.language") }}</label>
      <languages class="input input--block" id="locale" v-model:locale="user.locale" @input="emitUpdate"></languages>
    </p>

    <p v-if="!isDefault">
      <input type="checkbox" :disabled="stateUser.perm?.admin" v-model="user.lockPassword" @input="emitUpdate" />
      {{ $t("settings.lockPassword") }}
    </p>

    <permissions :perm="localUser.perm" />
    <commands v-if="isExecEnabled" v-model:commands="user.commands" />
  </div>
</template>

<script>
import Languages from "./Languages.vue";
import Permissions from "./Permissions.vue";
import Commands from "./Commands.vue";
import { enableExec } from "@/utils/constants";
import { state } from "@/store";

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
      usedSources: [],
      availableSources: [],
      selectedSources: [],
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
        this.localUser = { ...newUser };
        this.availableSources = Object.keys(state.user.scopes);

        this.selectedSources = [];

        if (this.isNew) {
          const newSource = this.availableSources.pop();
          if (newSource) {
            this.selectedSources.push({ name: newSource, scope: "" });
          }
        } else {
          // Populate selectedSources with existing user scopes
          if (this.user.scopes && typeof this.user.scopes === "object") {
            Object.entries(this.user.scopes).forEach(([sourceName, scope]) => {
              this.selectedSources.push({ name: sourceName, scope });
              this.availableSources = this.availableSources.filter((source) => source !== sourceName);
            });
          }
        }
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
      this.user.scopes = newVal ? { default: "" } : this.originalUserScope;
    },
  },
  computed: {
    hasMoreSources() {
      return this.availableSources.length > 0;
    },
    sourceInfo() {
      return state.sources.info;
    },
    stateUser() {
      return state.user;
    },
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
  methods: {
    updateParent(value) {
      const updatedScopes = {};
      // Update the selectedSources array directly
      // Update the selectedSources array directly
      this.selectedSources.forEach((source, index) => {
        if (source.name === value.name) {
          this.selectedSources[index] = { ...source, scope: value.input.target.value };
        }
      });

      this.selectedSources.forEach(source => {
        updatedScopes[source.name] = source.scope;
      });
      this.$emit("update:user", { ...this.user, scopes: updatedScopes });
    },
    addNewScopeSource(event) {
      event.preventDefault();
      if (this.availableSources.length > 0) {
        const newSource = this.availableSources.pop();
        if (newSource) {
          this.selectedSources.push({ name: newSource, scope: "" });
        }
      }
    },
    removeScope(index) {
      const removedSource = this.selectedSources.splice(index, 1)[0];
      this.availableSources.push(removedSource.name); // Make source available again
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
