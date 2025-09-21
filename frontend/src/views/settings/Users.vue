<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card-title">
    <h2>{{ $t("settings.users") }}</h2>
      <button @click="openPrompt(null)" v-if="isAdmin" class="button" aria-label="Add New User">
        {{ $t("buttons.new") }}
      </button>
  </div>

  <div class="card-content full">
    <table aria-label="Users">
      <thead>
        <tr>
          <th>{{ $t("settings.username") }}</th>
          <th>{{ $t("settings.loginMethod") }}</th>
          <th>{{ $t("settings.admin") }}</th>
          <th>{{ $t("settings.userScopes") }}</th>
          <th></th>
        </tr>
      </thead>
      <tbody class="settings-items">
        <tr class="item" v-for="user in users" :key="user.id">
          <td>{{ user.username }}</td>
          <td>{{ user.loginMethod }}</td>
          <td>
            <i v-if="user.permissions.admin" class="material-icons">done</i>
            <i v-else class="material-icons">close</i>
          </td>
          <td>{{ formatScopes(user.scopes) }}</td>
          <td class="small" aria-label="Edit User">
            <div @click="openPrompt(user.id)" class="clickable">
              <i class="material-icons">mode_edit</i>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>

</template>

<script>
import { state, mutations, getters } from "@/store";
import { usersApi } from "@/api";
import Errors from "@/views/Errors.vue";
import { eventBus } from "@/store/eventBus";

export default {
  name: "users",
  components: {
    Errors,
  },
  data: function () {
    return {
      error: null,
      users: [],
    };
  },
  async created() {
    await this.reloadUsers();
  },
  mounted() {
    // Listen for user changes
    eventBus.on('usersChanged', this.reloadUsers);
  },
  beforeUnmount() {
    // Clean up event listener
    eventBus.removeEventListener('usersChanged', this.reloadUsers);
  },
  computed: {
    settings() {
      return state.settings;
    },
    isAdmin() {
      return state.user.permissions.admin;
    },
    // Access the loading state directly from the store
    loading() {
      return getters.isLoading();
    },
  },
  methods: {
    async reloadUsers() {
      mutations.setLoading("users", true);
      try {
        this.users = await usersApi.getAllUsers();
        this.error = null; // Clear any previous errors
      } catch (e) {
        this.error = e;
      } finally {
        mutations.setLoading("users", false);
      }
    },
    formatScopes(scopes) {
      if (!Array.isArray(scopes)) {
        return scopes;
      }
      return scopes
        .map((scope) => `"${scope.name}": "${scope.scope}"`)
        .join(", ");
    },
    openPrompt(userId) {
      if (userId) {
        mutations.showHover({ name: "user-edit", props: { userId } });
      } else {
        mutations.showHover({ name: "user-edit"});
      }
    },
  },
};
</script>

<style scoped>
.clickable {
  cursor: pointer;
}
</style>
