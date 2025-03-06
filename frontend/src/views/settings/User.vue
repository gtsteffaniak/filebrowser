<template>
  <errors v-if="error" :errorCode="error.status" />
  <form @submit="save" class="card active" v-if="loaded">
    <div class="card-title">
      <h2 v-if="isNew">{{ $t("settings.newUser") }}</h2>
      <h2 v-else-if="actor.id == user.id">modify current user ({{ user.username }})</h2>
      <h2 v-else>modify user: {{ user.username }}</h2>
    </div>

    <div class="card-content">
      <user-form
        v-model:user="user"
        v-model:updatePassword="updatePassword"
        :createUserDir="createUserDir"
        :isDefault="false"
        :isNew="isNew"
        @update:createUserDir="(updatedDir) => (createUserDir = updatedDir)"
      />
    </div>

    <div class="card-action">
      <button
        v-if="!isNew && actor.perm.admin"
        @click.prevent="deletePrompt"
        type="button"
        class="button button--flat button--red"
        :aria-label="$t('buttons.delete')"
        :title="$t('buttons.delete')"
      >
        {{ $t("buttons.delete") }}
      </button>
      <input class="button button--flat" type="submit" :value="$t('buttons.save')" />
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
        perm: { admin: false },
      },
      showDelete: false,
      createUserDir: false,
      loaded: false,
      updatePassword: false,
    };
  },
  created() {
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
        } else {
          const id = Array.isArray(state.route.params.id)
            ? state.route.params.id.join("")
            : state.route.params.id;
          if (id === undefined) {
            return;
          }
          this.user = { ...(await usersApi.get(id)) };
          console.log("this is user", this.user);
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
        if (this.isNew) {
          await usersApi.create(this.user); // Use the computed property
          this.$router.push({ path: "/settings", hash: "#users-main" });
        } else {
          let which = ["all"];
          if (!this.updatePassword) {
            this.user.password = "";
          }
          console.log("this is user", this.user);
          await usersApi.update(this.user, which);
          notify.showSuccess(this.$t("settings.userUpdated"));
        }
      } catch (e) {
        notify.showError(e);
      }
    },
  },
};
</script>
