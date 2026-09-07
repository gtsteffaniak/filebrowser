<template>
  <i
    :class="[iconClasses, { 'tooltip-info-icon--pressed': pressed }]"
    role="button"
    tabindex="0"
    :aria-label="ariaLabel || undefined"
    @click.stop="onClick"
    @touchstart.stop="onTouchStart"
    @touchend.stop="onTouchEnd"
    @touchcancel.stop="onTouchCancel"
    @mouseenter="onMouseEnter"
    @mouseleave="onMouseLeave"
  >
    {{ icon }}
  </i>
</template>

<script>
import {
  onTooltipHelpClick,
  onTooltipHelpMouseEnter,
  onTooltipHelpMouseLeave,
  onTooltipHelpTouchEnd,
  useTapForTooltip,
} from "@/utils/tooltipHelp.js";

export default {
  name: "HelpTooltipIcon",
  props: {
    text: {
      type: String,
      required: true,
    },
    icon: {
      type: String,
      default: "help",
    },
    iconStyle: {
      type: String,
      default: "outlined",
      validator: (value) => value === "outlined" || value === "symbols",
    },
    iconClass: {
      type: String,
      default: "",
    },
    ariaLabel: {
      type: String,
      default: "",
    },
  },
  data() {
    return {
      pressed: false,
    };
  },
  computed: {
    iconClasses() {
      return [
        "no-select",
        "tooltip-info-icon",
        this.iconStyle === "symbols"
          ? "material-symbols"
          : "material-symbols-outlined",
        this.iconClass,
      ];
    },
  },
  methods: {
    onClick(event) {
      onTooltipHelpClick(event, this.text);
    },
    onTouchStart() {
      if (useTapForTooltip()) {
        this.pressed = true;
      }
    },
    onTouchEnd(event) {
      this.pressed = false;
      onTooltipHelpTouchEnd(event, this.text);
    },
    onTouchCancel() {
      this.pressed = false;
    },
    onMouseEnter(event) {
      onTooltipHelpMouseEnter(event, this.text);
    },
    onMouseLeave() {
      this.pressed = false;
      onTooltipHelpMouseLeave();
    },
  },
};
</script>
