<template>
  <div class="card-content">
    <p v-if="isNew">
      <label for="group-name">{{ $t("access.groupName") }}</label>
      <input
        id="group-name"
        class="input"
        type="text"
        v-model.trim="name"
        :placeholder="$t('access.groupName')"
        v-focus
        @keyup.enter="submit"
      />
    </p>
    <p v-else>
      <label>{{ $t("access.groupName") }}</label>
      <input class="input" type="text" :value="name" disabled />
    </p>
    <label for="group-member-input">{{ membersLabel }}</label>
    <div class="form-flex-group">
      <input
        id="group-member-input"
        class="input flat-right form-grow"
        type="text"
        list="group-member-suggestions"
        v-model.trim="newMember"
        :placeholder="$t('access.enterUsername')"
        @keydown.enter.prevent="addMember"
      />
      <datalist id="group-member-suggestions">
        <option v-for="username in suggestedUsers" :key="username" :value="username"></option>
      </datalist>
      <button
        type="button"
        class="button form-button flat-left"
        :aria-label="$t('access.addMember')"
        :title="$t('access.addMember')"
        @click="addMember"
      >
        <i class="material-symbols">add</i>
      </button>
    </div>
    <settings-table
      v-if="selected.length > 0"
      :columns="columns"
      :items="memberRows"
      item-key="name"
      default-sort-key="name"
      :aria-label="membersLabel"
    >
      <template #cell-edit="{ row }">
        <button
          type="button"
          class="action"
          :aria-label="$t('general.delete')"
          :title="$t('general.delete')"
          @click="removeMember(row.name)"
        >
          <i class="material-symbols">delete</i>
        </button>
      </template>
    </settings-table>
  </div>

  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--grey"
      @click="closeTopPrompt"
      :aria-label="$t('general.cancel')"
    >
      {{ $t("general.cancel") }}
    </button>
    <button
      type="button"
      class="button button--flat"
      :disabled="!name || saving"
      @click="submit"
      :aria-label="$t('general.save')"
    >
      {{ $t("general.save") }}
    </button>
  </div>
</template>

<script>
import { accessApi, usersApi } from "@/api";
import { mutations } from "@/store";
import { notify } from "@/notify";
import { eventBus } from "@/store/eventBus";
import SettingsTable from "@/components/settings/Table.vue";

export default {
  name: "group-edit",
  components: { SettingsTable },
  props: {
    group: { type: String, default: "" },
    members: { type: Array, default: () => [] },
  },
  data() {
    return {
      name: this.group,
      selected: [...this.members],
      allUsers: [],
      newMember: "",
      saving: false,
    };
  },
  async created() {
    try {
      const users = await usersApi.getAllUsers();
      const names = new Set(users.map((u) => u.username));
      // Keep members that no longer map to a local user (e.g. OIDC-only) visible.
      for (const m of this.members) {
        names.add(m);
      }
      this.allUsers = [...names].sort((a, b) => a.localeCompare(b));
    } catch (e) {
      notify.showError(e);
    }
  },
  computed: {
    membersLabel() {
      return `${this.$t("access.groupMembers")} (${this.selected.length})`;
    },
    isNew() {
      return !this.group;
    },
    memberRows() {
      return this.selected.map((name) => ({ name }));
    },
    columns() {
      return [
        { key: "name", label: this.$t("general.name", { suffix: "" }), sortable: true },
        { key: "edit", label: this.$t("general.edit"), narrow: true, align: "right" },
      ];
    },
    suggestedUsers() {
      return this.allUsers.filter((u) => !this.selected.includes(u));
    },
  },
  methods: {
    addMember() {
      if (!this.newMember) return;
      if (!this.allUsers.includes(this.newMember)) {
        notify.showError(this.$t("access.unknownUser", { name: this.newMember }));
        return;
      }
      if (!this.selected.includes(this.newMember)) {
        this.selected.push(this.newMember);
      }
      this.newMember = "";
    },
    removeMember(username) {
      this.selected = this.selected.filter((u) => u !== username);
    },
    closeTopPrompt() {
      mutations.closeTopPrompt();
    },
    async submit() {
      if (!this.name || this.saving) return;
      this.saving = true;
      try {
        await accessApi.saveGroup(this.name, this.selected);
        eventBus.emit("groupsChanged");
        mutations.closeTopPrompt();
      } catch (e) {
        notify.showError(e);
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>

<style scoped>
.form-flex-group {
  margin: 0.5em 0 1em;
}
</style>
