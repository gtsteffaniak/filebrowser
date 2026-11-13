<template>
  <div class="card-content" :class="{ 'embedded-content': embedded }">
    <div v-if="loading" class="loading-spinner-wrapper">
      <LoadingSpinner size="medium" />
    </div>
    <template v-else>
      <div v-if="!pathExists && baseEntries.length > 0 && !isEditingPath" class="warning-banner">
        <i class="material-symbols-outlined">warning</i>
        <span>{{ $t("messages.pathNotFoundMessage") }}</span>
        <button type="button" class="button button--flat button--blue" @click="startPathReassignment">
          {{ $t("messages.reassignPath") }}
        </button>
      </div>
      <div v-if="isEditingPath" :class="{ 'embedded-list': embedded }">
        <FileList
          @update:selected="updateTempPath"
          :browseSource="sourceName"
          :browsePath="embedded ? currentPath : null"
          :hideDestinationSource="true"
          :showFiles="true"
        ></FileList>
      </div>
      <div v-else>
      <PathPickerButton
        v-if="!embedded"
        class="path-picker"
        v-model:path="currentPath"
        v-model:source="currentSource"
        aria-label="access-path"
        :show-files="true"
        :show-folders="true"
        :placeholder="$t('sidebar.chooseSource')"
        @navigate="onPathPickerNavigate"
      />
      <div v-if="!embedded" class="settings-items">
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
      <div class="form-flex-group">
        <ExpandDropdown
          v-model="addType"
          class="flat-right form-compact access-type"
          :options="addTypeOptions"
          :aria-label="$t('access.userGroup')"
        />
        <ExpandDropdown
          v-if="addType !== 'all'"
          v-model="addListType"
          class="flat-right flat-left form-compact form-grow access-list"
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
      <div v-if="entries.length > 0" class="cascade-toggle-section settings-items">
        <ToggleSwitch class="item" v-model="cascadeDelete"
          :name="$t('access.cascadeDelete')"
          :description="$t('access.cascadeDeleteDescription')" />
      </div>
      <SettingsTable
        v-if="entries.length > 0"
        :columns="columns"
        :items="tableRows"
        :aria-label="$t('general.access')"
      >
        <template #cell-edit="{ row }">
          <button type="button" @click="deleteAccess(row.entry)" class="action" :aria-label="$t('general.delete')"
            :title="$t('general.delete')">
            <i class="material-symbols">delete</i>
          </button>
        </template>
      </SettingsTable>
      </div>
    </template>
  </div>
  <div class="card-actions" :class="{ 'embedded-actions': embedded }">
    <template v-if="isEditingPath">
      <button type="button" class="button button--flat button--grey" @click="cancelPathChange" :aria-label="$t('general.cancel')" :title="$t('general.cancel')">
        {{ $t("general.cancel") }}
      </button>
      <button type="button" class="button button--flat" @click="confirmPathChange" :aria-label="$t('general.ok')" :title="$t('general.ok')">
        {{ $t("general.ok") }}
      </button>
    </template>
    <template v-else>
      <button v-if="!embedded" type="button" class="button button--flat button--grey" @click="cancelChanges" :aria-label="$t('general.cancel')" :title="$t('general.cancel')">
        {{ $t("general.cancel") }}
      </button>
      <button type="button" class="button button--flat" :disabled="!dirty || saving" @click="saveChanges" :aria-label="$t('general.save')" :title="$t('general.save')">
        {{ $t("general.save") }}
      </button>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { notify } from "@/notify";
import { mutations } from "@/store";
import { accessApi } from "@/api";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";
import FileList from "../files/FileList.vue";
import type { SelectionPayload } from "../files/FileList.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import PathPickerButton from "@/components/files/PathPickerButton.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import ActivityViewerButton from "@/components/settings/ActivityViewerButton.vue";
import SettingsTable from "@/components/settings/Table.vue";
import EntityPickerButton from "@/components/settings/EntityPickerButton.vue";
import { activityViewerPresets } from "@/utils/activityViewerLink";
import { eventBus } from "@/store/eventBus";
import type { AccessRule, AccessRuleCategory } from "@/store/types.ts";

type AccessListType = "allow" | "deny";

interface AccessEntry {
  allow: boolean;
  type: AccessRuleCategory;
  name: string;
}

interface AccessOp {
  allow: boolean;
  ruleCategory: AccessRuleCategory;
  value: string;
  cascade?: boolean;
}

interface TableColumn {
  key: string;
  label: string;
  sortable?: boolean;
  narrow?: boolean;
  align?: "left" | "center" | "right";
}

const props = withDefaults(
  defineProps<{
    promptId?: number | null;
    sourceName: string;
    path?: string;
    embedded?: boolean;
  }>(),
  {
    promptId: null,
    path: "/",
    embedded: false,
  }
);

const { t } = useI18n();

function emptyRule(): AccessRule {
  return {
    denyAll: false,
    deny: { users: [], groups: [] },
    allow: { users: [], groups: [] },
    sourceDenyDefault: false,
    pathExists: true,
  };
}

const loading = ref(false);
const isEditingPath = ref(false);
const isReassigningPath = ref(false);
const tempPath = ref(props.path);
const currentPath = ref(props.path);
const currentSource = ref(props.sourceName);
const tempSource = ref(props.sourceName);
const originalPath = ref(props.path);
const rule = ref<AccessRule>(emptyRule());
const addType = ref<AccessRuleCategory>("user");
const addListType = ref<AccessListType>("deny");
const cascadeDelete = ref(false);
const pendingAdds = ref<AccessOp[]>([]);
const pendingDeletes = ref<AccessOp[]>([]);
const saving = ref(false);

const addTypeOptions = computed<{ value: AccessRuleCategory; label: string }[]>(() => [
  { value: "user", label: t("general.user") },
  { value: "group", label: t("general.group") },
  { value: "all", label: t("access.all") },
]);
const addListTypeOptions = computed<{ value: AccessListType; label: string }[]>(() => [
  { value: "deny", label: t("access.deny") },
  { value: "allow", label: t("access.allow") },
]);
/** Entries persisted on the server (before staged edits). */
const baseEntries = computed(() => {
  const entries: AccessEntry[] = [];
  if (rule.value.denyAll) {
    entries.push({ allow: false, type: "all", name: t("access.all") });
  }
  for (const name of rule.value.deny.users) {
    entries.push({ allow: false, type: "user", name });
  }
  for (const name of rule.value.deny.groups) {
    entries.push({ allow: false, type: "group", name });
  }
  for (const name of rule.value.allow.users) {
    entries.push({ allow: true, type: "user", name });
  }
  for (const name of rule.value.allow.groups) {
    entries.push({ allow: true, type: "group", name });
  }
  return entries;
});
/** What the rule will look like once Save applies the staged edits. */
const entries = computed(() => {
  const deleted = new Set(pendingDeletes.value.map(opKey));
  const base = baseEntries.value.filter(e => !deleted.has(opKey({
    allow: e.allow,
    ruleCategory: e.type,
    value: e.type === "all" ? "" : e.name,
  })));
  const staged = pendingAdds.value.map((op): AccessEntry => ({
    allow: op.allow,
    type: op.ruleCategory,
    name: op.ruleCategory === "all" ? t("access.all") : op.value,
  }));
  return [...base, ...staged];
});
const sourceDenyDefault = computed(() => rule.value.sourceDenyDefault);
const pathExists = computed(() => rule.value.pathExists);
const dirty = computed(() => pendingAdds.value.length > 0 || pendingDeletes.value.length > 0);
const columns = computed<TableColumn[]>(() => [
  { key: "allowDeny", label: t("access.allowDeny"), sortable: true },
  { key: "userGroup", label: t("access.userGroup"), sortable: true },
  { key: "name", label: t("general.name"), sortable: true },
  { key: "edit", label: t("general.edit"), narrow: true, align: "right" },
]);
/** Names already present on the selected allow/deny list for the category (excluded from the picker). */
const existingNames = computed(() =>
  entries.value
    .filter((e) => e.type === addType.value && e.allow === (addListType.value === "allow"))
    .map((e) => e.name)
);
const tableRows = computed(() =>
  entries.value.map((entry) => ({
    id: `${entry.type}-${entry.name}-${entry.allow ? "allow" : "deny"}`,
    allowDeny: entry.allow ? t("access.allow") : t("access.deny"),
    userGroup: entry.type === "user"
      ? t("general.user")
      : (entry.type === "group" ? t("general.group") : t("access.all")),
    name: entry.name,
    entry,
  }))
);
const activityViewerHref = computed(() => activityViewerPresets.access(currentSource.value, currentPath.value));

onMounted(fetchRule);

watch(() => props.sourceName, (newSourceName) => {
  currentSource.value = newSourceName;
  tempSource.value = newSourceName;
  resetPending();
  fetchRule();
});

watch(() => props.path, (newPath) => {
  currentPath.value = newPath;
  tempPath.value = newPath;
  isEditingPath.value = false;
  resetPending();
  fetchRule();
});

async function onPathPickerNavigate() {
  resetPending();
  await fetchRule();
  eventBus.emit("accessRulesChanged");
}

function updateTempPath(payload: SelectionPayload) {
  if (payload.path) {
    tempPath.value = payload.path;
    tempSource.value = payload.source ?? tempSource.value;
  }
}

async function confirmPathChange() {
  if (isReassigningPath.value) {
    // Reassigning path - call API to update
    try {
      await accessApi.updatePath(currentSource.value, originalPath.value, tempPath.value);
      notify.showSuccessToast(t("messages.pathReassigned"));
      originalPath.value = tempPath.value;
      currentPath.value = tempPath.value;
      currentSource.value = tempSource.value;
      isEditingPath.value = false;
      isReassigningPath.value = false;
      await fetchRule();
      // Emit event to refresh access rules list
      eventBus.emit("accessRulesChanged");
    } catch (e) {
      notify.showError(t("messages.pathReassignFailed"));
      console.error(e);
    }
  } else {
    // Just viewing a different path
    currentPath.value = tempPath.value;
    currentSource.value = tempSource.value;
    isEditingPath.value = false;
    resetPending();
    await fetchRule();
  }
}

function cancelPathChange() {
  isEditingPath.value = false;
  isReassigningPath.value = false;
}

function startPathReassignment() {
  isReassigningPath.value = true;
  tempPath.value = currentPath.value;
  isEditingPath.value = true;
}

async function fetchRule() {
  loading.value = true;
  try {
    const response: AccessRule = await accessApi.get(currentSource.value, currentPath.value);
    // Handle new API response structure - now sourceDenyDefault is part of the rule
    rule.value = response;
  } catch {
    rule.value = emptyRule();
  } finally {
    loading.value = false;
  }
}

function resetPending() {
  pendingAdds.value = [];
  pendingDeletes.value = [];
}

/** Stable key for matching entries to staged ops. */
function opKey(op: Pick<AccessOp, "allow" | "ruleCategory" | "value">) {
  return `${op.allow ? "allow" : "deny"}|${op.ruleCategory}|${op.value}`;
}

/**
 * Stages a delete. Pending adds are simply un-staged; persisted
 * entries get a delete op applied on Save.
 */
function deleteAccess(entry: AccessEntry) {
  const op: AccessOp = {
    allow: entry.allow,
    ruleCategory: entry.type,
    value: entry.type === "all" ? "" : entry.name,
    cascade: cascadeDelete.value && entry.type !== "all"
  };
  const pendingIdx = pendingAdds.value.findIndex(a => opKey(a) === opKey(op));
  if (pendingIdx !== -1) {
    pendingAdds.value.splice(pendingIdx, 1);
    return;
  }
  pendingDeletes.value.push(op);
}

/**
 * Stages rule entries. `names` are picked users/groups; for the 'all'
 * category an empty list stages the single deny-all entry.
 */
function addEntries(names: string[]) {
  const values = addType.value === "all" ? [""] : names;
  for (const value of values) {
    const op: AccessOp = {
      allow: addListType.value === "allow" && addType.value !== "all",
      ruleCategory: addType.value,
      value
    };
    // Re-adding a staged-for-delete entry just cancels the delete.
    const delIdx = pendingDeletes.value.findIndex(d => opKey(d) === opKey(op));
    if (delIdx !== -1) {
      pendingDeletes.value.splice(delIdx, 1);
      continue;
    }
    if (pendingAdds.value.some(a => opKey(a) === opKey(op))) {
      continue;
    }
    // Same name on the opposite list (allow vs deny) is allowed by the backend.
    pendingAdds.value.push(op);
  }
}

/** Applies staged deletes then adds; keeps unapplied ops on failure. */
async function saveChanges() {
  if (saving.value) return;
  saving.value = true;
  let failed = false;
  try {
    for (const op of [...pendingDeletes.value]) {
      await accessApi.del(currentSource.value, currentPath.value, {
        allow: op.allow,
        ruleCategory: op.ruleCategory,
        value: op.value,
        cascade: op.cascade
      });
      pendingDeletes.value.splice(pendingDeletes.value.indexOf(op), 1);
    }
    for (const op of [...pendingAdds.value]) {
      await accessApi.add(currentSource.value, currentPath.value, {
        allow: op.allow,
        ruleCategory: op.ruleCategory,
        value: op.value
      });
      pendingAdds.value.splice(pendingAdds.value.indexOf(op), 1);
    }
  } catch (e) {
    failed = true;
    notify.showError(e);
    console.error(e);
  }
  await fetchRule();
  // Emit event to refresh access rules list
  eventBus.emit("accessRulesChanged");
  saving.value = false;
  if (!failed && props.embedded) {
    notify.showSuccessToast(t("settings.settingsUpdated"));
  }
  if (!dirty.value && !props.embedded) {
    mutations.closeTopPrompt(props.promptId ?? undefined);
  }
}

function cancelChanges() {
  resetPending();
  mutations.closeTopPrompt(props.promptId ?? undefined);
}
</script>

<style scoped>
.form-flex-group {
  margin-top: 1em;
}

.form-flex-group > .expand-dropdown.form-compact.access-type {
  flex: 0 0 8em;
  width: 8em;
  min-width: 8em;
}

.form-flex-group > .expand-dropdown.form-compact.access-list {
  flex: 1 1 0;
  width: auto;
  min-width: 0;
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

.embedded-content {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
}

.embedded-list {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
}

.embedded-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.25em;
}

.loading-spinner-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
}

</style>
