<template>
  <errors v-if="error" :errorCode="error.status" />
  <form @submit.prevent="save" v-if="loaded">
    <div class="card-title">
      <h2 v-if="isNew">{{ $t("settings.newUser") }}</h2>
      <h2 v-else-if="actor.id == user.id">
        {{ $t("settings.modifyCurrentUser") }} {{ user.username }}
      </h2>
      <h2 v-else>{{ $t("settings.modifyOtherUser") }} {{ user.username }}</h2>
    </div>
    <div class="card-content minimal-card">
      <user-form v-model:user="user" :createUserDir="createUserDir" :isNew="isNew"
        @update:createUserDir="(updatedDir) => (createUserDir = updatedDir)" />
    </div>
    <div v-if="actor.permissions.admin" class="card-action">
      <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')">
        {{ $t("buttons.cancel") }}
      </button>
      <button v-if="!isNew" @click.prevent="deletePrompt" type="button" class="button button--flat button--red"
        aria-label="Delete User" :title="$t('buttons.delete')">
        {{ $t("buttons.delete") }}
      </button>
      <input aria-label="Save User" class="save-button button button--flat" type="submit" :value="$t('buttons.save')" />
    </div>
  </form>
</template>

<script>
import { mutations, state } from "@/store";
import { usersApi, settingsApi } from "@/api";
import UserForm from "@/components/settings/UserForm.vue";
import Errors from "@/views/Errors.vue";
import { notify } from "@/notify";

export default {
  name: "user-edit",
  components: {
    UserForm,
    Errors,
  },
  props: {
    userId: {
      type: [String, Number],
      required: false,
    },
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
        otpEnabled: false,
      },
      showDelete: false,
      createUserDir: false,
      loaded: false,
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
      return !this.userId;
    },
  },
  methods: {
    closeHovers() {
      mutations.closeHovers();
    },
    async fetchData() {
      mutations.setLoading("users", true);
      try {
        if (this.isNew) {
          let defaults = await settingsApi.get("userDefaults");
          this.user = defaults;
          this.user.password = "";
        } else {
          const id = this.userId;
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
        window.location.reload();
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

.save-button {
  width: 33%;
}
</style>
