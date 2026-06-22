<template>
  <SettingsTable
    class="activity-details-table"
    :header-title="headerTitle"
    :columns="columns"
    :items="rows"
    item-key="id"
    :aria-label="headerTitle"
  />
</template>

<script>
import SettingsTable from "@/components/settings/Table.vue";
import { buildActivityDetailRows } from "@/utils/activityDetails";

export default {
  name: "ActivityDetailsInfo",
  components: {
    SettingsTable,
  },
  props: {
    row: {
      type: Object,
      required: true,
    },
    eventLabel: {
      type: String,
      default: "",
    },
  },
  computed: {
    headerTitle() {
      return this.eventLabel || this.$t("general.details");
    },
    columns() {
      return [
        { key: "label", label: "", sortable: false, align: "left" },
        { key: "value", label: "", sortable: false, align: "left" },
      ];
    },
    rows() {
      return buildActivityDetailRows(this.row, this.$t);
    },
  },
};
</script>

<style scoped>
.activity-details-table :deep(thead th),
.activity-details-table :deep(tbody td) {
  padding: 0.5em;
}

.activity-details-table :deep(tbody td:first-child) {
  font-weight: 500;
  color: var(--textSecondary);
  white-space: nowrap;
  vertical-align: top;
}

.activity-details-table :deep(tbody td:last-child) {
  word-break: break-word;
  white-space: pre-wrap;
}
</style>
