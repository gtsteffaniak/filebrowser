<template>
  <errors v-if="error" :errorCode="error.status" />
  <form @submit="save" class="card">
    <div class="card-title">
      <h2 v-if="user.id === 0">{{ $t("settings.newUser") }}</h2>
      <h2 v-else>{{ $t("settings.user") }} {{ user.username }}</h2>
    </div>

    <div class="card-content">
      <user-form
        :user="user"
        :createUserDir="createUserDir"
        :isDefault="false"
        :isNew="isNew"
        @update:user="(updatedUser) => (user = updatedUser)"
        @update:createUserDir="(updatedDir) => (createUserDir = updatedDir)"
      />
    </div>

    <div class="card-action">
      <button
        v-if="!isNew"
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
import { mutations, state } from "@/store";
import { users as api, settings } from "@/api";
import UserForm from "@/components/settings/UserForm.vue";
import Errors from "@/views/Errors.vue";
import { showSuccess, showError } from "@/notify";

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
      user: { perm: { admin: false } },
      showDelete: false,
      createUserDir: false,
      loading: false, // Replaces Vuex state `loading`
      currentPrompt: null, // Replaces Vuex getter `currentPrompt`
      currentPromptName: null, // Replaces Vuex getter `currentPromptName`
    };
  },
  created() {
    this.fetchData();
  },
  computed: {
    isNew() {
      return state.route.path === "/settings/users/new";
    },
  },
  watch: {
    $route: "fetchData",
  },
  methods: {
    async fetchData() {
      mutations.setLoading(true);
      try {
        if (this.isNew) {
          let { defaults, createUserDir } = await settings.get();
          this.createUserDir = createUserDir;
          this.user = {
            ...defaults,
            username: "",
            password: "",
            rules: [],
            lockPassword: false,
            id: 0,
          };
        } else {
          const id = state.route.params.id;
          this.user = { ...(await api.get(id)) };
        }
      } catch (e) {
        showError(e);
        this.error = e;
      } finally {
        mutations.setLoading(false);
      }
    },
    deletePrompt() {
      mutations.showHover({ name: "deleteUser", props: { user: this.user } });
    },
    async save(event) {
      event.preventDefault();
      let user = {
        ...this.originalUser,
        ...this.user,
      };

      try {
        if (this.isNew) {
          const loc = await api.create(user);
          this.$router.push({ path: loc });
          showSuccess(this.$t("settings.userCreated"));
        } else {
          await api.update(user);
          if (user.id === state.user.id) {
            mutations.setUser(user);
          }
          showSuccess(this.$t("settings.userUpdated"));
        }
      } catch (e) {
        showError(e);
      }
    },
  },
};
</script>
