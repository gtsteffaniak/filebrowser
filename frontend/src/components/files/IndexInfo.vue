<template>
  <table class="index-info-table">
    <thead>
      <tr>
        <th colspan="2">{{ info.name || 'Source' }}</th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td>{{ $t("general.status") }}</td>
        <td>{{ getStatusLabel(info.status) }}</td>
      </tr>
      <tr>
        <td>{{ $t("index.assessment") }}</td>
        <td>{{ getComplexityLabel(info.complexity || 0) }}</td>
      </tr>
      <tr>
        <td>{{ $t("general.files") }}</td>
        <td>{{ info.files || 0 }}</td>
      </tr>
      <tr>
        <td>{{ $t("general.folders") }}</td>
        <td>{{ info.folders || 0 }}</td>
      </tr>
      <tr>
        <td>{{ $t("index.lastScanned") }}</td>
        <td>{{ getHumanReadable(info.lastIndex) }}</td>
      </tr>
      <tr>
        <td>{{ $t("index.scanTime") }}</td>
        <td>{{ formatDuration(info.scanDurationSeconds) }}</td>
      </tr>
    </tbody>
  </table>
</template>

<script>
import { state } from "@/store";
import { fromNow } from "@/utils/moment";

export default {
  name: "IndexInfo",
  props: {
    info: {
      type: Object,
      required: true,
    },
  },
  methods: {
    getComplexityLabel(complexity) {
      return getComplexityLabel(complexity, this.$t);
    },
    getStatusLabel(status) {
      return getStatusLabel(status, this.$t);
    },
    getHumanReadable(lastIndex) {
      return getHumanReadableTime(lastIndex, state.user.locale);
    },
    formatDuration(seconds) {
      return formatDuration(seconds);
    },
  },
};

// Export utility functions for use in other components (e.g., generating tooltip HTML)
export function getComplexityLabel(complexity, $t) {
  // Frontend interprets: 0=unknown, 1=simple, 2-6=normal, 7-9=complex, 10=highlyComplex
  if (complexity === 0) return $t("index.unknown");
  if (complexity === 1) return $t("index.simple");
  if (complexity >= 2 && complexity <= 6) return $t("index.normal");
  if (complexity >= 7 && complexity <= 9) return $t("index.complex");
  if (complexity === 10) return $t("index.highlyComplex");
  return $t("index.unknown");
}

export function getStatusLabel(status, $t) {
  switch (status) {
    case "ready": return $t("index.ready");
    case "indexing": return $t("index.indexing");
    case "unavailable": return $t("index.unavailable");
    case "error": return $t("index.error");
    default: return $t("index.unknown");
  }
}

export function getHumanReadableTime(lastIndex, locale) {
  if (isNaN(Number(lastIndex))) return "";
  let val = Number(lastIndex);
  if (val === 0) return "now";
  return fromNow(val, locale);
}

export function formatDuration(seconds) {
  if (isNaN(Number(seconds))) return '';
  return Number(seconds);
}

// Generate HTML tooltip content using the same logic as the component
export function buildIndexInfoTooltipHTML(info, $t, locale) {
  return `
    <table style="border-collapse: collapse; text-align: left;">
      <thead>
        <tr>
          <th colspan="2" style="text-align: center; font-weight: bold; font-size: 1.1em; padding-bottom: 0.3em; border-bottom: 1px solid #888;">${info.name || 'Source'}</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${$t("general.status")}</td>
          <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${getStatusLabel(info.status, $t)}</td>
        </tr>
        <tr>
          <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${$t("index.assessment")}</td>
          <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${getComplexityLabel(info.complexity || 0, $t)}</td>
        </tr>
        <tr>
          <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${$t("general.files")}</td>
          <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${info.files || 0}</td>
        </tr>
        <tr>
          <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${$t("general.folders")}</td>
          <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${info.folders || 0}</td>
        </tr>
        <tr>
          <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${$t("index.lastScanned")}</td>
          <td style="padding: 0.2em 0.5em; border-bottom: 1px solid #ccc;">${getHumanReadableTime(info.lastIndex, locale)}</td>
        </tr>
        <tr>
          <td style="padding: 0.2em 0.5em;">${$t("index.scanTime")}</td>
          <td style="padding: 0.2em 0.5em;">${formatDuration(info.scanDurationSeconds)}</td>
        </tr>
      </tbody>
    </table>
  `;
}
</script>

<style scoped>
.index-info-table {
  border-collapse: collapse;
  text-align: left;
  width: 100%;
}

.index-info-table thead th {
  text-align: center;
  font-weight: bold;
  font-size: 1.1em;
  padding-bottom: 0.3em;
  border-bottom: 1px solid #888;
}

.index-info-table tbody td {
  padding: 0.2em 0.5em;
  border-bottom: 1px solid #ccc;
}

.index-info-table tbody tr:last-child td {
  border-bottom: none;
}

.index-info-table tbody td:first-child {
  font-weight: 500;
  color: var(--textSecondary);
}
</style>

