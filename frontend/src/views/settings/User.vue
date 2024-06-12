<template>
  <errors v-if="error" :errorCode="error.status" />
  <div class="row" v-else-if="!loading">
    <div class="column">
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
    </div>
  </div>
</template>

<script>
import { mapState, mapMutations, mapGetters } from "vuex";
import { users as api, settings } from "@/api";
import UserForm from "@/components/settings/UserForm";
import Errors from "@/views/Errors";

import deepClone from "@/utils/deepclone";

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
      user: {},
      showDelete: false,
      createUserDir: false,
    };
  },
  created() {
    this.fetchData();
  },
  computed: {
    isNew() {
      return this.$route.path === "/settings/users/new";
    },
    ...mapState(["loading"]),
    ...mapGetters(["currentPrompt", "currentPromptName"]),
  },
  watch: {
    $route: "fetchData",
    "user.perm.admin": function () {
      if (!this.user.perm.admin) return;
      this.user.lockPassword = false;
    },
  },
  methods: {
    ...mapMutations(["closeHovers", "showHover", "setUser", "setLoading"]),
    async fetchData() {
      this.setLoading(true);

      try {
        if (this.isNew) {
          let { defaults, createUserDir } = await settings.get();
          this.createUserDir = createUserDir;
          this.user = {
            ...defaults,
            username: "",
            passsword: "",
            rules: [],
            lockPassword: false,
            id: 0,
          };
        } else {
          const id = this.$route.params.pathMatch;
          this.user = { ...(await api.get(id)) };
        }
      } catch (e) {
        this.error = e;
      } finally {
        this.setLoading(false);
      }
    },
    deletePrompt() {
      this.$store.commit("showHover", { prompt: "deleteUser", props: { "user": this.user } });
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
          this.$showSuccess(this.$t("settings.userCreated"));
        } else {
          await api.update(user);

          if (user.id === this.$store.state.user.id) {
            this.setUser({ ...deepClone(user) });
          }

          this.$showSuccess(this.$t("settings.userUpdated"));
        }
      } catch (e) {
        this.$showError(e);
      }
    },
  },
};
</script>
