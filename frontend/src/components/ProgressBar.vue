<!-- This component taken directly from vue-simple-progress
since it didnt support Vue 3 but the component itself does
https://raw.githubusercontent.com/dzwillia/vue-simple-progress/master/src/components/Progress.vue -->
<template>
  <div class="vue-simple-progress" :style="progress_style">
    <div
      class="vue-simple-progress-text"
      :style="text_style"
      v-if="textPosition == 'middle'"
    >
      {{ displayed_text }}
    </div>

    <div class="vue-simple-progress-bar" :style="bar_style"></div>
    <div
      class="vue-simple-progress-text"
      :style="text_style"
      v-if="textPosition == 'inside'"
    >
      {{ displayed_text }}
      <i
        v-if="helpText && status === 'error'"
        class="no-select material-symbols-outlined tooltip-info-icon"
        @mouseenter="showTooltip"
        @mouseleave="hideTooltip"
      >
        help
      </i>
    </div>
  </div>
</template>

<script>
import { getHumanReadableFilesize } from "@/utils/filesizes.js";
import { mutations } from "@/store";

// We're leaving this untouched as you can read in the beginning
var isNumber = function (n) {
  return !isNaN(parseFloat(n)) && isFinite(n);
};

export default {
  name: "progress-bar",
  props: {
    val: {
      default: 0,
    },
    max: {
      default: 100,
    },
    unit: {
      type: String,
      default: "",
    },
    size: {
      // either a number (pixel width/height) or 'tiny', 'small',
      // 'medium', 'large', 'huge', 'massive' for common sizes
      default: "big",
    },
    "bg-color": {
      type: String,
      default: "#eee",
    },
    "bar-color": {
      type: String,
      default: "var(--primaryColor)", // match .blue color to Material Design's 'Blue 500' color
    },
    "bar-transition": {
      type: String,
      default: "all 0.5s ease",
    },
    "bar-border-radius": {
      type: Number,
      default: 4,
    },
    spacing: {
      type: Number,
      default: 4,
    },
    text: {
      type: String,
      default: "",
    },
    "text-align": {
      type: String,
      default: "center", // 'left', 'right'
    },
    "text-position": {
      type: String,
      default: "inside", // 'bottom', 'top', 'middle', 'inside'
    },
    "font-size": {
      type: Number,
      default: 13,
    },
    "text-fg-color": {
      type: String,
      default: "#000",
    },
    status: {
      type: String,
      default: 'default',
    },
    "help-text": {
      type: String,
      default: "",
    },
  },
  computed: {
    isValNumeric() {
      return isNumber(this.val);
    },
    pct() {
      if (!this.isValNumeric) return 100;
      if (this.max <= 0) return 0;
      var pct = (this.val / this.max) * 100;
      var finalPct = Math.max(0, Math.min(pct.toFixed(2), 100));
      return finalPct < 7 ? 0 : finalPct;
    },
    displayed_text() {
      if (!this.isValNumeric) return this.val;

      const percentage =
        this.max > 0 ? Math.round((this.val / this.max) * 100) : 0;

      if (this.unit === "bytes" && this.isValNumeric) {
        const valFormatted = getHumanReadableFilesize(this.val);
        const maxFormatted = getHumanReadableFilesize(this.max);
        return `${valFormatted} / ${maxFormatted} (${percentage}%)`;
      }

      const unit_string = this.unit ? ` ${this.unit}` : "";
      return `${this.val}${unit_string} / ${this.max}${unit_string} (${percentage}%)`;
    },
    size_px() {
      switch (this.size) {
        case "tiny":
          return 2;
        case "small":
          return 4;
        case "medium":
          return 8;
        case "large":
          return 10;
        case "big":
          return 16;
        case "huge":
          return 32;
        case "massive":
          return 64;
      }

      return isNumber(this.size) ? this.size : 32;
    },
    text_padding() {
      switch (this.size) {
        case "tiny":
        case "small":
        case "medium":
        case "large":
        case "big":
        case "huge":
        case "massive":
          return Math.min(Math.max(Math.ceil(this.size_px / 8), 3), 12);
      }

      return isNumber(this.spacing) ? this.spacing : 4;
    },
    text_font_size() {
      switch (this.size) {
        case "tiny":
        case "small":
        case "medium":
        case "large":
        case "big":
        case "huge":
        case "massive":
          return Math.min(Math.max(Math.ceil(this.size_px * 0.8), 11), 32);
      }

      return isNumber(this.fontSize) ? this.fontSize : 13;
    },
    progress_style() {
      var style = {
        background: this.bgColor,
        position: 'relative'
      };

      if (this.textPosition == "middle" || this.textPosition == "inside") {
        style["min-height"] = this.size_px + "px";
      }

      if (this.barBorderRadius > 0) {
        style["border-radius"] = this.barBorderRadius + "px";
      }

      return style;
    },
    bar_style() {
      let barColor = this.barColor;
      if (this.status === 'error') {
        barColor = '#f44336';
      } else if (this.status === 'conflict') {
        barColor = '#ff9800';
      }

      var style = {
        width: this.pct + "%",
        height: this.size_px + "px",
        background: barColor,
        transition: this.barTransition,
      };

      if (this.barBorderRadius > 0) {
        style["border-radius"] = this.barBorderRadius + "px";
      }

      if (this.textPosition == "middle") {
        style["position"] = "absolute";
        style["top"] = "0";
        style["height"] = "100%";
        style["min-width"] = "1.5em";
        style["min-height"] = this.size_px + "px";
        style["z-index"] = "-1";
      }

      return style;
    },
    text_style() {
      var style = {
        "color": this.textFgColor,
        "font-size": this.text_font_size + "px",
        "text-align": this.textAlign,
      };

      if (this.textPosition === 'inside') {
        style['position'] = 'absolute';
        style['left'] = '0';
        style['right'] = '0';
        style['top'] = '50%';
        style['transform'] = 'translateY(-50%)';
        style['width'] = '100%';
        style['padding'] = '0 0.5em';
        style['box-sizing'] = 'border-box';
      }

      if (
        this.textPosition == "top" ||
        this.textPosition == "middle"
      )
        style["padding-bottom"] = this.text_padding + "px";
      if (
        this.textPosition == "bottom" ||
        this.textPosition == "middle"
      )
        style["padding-top"] = this.text_padding + "px";
      return style;
    },
  },
  methods: {
    showTooltip(event) {
      if (this.helpText) {
        mutations.showTooltip({
          content: this.helpText,
          x: event.clientX,
          y: event.clientY,
        });
      }
    },
    hideTooltip() {
      mutations.hideTooltip();
    },
  },
};
</script>

<style>
.vue-simple-progress {
  margin: 0.5em;
}

.vue-simple-progress,
.vue-simple-progress-bar {
  border-radius: 0.5em;
}

.vue-simple-progress {
  background: var(--primaryColor);
}
.vue-simple-progress-text {
  color: black;
}

.tooltip-info-icon {
  font-size: 1rem;
  cursor: pointer;
  margin-left: 0.3em;
  vertical-align: middle;
  opacity: 0.7;
}

.tooltip-info-icon:hover {
  opacity: 1;
}
</style>
