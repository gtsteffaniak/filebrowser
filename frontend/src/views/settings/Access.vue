<template>
  <button class="button floating-action-button" @click="addAccess">{{ $t("general.new") }}</button>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card-title">
    <h2>{{ $t("access.accessManagement") }}</h2>
    <div class="form-flex-group">
      <label for="source-select">{{ $t("general.source",{suffix: ":"})  }}</label>
      <select class="input" id="source-select" v-model="selectedSource" @change="fetchRules">
        <option v-for="source in availableSources" :key="source" :value="source">
          {{ source }}
        </option>
      </select>
    </div>
  </div>
  <div class="card-content full">
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
        <button class="action" @click="editAccess(row.path)" :aria-label="$t('general.edit')"
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
import { state, mutations } from "@/store";
import Errors from "@/views/Errors.vue";
import SettingsTable from "@/components/settings/Table.vue";
import { eventBus } from "@/store/eventBus";
export default {
  name: "accessSettings",
  components: {
    Errors,
    SettingsTable,
  },
  data: function () {
    return {
      rules: {},
      accessPath: "",
      error: null,
      selectedSource: "",
      /** True until first `fetchRules` completes so the table does not flash the empty state. */
      loading: true,
    };
  },
  async mounted() {
    this.selectedSource = state.sources.current;
    await this.fetchRules();
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
  },
  methods: {
    async fetchRules() {
      this.loading = true;
      this.error = null;
      this.accessPath = state.req.path || '/';
      try {
        this.rules = await accessApi.getAll(this.selectedSource);
      } catch (e) {
        this.error = e;
      } finally {
        this.loading = false;
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
</style>