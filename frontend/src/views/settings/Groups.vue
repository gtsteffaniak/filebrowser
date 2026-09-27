<template>
  <button
    type="button"
    @click="openPrompt(null)"
    class="button floating-action-button"
    :aria-label="$t('access.newGroup')"
  >
    {{ $t("general.new") }}
  </button>
  <errors v-if="error" :errorCode="error.status" />
  <div class="card-title">
    <h2>{{ $t("access.groups") }}</h2>
  </div>

  <div class="card-content full">
    <p class="note">{{ $t("access.groupsOidcNote") }}</p>
    <settings-table
      :columns="columns"
      :items="rows"
      item-key="name"
      default-sort-key="name"
      :aria-label="$t('access.groups')"
      :loading="loading"
    >
      <template #cell-members="{ row }">{{ row.members.join(", ") }}</template>
      <template #cell-actions="{ row }">
        <div class="row-actions">
        <div
          @click="openPrompt(row.name)"
          class="clickable action button"
          role="button"
          tabindex="0"
          :aria-label="$t('general.edit')"
          :title="$t('general.edit')"
          @keydown.enter.prevent="openPrompt(row.name)"
          @keydown.space.prevent="openPrompt(row.name)"
        >
          <i class="material-symbols">edit</i>
        </div>
        <div
          @click="remove(row.name)"
          class="clickable action button"
          role="button"
          tabindex="0"
          :aria-label="$t('general.delete')"
          :title="$t('general.delete')"
          @keydown.enter.prevent="remove(row.name)"
          @keydown.space.prevent="remove(row.name)"
        >
          <i class="material-symbols">delete</i>
        </div>
        </div>
      </template>
    </settings-table>
  </div>
</template>

<script>
import { mutations } from "@/store";
import { accessApi } from "@/api";
import { notify } from "@/notify";
import Errors from "@/views/Errors.vue";
import SettingsTable from "@/components/settings/Table.vue";
import { eventBus } from "@/store/eventBus";

export default {
  name: "groups",
  components: { Errors, SettingsTable },
  data() {
    return {
      error: null,
      groups: [],
      members: new Map(),
      loading: true,
    };
  },
  async created() {
    await this.reload();
  },
  mounted() {
    eventBus.on("groupsChanged", this.reload);
  },
  beforeUnmount() {
    eventBus.off("groupsChanged", this.reload);
  },
  computed: {
    rows() {
      return this.groups.map((name) => {
        const members = this.members.get(name) || [];
        return { name, members, memberCount: members.length };
      });
    },
    columns() {
      return [
        { key: "name", label: this.$t("access.groupName"), sortable: true },
        {
          key: "memberCount",
          label: this.$t("access.groupMembers"),
          sortable: true,
          // Compare numerically; the default comparator sorts as strings ("10" < "9").
          sortFn: (a, b) => a.memberCount - b.memberCount,
        },
        { key: "members", label: this.$t("general.users", { suffix: "" }) },
        { key: "actions", label: "", align: "right", narrow: true },
      ];
    },
  },
  methods: {
    async reload() {
      this.loading = true;
      try {
        const res = await accessApi.getGroupsWithMembers();
        this.groups = res.groups || [];
        this.members = new Map(Object.entries(res.members || {}));
        this.error = null;
      } catch (e) {
        this.error = e;
      } finally {
        this.loading = false;
      }
    },
    openPrompt(group) {
      mutations.showPrompt({
        name: "group-edit",
        props: {
          title: this.$t(group ? "access.editGroup" : "access.newGroup"),
          ...(group ? { group, members: this.members.get(group) || [] } : {}),
        },
      });
    },
    escapeHtml(text) {
      const el = document.createElement("div");
      el.textContent = text;
      return el.innerHTML;
    },
    remove(group) {
      mutations.showPrompt({
        name: "generic",
        props: {
          title: this.$t("general.delete"),
          // Generic renders body via v-html, so escape the user-supplied name.
          body: this.$t("access.deleteGroupConfirm", { name: this.escapeHtml(group) }),
          buttons: [
            {
              label: this.$t("general.cancel"),
              className: "button--grey",
              action: () => mutations.closeTopPrompt(),
            },
            {
              label: this.$t("general.delete"),
              className: "button--red",
              action: async () => {
                try {
                  await accessApi.deleteGroup(group);
                  eventBus.emit("groupsChanged");
                } catch (e) {
                  notify.showError(e);
                } finally {
                  mutations.closeTopPrompt();
                }
              },
            },
          ],
        },
      });
    },
  },
};
</script>

<style scoped>
.card-content.full :deep(.settings-table-wrapper) {
  margin-top: 0.75rem;
}
.row-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.25rem;
}
.clickable {
  cursor: pointer;
}
.note {
  opacity: 0.75;
  margin: 0.5rem 1rem 0;
}
</style>
