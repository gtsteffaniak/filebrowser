<template>
  <div
    ref="tooltipRef"
    v-show="tooltip.show"
    class="floating-tooltip floating-window"
    :class="{
      'dark-mode': isDarkMode,
      'pointer-enabled': tooltip.pointerEvents,
      'floating-tooltip--component': hasComponent,
    }"
    :style="tooltipStyle"
  >
    <component
      v-if="hasComponent"
      :is="tooltip.component"
      v-bind="tooltip.componentProps || {}"
    />
    <div v-else class="tooltip-text">{{ tooltip.content }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import type { CSSProperties } from "vue";
import { useRoute } from "vue-router";
import { getters, state } from "@/store";
import { hideInteractiveTooltip } from "@/utils/tooltipHelp.js";

const margin = 15;
const adjustedX = ref(0);
const adjustedY = ref(0);
const tooltipRef = ref<HTMLElement | null>(null);
const route = useRoute();

const tooltip = computed(() => state.tooltip);
const hasComponent = computed(() => Boolean(tooltip.value.component));
const isDarkMode = computed(() => getters.isDarkMode());

const tooltipStyle = computed(() => {
  const style: CSSProperties = {
    top: `${adjustedY.value}px`,
    left: `${adjustedX.value}px`,
  };

  if (tooltip.value.width) {
    style.maxWidth = tooltip.value.width;
    style.width = tooltip.value.width;
  }

  if (tooltip.value.width || tooltip.value.pointerEvents) {
    style.maxHeight = "80vh";
    style.overflowY = "auto";
  }

  return style;
});

async function updatePosition(x: number, y: number) {
  await nextTick();
  const tooltipEl = tooltipRef.value;
  if (!tooltipEl) return;

  const tooltipRect = tooltipEl.getBoundingClientRect();
  const windowWidth = window.innerWidth;
  const windowHeight = window.innerHeight;

  if (x + margin + tooltipRect.width > windowWidth) {
    adjustedX.value = x - margin - tooltipRect.width;
  } else {
    adjustedX.value = x + margin;
  }

  if (adjustedX.value < 0) {
    adjustedX.value = margin;
  }

  if (y + margin + tooltipRect.height > windowHeight) {
    adjustedY.value = y - margin - tooltipRect.height;
  } else {
    adjustedY.value = y + margin;
  }

  if (adjustedY.value < 0) {
    adjustedY.value = margin;
  }
}

watch(
  () => route.fullPath,
  () => hideInteractiveTooltip(true),
);

watch(
  tooltip,
  (newTooltip) => {
    if (newTooltip.show) {
      void updatePosition(newTooltip.x, newTooltip.y);
    }
  },
  { deep: true },
);
</script>

<style>
.floating-tooltip {
  position: fixed;
  padding: 0.5em;
  background-color: var(--alt-background);
  color: var(--textPrimary);
  border-radius: var(--borderRadius);
  box-shadow: 0 0.25em 1em rgb(0 0 0 / 20%);
  z-index: 9999;
  pointer-events: none;
  max-width: 20em;
  /* Preserve newlines from i18n strings (e.g. search help) while still wrapping long lines */
  white-space: pre-line;
  overflow-wrap: break-word;
}

.floating-tooltip--component {
  padding: 0;
  max-width: min(28em, 90vw);
  white-space: normal;
}

.floating-tooltip.pointer-enabled {
  pointer-events: auto;
  cursor: auto;
}

.floating-tooltip.pointer-enabled a {
  pointer-events: auto;
  cursor: pointer;
}

.tooltip-info-icon {
  font-size: 1em !important;
  padding: 0.1em !important;
  padding-left: 0.5em !important;
  touch-action: manipulation;
}

.tooltip-info-icon[role="button"] {
  cursor: pointer;
}

.tooltip-info-icon:active,
.tooltip-info-icon.tooltip-info-icon--pressed,
.tooltip-info-icon:hover {
  font-variation-settings: 'FILL' 1;
  opacity: 1;
}
</style>
