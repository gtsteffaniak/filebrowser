<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card">
    <div class="card-title">
      <h2>{{ $t("settings.users") }}</h2>
      <router-link v-if="isAdmin" to="/settings/users/new">
        <button class="button">
          {{ $t("buttons.new") }}
        </button>
      </router-link>
    </div>

    <div class="card-content full">
      <table>
        <thead>
          <tr>
            <th>{{ $t("settings.username") }}</th>
            <th>{{ $t("settings.admin") }}</th>
            <th>scopes</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users" :key="user.id">
            <td>{{ user.username }}</td>
            <td>
              <i v-if="user.perm.admin" class="material-icons">done</i>
              <i v-else class="material-icons">close</i>
            </td>
            <td>{{ user.scopes }}</td>
            <td class="small">
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
      return state.user.perm.admin;
    },
    // Access the loading state directly from the store
    loading() {
      return getters.isLoading();
    },
  },
};
</script>
