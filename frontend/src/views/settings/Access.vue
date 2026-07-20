<template>
  <button
    type="button"
    class="button floating-action-button"
    @click="addAccess"
  >
   {{ $t("general.new") }}
  </button>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card-title">
    <h2>{{ $t("access.accessManagement") }}</h2>
    <div class="form-flex-group">
      <ExpandDropdown
        input-id="source-select"
        v-model="selectedSource"
        :options="sourceOptions"
        :aria-label="$t('general.source')"
        @update:model-value="fetchRules"
      />
    </div>
    <a class="button button--flat button--blue activity-viewer-link" :href="activityViewerHref">{{ $t("tools.activityViewer.viewActivity") }}</a>
  </div>
  <div class="card-content full">
    <SettingsItem
      :title="$t('general.permissions')"
      :collapsable="true"
      :start-collapsed="true"
    >
      <p class="small">{{ $t('settings.sourcePermissionsHelp') }}</p>
      <SourceFilePermissions
        v-if="!defaultsLoading"
        :permissions="sourceAccessDefaults"
        @changed="onSourceAccessDefaultsChange"
      />
      <div v-else class="loading-hint">{{ $t('general.loading') }}</div>
    </SettingsItem>
    <settings-table
      :columns="accessTableColumns"
      :items="accessTableRows"
      item-key="path"
      default-sort-key="path"
      :aria-label="$t('access.accessManagement')"
      :loading="loading"
    >
      <template #cell-warning="{ row }">
        <i
          v-if="!row.rule.pathExists"
          class="material-symbols warning-icon"
          :title="$t('messages.pathNotFound')"
        >warning</i>
      </template>
      <template #cell-edit="{ row }">
        <button
          type="button"
          class="action"
          @click="editAccess(row.path)"
          :aria-label="$t('general.edit')"
          :title="$t('general.edit')"
        >
          <i class="material-symbols">edit</i>
        </button>
      </template>
    </settings-table>
  </div>
</template>

<script>
import { accessApi } from "@/api";
import * as settingsApi from "@/api/settings";
import { state, mutations } from "@/store";
import Errors from "@/views/Errors.vue";
import SettingsTable from "@/components/settings/Table.vue";
import SettingsItem from "@/components/settings/SettingsItem.vue";
import SourceFilePermissions from "@/components/settings/SourceFilePermissions.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import { notify } from "@/notify";
import { activityViewerPresets } from "@/utils/activityViewerLink";
import { eventBus } from "@/store/eventBus";
export default {
  name: "accessSettings",
  components: {
    Errors,
    SettingsTable,
    SettingsItem,
    SourceFilePermissions,
    ExpandDropdown,
  },
  data: () => ({
    rules: {},
    accessPath: "",
    error: null,
    selectedSource: "",
    /** True until first `fetchRules` completes so the table does not flash the empty state. */
    loading: true,
    defaultsLoading: true,
    savingDefaults: false,
    sourceAccessDefaults: {
      view: true,
      download: true,
      modify: false,
      create: false,
      delete: false,
    },
  }),
  async mounted() {
    this.selectedSource = state.sources.current;
    await Promise.all([this.fetchRules(), this.loadSourceAccessDefaults()]);
    // Listen for access rule changes
    eventBus.on('accessRulesChanged', this.fetchRules);
  },
  beforeUnmount() {
    // Clean up event listener
    eventBus.off('accessRulesChanged', this.fetchRules);
  },
  computed: {
    availableSources() {
      return Object.keys(state.sources.info);
    },
    sourceOptions() {
      return this.availableSources.map((source) => ({
        value: source,
        label: source,
      }));
    },
    accessTableRows() {
      return Object.entries(this.rules).map(([path, rule]) => ({
        path,
        rule,
        denyTotal:
          rule.deny.users.length + rule.deny.groups.length + (rule.denyAll ? 1 : 0),
        allowTotal: rule.allow.users.length + rule.allow.groups.length,
      }));
    },
    accessTableColumns() {
      return [
        { key: "path", label: this.$t("general.path"), sortable: true },
        {
          key: "denyTotal",
          label: this.$t("access.totalDenied"),
          sortable: true,
          sortFn: (a, b) => (a.denyTotal ?? 0) - (b.denyTotal ?? 0),
        },
        {
          key: "allowTotal",
          label: this.$t("access.totalAllowed"),
          sortable: true,
          sortFn: (a, b) => (a.allowTotal ?? 0) - (b.allowTotal ?? 0),
        },
        { key: "warning", label: "", narrow: true },
        {
          key: "edit",
          label: this.$t("general.edit"),
          narrow: true,
          align: "right",
        },
      ];
    },
    activityViewerHref() {
      return activityViewerPresets.access(this.selectedSource, "/");
    },
  },
  methods: {
    async loadSourceAccessDefaults() {
      this.defaultsLoading = true;
      try {
        const settings = await settingsApi.getSourceSettings();
        const perms = settings?.defaultFilePermissions ?? {};
        this.sourceAccessDefaults = {
          view: perms.view !== false,
          download: perms.download !== false,
          modify: !!perms.modify,
          create: !!perms.create,
          delete: !!perms.delete,
        };
      } catch (e) {
        console.error(e);
        if (e?.message) {
          notify.showError(e.message);
        }
      } finally {
        this.defaultsLoading = false;
      }
    },
    async onSourceAccessDefaultsChange() {
      if (this.savingDefaults) {
        return;
      }
      this.savingDefaults = true;
      try {
        const settings = await settingsApi.patchSourceSettings({
          defaultFilePermissions: this.sourceAccessDefaults,
        });
        const perms = settings?.defaultFilePermissions ?? {};
        this.sourceAccessDefaults = {
          view: perms.view !== false,
          download: perms.download !== false,
          modify: !!perms.modify,
          create: !!perms.create,
          delete: !!perms.delete,
        };
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
      } catch (e) {
        console.error(e);
        if (e?.message) {
          notify.showError(e.message);
        }
        await this.loadSourceAccessDefaults();
      } finally {
        this.savingDefaults = false;
      }
    },
    async fetchRules() {
      const source = this.selectedSource;
      this.loading = true;
      this.error = null;
      this.accessPath = state.req.path || '/';
      try {
        const rules = await accessApi.getAll(source);
        if (source !== this.selectedSource) {
          return;
        }
        this.rules = rules;
      } catch (e) {
        if (source !== this.selectedSource) {
          return;
        }
        this.error = e;
      } finally {
        if (source === this.selectedSource) {
          this.loading = false;
        }
      }
    },
    addAccess() {
      mutations.showPrompt({
        name: "access",
        props: {
          sourceName: this.selectedSource,
          path: "/"
        }
      });
    },
    editAccess(path) {
      mutations.showPrompt({
        name: "access",
        props: {
          sourceName: this.selectedSource,
          path: path
        }
      });
    },
  },
};
</script>
<style scoped>
.form-flex-group {
  margin-bottom: 1em;
}
.card-content.full :deep(.settings-group) {
  margin-bottom: 0.75rem;
}
.loading-hint {
  opacity: 0.7;
}
</style>
