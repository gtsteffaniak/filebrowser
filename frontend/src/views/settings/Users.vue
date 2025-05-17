<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card">
    <div class="card-title">
      <h2>{{ $t("settings.users") }}</h2>
      <router-link v-if="isAdmin" to="/settings/users/new">
        <button class="button" aria-label="Add New User">
          {{ $t("buttons.new") }}
        </button>
      </router-link>
    </div>

    <div class="card-content full">
      <table aria-label="Users">
        <thead>
          <tr>
            <th>{{ $t("settings.username") }}</th>
            <th>{{ $t("settings.loginMethod") }}</th>
            <th>{{ $t("settings.admin") }}</th>
            <th>Scopes</th> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
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
            <td>{{ user.scopes }}</td>
            <td class="small" aria-label="Edit User">
              <router-link :to="'/settings/users/' + user.id">
                <i class="material-icons">mode_edit</i>
              </router-link>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script>
import { state, mutations, getters } from "@/store";
import { usersApi } from "@/api";
import Errors from "@/views/Errors.vue";

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
    mutations.setLoading("users", true);
    // Set loading state to true
    this.users = await usersApi.getAllUsers();
    mutations.setLoading("users", false);
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
};
</script>
