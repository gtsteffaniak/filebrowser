<template>
  <errors v-if="error" :errorCode="error.status" />
  <form @submit="save" class="card active" v-if="loaded">
    <div class="card-title">
      <h2 v-if="isNew">{{ $t("settings.newUser") }}</h2>
      <h2 v-else-if="actor.id == user.id">
        {{ $t("settings.modifyCurrentUser") }} {{ user.username }}
      </h2>
      <h2 v-else>{{ $t("settings.modifyOtherUser") }} {{ user.username }}</h2>
    </div>

    <div class="card-content minimal-card">
      <user-form
        v-model:user="user"
        :createUserDir="createUserDir"
        :isNew="isNew"
        @update:createUserDir="(updatedDir) => (createUserDir = updatedDir)"
      />
    </div>

    <div v-if="actor.permissions.admin" class="card-action">
      <button
        v-if="!isNew"
        @click.prevent="deletePrompt"
        type="button"
        class="button button--flat button--red"
        aria-label="Delete User"
        :title="$t('buttons.delete')"
      >
        {{ $t("buttons.delete") }}
      </button>
      <input
        aria-label="Save User"
        class="button button--flat"
        type="submit"
        :value="$t('buttons.save')"
      />
    </div>
  </form>
</template>

<script>
import { getters, mutations, state } from "@/store";
import { usersApi, settingsApi } from "@/api";
import UserForm from "@/components/settings/UserForm.vue";
import Errors from "@/views/Errors.vue";
import { notify } from "@/notify";

export default {
  name: "user",
  components: {
    UserForm,
    Errors,
  },
  data() {
    return {
      error: null,
      originalUser: null,
      user: {
        scopes: [],
        username: "",
        password: "",
        permissions: { admin: false },
      },
      showDelete: false,
      createUserDir: false,
      loaded: false,
    };
  },
  created() {
    mutations.setActiveSettingsView("");
    this.fetchData();
  },
  computed: {
    actor() {
      return state.user;
    },
    settings() {
      return state.settings;
    },
    isNew() {
      return getters.routePath().endsWith("settings/users/new");
    },
  },
  methods: {
    async fetchData() {
      mutations.setLoading("users", true);
      try {
        if (this.isNew) {
          let defaults = await settingsApi.get("userDefaults");
          this.user = defaults;
          this.user.password = "";
        } else {
          const id = Array.isArray(state.route.params.id)
            ? state.route.params.id.join("")
            : state.route.params.id;
          if (id === undefined) {
            return;
          }
          this.user = { ...(await usersApi.get(id)) };
          this.user.password = "";
        }
      } catch (e) {
        notify.showError(e);
        this.error = e;
      } finally {
        mutations.setLoading("users", false);
        this.loaded = true;
      }
    },
    deletePrompt() {
      mutations.showHover({ name: "deleteUser", props: { user: this.user } });
    },
    async save(event) {
      event.preventDefault();
      try {
        let fields = ["all"];
        if (!state.user.permissions.admin) {
          notify.showError(this.$t("settings.userNotAdmin"));
          return;
        }
        if (this.isNew) {
          await usersApi.create(this.user); // Use the computed property
          this.$router.push({ path: "/settings", hash: "#users-main" });
        } else {
          await usersApi.update(this.user, fields);
          notify.showSuccess(this.$t("settings.userUpdated"));
        }
      } catch (e) {
        notify.showError(e);
      }
    },
  },
};
</script>

<style scoped>
.minimal-card {
  /* margin-bottom: 16px; */
  padding-top: 0 !important;
  padding-bottom: 0 !important;
}
</style>
