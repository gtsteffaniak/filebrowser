<template>
  <div
    v-show="tooltip.show"
    class="floating-tooltip"
    :class="{ 'dark-mode': isDarkMode }"
    :style="{ top: tooltip.y + 'px', left: tooltip.x + 'px' }"
    v-html="tooltip.content"
  ></div>
</template>

<script>
import { state } from "@/store";

export default {
  name: "Tooltip",
  computed: {
    tooltip() {
      return state.tooltip;
    },
    isDarkMode() {
      return state.user.darkMode;
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
  transform: translate(15px, 15px); /* Offset from cursor */
  max-width: 20em;
  white-space: normal;
  overflow-wrap: break-word;
}
.tooltip-info-icon {
  font-size: 1em !important;
  padding: 0.1em !important;
  padding-left: 0.5em !important;
}
</style>