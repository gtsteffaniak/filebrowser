<template>
  <SettingsTable
    class="activity-table-value-info"
    :header-title="label"
    :columns="columns"
    :items="rows"
    item-key="id"
    :aria-label="label"
  />
</template>

<script>
import SettingsTable from "@/components/settings/Table.vue";

export default {
  name: "ActivityTableValueInfo",
  components: {
    SettingsTable,
  },
  props: {
    label: {
      type: String,
      required: true,
    },
    value: {
      type: String,
      required: true,
    },
  },
  computed: {
    columns() {
      return [
        { key: "value", label: "", sortable: false, align: "left" },
      ];
    },
    rows() {
      return [{ id: "value", value: this.value }];
    },
  },
};
</script>

<style scoped>
.activity-table-value-info :deep(thead th),
.activity-table-value-info :deep(tbody td) {
  padding: 0.5em;
}

.activity-table-value-info :deep(tbody td) {
  word-break: break-word;
  white-space: pre-wrap;
}
</style>
