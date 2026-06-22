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
        <div v-for="item in detailRows" :key="item.id" class="info-item">
          <strong>{{ item.label }}</strong>
          <span class="break-word detail-value">{{ item.value }}</span>
        </div>
      </div>

      <p v-else-if="!admin" class="details-restricted-msg">{{ $t("general.unavailable") }}</p>
    </div>
  </div>
</template>

<script>
import { getters } from "@/store";
import { formatTimestamp } from "@/utils/moment";
import { buildActivityDetailRows, activityEventLabel } from "@/utils/activityDetails";

export default {
  name: "ActivityEventDetails",
  props: {
    row: {
      type: Object,
      required: true,
    },
  },
  computed: {
    admin() {
      return getters.isAdmin();
    },
    eventLabel() {
      return activityEventLabel(this.row.eventType, this.$t);
    },
    summaryRows() {
      const rows = [
        {
          key: "time",
          label: this.$t("general.time"),
          value: formatTimestamp(this.row.createdAt * 1000),
        },
        {
          key: "username",
          label: this.$t("general.username"),
          value: this.row.username || this.$t("general.unavailable"),
        },
        {
          key: "eventType",
          label: this.$t("tools.activityViewer.eventType"),
          value: this.eventLabel,
        },
      ];
      if (this.row.ipAddress) {
        rows.push({
          key: "ipAddress",
          label: this.$t("general.ipAddress"),
          value: this.row.ipAddress,
        });
      }
      if (this.row.status) {
        rows.push({
          key: "status",
          label: this.$t("general.status"),
          value: String(this.row.status),
        });
      }
      return rows;
    },
    detailRows() {
      if (!this.admin) {
        return [];
      }
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

.details-restricted-msg {
  margin: 0;
  color: var(--textSecondary);
  font-size: 0.95rem;
}

@media (max-width: 768px) {
  .info-item strong {
    min-width: 100px;
  }
}
</style>
