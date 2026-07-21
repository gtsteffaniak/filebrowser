<template>
  <button
    type="button"
    @click.prevent="createPrompt"
    class="button floating-action-button"
  >
    {{ $t("general.new") }}
  </button>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card-title">
    <h2>{{ $t("api.title") }}</h2>
  </div>

  <div class="card-content full">
    <div class="settings-items">
      <ActivityViewerButton class="item" :href="activityViewerHref" />
    </div>
    <template v-if="links.length > 0">
      <p>
        {{ $t("api.description") }}
        <a class="link" href="swagger/index.html">{{ $t("api.swaggerLinkText") }}</a>
      </p>
    </template>
    <settings-table
      :columns="apiTableColumns"
      :items="links"
      item-key="name"
      default-sort-key="name"
      :aria-label="$t('api.title')"
      :loading="loading"
    >
        <template #cell-issuedAt="{ row }">{{ formatTime(row.issuedAt) }}</template>
        <template #cell-expiresAt="{ row }">{{ formatTime(row.expiresAt) }}</template>
        <template #cell-permissions="{ row }">
          <template v-if="permissionsForRow(row)">
            <span
              v-for="(value, permission) in permissionsForRow(row)"
              :key="permission"
              :title="`${permission}: ${value ? $t('general.enabled') : $t('general.disabled')}`"
              class="clickable"
              @click.prevent="infoPrompt(row.name, row)"
            >
              {{ showResult(value) }}
            </span>
          </template>
          <span v-else>-</span> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        </template>
        <template #cell-actions="{ row }">
          <div class="api-table-actions">
            <button
              type="button"
              class="action"
              @click.prevent="infoPrompt(row.name, row)"
              :aria-label="$t('general.actions')"
              :title="$t('general.actions')"
            >
              <i class="material-symbols">info</i>
            </button>
            <button
              type="button"
              class="action"
              @click.stop="copyToClipboard(row.token)"
              :aria-label="$t('buttons.copyToClipboard')"
              :title="$t('buttons.copyToClipboard')"
            >
              <i class="material-symbols">content_paste</i>
            </button>
          </div>
        </template>
    </settings-table>
  </div>

</template>

<script>
import { authApi } from "@/api";
import { state, mutations, getters } from "@/store";
import { copyToClipboard } from "@/utils/clipboard";
import Errors from "@/views/Errors.vue";
import SettingsTable from "@/components/settings/Table.vue";
import ActivityViewerButton from "@/components/settings/ActivityViewerButton.vue";
import { activityViewerPresets } from "@/utils/activityViewerLink";
import { eventBus } from "@/store/eventBus";

export default {
  name: "api",
  components: {
    Errors,
    SettingsTable,
    ActivityViewerButton,
  },
  data: () => ({
    error: null,
    links: [],
    user: {
      permissions: { ...state.user.permissions}
    },
    /** Local fetch state; avoids global Settings overlay spinner (table shows its own). */
    loading: true,
  }),
  async created() {
    await this.reloadApiKeys();
  },
  mounted() {
    // Listen for API key changes
    eventBus.on('apiKeysChanged', this.reloadApiKeys);
  },
  beforeUnmount() {
    // Clean up event listener
    eventBus.off('apiKeysChanged', this.reloadApiKeys);
  },
  computed: {
    settings() {
      return state.settings;
    },
    active() {
      return state.activeSettingsView === "shares-main";
    },
    apiTableColumns() {
      return [
        { key: "name", label: this.$t("general.name"), sortable: true },
        {
          key: "issuedAt",
          label: this.$t("general.created"),
          sortable: true,
          sortFn: (a, b) =>
            ((a?.issuedAt ?? 0) - (b?.issuedAt ?? 0)),
        },
        {
          key: "expiresAt",
          label: this.$t("general.expires"),
          sortable: true,
          sortFn: (a, b) =>
            ((a?.expiresAt ?? 0) - (b?.expiresAt ?? 0)),
        },
        { key: "permissions", label: this.$t("general.permissions") },
        {
          key: "actions",
          label: this.$t("general.actions"),
          align: "right",
          narrow: true,
        },
      ];
    },
    activityViewerHref() {
      return activityViewerPresets.apiTokens();
    },
  },
  methods: {
    async copyToClipboard(text) {
      await copyToClipboard(text);
    },
    async reloadApiKeys() {
      this.loading = true;
      try {
        // Fetch the API keys from the specified endpoint
        this.links = await authApi.getApiKeys();
        this.error = null; // Clear errors
      } catch (e) {
        // ignore 404 errors
        if (e.status !== 404) {
          this.error = e;
        }
      } finally {
        this.loading = false;
      }
    },
    showResult(value) {
      return value ? "✓" : "✗";
    },
    permissionsForRow(row) {
      if (row?.Permissions && Object.keys(row.Permissions).length > 0) {
        return row.Permissions;
      }
      if (row?.minimal && state.user?.permissions) {
        return getters.apiTokenPermissionCaps();
      }
      return null;
    },
    createPrompt() {
      mutations.showPrompt({
        name: "CreateApi",
        props: {
          permissions: getters.apiTokenPermissionCaps(),
          userPermissions: getters.apiTokenPermissionCaps(),
        },
      });
    },
    infoPrompt(name, info) {
      mutations.showPrompt({ name: "ActionApi", props: { name: name, info: info } });
    },
    formatTime(time) {
      return new Date(time * 1000).toLocaleDateString("en-US", {
        year: "numeric",
        month: "long",
        day: "numeric",
      });
    },
  },
};
</script>
<style>
.permissions-cell {
  position: relative;
  display: inline-block;
}

.permissions-placeholder {
  color: #888;
  /* Styling for the placeholder text */
}

.permissions-list {
  display: none;
  position: absolute;
  top: 100%;
  /* Position the popup below the cell */
  left: 0;
  background-color: white;
  border: 1px solid #ccc;
  padding: 8px;
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
  z-index: 10;
  width: max-content;
}

.permissions-cell:hover .permissions-list {
  display: block;
}

.api-table-actions {
  display: inline-flex;
  flex-direction: row;
  flex-wrap: nowrap;
  gap: 0.25em;
  justify-content: flex-end;
  align-items: center;
}

.api-table-actions .action {
  flex-shrink: 0;
}
</style>
