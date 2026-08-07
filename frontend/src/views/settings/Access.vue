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
  </div>
  <div class="card-content full">
    <div class="settings-items">
      <ActivityViewerButton class="item" :href="activityViewerHref" />
    </div>
    <SettingsItem
      :title="$t('general.permissions')"
      :collapsable="true"
      :start-collapsed="true"
    >
      <p class="small">{{ $t('settings.sourceAccessDefaultsHelp') }}</p>
      <SourceFilePermissions
        v-if="!defaultsLoading"
        enforceable
        :permissions="sourceAccessDefaults"
        :enforced-permissions="sourceAccessEnforced"
        :config-locked-paths="lockedFromConfigPaths"
        @changed="onSourceAccessDefaultsChange"
        @enforced-change="onSourceAccessEnforcedChange"
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
import { getSourceSettings, patchSourceSettings } from "@/api/settings";
import { state, mutations } from "@/store";
import Errors from "@/views/Errors.vue";
import SettingsTable from "@/components/settings/Table.vue";
import SettingsItem from "@/components/settings/SettingsItem.vue";
import SourceFilePermissions from "@/components/settings/SourceFilePermissions.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import ActivityViewerButton from "@/components/settings/ActivityViewerButton.vue";
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
    ActivityViewerButton,
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
    hydratingDefaults: false,
    sourceAccessDefaults: {
      view: true,
      download: true,
      modify: false,
      create: false,
      delete: false,
    },
    sourceAccessEnforced: {
      view: false,
      download: false,
      modify: false,
      create: false,
      delete: false,
    },
    lockedFromConfigPaths: [],
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
      this.hydratingDefaults = true;
      try {
        const settings = await getSourceSettings();
        this.applySourceSettingsResponse(settings);
      } catch (e) {
        console.error(e);
        if (e?.message) {
          notify.showError(e.message);
        }
      } finally {
        this.defaultsLoading = false;
        this.$nextTick(() => {
          this.hydratingDefaults = false;
        });
      }
    },
    canSaveSourceDefaults() {
      return (
        !this.defaultsLoading &&
        !this.savingDefaults &&
        !this.hydratingDefaults
      );
    },
    applySourceSettingsResponse(settings) {
      const perms = settings?.defaultPermissions ?? {};
      const enforced = settings?.enforcedPermissions ?? {};
      this.lockedFromConfigPaths = Array.isArray(settings?.lockedFromConfigPaths)
        ? settings.lockedFromConfigPaths
        : [];
      this.sourceAccessDefaults = {
        view: perms.view !== false,
        download: perms.download !== false,
        modify: !!perms.modify,
        create: !!perms.create,
        delete: !!perms.delete,
      };
      this.sourceAccessEnforced = {
        view: !!enforced.view,
        download: !!enforced.download,
        modify: !!enforced.modify,
        create: !!enforced.create,
        delete: !!enforced.delete,
      };
    },
    async onSourceAccessEnforcedChange(flag, value) {
      if (!this.canSaveSourceDefaults()) {
        return;
      }
      this.savingDefaults = true;
      try {
        const settings = await patchSourceSettings({
          enforcedPermissions: { [flag]: value },
        });
        this.hydratingDefaults = true;
        this.applySourceSettingsResponse(settings);
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
      } catch (e) {
        console.error(e);
        if (e?.message) {
          notify.showError(e.message);
        }
        await this.loadSourceAccessDefaults();
      } finally {
        this.savingDefaults = false;
        this.$nextTick(() => {
          this.hydratingDefaults = false;
        });
      }
    },
    isConfigLockedPermission(flag) {
      return this.lockedFromConfigPaths.includes(`defaultPermissions.${flag}`);
    },
    sourceDefaultPermissionsPatch(flag) {
      const perms = this.sourceAccessDefaults;
      switch (flag) {
        case "view":
          return { defaultPermissions: { view: perms.view } };
        case "download":
          return { defaultPermissions: { download: perms.download } };
        case "modify":
          return { defaultPermissions: { modify: perms.modify } };
        case "create":
          return { defaultPermissions: { create: perms.create } };
        case "delete":
          return { defaultPermissions: { delete: perms.delete } };
        default:
          return null;
      }
    },
    async onSourceAccessDefaultsChange(flag) {
      if (!this.canSaveSourceDefaults() || !flag) {
        return;
      }
      if (this.isConfigLockedPermission(flag)) {
        return;
      }
      const patch = this.sourceDefaultPermissionsPatch(flag);
      if (!patch) {
        return;
      }
      this.savingDefaults = true;
      try {
        const settings = await patchSourceSettings(patch);
        this.hydratingDefaults = true;
        this.applySourceSettingsResponse(settings);
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
      } catch (e) {
        console.error(e);
        if (e?.message) {
          notify.showError(e.message);
        }
        await this.loadSourceAccessDefaults();
      } finally {
        this.savingDefaults = false;
        this.$nextTick(() => {
          this.hydratingDefaults = false;
        });
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
