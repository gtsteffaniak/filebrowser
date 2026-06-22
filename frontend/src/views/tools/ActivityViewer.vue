<template>
  <div class="activity-viewer">
    <div class="card activity-scope-card padding-normal">
      <div class="universal-filters" :class="{ mobile: isMobile }">
        <div class="filter-field">
          <label class="filter-label" for="activity-scope">{{ $t("tools.activityViewer.activityScope") }}</label>
          <select
            id="activity-scope"
            v-model="activityScope"
            class="input activity-scope-select border-radius"
            @change="onScopeChange"
          >
            <option value="all">{{ $t("tools.activityViewer.scopeAll") }}</option>
            <option value="files">{{ $t("tools.activityViewer.scopeFiles") }}</option>
            <option value="shares">{{ $t("tools.activityViewer.scopeShares") }}</option>
          </select>
        </div>

        <div class="filter-field">
          <label class="filter-label">{{ $t("tools.activityViewer.timeRange") }}</label>
          <select v-model="timePreset" class="input" @change="onTimePresetChange">
            <option value="1h">{{ $t("tools.activityViewer.last1h") }}</option>
            <option value="24h">{{ $t("tools.activityViewer.last24h") }}</option>
            <option value="7d">{{ $t("tools.activityViewer.last7d") }}</option>
            <option value="30d">{{ $t("tools.activityViewer.last30d") }}</option>
            <option value="custom">{{ $t("tools.activityViewer.customRange") }}</option>
          </select>
        </div>

        <div v-if="timePreset === 'custom'" class="filter-field filter-field-wide">
          <label class="filter-label">{{ $t("tools.activityViewer.customRange") }}</label>
          <div class="custom-range">
            <input v-model="customFrom" type="datetime-local" class="input" />
            <input v-model="customTo" type="datetime-local" class="input" />
          </div>
        </div>

        <div v-if="isAdmin" class="filter-field">
          <label class="filter-label">{{ $t("general.user") }}</label>
          <select v-model="selectedUsername" class="input">
            <option value="">{{ $t("general.all", { suffix: " " }) }}{{ $t("general.users") }}</option>
            <option :value="anonymousUsername">{{ $t("general.anonymous") }}</option>
            <option v-for="u in users" :key="u.username" :value="u.username">
              {{ u.username }}
            </option>
          </select>
        </div>

        <div class="filter-field">
          <label class="filter-label">{{ $t("tools.activityViewer.viewType") }}</label>
          <select v-model="viewType" class="input">
            <option value="chart">{{ $t("tools.activityViewer.chartView") }}</option>
            <option value="line">{{ $t("tools.activityViewer.lineView") }}</option>
            <option value="pie">{{ $t("tools.activityViewer.pieView") }}</option>
            <option value="summary">{{ $t("tools.activityViewer.summaryView") }}</option>
            <option value="table">{{ $t("tools.activityViewer.tableView") }}</option>
          </select>
        </div>
      </div>
      <p class="activity-scope-hint">{{ scopeHint }}</p>
    </div>

    <div class="card activity-viewer-config padding-normal">
      <div class="card-content config-grid" :class="{ mobile: isMobile }">
        <div v-if="showEventTypeFilter" class="config-field">
          <h3>{{ $t("tools.activityViewer.eventType") }}</h3>
          <select v-model="selectedEventType" class="input">
            <option value="">{{ $t("tools.activityViewer.allEvents") }}</option>
            <option v-for="et in visibleEventTypes" :key="et" :value="et">
              {{ eventTypeLabel(et) }}
            </option>
          </select>
        </div>

        <div v-if="showTimeSeriesOptions" class="config-field">
          <h3>{{ $t("tools.activityViewer.interval") }}</h3>
          <select v-model="chartInterval" class="input">
            <option value="minute">{{ $t("tools.activityViewer.byMinute") }}</option>
            <option value="hour">{{ $t("tools.activityViewer.byHour") }}</option>
            <option value="day">{{ $t("tools.activityViewer.byDay") }}</option>
          </select>
        </div>

        <div v-if="showChartOptions" class="config-field">
          <h3>{{ $t("tools.activityViewer.splitBy") }}</h3>
          <select v-model="splitBy" class="input">
            <option value="eventType">{{ $t("tools.activityViewer.eventType") }}</option>
            <option value="user">{{ $t("general.user") }}</option>
            <option v-if="showTimeSeriesOptions" value="none">{{ $t("tools.activityViewer.splitByTotal") }}</option>
          </select>
        </div>

        <div v-if="showFileFilters" class="config-field config-field-wide path-filters">
          <h3>{{ $t("tools.activityViewer.pathFilter") }}</h3>
          <div class="config-field">
            <label class="filter-label">{{ $t("tools.activityViewer.pathFilterMode") }}</label>
            <select v-model="filePathFilterMode" class="input">
              <option value="picker">{{ $t("general.browse") }}</option>
              <option value="glob">{{ $t("tools.activityViewer.pathFilterPattern") }}</option>
            </select>
          </div>
          <div v-if="filePathFilterMode === 'picker'" class="path-filter-picker">
            <PathPickerButton
              v-model:path="filterPath"
              v-model:source="filterSource"
              aria-label="activity-file-path"
              :show-files="true"
              :show-folders="true"
              :placeholder="$t('sidebar.chooseSource')"
              @navigate="resetPageAndLoad"
            />
          </div>
          <div v-else class="path-filter-glob">
            <div class="glob-fields">
              <div class="glob-field">
                <label class="filter-label">{{ $t("general.source") }}</label>
                <select v-model="filterSource" class="input">
                  <option value="">{{ $t("general.all", { suffix: " " }) }}{{ $t("general.sources") }}</option>
                  <option v-for="name in sourceNames" :key="name" :value="name">{{ name }}</option>
                </select>
              </div>
              <div class="glob-field glob-field-wide">
                <label class="filter-label">{{ $t("tools.activityViewer.pathGlob") }}</label>
                <input
                  v-model="filterPathGlob"
                  type="text"
                  class="input"
                  placeholder="/docs/*"
                />
              </div>
            </div>
          </div>
        </div>

        <div v-if="showShareFilters" class="config-field config-field-wide path-filters">
          <h3>{{ $t("tools.activityViewer.shareFilter") }}</h3>
          <SharePickerButton
            v-model:shareHash="filterShareHash"
            aria-label="activity-share-picker"
            :placeholder="$t('tools.activityViewer.allShares')"
            @select="resetPageAndLoad"
          />
        </div>

        <div class="config-actions">
          <button type="button" class="button" @click="loadData" :disabled="loading">
            <i v-if="loading" class="material-symbols spin">autorenew</i>
            <span v-else>{{ $t("buttons.refresh") }}</span>
          </button>
          <button type="button" class="button button--flat" @click="exportCsv" :disabled="loading">
            {{ $t("tools.activityViewer.exportCsv") }}
          </button>
        </div>
      </div>
    </div>

    <section class="activity-viewer-results">
      <errors v-if="error" :errorCode="error.status" />

      <div v-if="loading && !hasResults" class="loading-state">
        <i class="material-symbols spin">progress_activity</i>
      </div>

      <div v-else-if="viewType === 'table'" class="results-table">
        <div v-if="totalEvents > 0" class="results-stats">
          <span>{{ $t("tools.activityViewer.totalEvents", { suffix: ": " }) }}<strong>{{ totalEvents }}</strong></span>
          <span v-if="totalEvents > items.length">
            {{ $t("tools.activityViewer.showingPage", { shown: items.length, total: totalEvents, page: currentPage, pages: totalPages }) }}
          </span>
        </div>
        <settings-table
          :columns="tableColumns"
          :items="items"
          item-key="id"
          default-sort-key="createdAt"
          default-sort-dir="desc"
          :loading="loading"
          row-clickable
          :aria-label="$t('tools.activityViewer.name')"
          :lonely-message-key="!loading && items.length === 0 ? 'files.lonely' : undefined"
          @row-click="openEventDetails"
        >
          <template #cell-createdAt="{ row }">
            {{ formatTime(row.createdAt) }}
          </template>
          <template #cell-eventType="{ row }">
            {{ eventTypeLabel(row.eventType) }}
          </template>
          <template #cell-details="{ row }">
            <div
              v-if="isAdmin && hasRowDetails(row)"
              class="details-cell-wrap"
              @mouseenter="showDetailsTooltip($event, row)"
              @mouseleave="hideDetailsTooltip"
            >
              <div class="details-badges">
                <span
                  v-for="badge in detailBadges(row)"
                  :key="badge.id"
                  class="detail-badge border-radius"
                >{{ badge.text }}</span>
              </div>
            </div>
            <span v-else-if="!isAdmin" class="details-restricted">{{ $t("general.unavailable") }}</span>
            <span v-else class="details-muted">{{ $t("general.unavailable") }}</span>
          </template>
          <template #cell-status="{ row }">
            <span
              v-if="row.status"
              class="status-badge border-radius"
              :class="statusBadgeClass(row.status)"
            >{{ row.status }}</span>
            <span v-else class="status-badge status-badge--muted border-radius">{{ $t("general.unavailable") }}</span>
          </template>
        </settings-table>
        <div v-if="totalPages > 1" class="pagination">
          <button
            type="button"
            class="button button--flat"
            :disabled="loading || currentPage <= 1"
            @click="goToPage(currentPage - 1)"
          >
            {{ $t("general.previous") }}
          </button>
          <span class="page-label">{{ $t("tools.activityViewer.pageOf", { page: currentPage, pages: totalPages }) }}</span>
          <button
            type="button"
            class="button button--flat"
            :disabled="loading || currentPage >= totalPages"
            @click="goToPage(currentPage + 1)"
          >
            {{ $t("general.next") }}
          </button>
        </div>
      </div>

      <div v-else-if="showChartPanel" class="results-chart">
        <div class="results-stats">
          <span>{{ $t("tools.activityViewer.totalEvents", { suffix: ": " }) }}<strong>{{ totalEventCount }}</strong></span>
        </div>
        <div class="chart-panel border-radius">
          <canvas
            :key="chartMountKey"
            ref="chartCanvas"
            :aria-label="chartAriaLabel"
          />
        </div>
      </div>

      <div v-else-if="viewType !== 'table' && !loading" class="results-empty">
        <h2 class="message lonely-message">
          <i class="material-symbols-outlined">sentiment_dissatisfied</i>
          <span>{{ $t("files.lonely") }}</span>
        </h2>
        <p class="results-empty-hint">{{ $t("files.lonely") }}</p>
      </div>
    </section>
  </div>
</template>

<script>
import {
  ArcElement,
  BarController,
  BarElement,
  CategoryScale,
  Chart,
  Legend,
  LineController,
  LineElement,
  LinearScale,
  PieController,
  PointElement,
  Title,
  Tooltip,
  Filler,
} from "chart.js";
import { toolsApi, usersApi } from "@/api";
import ActivityDetailsInfo from "@/components/tools/ActivityDetailsInfo.vue";
import SettingsTable from "@/components/settings/Table.vue";
import Errors from "@/views/Errors.vue";
import { getters, mutations, state } from "@/store";
import { toStandardLocale } from "@/i18n";
import { buildActivityDetailBadges, activityEventLabel, hasActivityDetails } from "@/utils/activityDetails";
import { formatTimestamp } from "@/utils/moment";
import SharePickerButton from "@/components/tools/SharePickerButton.vue";
import PathPickerButton from "@/components/files/PathPickerButton.vue";
import { globalVars } from "@/utils/constants";
import { getObjectProperty } from "@/utils/object.js";

function queryValuePresent(value) {
  return value !== undefined && value !== null;
}

function queryParamString(query, key) {
  const value = getObjectProperty(query, key);
  return queryValuePresent(value) ? String(value) : "";
}

const FILE_EVENT_TYPES = [
  "download",
  "move",
  "copy",
  "rename",
  "upload",
  "delete",
  "bulkDelete",
  "archive",
  "unarchive",
  "accessUpdate",
];

const SHARE_EVENT_TYPES = [
  "shareCreate",
  "shareUpdate",
  "shareDelete",
];

const AUTH_EVENT_TYPES = [
  "login",
  "logout",
  "signup",
  "passkeyRegister",
  "passkeyDelete",
  "tokenCreate",
  "tokenDelete",
];

const TOOL_EVENT_TYPES = [
  "duplicateFinder",
];

const ADMIN_EVENT_TYPES = [
  "userCreate",
  "userUpdate",
  "userDelete",
];

const EVENT_TYPES = [
  ...FILE_EVENT_TYPES,
  ...SHARE_EVENT_TYPES,
  ...AUTH_EVENT_TYPES,
  ...TOOL_EVENT_TYPES,
  ...ADMIN_EVENT_TYPES,
];

const CHART_COLORS = [
  "#4e79a7",
  "#f28e2b",
  "#e15759",
  "#76b7b2",
  "#59a14f",
  "#edc948",
  "#b07aa1",
  "#ff9da7",
  "#9c755f",
  "#bab0ac",
];

function hexToRgba(hex, alpha) {
  const normalized = hex.replace("#", "");
  const r = parseInt(normalized.slice(0, 2), 16);
  const g = parseInt(normalized.slice(2, 4), 16);
  const b = parseInt(normalized.slice(4, 6), 16);
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}

function truncateChartLabel(text, max = 16) {
  if (!text || text.length <= max) {
    return text || "";
  }
  return `${text.slice(0, max - 1)}…`;
}

const pieSliceLabelsPlugin = {
  id: "activityPieSliceLabels",
  afterDatasetDraw(chart, args) {
    if (chart.config.type !== "pie") {
      return;
    }

    const { ctx } = chart;
    if (!ctx) {
      return;
    }
    const dataset = chart.data.datasets[args.index];
    const labels = chart.data.labels || [];
    const total = dataset.data.reduce((sum, value) => sum + Number(value || 0), 0);
    if (total <= 0) {
      return;
    }

    args.meta.data.forEach((arc, index) => {
      const value = Number(dataset.data.at(index) || 0);
      if (value <= 0) {
        return;
      }

      const percentage = (value / total) * 100;
      if (percentage < 3.5) {
        return;
      }

      const { x, y } = arc.tooltipPosition();
      const label = truncateChartLabel(String(labels.at(index) || ""), 14);
      const valueLine = `${value} (${percentage.toFixed(1)}%)`;

      ctx.save();
      ctx.textAlign = "center";
      ctx.textBaseline = "middle";
      ctx.fillStyle = "#ffffff";
      ctx.shadowColor = "rgba(0, 0, 0, 0.5)";
      ctx.shadowBlur = 4;
      ctx.font = "600 11px system-ui, -apple-system, sans-serif";
      if (label) {
        ctx.fillText(label, x, y - 9);
      }
      ctx.font = "500 10px system-ui, -apple-system, sans-serif";
      ctx.fillText(valueLine, x, label ? y + 8 : y);
      ctx.restore();
    });
  },
};

Chart.register(
  BarController,
  BarElement,
  LineController,
  LineElement,
  PointElement,
  PieController,
  ArcElement,
  CategoryScale,
  LinearScale,
  Title,
  Tooltip,
  Legend,
  Filler,
  pieSliceLabelsPlugin,
);

const TIME_SERIES_VIEWS = new Set(["chart", "line"]);
const VALID_RANGES = new Set(["1h", "24h", "7d", "30d", "custom"]);
const VALID_VIEWS = new Set(["table", "chart", "line", "pie", "summary"]);
const VALID_INTERVALS = new Set(["minute", "hour", "day"]);
const VALID_SPLITS = new Set(["eventType", "user", "none"]);
const VALID_SCOPES = new Set(["all", "files", "shares"]);
const ANONYMOUS_USERNAME = "anonymous";
const ACTIVITY_QUERY_KEYS = [
  "range",
  "scope",
  "from",
  "to",
  "eventType",
  "username",
  "source",
  "path",
  "pathGlob",
  "shareSource",
  "sharePath",
  "shareHash",
  "view",
  "interval",
  "splitBy",
  "page",
];

export default {
  name: "ActivityViewer",
  components: {
    SettingsTable,
    Errors,
    SharePickerButton,
    PathPickerButton,
  },
  data() {
    return {
      loading: false,
      error: null,
      items: [],
      totalEvents: 0,
      statsBuckets: [],
      viewType: "table",
      activityScope: "all",
      timePreset: "24h",
      customFrom: "",
      customTo: "",
      selectedEventType: "",
      selectedUsername: "",
      anonymousUsername: ANONYMOUS_USERNAME,
      filterSource: "",
      filterPath: "",
      filterPathGlob: "",
      filterShareHash: "",
      filePathFilterMode: "picker",
      chartInterval: "hour",
      splitBy: "eventType",
      currentPage: 1,
      totalPages: 1,
      users: [],
      chart: null,
      chartMountKey: 0,
      chartRenderPending: false,
      chartRenderToken: 0,
      isInitializing: true,
      loadRequestId: 0,
      loadDebounceTimer: null,
    };
  },
  computed: {
    visibleEventTypes() {
      if (this.activityScope === "files") return FILE_EVENT_TYPES;
      if (this.activityScope === "shares") return [...SHARE_EVENT_TYPES, "download"];
      return EVENT_TYPES;
    },
    showEventTypeFilter() {
      return this.activityScope === "all";
    },
    showFileFilters() {
      return this.activityScope === "files";
    },
    showShareFilters() {
      return this.activityScope === "shares";
    },
    scopeHint() {
      return this.$t(`tools.activityViewer.scopeHint.${this.activityScope}`);
    },
    isMobile() {
      return state.isMobile;
    },
    isAdmin() {
      return getters.isAdmin();
    },
    sourceNames() {
      return Object.keys(state.sources?.info || {}).sort();
    },
    showChartOptions() {
      return this.viewType !== "table";
    },
    showTimeSeriesOptions() {
      return TIME_SERIES_VIEWS.has(this.viewType);
    },
    effectiveInterval() {
      if (!this.showTimeSeriesOptions) {
        return "none";
      }
      return this.chartInterval;
    },
    tableColumns() {
      return [
        { key: "createdAt", label: this.$t("general.time"), sortable: false },
        { key: "username", label: this.$t("general.username"), sortable: false },
        { key: "eventType", label: this.$t("tools.activityViewer.eventType"), sortable: false },
        { key: "details", label: this.$t("general.details"), sortable: false },
        { key: "ipAddress", label: this.$t("general.ipAddress"), sortable: false },
        { key: "status", label: this.$t("general.status"), sortable: false, narrow: true },
      ];
    },
    queryRange() {
      const now = Math.floor(Date.now() / 1000);
      if (this.timePreset === "1h") {
        return { from: now - 3600, to: now };
      }
      if (this.timePreset === "24h") {
        return { from: now - 86400, to: now };
      }
      if (this.timePreset === "7d") {
        return { from: now - 7 * 86400, to: now };
      }
      if (this.timePreset === "30d") {
        return { from: now - 30 * 86400, to: now };
      }
      const fromRaw = this.customFrom ? Math.floor(new Date(this.customFrom).getTime() / 1000) : now - 86400;
      const toRaw = this.customTo ? Math.floor(new Date(this.customTo).getTime() / 1000) : now;
      const from = Number.isFinite(fromRaw) ? fromRaw : now - 86400;
      const to = Number.isFinite(toRaw) ? toRaw : now;
      return { from, to };
    },
    filterParams() {
      const params = {
        ...this.queryRange,
        scope: this.activityScope,
        limit: 500,
        page: this.currentPage,
      };
      if (this.selectedEventType) {
        params.eventType = this.selectedEventType;
      }
      if (this.isAdmin && this.selectedUsername) {
        params.username = this.selectedUsername;
      }
      if (this.showFileFilters) {
        if (this.filePathFilterMode === "picker") {
          if (this.filterSource) {
            params.source = this.filterSource;
          }
          if (this.filterPath && this.filterPath !== "/") {
            params.path = this.filterPath;
          }
        } else if (this.filterSource) {
          params.source = this.filterSource;
          if (this.filterPathGlob) {
            params.pathGlob = this.filterPathGlob;
          }
        }
      }
      if (this.showShareFilters && this.filterShareHash) {
        params.shareHash = this.filterShareHash;
      }
      if (this.viewType !== "table") {
        params.interval = this.effectiveInterval;
        params.splitBy = this.splitBy;
      }
      return params;
    },
    hasResults() {
      return this.viewType === "table" ? this.items.length > 0 : this.hasChartData;
    },
    hasChartData() {
      return (this.statsBuckets || []).some((b) => b.count > 0);
    },
    showChartPanel() {
      return this.viewType !== "table" && !this.loading && this.hasChartData;
    },
    totalEventCount() {
      return (this.statsBuckets || []).reduce((sum, b) => sum + b.count, 0);
    },
    chartAriaLabel() {
      return this.$t(`tools.activityViewer.${this.viewType}View`);
    },
    chartTitleKey() {
      if (this.viewType === "pie") {
        return this.splitBy === "user"
          ? "tools.activityViewer.pieTitleByUser"
          : "tools.activityViewer.pieTitle";
      }
      if (this.viewType === "summary") {
        return this.splitBy === "user"
          ? "tools.activityViewer.summaryTitleByUser"
          : "tools.activityViewer.summaryTitle";
      }
      if (this.splitBy === "user") {
        return "tools.activityViewer.chartTitleByUser";
      }
      if (this.splitBy === "none") {
        return "tools.activityViewer.chartTitleTotal";
      }
      return "tools.activityViewer.chartTitle";
    },
  },
  watch: {
    "$route.query": {
      handler(newQuery, oldQuery) {
        if (this.isInitializing) {
          return;
        }
        if (!this.routeQueryChanged(newQuery, oldQuery)) {
          return;
        }
        void this.applyRouteQuery().then(() => {
          this.updateUrl();
        });
      },
      deep: true,
    },
    activityScope() {
      if (!this.isInitializing) {
        this.updateUrl();
      }
    },
    viewType() {
      this.destroyChart();
      if (this.viewType === "table") {
        if (!this.isInitializing && this.items.length === 0) {
          this.loadData();
        }
      } else if (!TIME_SERIES_VIEWS.has(this.viewType) && this.splitBy === "none") {
        this.splitBy = "eventType";
      }
      if (!this.isInitializing) {
        this.updateUrl();
        if (this.viewType !== "table" && this.hasChartData) {
          this.scheduleChartRender();
        } else if (this.viewType !== "table") {
          this.loadData();
        }
      }
    },
    chartInterval() {
      if (!this.isInitializing && this.viewType !== "table") {
        this.updateUrl();
        this.loadData();
      }
    },
    splitBy() {
      if (!this.isInitializing && this.viewType !== "table") {
        this.updateUrl();
        this.loadData();
      }
    },
    timePreset() {
      this.clampChartInterval();
      if (!this.isInitializing) {
        this.updateUrl();
      }
    },
    customFrom() {
      if (!this.isInitializing && this.timePreset === "custom") {
        this.resetPageAndLoad();
        this.updateUrl();
      }
    },
    customTo() {
      if (!this.isInitializing && this.timePreset === "custom") {
        this.resetPageAndLoad();
        this.updateUrl();
      }
    },
    selectedEventType() {
      if (!this.isInitializing) {
        this.updateUrl();
        this.resetPageAndLoad();
      }
    },
    selectedUsername() {
      if (!this.isInitializing) {
        this.updateUrl();
        this.resetPageAndLoad();
      }
    },
    filterSource() {
      if (!this.isInitializing) {
        this.updateUrl();
        this.resetPageAndLoad();
      }
    },
    filterPath() {
      if (!this.isInitializing) {
        this.updateUrl();
        this.resetPageAndLoad();
      }
    },
    filterPathGlob() {
      if (!this.isInitializing) {
        this.updateUrl();
        this.debouncedResetPageAndLoad();
      }
    },
    filterShareHash() {
      if (!this.isInitializing) {
        this.updateUrl();
        this.resetPageAndLoad();
      }
    },
    filePathFilterMode() {
      if (!this.isInitializing) {
        this.filterPath = "";
        this.filterPathGlob = "";
        this.updateUrl();
      }
    },
  },
  async mounted() {
    document.title = `${globalVars.name} - ${this.$t("tools.title")} - ${this.$t("tools.activityViewer.name")}`;
    void this.fetchAdminUsers();
    await this.applyRouteQuery();
    this.isInitializing = false;
    this.updateUrl();
  },
  beforeUnmount() {
    clearTimeout(this.loadDebounceTimer);
    this.destroyChart();
  },
  methods: {
    formatTime(ts) {
      return formatTimestamp(ts * 1000);
    },
    destroyChartInstance() {
      const instance = this.chart;
      this.chart = null;
      if (!instance) {
        return;
      }
      try {
        instance.stop();
      } catch {
        // Chart may already be torn down.
      }
      try {
        instance.destroy();
      } catch {
        // Ignore destroy errors during rapid view switches.
      }
    },
    destroyChart() {
      this.chartRenderToken += 1;
      this.destroyChartInstance();
      this.chartMountKey += 1;
    },
    scheduleChartRender() {
      if (this.chartRenderPending) {
        return;
      }
      this.chartRenderPending = true;
      const token = ++this.chartRenderToken;
      this.$nextTick(() => {
        requestAnimationFrame(() => {
          this.chartRenderPending = false;
          if (!this.isCurrentChartRenderToken(token)) {
            return;
          }
          this.renderChart();
        });
      });
    },
    isCurrentChartRenderToken(token) {
      return this.chartRenderToken === token;
    },
    onScopeChange() {
      if (!this.visibleEventTypes.includes(this.selectedEventType)) {
        this.selectedEventType = "";
      }
      if (this.activityScope === "files" || this.activityScope === "all") {
        this.filterShareHash = "";
      }
      if (this.activityScope === "shares") {
        this.filterSource = "";
        this.filterPath = "";
        this.filterPathGlob = "";
      }
      this.resetPageAndLoad();
      if (!this.isInitializing) {
        this.updateUrl();
      }
    },
    onTimePresetChange() {
      if (this.timePreset === "1h" && TIME_SERIES_VIEWS.has(this.viewType)) {
        this.chartInterval = "minute";
      }
      this.clampChartInterval();
      if (this.timePreset !== "custom") {
        this.resetPageAndLoad();
        if (!this.isInitializing) {
          this.updateUrl();
        }
      }
    },
    clampChartInterval() {
      const range = this.queryRange.to - this.queryRange.from;
      if (this.chartInterval === "minute" && range > 2 * 86400) {
        this.chartInterval = "hour";
      }
      if (this.chartInterval === "hour" && range > 90 * 86400) {
        this.chartInterval = "day";
      }
    },
    resetPageAndLoad() {
      this.currentPage = 1;
      void this.loadData();
    },
    debouncedResetPageAndLoad() {
      clearTimeout(this.loadDebounceTimer);
      this.loadDebounceTimer = setTimeout(() => {
        this.resetPageAndLoad();
      }, 350);
    },
    goToPage(page) {
      if (page < 1 || page > this.totalPages) return;
      this.currentPage = page;
      void this.loadData();
      this.updateUrl();
    },
    routeQueryChanged(newQuery = {}, oldQuery = {}) {
      return ACTIVITY_QUERY_KEYS.some((key) => {
        return queryParamString(newQuery, key) !== queryParamString(oldQuery, key);
      });
    },
    inferActivityScopeFromQuery(query) {
      if (queryValuePresent(query.shareSource)
        || queryValuePresent(query.sharePath)
        || queryValuePresent(query.shareHash)) {
        return "shares";
      }
      if (queryValuePresent(query.source)
        || queryValuePresent(query.path)
        || queryValuePresent(query.pathGlob)) {
        return "files";
      }
      return null;
    },
    async applyRouteQuery() {
      this.initializeFromQuery();
      this.clampChartInterval();
      await this.loadData();
    },
    fetchAdminUsers() {
      if (!this.isAdmin) {
        return Promise.resolve();
      }
      return usersApi.getAllUsers()
        .then((users) => {
          this.users = users;
        })
        .catch(() => {
          this.users = [];
        });
    },
    initializeFromQuery() {
      const query = this.$route.query;

      if (queryValuePresent(query.range) && VALID_RANGES.has(String(query.range))) {
        this.timePreset = String(query.range);
      }
      if (queryValuePresent(query.scope) && VALID_SCOPES.has(String(query.scope))) {
        this.activityScope = String(query.scope);
      } else {
        const inferredScope = this.inferActivityScopeFromQuery(query);
        if (inferredScope) {
          this.activityScope = inferredScope;
        }
      }
      if (queryValuePresent(query.from)) {
        this.customFrom = String(query.from);
      }
      if (queryValuePresent(query.to)) {
        this.customTo = String(query.to);
      }
      if (queryValuePresent(query.eventType) && this.visibleEventTypes.includes(String(query.eventType))) {
        this.selectedEventType = String(query.eventType);
      } else if (query.eventType === "" || query.eventType === null) {
        this.selectedEventType = "";
      }
      if (this.isAdmin && queryValuePresent(query.username)) {
        this.selectedUsername = String(query.username);
      } else if (!this.isAdmin) {
        this.selectedUsername = "";
      }
      const scopeForFilters = this.activityScope;
      if (queryValuePresent(query.source)) {
        if (scopeForFilters !== "shares") {
          this.filterSource = String(query.source);
        }
      }
      if (queryValuePresent(query.path)) {
        if (scopeForFilters !== "shares") {
          this.filterPath = String(query.path);
        }
      }
      if (queryValuePresent(query.pathGlob)) {
        this.filterPathGlob = String(query.pathGlob);
        this.filePathFilterMode = "glob";
      }
      if (queryValuePresent(query.shareHash)) {
        this.filterShareHash = String(query.shareHash);
      }
      if (queryValuePresent(query.view) && VALID_VIEWS.has(String(query.view))) {
        this.viewType = String(query.view);
      }
      if (queryValuePresent(query.interval) && VALID_INTERVALS.has(String(query.interval))) {
        this.chartInterval = String(query.interval);
      }
      if (queryValuePresent(query.splitBy) && VALID_SPLITS.has(String(query.splitBy))) {
        this.splitBy = String(query.splitBy);
      }
      if (queryValuePresent(query.page)) {
        const page = parseInt(String(query.page), 10);
        if (!Number.isNaN(page) && page >= 1) {
          this.currentPage = page;
        }
      }
    },
    updateUrl() {
      if (!this.$route.path.startsWith("/tools/activityViewer")) return;

      this.$nextTick(() => {
        const query = {};

        if (this.timePreset !== "24h") {
          query.range = this.timePreset;
        }
        if (this.activityScope !== "all") {
          query.scope = this.activityScope;
        }
        if (this.timePreset === "custom") {
          if (this.customFrom) query.from = this.customFrom;
          if (this.customTo) query.to = this.customTo;
        }
        if (this.selectedEventType) {
          query.eventType = this.selectedEventType;
        }
        if (this.isAdmin && this.selectedUsername) {
          query.username = this.selectedUsername;
        }
        if (this.showFileFilters) {
          if (this.filePathFilterMode === "picker") {
            if (this.filterSource) {
              query.source = this.filterSource;
            }
            if (this.filterPath && this.filterPath !== "/") {
              query.path = this.filterPath;
            }
          } else {
            if (this.filterSource) {
              query.source = this.filterSource;
            }
            if (this.filterPathGlob) {
              query.pathGlob = this.filterPathGlob;
            }
          }
        }
        if (this.showShareFilters && this.filterShareHash) {
          query.shareHash = this.filterShareHash;
        }
        if (this.viewType !== "table") {
          query.view = this.viewType;
        }
        if (this.viewType !== "table" && this.chartInterval !== "hour") {
          query.interval = this.chartInterval;
        }
        if (this.viewType !== "table" && this.splitBy !== "eventType") {
          query.splitBy = this.splitBy;
        }
        if (this.viewType === "table" && this.currentPage > 1) {
          query.page = String(this.currentPage);
        }

        const newQueryString = new URLSearchParams(query).toString();
        const currentQuery = this.$route.query || {};
        const filteredEntries = Object.entries(currentQuery)
          .filter(([_, value]) => value !== null && value !== undefined)
          .map(([key, value]) => [key, String(value)]);
        const currentQueryString = new URLSearchParams(Object.fromEntries(filteredEntries)).toString();

        if (newQueryString !== currentQueryString) {
          this.$router.replace({
            path: this.$route.path,
            query: Object.keys(query).length > 0 ? query : undefined,
          }).catch(() => {});
        }
      });
    },
    openEventDetails(row) {
      mutations.showPrompt({
        name: "ActivityEventDetails",
        props: { row },
      });
    },
    detailBadges(row) {
      return buildActivityDetailBadges(row, this.$t);
    },
    hasRowDetails(row) {
      return hasActivityDetails(row);
    },
    showDetailsTooltip(event, row) {
      mutations.showTooltip({
        component: ActivityDetailsInfo,
        componentProps: {
          row,
          eventLabel: this.eventTypeLabel(row.eventType),
        },
        x: event.clientX,
        y: event.clientY,
        width: "22rem",
      });
    },
    hideDetailsTooltip() {
      mutations.hideTooltip();
    },
    chartTheme() {
      const root = getComputedStyle(document.documentElement);
      const text = root.getPropertyValue("--textPrimary").trim()
        || root.getPropertyValue("--textSecondary").trim()
        || "#e2e8f0";
      const textMuted = root.getPropertyValue("--textSecondary").trim() || "#94a3b8";
      const divider = root.getPropertyValue("--divider").trim() || "rgba(148, 163, 184, 0.22)";
      const surface = root.getPropertyValue("--surface1").trim()
        || root.getPropertyValue("--surface").trim()
        || "#1e293b";
      const mutedHex = textMuted.startsWith("#") ? textMuted : "#94a3b8";
      return {
        text,
        textMuted,
        divider,
        surface,
        gridLine: hexToRgba(mutedHex, 0.1),
        tooltipBg: surface,
      };
    },
    chartPluginOptions(theme, { title = "", showLegend = true } = {}) {
      return {
        legend: showLegend
          ? {
              position: "bottom",
              labels: {
                boxWidth: 12,
                boxHeight: 12,
                padding: 18,
                color: theme.textMuted,
                font: { size: 12, weight: "500" },
                usePointStyle: true,
                pointStyle: "circle",
              },
            }
          : { display: false },
        title: {
          display: Boolean(title),
          text: title,
          color: theme.text,
          font: { size: 15, weight: "600" },
          padding: { top: 4, bottom: 18 },
          align: "start",
        },
        tooltip: {
          backgroundColor: theme.tooltipBg,
          titleColor: theme.text,
          bodyColor: theme.textMuted,
          borderColor: theme.divider,
          borderWidth: 1,
          padding: 12,
          cornerRadius: 10,
          displayColors: true,
        },
      };
    },
    axisScaleOptions(theme, { stacked = false, beginAtZero = true } = {}) {
      return {
        x: {
          stacked,
          border: { display: false },
          grid: { display: false },
          ticks: {
            color: theme.textMuted,
            font: { size: 11 },
            maxRotation: 45,
            padding: 8,
          },
        },
        y: {
          stacked,
          beginAtZero,
          border: { display: false },
          grid: {
            color: theme.gridLine,
            drawBorder: false,
            lineWidth: 1,
          },
          ticks: {
            precision: 0,
            color: theme.textMuted,
            font: { size: 11 },
            padding: 8,
          },
        },
      };
    },
    createLineFillGradient(ctx, color, height) {
      const gradient = ctx.createLinearGradient(0, 0, 0, height || 400);
      gradient.addColorStop(0, hexToRgba(color, 0.28));
      gradient.addColorStop(1, hexToRgba(color, 0.02));
      return gradient;
    },
    statusBadgeClass(code) {
      if (!code) return "status-badge--muted";
      if (code >= 500) return "status-badge--5xx";
      if (code >= 400) return "status-badge--4xx";
      if (code >= 300) return "status-badge--3xx";
      return "status-badge--2xx";
    },
    async loadData() {
      const requestId = ++this.loadRequestId;
      this.chartRenderToken += 1;
      this.destroyChartInstance();
      this.loading = true;
      this.error = null;
      try {
        if (this.viewType === "table") {
          const listRes = await toolsApi.activityList(this.filterParams);
          if (requestId !== this.loadRequestId) return;
          this.items = listRes.items || [];
          this.totalEvents = listRes.total || 0;
          this.totalPages = listRes.totalPages || 1;
          this.currentPage = listRes.page || this.currentPage;
          this.statsBuckets = [];
        } else {
          const statsRes = await toolsApi.activityGrouped(this.filterParams);
          if (requestId !== this.loadRequestId) return;
          this.statsBuckets = statsRes.buckets || [];
          this.items = [];
          this.totalEvents = 0;
        }
      } catch (e) {
        if (requestId !== this.loadRequestId) return;
        this.error = e;
        if (this.viewType !== "table") {
          this.statsBuckets = [];
        }
      } finally {
        if (requestId === this.loadRequestId) {
          this.loading = false;
          if (this.showChartPanel) {
            this.scheduleChartRender();
          }
        }
      }
    },
    exportCsv() {
      const params = { ...this.filterParams };
      delete params.interval;
      delete params.splitBy;
      delete params.groupBy;
      const url = toolsApi.activityExportUrl(params);
      const a = document.createElement("a");
      a.href = url;
      a.download = "";
      a.rel = "noopener noreferrer";
      document.body.appendChild(a);
      a.click();
      a.remove();
    },
    chartLocale() {
      return toStandardLocale(this.$i18n.locale);
    },
    formatAxisLabelForSpan(date, spanSec, interval, locale) {
      if (spanSec <= 3600) {
        return new Intl.DateTimeFormat(locale, {
          hour: "numeric",
          minute: "2-digit",
        }).format(date);
      }
      if (spanSec <= 2 * 86400) {
        if (interval === "day") {
          return new Intl.DateTimeFormat(locale, {
            month: "short",
            day: "numeric",
          }).format(date);
        }
        return new Intl.DateTimeFormat(locale, {
          month: "short",
          day: "numeric",
          hour: "numeric",
        }).format(date);
      }
      if (spanSec <= 14 * 86400) {
        return new Intl.DateTimeFormat(locale, {
          weekday: "short",
          month: "short",
          day: "numeric",
        }).format(date);
      }
      return new Intl.DateTimeFormat(locale, {
        month: "short",
        day: "numeric",
        year: "numeric",
      }).format(date);
    },
    colorForIndex(idx) {
      return CHART_COLORS[idx % CHART_COLORS.length];
    },
    bucketSeriesKey(bucket) {
      return bucket.seriesKey || bucket.eventType || "total";
    },
    seriesDisplayLabel(seriesKey, buckets) {
      if (this.splitBy === "user") {
        const match = buckets.find((b) => this.bucketSeriesKey(b) === seriesKey);
        return match?.seriesLabel || seriesKey;
      }
      if (seriesKey === "total") {
        return this.$t("tools.activityViewer.totalEvents");
      }
      return this.eventTypeLabel(seriesKey);
    },
    formatBucketLabel(bucketTs) {
      const n = Number(bucketTs);
      if (n === 0) {
        return this.$t("tools.activityViewer.ungrouped");
      }

      const date = new Date(n * 1000);
      const locale = this.chartLocale();
      const interval = this.effectiveInterval;

      if (this.timePreset === "custom") {
        const span = this.queryRange.to - this.queryRange.from;
        return this.formatAxisLabelForSpan(date, span, interval, locale);
      }

      if (this.timePreset === "1h") {
        return new Intl.DateTimeFormat(locale, {
          hour: "numeric",
          minute: "2-digit",
        }).format(date);
      }

      if (this.timePreset === "24h") {
        if (interval === "day") {
          return new Intl.DateTimeFormat(locale, {
            month: "short",
            day: "numeric",
          }).format(date);
        }
        if (interval === "minute") {
          return new Intl.DateTimeFormat(locale, {
            hour: "numeric",
            minute: "2-digit",
          }).format(date);
        }
        return new Intl.DateTimeFormat(locale, {
          hour: "numeric",
        }).format(date);
      }

      if (this.timePreset === "7d") {
        if (interval === "hour") {
          return new Intl.DateTimeFormat(locale, {
            weekday: "short",
            hour: "numeric",
          }).format(date);
        }
        return new Intl.DateTimeFormat(locale, {
          weekday: "short",
          month: "short",
          day: "numeric",
        }).format(date);
      }

      if (this.timePreset === "30d") {
        if (interval === "hour") {
          return new Intl.DateTimeFormat(locale, {
            month: "short",
            day: "numeric",
            hour: "numeric",
          }).format(date);
        }
        return new Intl.DateTimeFormat(locale, {
          month: "short",
          day: "numeric",
        }).format(date);
      }

      return this.formatTime(n);
    },
    eventTypeLabel(eventType) {
      return activityEventLabel(eventType, this.$t);
    },
    buildTimeSeriesChart(canvas) {
      const buckets = this.statsBuckets || [];
      const labels = [...new Set(buckets.map((b) => String(b.bucket)))].sort(
        (a, b) => Number(a) - Number(b),
      );
      const seriesKeys = [...new Set(buckets.map((b) => this.bucketSeriesKey(b)))];
      const isLine = this.viewType === "line";
      const chartLabels = labels.map((l) => this.formatBucketLabel(l));
      const theme = this.chartTheme();
      const ctx = canvas.getContext("2d");
      const chartHeight = canvas.parentElement?.clientHeight || 400;

      const datasets = seriesKeys.map((seriesKey, idx) => {
        const color = this.colorForIndex(idx);
        const data = labels.map((label) => {
          const match = buckets.find(
            (b) => String(b.bucket) === label && this.bucketSeriesKey(b) === seriesKey,
          );
          return match ? match.count : 0;
        });
        const label = this.seriesDisplayLabel(seriesKey, buckets);
        return isLine
          ? {
              label,
              data,
              borderColor: color,
              backgroundColor: ctx
                ? this.createLineFillGradient(ctx, color, chartHeight)
                : hexToRgba(color, 0.15),
              borderWidth: 2.5,
              pointBackgroundColor: color,
              pointBorderColor: theme.surface,
              pointBorderWidth: 2,
              pointRadius: 4,
              pointHoverRadius: 6,
              tension: 0.35,
              fill: true,
            }
          : {
              label,
              data,
              backgroundColor: hexToRgba(color, 0.88),
              borderColor: color,
              borderWidth: 1,
              borderRadius: 6,
              borderSkipped: false,
              stack: this.splitBy === "none" ? undefined : "activity",
            };
      });

      const stacked = this.splitBy !== "none" && !isLine;
      const chartTitle = this.$t(this.chartTitleKey);
      return {
        type: isLine ? "line" : "bar",
        data: { labels: chartLabels, datasets },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          interaction: { mode: "index", intersect: false },
          plugins: this.chartPluginOptions(theme, { title: chartTitle }),
          datasets: {
            bar: {
              barPercentage: 0.72,
              categoryPercentage: 0.82,
            },
          },
          scales: this.axisScaleOptions(theme, { stacked }),
        },
      };
    },
    buildDimensionTotals() {
      const totals = new Map();
      for (const b of this.statsBuckets || []) {
        const key = this.bucketSeriesKey(b);
        const label = this.seriesDisplayLabel(key, this.statsBuckets);
        const prev = totals.get(key);
        totals.set(key, {
          label,
          count: (prev?.count || 0) + b.count,
        });
      }
      return [...totals.entries()]
        .map(([key, { label, count }]) => ({ key, label, count }))
        .sort((a, b) => b.count - a.count);
    },
    buildPieChart(_canvas) {
      const totals = this.buildDimensionTotals();
      const theme = this.chartTheme();
      const chartTitle = this.$t(this.chartTitleKey);
      return {
        type: "pie",
        data: {
          labels: totals.map((t) => t.label),
          datasets: [{
            data: totals.map((t) => t.count),
            backgroundColor: totals.map((_, idx) => hexToRgba(this.colorForIndex(idx), 0.9)),
            borderColor: theme.surface,
            borderWidth: 3,
            hoverOffset: 10,
          }],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          layout: { padding: { top: 8, bottom: 4 } },
          plugins: this.chartPluginOptions(theme, { title: chartTitle }),
        },
      };
    },
    buildSummaryChart(_canvas) {
      const totals = this.buildDimensionTotals();
      const theme = this.chartTheme();
      const chartTitle = this.$t(this.chartTitleKey);
      return {
        type: "bar",
        data: {
          labels: totals.map((t) => t.label),
          datasets: [{
            label: this.$t("tools.activityViewer.totalEvents"),
            data: totals.map((t) => t.count),
            backgroundColor: totals.map((_, idx) => hexToRgba(this.colorForIndex(idx), 0.88)),
            borderColor: totals.map((_, idx) => this.colorForIndex(idx)),
            borderWidth: 1,
            borderRadius: 6,
            borderSkipped: false,
          }],
        },
        options: {
          indexAxis: "y",
          responsive: true,
          maintainAspectRatio: false,
          plugins: this.chartPluginOptions(theme, { title: chartTitle, showLegend: false }),
          datasets: {
            bar: {
              barPercentage: 0.65,
              categoryPercentage: 0.85,
            },
          },
          scales: {
            x: {
              beginAtZero: true,
              border: { display: false },
              grid: {
                color: theme.gridLine,
                drawBorder: false,
                lineWidth: 1,
              },
              ticks: { precision: 0, color: theme.textMuted, font: { size: 11 }, padding: 8 },
            },
            y: {
              border: { display: false },
              grid: { display: false },
              ticks: { color: theme.textMuted, font: { size: 11 }, padding: 8 },
            },
          },
        },
      };
    },
    renderChart(retryCount = 0) {
      const renderToken = this.chartRenderToken;
      if (this.loading || this.viewType === "table" || !this.hasChartData) {
        this.destroyChartInstance();
        return;
      }

      const canvas = this.$refs.chartCanvas;
      if (!canvas || typeof canvas.getContext !== "function" || !canvas.isConnected) {
        if (retryCount < 5 && this.isCurrentChartRenderToken(renderToken)) {
          this.$nextTick(() => {
            requestAnimationFrame(() => {
              if (this.isCurrentChartRenderToken(renderToken)) {
                this.renderChart(retryCount + 1);
              }
            });
          });
        }
        return;
      }

      const ctx = canvas.getContext("2d");
      if (!ctx) {
        return;
      }

      this.destroyChartInstance();

      let config;
      if (this.viewType === "pie") {
        config = this.buildPieChart(canvas);
      } else if (this.viewType === "summary") {
        config = this.buildSummaryChart(canvas);
      } else {
        config = this.buildTimeSeriesChart(canvas);
      }

      if (!config?.data?.datasets?.length) {
        return;
      }
      if (!this.isCurrentChartRenderToken(renderToken)) {
        return;
      }

      this.chart = new Chart(canvas, config);
    },
  },
};
</script>

<style scoped>
.activity-viewer {
  max-width: 1200px;
  margin-left: auto;
  margin-right: auto;
  padding: 1em;
}

.activity-scope-card {
  margin-bottom: 1rem;
}

.universal-filters {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(12rem, 1fr));
  gap: 1rem;
  align-items: end;
}

.universal-filters.mobile {
  grid-template-columns: 1fr;
}

.filter-field {
  min-width: 0;
}

.filter-label {
  display: block;
  margin-bottom: 0.5rem;
  font-size: 0.95rem;
  font-weight: 600;
}

.activity-scope-select {
  width: 100%;
  font-size: 1rem;
  padding: 0.65rem 0.85rem;
}

.activity-scope-hint {
  margin: 0.85rem 0 0;
  font-size: 0.9rem;
  color: var(--textSecondary);
}

.path-filters h3 {
  margin: 0 0 0.5rem;
  font-size: 0.95rem;
  font-weight: 600;
}

.path-filter-mode {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  margin-bottom: 0.75rem;
}

.mode-option {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.9rem;
  cursor: pointer;
}

.path-filter-picker :deep(.unified-path-picker) {
  max-width: 100%;
}

.glob-fields {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.glob-field {
  flex: 1;
  min-width: 10rem;
}

.glob-field-wide {
  flex: 2;
  min-width: 14rem;
}

.share-hash-field {
  margin-top: 0.75rem;
}

.filter-field-wide {
  grid-column: 1 / -1;
}

.activity-viewer-results {
  margin-top: 1.25rem;
  margin-bottom: 2em;
}

.results-stats {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.75rem 2rem;
  margin-bottom: 0.75rem;
  font-size: 0.95rem;
  color: var(--textSecondary);
}

.results-chart {
  margin-top: 0.25rem;
}

.results-empty {
  text-align: center;
  padding: 3rem 1.5rem 4rem;
}

.results-empty-hint {
  margin: 0;
  font-size: 0.95rem;
  color: var(--textSecondary);
}

.results-empty .lonely-message {
  margin-bottom: 0.5rem;
}

.config-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(12rem, 1fr));
  gap: 1rem;
  align-items: end;
}

.config-grid.mobile {
  grid-template-columns: 1fr;
}

.config-field h3 {
  margin: 0 0 0.5rem;
  font-size: 0.95rem;
  font-weight: 600;
}

.config-field-wide {
  grid-column: 1 / -1;
}

.custom-range {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.custom-range .input {
  flex: 1;
  min-width: 10rem;
}

.config-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
  grid-column: 1 / -1;
  margin-top: 0.5rem;
}

.button {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.button .material-symbols {
  font-size: 1.2rem;
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  margin-top: 1rem;
}

.page-label {
  opacity: 0.85;
}

.details-cell-wrap {
  max-width: 18rem;
}

.details-badges {
  display: flex;
  flex-wrap: nowrap;
  gap: 0.35rem;
  overflow: hidden;
  max-width: 100%;
}

.detail-badge {
  display: inline-block;
  flex-shrink: 1;
  min-width: 0;
  max-width: 10rem;
  padding: 0.12rem 0.5rem;
  border: 1px solid var(--divider);
  font-size: 0.8em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--textPrimary, inherit);
  background: transparent;
}

.details-restricted,
.details-muted {
  font-size: 0.9em;
  color: var(--textSecondary);
}

.status-badge {
  display: inline-block;
  min-width: 2.5rem;
  padding: 0.15rem 0.45rem;
  font-size: 0.85em;
  font-weight: 600;
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.status-badge--2xx {
  background: rgba(76, 175, 80, 0.18);
  color: #2e7d32;
}

.status-badge--3xx {
  background: rgba(33, 150, 243, 0.18);
  color: #1565c0;
}

.status-badge--4xx {
  background: rgba(255, 152, 0, 0.22);
  color: #e65100;
}

.status-badge--5xx {
  background: rgba(244, 67, 54, 0.2);
  color: #c62828;
}

.status-badge--muted {
  background: rgba(128, 128, 128, 0.12);
  color: var(--textSecondary);
}

.admin-filters {
  grid-column: 1 / -1;
}

.admin-filters-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(11rem, 1fr));
  gap: 0.75rem;
}

.filter-label {
  display: block;
  margin-bottom: 0.35rem;
  font-size: 0.85rem;
  color: var(--textSecondary);
}

.chart-panel {
  position: relative;
  width: 100%;
  height: 440px;
  padding: 1.25rem 1.5rem 1rem;
  border: 1px solid var(--divider);
  background: linear-gradient(
    165deg,
    var(--surface1, rgba(255, 255, 255, 0.03)) 0%,
    var(--surface2, transparent) 100%
  );
  box-shadow:
    0 1px 2px rgba(0, 0, 0, 0.04),
    0 6px 20px rgba(0, 0, 0, 0.06);
  overflow: hidden;
}

.chart-panel canvas {
  width: 100% !important;
  height: 100% !important;
}

.loading-state {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 200px;
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@media (max-width: 768px) {
  .activity-viewer {
    padding: 1rem;
  }

  .chart-panel {
    height: 320px;
    padding: 1rem;
  }

  .results-stats {
    flex-direction: column;
    gap: 0.5rem;
  }
}
</style>
