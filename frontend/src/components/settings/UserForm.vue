<template>
  <div v-if="!stateUser.permissions.admin && !isNew">
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
    <p>
      <label for="username">{{ $t("settings.username") }}</label>
      <input
        class="input input--block"
        type="text"
        v-model="user.username"
        id="username"
        @input="emitUpdate"
      />
    </p>

    <p>
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

    <p v-if="!isNew">
      <input
        type="checkbox"
        :checked="updatePassword"
        @change="(event) => $emit('update:updatePassword', event.target.checked)"
      />
      Change password on save
    </p>

    <p>
      <input
        type="checkbox"
        :disabled="!stateUser.permissions?.admin"
        v-model="user.lockPassword"
        @input="emitUpdate"
      />
      {{ $t("settings.lockPassword") }}
    </p>

    <label for="scopes">{{ $t("settings.scopes") }}</label>
    <div class="scope-list" v-for="(source, index) in selectedSources" :key="index">
      <!-- Select dropdown -->
      <select class="input flat-right" v-model="source.name">
        <option v-for="s in sourceList" :key="s" :value="s.name">
          {{ s.name }}
        </option>
      </select>

      <!-- Input field for scope, bound to the selectedSources array -->
      <input
        class="input flat-left scope-input"
        placeholder="scope eg. 'subfolder', leave blank for root"
        @input="updateParent({ source: source, input: $event })"
        :value="source.scope"
        :class="{ 'flat-right': index != 0 }"
      />
      <!-- Remove button -->
      <button
        v-if="index != 0"
        class="button flat-left no-height"
        @click="removeScope(index)"
      >
        <i class="material-icons material-size">delete</i>
      </button>
    </div>

    <!-- Button to add more sources -->
    <button v-if="hasMoreSources" @click="addNewScopeSource" class="button no-height">
      <i class="material-icons material-size">add</i>
    </button>

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

    <permissions :perm="localUser.permissions" />
  </div>
</template>

<script>
import Languages from "./Languages.vue";
import Permissions from "./Permissions.vue";
import { state } from "@/store";
import { settingsApi } from "@/api";

export default {
  name: "UserForm",
  components: {
    Permissions,
    Languages,
  },
  data() {
    return {
      createUserDir: false,
      originalUserScope: ".",
      localUser: { ...this.user },
      sourceList: [],
      availableSources: [],
      selectedSources: [],
    };
  },
  props: {
    user: Object, // Define user as a prop
    updatePassword: Boolean,
    isNew: Boolean,
  },
  async mounted() {
    if (!this.stateUser.permissions.admin) {
      this.sourceList = this.user.scopes;
    } else {
      this.sourceList = await settingsApi.get("sources");
    }
    this.localUser = { ...this.user };
    this.selectedSources = [];
    this.availableSources = [...this.sourceList];
    if (this.isNew) {
      const newSource = this.availableSources.shift(); // Take the first item instead of last
      if (newSource) {
        this.selectedSources.push(newSource);
      }
    } else {
      // Populate selectedSources with existing user scopes
      if (this.user.scopes) {
        this.selectedSources = this.user.scopes;
        // remove items with same name from availableSources
        for (const source of this.selectedSources) {
          this.availableSources = this.availableSources.filter(
            (s) => source.name != s.name
          );
        }
      }
    }
  },
  watch: {
    "user.permissions.admin": function (newValue) {
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
  },
  methods: {
    setUpdatePassword() {
      this.$emit("update:updatePassword", true);
    },
    updateParent(input) {
      let updatedScopes = {};
      // Update the selectedSources array directly
      this.selectedSources.forEach((source) => {
        if (source.name === input.source.name) {
          updatedScopes[source.name] = input.input.target.value;
        } else {
          updatedScopes[source.name] = source.scope;
        }
      });
      let intermediate = [];
      Object.entries(updatedScopes).forEach(([key, value]) => {
        intermediate.push({ name: key, scope: value });
      });
      let final = [];
      for (const source of intermediate) {
        final.push(source);
      }
      this.selectedSources = final;
      this.$emit("update:user", { ...this.user, scopes: this.selectedSources });
    },
    addNewScopeSource(event) {
      event.preventDefault();
      if (this.availableSources.length > 0) {
        const newSource = this.availableSources.pop();
        if (newSource) {
          const scope = { name: newSource.name, scope: "" };
          this.selectedSources.push(scope);
          this.updateParent({ source: scope, input: { target: { value: "" } } });
        }
      }
    },
    removeScope(index) {
      const removedSource = this.selectedSources.splice(index, 1)[0];
      this.availableSources.push({ name: removedSource.name }); // Make source available again
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
