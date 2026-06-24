<template>
  <div class="card-content info-content">
    <div class="info-grid">
      <div class="info-section">
        <h3 class="section-title">{{ $t("tools.activityViewer.eventSummary") }}</h3>
        <div v-for="item in summaryRows" :key="item.key" class="info-item">
          <strong>{{ item.label }}</strong>
          <span class="break-word detail-value">{{ item.value }}</span>
        </div>
      </div>

      <div v-if="detailRows.length > 0" class="info-section">
        <h3 class="section-title">{{ $t("general.details") }}</h3>
        <SettingsTable
          class="activity-details-table"
          :columns="detailColumns"
          :items="detailRows"
          item-key="id"
          :aria-label="$t('general.details')"
        />
      </div>
    </div>
  </div>
</template>

<script>
import SettingsTable from "@/components/settings/Table.vue";
import { formatTimestamp } from "@/utils/moment";
import {
  buildActivityDetailRows,
  activityEventLabel,
  activityRowPath,
  activityRowSource,
} from "@/utils/activityDetails";

export default {
  name: "ActivityEventDetails",
  components: {
    SettingsTable,
  },
  props: {
    row: {
      type: Object,
      required: true,
    },
  },
  computed: {
    eventLabel() {
      return activityEventLabel(this.row.eventType, this.$t);
    },
    detailColumns() {
      return [
        { key: "label", label: this.$t("general.name"), sortable: false, align: "left" },
        { key: "value", label: this.$t("general.value"), sortable: false, align: "left" },
      ];
    },
    summaryRows() {
      const rows = [
        {
          key: "time",
          label: this.$t("time.time"),
          value: formatTimestamp(this.row.createdAt * 1000),
        },
        {
          key: "username",
          label: this.$t("general.username"),
          value: this.row.username || "—",
        },
        {
          key: "eventType",
          label: this.$t("tools.activityViewer.eventType"),
          value: this.eventLabel,
        },
      ];
      const source = activityRowSource(this.row);
      if (source) {
        rows.push({
          key: "source",
          label: this.$t("general.source"),
          value: source,
        });
      }
      const path = activityRowPath(this.row);
      if (path) {
        rows.push({
          key: "path",
          label: this.$t("general.path"),
          value: path,
        });
      }
      if (this.row.ipAddress) {
        rows.push({
          key: "ipAddress",
          label: this.$t("general.ipAddress"),
          value: this.row.ipAddress,
        });
      }
      return rows;
    },
    detailRows() {
      return buildActivityDetailRows(this.row, this.$t);
    },
  },
};
</script>

<style scoped>
.info-content {
  height: 100%;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.info-grid {
  display: grid;
  gap: 1.5em;
  flex: 1;
}

.info-section {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
}

.section-title {
  font-size: 0.95em;
  font-weight: 600;
  color: var(--textPrimary);
  margin: 0 0 0.75em 0;
  padding-bottom: 0.5em;
  border-bottom: 1px solid var(--divider);
}

.info-item {
  display: flex;
  align-items: flex-start;
  gap: 0.75em;
  padding: 0.5em;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.info-item:hover {
  background-color: var(--surfaceSecondary);
}

.info-item strong {
  min-width: 120px;
  font-weight: 600;
  color: var(--textPrimary);
}

.detail-value {
  flex: 1;
  color: var(--textSecondary);
  word-break: break-word;
  white-space: pre-wrap;
  user-select: text;
}

.break-word {
  word-break: break-word;
}

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

@media (max-width: 768px) {
  .info-item strong {
    min-width: 100px;
  }
}
</style>
