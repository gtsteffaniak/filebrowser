<template>
  <errors v-if="error" :errorCode="error.status" />
  <form @submit="save" class="card active">
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
        scope: ".",
        username: "",
        perm: { admin: false },
      },
      showDelete: false,
      createUserDir: false,
    };
  },
  created() {
    this.fetchData();
  },
  computed: {
    settings() {
      return state.settings;
    },
    isNew() {
      return state.route.path.startsWith("/settings/users/new");
    },
  },
  methods: {
    async fetchData() {
      if (!state.route.path.startsWith("/settings")) {
        return;
      }
      mutations.setLoading("users", true);
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
          const id = Array.isArray(state.route.params.id)
            ? state.route.params.id.join("")
            : state.route.params.id;
          this.user = { ...(await api.get(id)) };
        }
      } catch (e) {
        notify.showError(e);
        this.error = e;
      } finally {
        mutations.setLoading("users", false);
      }
    },
    deletePrompt() {
      mutations.showHover({ name: "deleteUser", props: { user: this.user } });
    },
    async save(event) {
      let user = this.user;
      event.preventDefault();
      try {
        if (this.isNew) {
          const loc = await api.create(user);
          this.$router.push({ path: loc });
          notify.showSuccess(this.$t("settings.userCreated"));
        } else {
          await api.update(user);
          notify.showSuccess(this.$t("settings.userUpdated"));
        }
      } catch (e) {
        notify.showError(e);
      }
    },
  },
};
</script>
