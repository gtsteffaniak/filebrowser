<template>
  <div class="card-content prompt-panel">
    <div ref="spotlights" class="spotlights">
      <div
        v-for="card in spotlights"
        :key="card.key"
        class="spotlight"
        :class="{ 'spotlight-all': card.key === 'all' }"
      >
        <div class="spotlight-title">{{ card.title }}</div>
        <div class="spotlight-grid">
          <div class="metric" :class="`tone-${card.successTone}`">
            <span class="metric-label">{{ $t("media.shadowRegressionsSucceeded") }}</span>
            <span class="metric-value">{{ card.succeeded }} / {{ card.runs }}</span>
            <span class="metric-sub">{{ card.successPct }}</span>
          </div>
          <div class="metric" :class="`tone-${card.regressionTone}`">
            <span class="metric-label">{{ $t("media.shadowRegressionsRegressions") }}</span>
            <span class="metric-value">{{ card.regressions }}</span>
            <span class="metric-sub">{{ card.regressionPct }}</span>
          </div>
          <div class="metric metric-wide" :class="`tone-${card.timingTone}`">
            <span class="metric-label">{{ $t("media.shadowRegressionsAverageTime") }}</span>
            <span class="metric-value">
              {{ $t("media.shadowRegressionsGoMedia") }} {{ card.goAvg }}
              <span class="metric-vs">{{ $t("media.shadowRegressionsVersus") }}</span>
              {{ $t("media.shadowRegressionsFfmpeg") }} {{ card.ffAvg }}
            </span>
            <span class="metric-sub">{{ card.timingRatio }}</span>
          </div>
        </div>
      </div>
    </div>
    <div class="incompatible" :style="incompatibleStyle">
      <div class="incompatible-title">{{ $t("media.shadowIncompatibleTitle") }}</div>
      <div v-if="!incompatible.length" class="incompatible-empty">
        {{ $t("media.shadowIncompatibleEmpty") }}
      </div>
      <table v-else class="incompatible-table">
        <thead>
          <tr>
            <th>{{ $t("media.shadowIncompatibleFile") }}</th>
            <th>{{ $t("media.shadowIncompatibleOperation") }}</th>
            <th>{{ $t("media.shadowIncompatibleFormat") }}</th>
            <th class="num">{{ $t("media.shadowIncompatibleSeen") }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in incompatible" :key="row.path">
            <td class="path" :title="row.path">{{ row.path }}</td>
            <td>{{ row.operation }}</td>
            <td>{{ row.format }}</td>
            <td class="num">{{ row.occurrences }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="editor-container">
      <Editor
        :viewer-mode="true"
        :content="regressionsContent"
        editor-mode="json"
        :read-only="true"
      />
    </div>
  </div>
  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--red"
      :disabled="clearing"
      :aria-label="$t('notifications.clearAll')"
      :title="$t('notifications.clearAll')"
      @click="clearAll"
    >
      {{ $t("notifications.clearAll") }}
    </button>
  </div>
</template>

<script>
import { markRaw } from "vue";
import i18n from "@/i18n/index.ts";
import { createAsyncComponent } from "@/utils/asyncComponent.js";
import { notify } from "@/notify/index.ts";
import {
  getShadowRegressions,
  getShadowRegressionStats,
  getShadowIncompatible,
  clearShadowRegressions,
} from "@/api/settings.js";

// Thresholds for the spotlight colours. Timing uses the same 2x ratio as the
// backend's timeRegressionRatio, so a red timing card matches a recorded regression.
const SUCCESS_GOOD = 0.95;
const SUCCESS_WARN = 0.8;
const REGRESSION_WARN = 0.1;
const TIMING_GOOD = 1;
const TIMING_BAD = 2;

function successTone(rate) {
  if (rate >= SUCCESS_GOOD) return "good";
  if (rate >= SUCCESS_WARN) return "warn";
  return "bad";
}

function regressionTone(rate) {
  if (rate === 0) return "good";
  if (rate <= REGRESSION_WARN) return "warn";
  return "bad";
}

function timingTone(ratio) {
  if (ratio === null) return "none";
  if (ratio <= TIMING_GOOD) return "good";
  if (ratio <= TIMING_BAD) return "warn";
  return "bad";
}

function pct(part, whole) {
  if (!whole) return "—";
  return `${Math.round((part / whole) * 100)}%`;
}

function formatMs(value) {
  if (!value) return "—";
  return value < 10 ? `${value.toFixed(1)} ms` : `${Math.round(value)} ms`;
}

// summarise turns one tally (a single operation, or all operations combined)
// into the values shown on a spotlight card.
// speedText describes go-media relative to ffmpeg as a factor: "2× faster" when
// go-media takes half the time, "1.5× slower" when it takes one and a half times as long.
function speedText(ratio) {
  if (Math.abs(ratio - 1) < 0.005) return i18n.global.t("media.shadowRegressionsSameSpeed");
  if (ratio < 1) {
    return i18n.global.t("media.shadowRegressionsFaster", { factor: (1 / ratio).toFixed(2) });
  }
  return i18n.global.t("media.shadowRegressionsSlower", { factor: ratio.toFixed(2) });
}

function summarise(key, title, s) {
  const successRate = s.runs ? s.succeeded / s.runs : 0;
  const regressionRate = s.runs ? s.regressions / s.runs : 0;
  const ratio = s.pairedRuns && s.ffmpegAvgMs ? s.goAvgMs / s.ffmpegAvgMs : null;
  return {
    key,
    title,
    runs: s.runs,
    succeeded: s.succeeded,
    regressions: s.regressions,
    successPct: pct(s.succeeded, s.runs),
    regressionPct: pct(s.regressions, s.runs),
    successTone: s.runs ? successTone(successRate) : "none",
    regressionTone: s.runs ? regressionTone(regressionRate) : "none",
    timingTone: timingTone(ratio),
    goAvg: formatMs(s.goAvgMs),
    ffAvg: formatMs(s.ffmpegAvgMs),
    timingRatio: ratio === null ? "—" : speedText(ratio),
  };
}

export default {
  name: "ShadowRegressions",
  components: {
    Editor: createAsyncComponent(() => import("@/views/files/Editor.vue")),
  },
  props: {
    promptId: {
      type: [String, Number],
      default: null,
    },
  },
  data() {
    return {
      regressionsContent: "",
      stats: [],
      incompatible: [],
      spotlightHeight: 0,
      observer: null,
      clearing: false,
    };
  },
  computed: {
    // The incompatible list is never taller than the spotlight cards. With no
    // cards the row is only padding, so it uses a fixed cap instead.
    incompatibleStyle() {
      const cardsShown = this.spotlights.length > 0 && this.spotlightHeight > 0;
      return { maxHeight: cardsShown ? `${this.spotlightHeight}px` : "12em" };
    },
    spotlights() {
      if (!this.stats.length) return [];
      const total = this.stats.reduce(
        (acc, s) => ({
          runs: acc.runs + s.runs,
          succeeded: acc.succeeded + s.succeeded,
          regressions: acc.regressions + s.regressions,
          pairedRuns: acc.pairedRuns + s.pairedRuns,
          goSum: acc.goSum + s.goAvgMs * s.pairedRuns,
          ffSum: acc.ffSum + s.ffmpegAvgMs * s.pairedRuns,
        }),
        { runs: 0, succeeded: 0, regressions: 0, pairedRuns: 0, goSum: 0, ffSum: 0 }
      );
      // Weight each operation's average by its paired runs, so the overall
      // average is the average over every paired run.
      const all = {
        runs: total.runs,
        succeeded: total.succeeded,
        regressions: total.regressions,
        pairedRuns: total.pairedRuns,
        goAvgMs: total.pairedRuns ? total.goSum / total.pairedRuns : 0,
        ffmpegAvgMs: total.pairedRuns ? total.ffSum / total.pairedRuns : 0,
      };
      return [
        summarise("all", this.$t("media.shadowRegressionsAllOperations"), all),
        ...this.stats.map((s) => summarise(s.operation, s.operation, s)),
      ];
    },
  },
  mounted() {
    void this.loadRegressions();
    const el = this.$refs.spotlights;
    if (el && typeof ResizeObserver !== "undefined") {
      this.observer = markRaw(
        new ResizeObserver(() => {
          this.spotlightHeight = el.offsetHeight;
        })
      );
      this.observer.observe(el);
    }
  },
  beforeUnmount() {
    this.observer?.disconnect();
  },
  methods: {
    async loadRegressions() {
      try {
        const [entries, stats, incompatible] = await Promise.all([
          getShadowRegressions(),
          getShadowRegressionStats(),
          getShadowIncompatible(),
        ]);
        this.regressionsContent = JSON.stringify(entries, null, 2);
        this.stats = stats;
        this.incompatible = incompatible;
      } catch (e) {
        console.error(e);
        notify.showErrorToast(this.$t("media.shadowRegressionsLoadFailed"));
        this.regressionsContent = "";
        this.stats = [];
        this.incompatible = [];
      }
    },
    async clearAll() {
      this.clearing = true;
      try {
        await clearShadowRegressions();
        notify.showSuccessToast(this.$t("media.shadowRegressionsCleared"));
        await this.loadRegressions();
      } catch (e) {
        console.error(e);
        notify.showErrorToast(this.$t("media.shadowRegressionsClearFailed"));
      } finally {
        this.clearing = false;
      }
    },
  },
};
</script>

<style scoped>
.prompt-panel {
  display: flex;
  flex-direction: column;
  gap: 0.75em;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

.spotlights {
  display: flex;
  gap: 0.75em;
  overflow-x: auto;
  flex-shrink: 0;
  padding-bottom: 0.25em;
}

.spotlight {
  flex: 0 0 auto;
  min-width: 320px;
  border: 1px solid var(--divider, #ccc);
  border-radius: 6px;
  padding: 0.6em 0.75em;
}

.spotlight-all {
  border-width: 2px;
}

.spotlight-title {
  font-weight: 600;
  margin-bottom: 0.4em;
  overflow-wrap: anywhere;
}

.spotlight-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.4em;
}

.metric {
  display: flex;
  flex-direction: column;
  border-radius: 4px;
  padding: 0.4em 0.5em;
  border-left: 4px solid var(--divider, #ccc);
  background: transparent;
}

.metric-wide {
  grid-column: 1 / -1;
}

.metric-label {
  font-size: 0.75em;
  opacity: 0.75;
}

.metric-value {
  font-weight: 600;
}

.metric-vs {
  font-weight: 400;
  opacity: 0.7;
  margin: 0 0.2em;
}

.metric-sub {
  font-size: 0.8em;
  opacity: 0.85;
}

.tone-good {
  border-left-color: #2e7d32;
  background: rgba(46, 125, 50, 0.12);
}

.tone-warn {
  border-left-color: #b26a00;
  background: rgba(178, 106, 0, 0.14);
}

.tone-bad {
  border-left-color: #c62828;
  background: rgba(198, 40, 40, 0.14);
}

.tone-none {
  border-left-color: var(--divider, #ccc);
}

.incompatible {
  flex-shrink: 0;
  overflow: auto;
  border: 1px solid var(--divider, #ccc);
  border-radius: 6px;
  padding: 0.5em 0.75em;
  font-size: 0.9em;
}

.incompatible-title {
  font-weight: 600;
  margin-bottom: 0.4em;
}

.incompatible-empty {
  opacity: 0.7;
}

.incompatible-table {
  width: 100%;
  border-collapse: collapse;
}

.incompatible-table th,
.incompatible-table td {
  text-align: left;
  padding: 0.25em 0.5em;
  border-bottom: 1px solid var(--divider, #ccc);
  vertical-align: top;
}

.incompatible-table .num {
  text-align: right;
  white-space: nowrap;
}

.incompatible-table .path {
  overflow-wrap: anywhere;
  word-break: break-all;
}

.editor-container {
  position: relative;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--divider, #ccc);
  border-radius: 4px;
}
</style>
