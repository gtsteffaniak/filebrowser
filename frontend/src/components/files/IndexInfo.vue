<template>
  <SettingsTable
    class="index-info-table"
    :header-title="info.name || 'Source'"
    :columns="columns"
    :items="rows"
    item-key="id"
    :aria-label="info.name || 'Source'"
  />
</template>

<script>
import i18n from "@/i18n/index.ts";
import { state } from "@/store/index.ts";
import { fromNow } from "@/utils/moment.ts";
import SettingsTable from "@/components/settings/Table.vue";

export default {
  name: "IndexInfo",
  components: {
    SettingsTable,
  },
  props: {
    info: {
      type: Object,
      required: true,
    },
  },
  computed: {
    columns() {
      return [
        { key: "label", label: "", sortable: false, align: "left" },
        { key: "value", label: "", sortable: false, align: "left" },
      ];
    },
    rows() {
      return buildIndexInfoRows(this.info, state.user?.locale);
    },
  },
};

const t = i18n.global.t.bind(i18n.global);

// Export utility functions for use in other components (e.g., generating tooltip HTML)
export function getComplexityLabel(complexity) {
  // Frontend interprets: 0=unknown, 1=simple, 2-6=normal, 7-9=complex, 10=highlyComplex
  if (complexity === 0) return t("general.unknown");
  if (complexity === 1) return t("general.simple");
  if (complexity >= 2 && complexity <= 6) return t("general.normal");
  if (complexity >= 7 && complexity <= 9) return t("general.complex");
  if (complexity === 10) return t("index.highlyComplex");
  return t("general.unknown");
}

export function getStatusLabel(status) {
  switch (status) {
    case "ready": return t("index.ready");
    case "indexing": return t("index.indexing");
    case "unavailable": return t("index.unavailable");
    case "error": return t("general.error");
    default: return t("general.unknown");
  }
}

export function getHumanReadableTime(lastIndex, locale) {
  if (Number.isNaN(Number(lastIndex))) return "";
  const val = Number(lastIndex);
  if (val === 0) return "now";
  return fromNow(val, locale);
}

export function formatDuration(seconds) {
  if (Number.isNaN(Number(seconds))) return '';
  return Number(seconds);
}

export function formatBooleanYesNo(value) {
  return value ? t("general.yes") : t("general.no");
}

/** @returns {{ id: string, label: string, value: string | number }[]} */
export function buildIndexInfoRows(info, locale) {
  return [
    { id: "status", label: t("general.status"), value: getStatusLabel(info.status) },
    { id: "readOnly", label: t("general.readOnly"), value: formatBooleanYesNo(info.readOnly) },
    { id: "private", label: t("general.private"), value: formatBooleanYesNo(info.private) },
    { id: "assessment", label: t("index.assessment"), value: getComplexityLabel(info.complexity || 0) },
    { id: "files", label: t("general.files"), value: info.files || 0 },
    { id: "folders", label: t("general.folders"), value: info.folders || 0 },
    { id: "lastScanned", label: t("index.lastScanned"), value: getHumanReadableTime(info.lastIndex, locale) },
    { id: "quickScan", label: t("index.quickScan"), value: formatDuration(info.quickScanDurationSeconds) },
    { id: "fullScan", label: t("index.fullScan"), value: formatDuration(info.fullScanDurationSeconds) },
  ];
}
</script>

<style scoped>
.index-info-table :deep(thead th),
.index-info-table :deep(tbody td) {
  padding: 0.5em;
}

.index-info-table :deep(tbody td:first-child) {
  font-weight: 500;
  color: var(--textSecondary);
}
</style>
