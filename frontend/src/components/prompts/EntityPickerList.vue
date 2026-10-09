<template>
  <div>
    <input
      v-if="!creatingGroup"
      ref="filterInput"
      v-model="filter"
      type="text"
      class="input entity-picker-filter"
      :placeholder="$t('general.search')"
      :aria-label="$t('general.search')"
      @keydown.enter.prevent="onEnter"
    />
    <div v-if="creatingGroup" class="entity-picker-create-row">
      <input
        ref="createInput"
        v-model="newGroupName"
        type="text"
        class="input entity-picker-create-input"
        :class="{ 'form-invalid': createError }"
        :placeholder="$t('access.groupName')"
        :aria-label="$t('access.groupName')"
        @keydown.enter.prevent="confirmCreateGroup"
        @keydown.esc.prevent="cancelCreateGroup"
      />
      <button
        type="button"
        class="button entity-picker-create-confirm"
        :aria-label="$t('general.save')"
        :title="$t('general.save')"
        @click="confirmCreateGroup"
      >
        <i class="material-symbols">check</i>
      </button>
      <button
        type="button"
        class="button entity-picker-create-cancel"
        :aria-label="$t('general.cancel')"
        :title="$t('general.cancel')"
        @click="cancelCreateGroup"
      >
        <i class="material-symbols">close</i>
      </button>
    </div>
    <p v-if="createError" class="entity-picker-error">{{ createError }}</p>
    <template v-if="!creatingGroup">
      <div v-if="loading" class="loading-spinner-wrapper">
        <LoadingSpinner size="medium" />
      </div>
      <div v-else-if="visibleNames.length === 0" class="entity-picker-empty">
        {{ filter ? $t("search.noResults") : $t("files.lonely") }}
      </div>
      <ul v-else class="entity-picker-list" role="listbox" :aria-multiselectable="multiple">
      <li
        v-for="name in visibleNames"
        :key="name"
        role="option"
        tabindex="0"
        class="entity-picker-row border-radius"
        :aria-selected="isPicked(name)"
        :class="{ 'entity-picker-row--picked': isPicked(name) }"
        @click="toggle(name)"
        @keydown.enter.prevent="toggle(name)"
        @keydown.space.prevent="toggle(name)"
      >
        <i v-if="isPicked(name)" class="material-symbols entity-picker-check">check_box</i>
        <i v-else class="material-symbols entity-picker-check">check_box_outline_blank</i>
        <span class="entity-picker-name">{{ name }}</span>
        <span v-if="metaCount(name) !== null" class="entity-picker-meta">
          {{ kind === "group" ? $t("access.membersCount", { count: metaCount(name) }) : $t("access.userGroupsCount", { count: metaCount(name) }) }}
        </span>
      </li>
      </ul>
      <p v-if="truncated" class="entity-picker-hint">
        {{ $t("access.refineSearch", { count: maxVisible }) }}
      </p>
    </template>
  </div>
</template>

<script>
import { accessApi, usersApi } from "@/api";
import { eventBus } from "@/store/eventBus";
import { notify } from "@/notify";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { groupNameErrorText } from "@/utils/groups.js";

/** Cap rendered rows so huge user/group directories stay responsive. */
const MAX_VISIBLE = 200;

export default {
  name: "entity-picker-list",
  components: { LoadingSpinner },
  expose: ["startCreate"],
  props: {
    /** What to list: "user" or "group". */
    kind: {
      type: String,
      default: "user",
      validator: (v) => v === "user" || v === "group",
    },
    /** Selected names (v-model). */
    modelValue: {
      type: Array,
      default: () => [],
    },
    /** Names hidden from the list entirely (e.g. already applied elsewhere). */
    exclude: {
      type: Array,
      default: () => [],
    },
    /** Names always listed even when not returned by the API (e.g. external-auth members). */
    extraOptions: {
      type: Array,
      default: () => [],
    },
    multiple: {
      type: Boolean,
      default: false,
    },
    /** Show the inline new-group row for kind="group". */
    allowCreate: {
      type: Boolean,
      default: true,
    },
  },
  emits: ["update:modelValue", "enter"],
  data() {
    return {
      loading: true,
      names: [],
      members: new Map(),
      /** username -> number of groups they belong to (kind="user"). */
      userGroupCounts: new Map(),
      filter: "",
      maxVisible: MAX_VISIBLE,
      creatingGroup: false,
      newGroupName: "",
      createError: null,
      creating: false,
    };
  },
  computed: {
    /** Fetched names plus any extras, de-duplicated. */
    allNames() {
      return [...new Set([...this.names, ...this.extraOptions])];
    },
    filtered() {
      const query = this.filter.trim().toLowerCase();
      const available = this.allNames.filter((n) => !this.exclude.includes(n));
      if (!query) {
        return available;
      }
      // Case-insensitive matching; prefix matches ranked before substring matches.
      const starts = [];
      const contains = [];
      for (const name of available) {
        const lower = name.toLowerCase();
        if (lower.startsWith(query)) {
          starts.push(name);
        } else if (lower.includes(query)) {
          contains.push(name);
        }
      }
      return [...starts, ...contains];
    },
    visibleNames() {
      return this.filtered.slice(0, this.maxVisible);
    },
    truncated() {
      return this.filtered.length > this.maxVisible;
    },
  },
  async created() {
    await this.loadEntities();
  },
  mounted() {
    this.$refs.filterInput?.focus();
    eventBus.on("groupsChanged", this.onGroupsChanged);
  },
  beforeUnmount() {
    eventBus.off("groupsChanged", this.onGroupsChanged);
  },
  methods: {
    async loadEntities() {
      this.loading = true;
      try {
        if (this.kind === "group") {
          const res = await accessApi.getGroupsWithMembers();
          this.names = res.groups || [];
          this.members = new Map(Object.entries(res.members || {}));
        } else {
          const users = await usersApi.getAllUsers();
          this.names = (users || []).map((u) => u.username).sort((a, b) => a.localeCompare(b));
          // Per-user group counts, mirroring the members meta on groups.
          try {
            const res = await accessApi.getGroupsWithMembers();
            const counts = new Map();
            for (const members of Object.values(res.members || {})) {
              for (const username of members) {
                counts.set(username, (counts.get(username) || 0) + 1);
              }
            }
            this.userGroupCounts = counts;
          } catch {
            this.userGroupCounts = new Map();
          }
        }
      } catch (e) {
        notify.showError(e);
      } finally {
        this.loading = false;
      }
    },
    async onGroupsChanged() {
      if (this.kind === "group") {
        await this.loadEntities();
      }
    },
    isPicked(name) {
      return this.modelValue.includes(name);
    },
    metaCount(name) {
      if (this.kind === "group") {
        return (this.members.get(name) || []).length;
      }
      if (!this.names.includes(name)) {
        // Extra option (e.g. external-auth member) has no local user record.
        return null;
      }
      return this.userGroupCounts.get(name) || 0;
    },
    toggle(name) {
      if (this.multiple) {
        this.$emit(
          "update:modelValue",
          this.isPicked(name)
            ? this.modelValue.filter((n) => n !== name)
            : [...this.modelValue, name]
        );
        return;
      }
      this.$emit("update:modelValue", [name]);
    },
    onEnter() {
      // Enter picks the first match when nothing is selected yet, then
      // bubbles up so a wrapping prompt can confirm.
      if (this.modelValue.length === 0 && this.filtered.length > 0) {
        this.$emit("update:modelValue", [this.filtered[0]]);
      }
      this.$emit("enter");
    },
    async startCreate() {
      if (!this.allowCreate || this.kind !== "group") {
        return;
      }
      this.creatingGroup = true;
      this.createError = null;
      await this.$nextTick();
      this.$refs.createInput?.focus();
    },
    cancelCreateGroup() {
      this.creatingGroup = false;
      this.newGroupName = "";
      this.createError = null;
    },
    async confirmCreateGroup() {
      const name = this.newGroupName.trim();
      this.createError = groupNameErrorText(name);
      if (this.createError) {
        return;
      }
      if (this.names.includes(name)) {
        this.createError = this.$t("access.groupExists");
        return;
      }
      if (this.creating) {
        return;
      }
      this.creating = true;
      try {
        await accessApi.saveGroup(name, [], true);
        eventBus.emit("groupsChanged");
        if (!this.exclude.includes(name) && !this.isPicked(name)) {
          this.$emit(
            "update:modelValue",
            this.multiple ? [...this.modelValue, name] : [name]
          );
        }
        this.cancelCreateGroup();
      } catch (e) {
        notify.showError(e);
      } finally {
        this.creating = false;
      }
    },
  },
};
</script>

<style scoped>
.entity-picker-filter {
  width: 100%;
  margin-bottom: 0.5em;
}

.entity-picker-create-row {
  display: flex;
  align-items: center;
  gap: 0.35em;
  margin-bottom: 0.5em;
}

.entity-picker-create-input {
  flex: 1;
}

.entity-picker-create-confirm,
.entity-picker-create-cancel {
  min-width: 2.2em;
  padding: 0 0.3em;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.entity-picker-error {
  margin: 0 0 0.5em;
  color: var(--red);
  font-size: 0.85em;
}

.entity-picker-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 320px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 0.25em;
}

.entity-picker-row {
  display: flex;
  align-items: center;
  gap: 0.5em;
  padding: 0.35em 0.5em;
  cursor: pointer;
  user-select: none;
}

.entity-picker-row:hover,
.entity-picker-row:focus-visible {
  background: color-mix(in srgb, var(--primaryColor) 10%, var(--surfacePrimary));
}

.entity-picker-row--picked {
  background: color-mix(in srgb, var(--primaryColor) 15%, var(--surfacePrimary));
}

.entity-picker-check {
  font-size: 1.1em;
  color: var(--primaryColor);
}

.entity-picker-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.entity-picker-meta {
  margin-left: auto;
  font-size: 0.85em;
  opacity: 0.7;
  white-space: nowrap;
}

.entity-picker-empty {
  padding: 1em;
  opacity: 0.7;
  text-align: center;
}

.entity-picker-hint {
  margin: 0.4em 0 0;
  font-size: 0.85em;
  opacity: 0.7;
}

.loading-spinner-wrapper {
  display: flex;
  justify-content: center;
  padding: 1em;
}
</style>
