<template>
  <div class="card-content">
    <div v-if="loading" class="loading-spinner-wrapper">
      <LoadingSpinner size="medium" />
    </div>
    <template v-else>
      <!-- Warning banner for missing path -->
      <div v-if="!pathExists && !isEditingPath" class="warning-banner">
        <i class="material-symbols-outlined">warning</i>
        <span>{{ $t("messages.pathNotFoundMessage") }}</span>
        <button type="button" class="button button--flat button--blue" @click="startPathReassignment">
          {{ $t("messages.reassignPath") }}
        </button>
      </div>

      <div v-if="isEditingPath">
        <file-list @update:selected="updateTempPath" :browseSource="sourceName" :showFiles="true"></file-list>
      </div>
      <div v-else>
      <PathPickerButton
        class="path-picker"
        v-model:path="currentPath"
        v-model:source="currentSource"
        aria-label="access-path"
        :show-files="true"
        :show-folders="true"
        :placeholder="$t('sidebar.chooseSource')"
        @navigate="onPathPickerNavigate"
      />
      <div class="settings-items">
        <ActivityViewerButton class="item" :href="activityViewerHref" />
      </div>
      <!-- Default behavior banner -->
      <div class="card item">
        <div class="card-content banner-content">
          <i class="material-symbols-outlined">{{ sourceDenyDefault ? 'do_not_disturb_on' : 'check_circle' }}</i>  <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          {{ $t("access.defaultBehavior", { suffix: ":" }) }} {{ sourceDenyDefault ? $t("access.deny") : $t("access.allow")
          }}
          <HelpTooltipIcon :text="$t('access.defaultBehaviorDescription')" />
        </div>

      </div>
      <!-- Add Form -->
      <div class="form-flex-group">
        <ExpandDropdown
          v-model="addType"
          class="flat-right form-compact"
          :options="addTypeOptions"
          :aria-label="$t('access.userGroup')"
        />
        <ExpandDropdown
          v-if="addType !== 'all'"
          v-model="addListType"
          class="flat-right flat-left form-compact form-grow"
          :options="addListTypeOptions"
          :aria-label="$t('access.allowDeny')"
        />
        <EntityPickerButton
          v-if="addType !== 'all'"
          :kind="addType"
          multiple
          compact
          :icon="addType === 'group' ? 'group_add' : 'person_add'"
          class="flat-left form-compact"
          :exclude="existingNames"
          @select="addEntries"
        />
        <button v-else type="button" class="button form-button flat-left form-compact" @click="addEntries([])">
          <i class="material-symbols-outlined">add</i>
        </button>
      </div>
      <!-- Cascade Delete Toggle -->
      <div v-if="entries.length > 0" class="cascade-toggle-section">
        <ToggleSwitch v-model="cascadeDelete"
          :name="$t('access.cascadeDelete')"
          :description="$t('access.cascadeDeleteDescription')" />
      </div>
      <settings-table
        v-if="entries.length > 0"
        :columns="columns"
        :items="tableRows"
        :aria-label="$t('access.access')"
      >
        <template #cell-edit="{ row }">
          <button type="button" @click="deleteAccess(row.entry)" class="action" :aria-label="$t('general.delete')"
            :title="$t('general.delete')">
            <i class="material-symbols">delete</i>
          </button>
        </template>
      </settings-table>
      </div>
    </template>
  </div>
  <div class="card-actions">
    <template v-if="isEditingPath">
      <button type="button" class="button button--flat button--grey" @click="cancelPathChange" :aria-label="$t('general.cancel')" :title="$t('general.cancel')">
        {{ $t("general.cancel") }}
      </button>
      <button type="button" class="button button--flat" @click="confirmPathChange" :aria-label="$t('general.ok')" :title="$t('general.ok')">
        {{ $t("general.ok") }}
      </button>
    </template>
    <template v-else>
      <button type="button" class="button button--flat button--grey" @click="cancelChanges" :aria-label="$t('general.cancel')" :title="$t('general.cancel')">
        {{ $t("general.cancel") }}
      </button>
      <button type="button" class="button button--flat" :disabled="!dirty || saving" @click="saveChanges" :aria-label="$t('general.save')" :title="$t('general.save')">
        {{ $t("general.save") }}
      </button>
    </template>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { mutations } from "@/store";
import { accessApi } from "@/api";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";
import FileList from "../files/FileList.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import PathPickerButton from "@/components/files/PathPickerButton.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import ActivityViewerButton from "@/components/settings/ActivityViewerButton.vue";
import SettingsTable from "@/components/settings/Table.vue";
import EntityPickerButton from "@/components/settings/EntityPickerButton.vue";
import { activityViewerPresets } from "@/utils/activityViewerLink";
import { eventBus } from "@/store/eventBus";

export default {
  name: "access",
  components: {
    HelpTooltipIcon,
    FileList,
    ToggleSwitch,
    LoadingSpinner,
    PathPickerButton,
    ExpandDropdown,
    ActivityViewerButton,
    SettingsTable,
    EntityPickerButton,
  },
  props: {
    promptId: { type: [String, Number], default: null },
    sourceName: { type: String, required: true },
    path: { type: String, required: true, default: "/" }
  },
  data() {
    return {
      loading: false,
      isEditingPath: false,
      isReassigningPath: false,
      tempPath: this.path,
      currentPath: this.path,
      currentSource: this.sourceName,
      tempSource: this.sourceName,
      originalPath: this.path,
      rule: { denyAll: false, deny: { users: [], groups: [] }, allow: { users: [], groups: [] } },
      sourceDenyDefault: false,
      pathExists: true,
      addType: "user",
      addListType: "deny",
      cascadeDelete: false,
      /** Staged rule edits; nothing is applied until Save. */
      pendingAdds: [],
      pendingDeletes: [],
      saving: false
    };
  },
  computed: {
    addTypeOptions() {
      return [
        { value: "user", label: this.$t("general.user") },
        { value: "group", label: this.$t("general.group") },
        { value: "all", label: this.$t("access.all") },
      ];
    },
    addListTypeOptions() {
      return [
        { value: "deny", label: this.$t("access.deny") },
        { value: "allow", label: this.$t("access.allow") },
      ];
    },
    /** Entries persisted on the server (before staged edits). */
    baseEntries() {
      /** @type {{allow: boolean, type: "user" | "group" | "all", name: string}[]} */
      const entries = [];
      if (this.rule.denyAll) {
        entries.push({ allow: false, type: "all", name: this.$t("access.all") });
      }
      (this.rule.deny?.users || []).forEach(name => {
        entries.push({ allow: false, type: "user", name });
      });
      (this.rule.deny?.groups || []).forEach(name => {
        entries.push({ allow: false, type: "group", name });
      });
      (this.rule.allow?.users || []).forEach(name => {
        entries.push({ allow: true, type: "user", name });
      });
      (this.rule.allow?.groups || []).forEach(name => {
        entries.push({ allow: true, type: "group", name });
      });
      return entries;
    },
    /** What the rule will look like once Save applies the staged edits. */
    entries() {
      const deleted = new Set(this.pendingDeletes.map(this.opKey));
      const base = this.baseEntries.filter(e => !deleted.has(this.opKey({
        allow: e.allow,
        ruleCategory: e.type,
        value: e.type === "all" ? "" : e.name,
      })));
      const staged = this.pendingAdds.map(op => ({
        allow: op.allow,
        type: op.ruleCategory,
        name: op.ruleCategory === "all" ? this.$t("access.all") : op.value,
      }));
      return [...base, ...staged];
    },
    dirty() {
      return this.pendingAdds.length > 0 || this.pendingDeletes.length > 0;
    },
    columns() {
      return [
        { key: "allowDeny", label: this.$t("access.allowDeny"), sortable: true },
        { key: "userGroup", label: this.$t("access.userGroup"), sortable: true },
        { key: "name", label: this.$t("general.name"), sortable: true },
        { key: "edit", label: this.$t("general.edit"), narrow: true, align: "right" },
      ];
    },
    /** Names already present on the selected allow/deny list for the category (excluded from the picker). */
    existingNames() {
      return this.entries
        .filter((e) => e.type === this.addType && e.allow === (this.addListType === "allow"))
        .map((e) => e.name);
    },
    tableRows() {
      return this.entries.map((entry) => ({
        id: `${entry.type}-${entry.name}-${entry.allow ? "allow" : "deny"}`,
        allowDeny: entry.allow ? this.$t("access.allow") : this.$t("access.deny"),
        userGroup: entry.type === "user"
          ? this.$t("general.user")
          : (entry.type === "group" ? this.$t("general.group") : this.$t("access.all")),
        name: entry.name,
        entry,
      }));
    },
    activityViewerHref() {
      return activityViewerPresets.access(this.currentSource, this.currentPath);
    },
  },
  async mounted() {
    await this.fetchRule();
  },
  watch: {
    sourceName(newSourceName) {
      this.currentSource = newSourceName;
      this.tempSource = newSourceName;
      this.resetPending();
      this.fetchRule();
    },
    path(newPath) {
      this.currentPath = newPath;
      this.tempPath = newPath;
      this.isEditingPath = false;
      this.resetPending();
      this.fetchRule();
    }
  },
  methods: {
    async onPathPickerNavigate() {
      this.resetPending();
      await this.fetchRule();
      eventBus.emit("accessRulesChanged");
    },
    /**
     * @param {{path: string, source: string}} pathOrData
     */
    updateTempPath(pathOrData) {
      if (pathOrData?.path) {
        this.tempPath = pathOrData.path;
        this.tempSource = pathOrData.source;
      }
    },
    async confirmPathChange() {
      if (this.isReassigningPath) {
        // Reassigning path - call API to update
        try {
          await accessApi.updatePath(this.currentSource, this.originalPath, this.tempPath);
          notify.showSuccessToast(this.$t("messages.pathReassigned"));
          this.originalPath = this.tempPath;
          this.currentPath = this.tempPath;
          this.currentSource = this.tempSource;
          this.pathExists = true;
          this.isEditingPath = false;
          this.isReassigningPath = false;
          await this.fetchRule();
          // Emit event to refresh access rules list
          eventBus.emit('accessRulesChanged');
        } catch (e) {
          notify.showError(this.$t("messages.pathReassignFailed"));
          console.error(e);
        }
      } else {
        // Just viewing a different path
        this.currentPath = this.tempPath;
        this.currentSource = this.tempSource;
        this.isEditingPath = false;
        this.resetPending();
        await this.fetchRule();
      }
    },
    cancelPathChange() {
      this.isEditingPath = false;
      this.isReassigningPath = false;
    },
    startPathReassignment() {
      this.isReassigningPath = true;
      this.tempPath = this.currentPath;
      this.isEditingPath = true;
    },
    async fetchRule() {
      this.loading = true;
      try {
        const response = await accessApi.get(this.currentSource, this.currentPath);
        // Handle new API response structure - now sourceDenyDefault is part of the rule
        this.rule = response;
        this.sourceDenyDefault = response.sourceDenyDefault || false;
        this.pathExists = response.pathExists !== false;
      } catch (_e) {
        this.rule = { denyAll: false, deny: { users: [], groups: [] }, allow: { users: [], groups: [] } };
        this.sourceDenyDefault = false;
        this.pathExists = true;
      } finally {
        this.loading = false;
      }
    },
    resetPending() {
      this.pendingAdds = [];
      this.pendingDeletes = [];
    },
    /** Stable key for matching entries to staged ops. */
    opKey(op) {
      return `${op.allow ? "allow" : "deny"}|${op.ruleCategory}|${op.value}`;
    },
    /**
     * Stages a delete. Pending adds are simply un-staged; persisted
     * entries get a delete op applied on Save.
     * @param {{allow: boolean, type: string, name: string}} entry
     */
    deleteAccess(entry) {
      const op = {
        allow: entry.allow,
        ruleCategory: entry.type,
        value: entry.type === 'all' ? '' : entry.name,
        cascade: this.cascadeDelete && entry.type !== 'all'
      };
      const pendingIdx = this.pendingAdds.findIndex(a => this.opKey(a) === this.opKey(op));
      if (pendingIdx !== -1) {
        this.pendingAdds.splice(pendingIdx, 1);
        return;
      }
      this.pendingDeletes.push(op);
    },
    /**
     * Stages rule entries. `names` are picked users/groups; for the 'all'
     * category an empty list stages the single deny-all entry.
     * @param {string[]} names
     */
    addEntries(names) {
      const values = this.addType === 'all' ? [''] : names;
      for (const value of values) {
        const op = {
          allow: this.addListType === 'allow' && this.addType !== 'all',
          ruleCategory: this.addType,
          value
        };
        // Re-adding a staged-for-delete entry just cancels the delete.
        const delIdx = this.pendingDeletes.findIndex(d => this.opKey(d) === this.opKey(op));
        if (delIdx !== -1) {
          this.pendingDeletes.splice(delIdx, 1);
          continue;
        }
        if (this.pendingAdds.some(a => this.opKey(a) === this.opKey(op))) {
          continue;
        }
        // Same name on the opposite list (allow vs deny) is allowed by the backend.
        this.pendingAdds.push(op);
      }
    },
    /** Applies staged deletes then adds; keeps unapplied ops on failure. */
    async saveChanges() {
      if (this.saving) return;
      this.saving = true;
      try {
        for (const op of [...this.pendingDeletes]) {
          await accessApi.del(this.currentSource, this.currentPath, {
            allow: op.allow,
            ruleCategory: op.ruleCategory,
            value: op.value,
            cascade: op.cascade
          });
          this.pendingDeletes.splice(this.pendingDeletes.indexOf(op), 1);
        }
        for (const op of [...this.pendingAdds]) {
          await accessApi.add(this.currentSource, this.currentPath, {
            allow: op.allow,
            ruleCategory: op.ruleCategory,
            value: op.value
          });
          this.pendingAdds.splice(this.pendingAdds.indexOf(op), 1);
        }
      } catch (e) {
        notify.showError(e);
        console.error(e);
      }
      await this.fetchRule();
      // Emit event to refresh access rules list
      eventBus.emit('accessRulesChanged');
      this.saving = false;
      if (!this.dirty) {
        mutations.closeTopPrompt(this.promptId ?? undefined);
      }
    },
    cancelChanges() {
      this.pendingAdds = [];
      this.pendingDeletes = [];
      mutations.closeTopPrompt(this.promptId ?? undefined);
    },
  }
};
</script>

<style scoped>
.form-flex-group {
  margin-top: 1em;
}

.banner-content {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 0.25em !important;
  gap: 0.5em;
  margin-top: 0.70rem;
}

.path-picker,
.banner-content {
  margin-bottom: 0.70rem;
}

.cascade-toggle-section {
  margin-top: 1em;
  margin-bottom: 1em;
}

.loading-spinner-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
}

</style>
