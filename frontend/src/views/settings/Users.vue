<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="row" v-else-if="!loading">
    <div class="column">
      <div class="card">
        <div class="card-title">
          <h2>{{ $t("settings.users") }}</h2>
          <router-link to="/settings/users/new"
            ><button class="button">
              {{ $t("buttons.new") }}
            </button></router-link
          >
        </div>

        <div class="card-content full">
          <table>
            <tr>
              <th>{{ $t("settings.username") }}</th>
              <th>{{ $t("settings.admin") }}</th>
              <th>{{ $t("settings.scope") }}</th>
              <th></th>
            </tr>

            <tr v-for="user in users" :key="user.id">
              <td>{{ user.username }}</td>
              <td>
                <i v-if="user.perm.admin" class="material-icons">done</i
                ><i v-else class="material-icons">close</i>
              </td>
              <td>{{ user.scope }}</td>
              <td class="small">
                <router-link :to="'/settings/users/' + user.id"
                  ><i class="material-icons">mode_edit</i></router-link
                >
              </td>
            </tr>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>
<script>
import { state, mutations } from "@/store";
import { users as api } from "@/api";
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
    // Set loading state to true
    mutations.setLoading(true);

    try {
      // Fetch all users from the API
      state.users = await api.getAll();
    } catch (e) {
      // Handle errors
      this.error = e;
    } finally {
      // Set loading state to false
      mutations.setLoading(false);
    }
  },
  computed: {
    // Access the loading state directly from the store
    loading() {
      return state.loading;
    },
  },
  methods: {
    // Define methods to call mutations directly
    setLoading(value) {
      mutations.setLoading(value);
    },
  },
};
</script>
