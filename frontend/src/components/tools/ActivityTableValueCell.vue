<template>
  <span v-if="isBlank" class="details-muted">—</span> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
  <div v-else class="table-value-cell">
    <span
      :class="badgeClasses"
      @mouseenter="showTooltip"
      @mouseleave="hideTooltip"
    >{{ displayText }}</span>
  </div>
</template>

<script>
import { mutations } from "@/store";
import ActivityTableValueInfo from "@/components/tools/ActivityTableValueInfo.vue";

export default {
  name: "ActivityTableValueCell",
  props: {
    value: {
      type: [String, Number],
      default: "",
    },
    display: {
      type: String,
      default: "",
    },
    label: {
      type: String,
      default: "",
    },
    tooltip: {
      type: String,
      default: "",
    },
    variant: {
      type: String,
      default: "default",
      validator: (value) => value === "default" || value === "eventType",
    },
    badgeClass: {
      type: [String, Array, Object],
      default: "",
    },
  },
  computed: {
    isBlank() {
      if (this.value === null || this.value === undefined) {
        return true;
      }
      if (typeof this.value === "number") {
        return false;
      }
      return String(this.value).trim() === "";
    },
    displayText() {
      if (this.display) {
        return this.display;
      }
      if (typeof this.value === "number") {
        return String(this.value);
      }
      return this.value || "";
    },
    tooltipText() {
      return this.tooltip || this.displayText;
    },
    badgeClasses() {
      if (this.variant === "eventType") {
        return ["event-type-badge", "border-radius", this.badgeClass].filter(Boolean);
      }
      return ["table-value-badge", "detail-badge", "border-radius"];
    },
  },
  methods: {
    showTooltip(event) {
      const text = this.tooltipText;
      if (!text || !this.label) {
        return;
      }
      mutations.showTooltip({
        component: ActivityTableValueInfo,
        componentProps: {
          label: this.label,
          value: text,
        },
        x: event.clientX,
        y: event.clientY,
        width: "22rem",
      });
    },
    hideTooltip() {
      mutations.hideTooltip();
    },
  },
};
</script>
