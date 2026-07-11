<template>
  <div class="card-content visualizer-settings-content settings-items">
    <div class="setting-row item">
      <div class="setting-label">
        <label for="vis-bar-count">{{ $t("player.visualizer.barCount") }}</label>
        <i
          class="no-select material-symbols-outlined tooltip-info-icon"
          @mouseenter="showTooltip($event, $t('player.visualizer.barCountDescription'))"
          @mouseleave="hideTooltip"
        >help</i>
      </div>
      <div class="setting-control">
        <input
          id="vis-bar-count"
          type="number"
          class="input"
          v-model.number="barCount"
          min="2"
          max="200"
          :aria-label="$t('player.visualizer.barCount')"
        />
      </div>
    </div>
    <div class="setting-row item">
      <div class="setting-label">
        <label id="vis-fft-size-label">{{ $t("player.visualizer.fftSize") }}</label>
        <i
          class="no-select material-symbols-outlined tooltip-info-icon"
          @mouseenter="showTooltip($event, $t('player.visualizer.fftSizeDescription'))"
          @mouseleave="hideTooltip"
        >help</i>
      </div>
      <div class="setting-control">
        <ExpandDropdown
          :options="fftSizeOptions"
          v-model="fftSize"
          :aria-label="$t('player.visualizer.fftSize')"
        />
      </div>
    </div>
    <div class="setting-row item">
      <div class="setting-label">
        <label id="vis-min-freq-label">{{ $t("player.visualizer.minFrequency") }}</label>
        <i
          class="no-select material-symbols-outlined tooltip-info-icon"
          @mouseenter="showTooltip($event, $t('player.visualizer.minFrequencyDescription'))"
          @mouseleave="hideTooltip"
        >help</i>
      </div>
      <div class="setting-control">
        <ExpandDropdown
          :options="minFrequencyOptions"
          v-model="minFrequency"
          :aria-label="$t('player.visualizer.minFrequency')"
        />
      </div>
    </div>
    <div class="setting-row item">
      <div class="setting-label">
        <label id="vis-max-freq-label">{{ $t("player.visualizer.maxFrequency") }}</label>
        <i
          class="no-select material-symbols-outlined tooltip-info-icon"
          @mouseenter="showTooltip($event, $t('player.visualizer.maxFrequencyDescription'))"
          @mouseleave="hideTooltip"
        >help</i>
      </div>
      <div class="setting-control">
        <ExpandDropdown
          :options="maxFrequencyOptions"
          v-model="maxFrequency"
          :aria-label="$t('player.visualizer.maxFrequency')"
        />
      </div>
    </div>
    <div class="setting-row slider-setting item">
      <div class="setting-label">
        <label for="vis-smoothing">{{ $t("player.visualizer.smoothing") }}</label>
        <i
          class="no-select material-symbols-outlined tooltip-info-icon"
          @mouseenter="showTooltip($event, $t('player.visualizer.smoothingDescription'))"
          @mouseleave="hideTooltip"
        >help</i>
      </div>
      <div class="setting-control slider-setting-value">
        <input id="vis-smoothing" type="range" min="0" max="1" step="0.01" v-model.number="smoothing" />
        <span class="slider-value">{{ smoothing.toFixed(2) }}</span>
      </div>
    </div>
    <div class="setting-row slider-setting item">
      <div class="setting-label">
        <label for="vis-min-db">{{ $t("player.visualizer.minDecibels") }}</label>
        <i
          class="no-select material-symbols-outlined tooltip-info-icon"
          @mouseenter="showTooltip($event, $t('player.visualizer.minDecibelsDescription'))"
          @mouseleave="hideTooltip"
        >help</i>
      </div>
      <div class="setting-control slider-setting-value">
        <input id="vis-min-db" type="range" min="-120" max="-60" step="1" v-model.number="minDecibels" />
        <span class="slider-value">{{ minDecibels }}</span>
      </div>
    </div>
    <div class="setting-row slider-setting item">
      <div class="setting-label">
        <label for="vis-max-db">{{ $t("player.visualizer.maxDecibels") }}</label>
        <i
          class="no-select material-symbols-outlined tooltip-info-icon"
          @mouseenter="showTooltip($event, $t('player.visualizer.maxDecibelsDescription'))"
          @mouseleave="hideTooltip"
        >help</i>
      </div>
      <div class="setting-control slider-setting-value">
        <input id="vis-max-db" type="range" min="-40" max="20" step="1" v-model.number="maxDecibels" />
        <span class="slider-value">{{ maxDecibels }}</span>
      </div>
    </div>
    <ToggleSwitch
      class="item"
      :name="$t('player.visualizer.showScales')"
      :description="$t('player.visualizer.showScalesDescription')"
      v-model="showScales"
    />
    <ToggleSwitch
      class="item"
      :name="$t('player.visualizer.showPeaks')"
      :description="$t('player.visualizer.showPeaksDescription')"
      v-model="showPeaks"
    />
  </div>
  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--grey"
      @click="resetVisualizerConfig"
      :aria-label="$t('player.visualizer.resetDefaults')"
      :title="$t('player.visualizer.resetDefaults')"
    > {{ $t("player.visualizer.resetDefaults") }}
    </button>
    <button
      type="button"
      class="button button--flat"
      @click="closeTopPrompt"
      :aria-label="$t('general.ok')"
      :title="$t('general.ok')"
    > {{ $t("general.ok") }}
    </button>
  </div>
</template>

<script>
import { mutations } from "@/store";
import { visualizerConfig, saveVisualizerConfig, resetVisualizerConfig } from "@/utils/visualizerConfig.js";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

function freqOptions(values) {
  return values.map((value) => ({ value, label: `${value} Hz` }));
}

export default {
  name: "VisualizerSettings",
  components: { ExpandDropdown, ToggleSwitch },
  data() {
    return {
      fftSizeOptions: [1024, 2048, 4096, 8192, 16384, 32768].map((value) => ({ value, label: `${value}` })),
      minFrequencyOptions: freqOptions([20, 30, 40, 50]),
      maxFrequencyOptions: freqOptions([8000, 10000, 15000, 20000, 22050]),
    };
  },
  computed: {
    barCount: {
      get() { return visualizerConfig.barCount; },
      set(value) {
        const clamped = Math.min(200, Math.max(2, Math.round(value) || 0));
        saveVisualizerConfig({ barCount: clamped });
      },
    },
    fftSize: {
      get() { return visualizerConfig.fftSize; },
      set(value) { saveVisualizerConfig({ fftSize: Number(value) }); },
    },
    minFrequency: {
      get() { return visualizerConfig.minFrequency; },
      set(value) { saveVisualizerConfig({ minFrequency: Number(value) }); },
    },
    maxFrequency: {
      get() { return visualizerConfig.maxFrequency; },
      set(value) { saveVisualizerConfig({ maxFrequency: Number(value) }); },
    },
    smoothing: {
      get() { return visualizerConfig.smoothing; },
      set(value) { saveVisualizerConfig({ smoothing: value }); },
    },
    minDecibels: {
      get() { return visualizerConfig.minDecibels; },
      set(value) { saveVisualizerConfig({ minDecibels: value }); },
    },
    maxDecibels: {
      get() { return visualizerConfig.maxDecibels; },
      set(value) { saveVisualizerConfig({ maxDecibels: value }); },
    },
    showScales: {
      get() { return visualizerConfig.showScales; },
      set(value) { saveVisualizerConfig({ showScales: value }); },
    },
    showPeaks: {
      get() { return visualizerConfig.showPeaks; },
      set(value) { saveVisualizerConfig({ showPeaks: value }); },
    },
  },
  methods: {
    closeTopPrompt() {
      mutations.closeTopPrompt();
    },
    resetVisualizerConfig() {
      resetVisualizerConfig();
    },
    showTooltip(event, text) {
      mutations.showTooltip({
        content: text,
        x: event.clientX,
        y: event.clientY,
      });
    },
    hideTooltip() {
      mutations.hideTooltip();
    },
  },
};
</script>

<style scoped>
.settings-items .item {
  padding-top: 0.75em;
  padding-bottom: 0.75em;
}

.visualizer-settings-content {
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 0 0.75em;
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1em;
  min-width: 0;
}

.setting-row label {
  color: var(--textPrimary);
  font-size: 0.95em;
  flex-shrink: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.setting-label {
  display: flex;
  align-items: center;
  gap: 0.3em;
  min-width: 0;
  flex-shrink: 1;
}

.setting-control {
  flex: 1 1 auto;
  min-width: 0;
  max-width: 13em;
  display: flex;
  justify-content: flex-end;
}

.setting-control input[type="number"] {
  width: 100%;
  max-width: 6em;
  text-align: right;
}

.slider-setting {
  align-items: center;
}

.slider-setting-value {
  align-items: center;
  gap: 0.6em;
}

.slider-setting-value input[type="range"] {
  flex: 1;
}

.slider-value {
  flex-shrink: 0;
  min-width: 2.8em;
  text-align: right;
  font-variant-numeric: tabular-nums;
  color: var(--textSecondary);
  font-size: 0.9em;
}
</style>
