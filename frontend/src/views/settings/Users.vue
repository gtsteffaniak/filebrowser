<template>
  <button @click="openPrompt(null)" v-if="isAdmin" class="button floating-action-button" aria-label="Add New User">
        {{ $t("general.new") }}
      </button>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card-title">
    <h2>{{ $t("general.users") }}</h2>
  </div>

  <div class="card-content full">
    <settings-table
      :columns="userTableColumns"
      :items="users"
      item-key="id"
      default-sort-key="username"
      :aria-label="$t('general.users')"
      :loading="loading"
    >
      <template #cell-admin="{ row }">
        <i v-if="row.permissions.admin" class="material-symbols">done</i>
        <i v-else class="material-symbols">close</i>
      </template>
      <template #cell-scopes="{ row }">{{ formatScopes(row.scopes) }}</template>
      <template #cell-actions="{ row }">
        <div
          @click="openPrompt(row.id)"
          class="clickable action button"
          role="button"
          tabindex="0"
          :aria-label="$t('general.edit')"
          :title="$t('general.edit')"
          @keydown.enter.prevent="openPrompt(row.id)"
          @keydown.space.prevent="openPrompt(row.id)"
        >
          <i class="material-symbols">edit</i>
        </div>
      </template>
    </settings-table>
  </div>

</template>

<script>
import { state, mutations } from "@/store";
import { usersApi } from "@/api";
import Errors from "@/views/Errors.vue";
import SettingsTable from "@/components/settings/Table.vue";
import { eventBus } from "@/store/eventBus";

export default {
  name: "users",
  components: {
    Errors,
    SettingsTable,
  },
  data: function () {
    return {
      error: null,
      users: [],
      /** Local fetch state; avoids global Settings overlay spinner (table shows its own). */
      loading: true,
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
    eventBus.off('usersChanged', this.reloadUsers);
  },
  computed: {
    settings() {
      return state.settings;
    },
    isAdmin() {
      return state.user.permissions.admin;
    },
    userTableColumns() {
      return [
        {
          key: "username",
          label: this.$t("general.username"),
          sortable: true,
        },
        {
          key: "loginMethod",
          label: this.$t("settings.loginMethod"),
          sortable: true,
        },
        {
          key: "admin",
          label: this.$t("general.admin"),
        },
        {
          key: "scopes",
          label: this.$t("general.scopes"),
        },
        {
          key: "actions",
          label: "",
          align: "right",
          narrow: true,
        },
      ];
    },
  },
  methods: {
    async reloadUsers() {
      this.loading = true;
      try {
        this.users = await usersApi.getAllUsers();
        this.error = null; // Clear any previous errors
      } catch (e) {
        this.error = e;
      } finally {
        this.loading = false;
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
        mutations.showPrompt({ name: "user-edit", props: { userId } });
      } else {
        mutations.showPrompt({ name: "user-edit" });
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
