<template>
  <div
    ref="tooltipRef"
    v-show="tooltip.show"
    class="floating-tooltip fb-shadow"
    :class="{ 'dark-mode': isDarkMode, 'pointer-enabled': tooltip.pointerEvents }"
    :style="tooltipStyle"
    v-html="tooltip.content"
  ></div>
</template>

<script>
import { state,getters } from "@/store";

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
    isDarkMode() {
      return getters.isDarkMode();
    },
    tooltipStyle() {
      const style = {
        top: `${this.adjustedY}px`,
        left: `${this.adjustedX}px`,
      };

      // Add custom width if specified
      if (this.tooltip.width) {
        style.maxWidth = this.tooltip.width;
        style.width = this.tooltip.width;
      }

      // Add max height and scrolling for viewport overflow
      if (this.tooltip.width || this.tooltip.pointerEvents) {
        style.maxHeight = '80vh';
        style.overflowY = 'auto';
      }

      return style;
    },
  },
  watch: {
    $route: {
      handler() {
        // hide tooltip when route changes
        this.tooltip.show = false;
        this.tooltip.content = "";
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
  white-space: normal;
  overflow-wrap: break-word;
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