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
    <div v-else v-html="tooltip.content"></div>
  </div>
</template>

<script>
import { getters, mutations, state } from "@/store";

export default {
  name: "Tooltip",
  data() {
    return {
      adjustedX: 0,
      adjustedY: 0,
      margin: 15,
    };
  },
  computed: {
    tooltip() {
      return state.tooltip;
    },
    hasComponent() {
      return Boolean(this.tooltip.component);
    },
    isDarkMode() {
      return getters.isDarkMode();
    },
    tooltipStyle() {
      const style = {
        top: `${this.adjustedY}px`,
        left: `${this.adjustedX}px`,
      };

      if (this.tooltip.width) {
        style.maxWidth = this.tooltip.width;
        style.width = this.tooltip.width;
      }

      if (this.tooltip.width || this.tooltip.pointerEvents) {
        style.maxHeight = "80vh";
        style.overflowY = "auto";
      }

      return style;
    },
  },
  watch: {
    $route: {
      handler() {
        mutations.hideTooltip();
      },
    },
    tooltip: {
      handler(newTooltip) {
        if (newTooltip.show) {
          this.updatePosition(newTooltip.x, newTooltip.y);
        }
      },
      deep: true,
    },
  },
  methods: {
    /**
     * @param {number} x - X coordinate
     * @param {number} y - Y coordinate
     */
    updatePosition(x, y) {
      this.$nextTick(() => {
        const tooltipEl = this.$refs.tooltipRef;
        if (!tooltipEl) return;

        const tooltipRect = tooltipEl.getBoundingClientRect();
        const windowWidth = window.innerWidth;
        const windowHeight = window.innerHeight;

        if (x + this.margin + tooltipRect.width > windowWidth) {
          this.adjustedX = x - this.margin - tooltipRect.width;
        } else {
          this.adjustedX = x + this.margin;
        }

        if (this.adjustedX < 0) {
          this.adjustedX = this.margin;
        }

        if (y + this.margin + tooltipRect.height > windowHeight) {
          this.adjustedY = y - this.margin - tooltipRect.height;
        } else {
          this.adjustedY = y + this.margin;
        }

        if (this.adjustedY < 0) {
          this.adjustedY = this.margin;
        }
      });
    },
  },
};
</script>

<style>
.floating-tooltip {
  position: fixed;
  padding: 0.5em;
  background-color: var(--alt-background);
  color: var(--textPrimary);
  border-radius: 1em;
  box-shadow: 0 0.25em 1em rgba(0, 0, 0, 0.2);
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
}
</style>
