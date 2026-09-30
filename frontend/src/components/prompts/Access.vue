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
          class="flat-right flat-left form-compact"
          :options="addListTypeOptions"
          :aria-label="$t('access.allowDeny')"
        />
        <input v-if="addType !== 'all'" class="input flat-right flat-left form-grow form-compact" v-model="addName"
          :placeholder="$t('access.enterName')" list="group-suggestions" />
        <datalist id="group-suggestions">
          <option v-for="group in groups" :key="group" :value="group"></option>
        </datalist>
        <button type="button" class="button form-button flat-left form-compact" @click="submitAdd">
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
  </div>
</template>

<script>
import { notify } from "@/notify";
import { accessApi } from "@/api";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";
import FileList from "../files/FileList.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import PathPickerButton from "@/components/files/PathPickerButton.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import ActivityViewerButton from "@/components/settings/ActivityViewerButton.vue";
import SettingsTable from "@/components/settings/Table.vue";
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
      addName: "",
      groups: [],
      cascadeDelete: false
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
    entries() {
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
    columns() {
      return [
        { key: "allowDeny", label: this.$t("access.allowDeny"), sortable: true },
        { key: "userGroup", label: this.$t("access.userGroup"), sortable: true },
        { key: "name", label: this.$t("general.name"), sortable: true },
        { key: "edit", label: this.$t("general.edit"), narrow: true, align: "right" },
      ];
    },
    tableRows() {
      return this.entries.map((entry) => ({
        id: `${entry.type}-${entry.name}`,
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
    await this.fetchGroups();
  },
  watch: {
    sourceName(newSourceName) {
      this.currentSource = newSourceName;
      this.tempSource = newSourceName;
      this.fetchRule();
    },
    path(newPath) {
      this.currentPath = newPath;
      this.tempPath = newPath;
      this.isEditingPath = false;
      this.fetchRule();
    }
  },
  methods: {
    async onPathPickerNavigate() {
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
    async fetchGroups() {
      try {
        const response = await accessApi.getGroups();
        this.groups = response.groups;
      } catch (_e) {
        this.groups = [];
      }
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
    /**
     * @param {{allow: boolean, type: string, name: string}} entry
     */
    async deleteAccess(entry) {
      try {
        const body = {
          allow: entry.allow,
          ruleCategory: entry.type,
          value: entry.type === 'all' ? '' : entry.name,
          cascade: this.cascadeDelete && entry.type !== 'all'
        };
        await accessApi.del(this.currentSource, this.currentPath, body);
        const message = this.cascadeDelete && entry.type !== 'all'
          ? this.$t("access.deletedCascade")
          : this.$t("access.deleted");
        notify.showSuccessToast(message);
        await this.fetchRule();
        // Emit event to refresh access rules list
        eventBus.emit('accessRulesChanged');
      } catch (e) {
        notify.showError(e);
        console.error(e);
      }
    },
    async submitAdd() {
      if (!this.addName.trim() && this.addType !== "all") {
        notify.showError(this.$t("access.enterName"));
        return;
      }
      try {
        const body = {
          allow: this.addListType === 'allow' && this.addType !== 'all',
          ruleCategory: this.addType,
          value: this.addName.trim()
        };
        await accessApi.add(
          this.currentSource,
          this.currentPath,
          body
        );
        notify.showSuccessToast(this.$t("access.added"));
        this.addName = "";
        await this.fetchRule();
        // Emit event to refresh access rules list
        eventBus.emit('accessRulesChanged');
      } catch (e) {
        notify.showError(e);
        console.error(e);
      }
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
