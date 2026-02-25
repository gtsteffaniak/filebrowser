<template>
  <div class="card-content no-buttons">
    <div v-if="!isMobile" class="shortcut-section">
      <h3 class="section-title">⌨️ {{ $t("threejs.keyboard") }}</h3>
      <div class="shortcut-item">
        <kbd>{{ $t("general.space") }}</kbd>
        <span>{{ spaceText }}</span>
      </div>
      <div class="shortcut-item">
        <kbd>R</kbd>
        <span>{{ $t("general.reset") }}</span>
      </div>
      <div class="shortcut-item">
        <kbd>Q / E</kbd>
        <span>{{ $t("threejs.rotateY") }}</span>
      </div>
      <div class="shortcut-item">
        <kbd>W / S</kbd>
        <span>{{ $t("threejs.rotateX") }}</span>
      </div>
      <div class="shortcut-item">
        <kbd>+ / -</kbd>
        <span>{{ $t("general.zoom") }}</span>
      </div>
    </div>

    <div class="shortcut-section">
      <h3 class="section-title">🎨 {{ $t("general.background") }}</h3>
      <div class="color-control">
        <input 
          type="color" 
          :value="backgroundColor" 
          @input="handleBackgroundChange"
          class="color-picker-input"
          :title="$t('threejs.changeBackground')"
        />
        <span class="color-label">{{ $t("threejs.changeBackground") }}</span>
      </div>
    </div>
  </div>
</template>

<script>
import { mutations } from "@/store";

export default {
  name: "threeJSControls",
  props: {
    backgroundColor: {
      type: String,
      required: true,
    },
    isMobile: {
      type: Boolean,
      default: false,
    },
    hasAnimations: {
      type: Boolean,
      default: false,
    },
  },
  computed: {
    spaceText() {
      return this.hasAnimations 
        ? `${this.$t("general.play")}/${this.$t("general.pause")}` 
        : this.$t("threejs.autoRotate");
    },
  },
  methods: {
    closeHovers() {
      mutations.closeTopHover();
    },
    handleBackgroundChange(event) {
      this.$emit('update:backgroundColor', event.target.value);
    },
  },
};
</script>

<style scoped>
.card-content {
  min-width: 300px;
  max-height: 70vh;
  overflow-y: auto;
}

.shortcut-section {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
  margin-bottom: 1.5em;
}

.shortcut-section:last-child {
  margin-bottom: 0;
}

.section-title {
  font-size: 0.95em;
  font-weight: 600;
  color: var(--textPrimary);
  margin: 0 0 0.75em 0;
  padding-bottom: 0.5em;
  border-bottom: 1px solid var(--divider);
}

.shortcut-item {
  display: flex;
  align-items: center;
  gap: 0.75em;
  padding: 0.5em;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.shortcut-item:hover {
  background-color: var(--surfaceSecondary);
}

.shortcut-item kbd {
  display: inline-block;
  min-width: 80px;
  padding: 0.35em 0.65em;
  font-family: monospace;
  font-size: 0.85em;
  font-weight: 600;
  line-height: 1.2;
  color: var(--textPrimary);
  background-color: var(--surfaceSecondary);
  border: 1px solid var(--divider);
  border-radius: 4px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.1);
  text-align: center;
}

.shortcut-item span {
  flex: 1;
  font-size: 0.9em;
  color: var(--textSecondary);
}

.color-control {
  display: flex;
  align-items: center;
  gap: 1em;
  padding: 0.5em;
}

.color-picker-input {
  width: 50px;
  height: 35px;
  border: 1px solid var(--divider);
  border-radius: 4px;
  cursor: pointer;
  background: transparent;
}

.color-picker-input::-webkit-color-swatch-wrapper {
  padding: 0;
}

.color-picker-input::-webkit-color-swatch {
  border: none;
  border-radius: 3px;
}

.color-label {
  font-size: 0.9em;
  color: var(--textSecondary);
}

/* Responsive adjustments */
@media (max-width: 768px) {
  .shortcut-item kbd {
    min-width: 70px;
    font-size: 0.8em;
  }
}
</style>
