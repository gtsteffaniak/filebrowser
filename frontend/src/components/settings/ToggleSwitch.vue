<template>
  <div
    class="toggle-container"
    :class="{
      'toggle-container--enforceable': enforceable,
      'toggle-container--neutral': variant === 'neutral',
    }"
  >
    <div
      class="toggle-row toggle-row--value"
      :class="{
        'toggle-row--disabled': disabled,
        'border-radius': enforceable,
        'toggle-row--icon-mode': iconMode,
      }"
      @mouseenter="showValueRowTooltipIfNeeded"
      @mouseleave="hideTooltip"
    >
      <template v-if="iconMode">
        <div class="toggle-icon-cluster">
          <label class="switch switch--icon-mode">
            <input
              type="checkbox"
              :checked="modelValue"
              @change="updateValue"
              :aria-label="effectiveAriaLabel"
              :disabled="disabled"
            />
            <span class="slider round">
              <span class="slider-icons" aria-hidden="true">
                <i
                  class="slider-icon slider-icon--slot"
                  :class="[
                    iconClass,
                    { 'slider-icon--active': modelValue },
                  ]"
                >{{ onIcon }}</i>
                <i
                  class="slider-icon slider-icon--slot"
                  :class="[
                    iconClass,
                    { 'slider-icon--active': !modelValue },
                  ]"
                >{{ offIcon }}</i>
              </span>
              <span class="slider-knob" aria-hidden="true"></span>
            </span>
          </label>
        </div>
      </template>
      <template v-else>
        <div class="toggle-name-container">
          <span class="toggle-name">{{ name }}</span>
          <HelpTooltipIcon v-if="description" :text="description" />
        </div>
        <label class="switch">
          <input
            type="checkbox"
            :checked="modelValue"
            @change="updateValue"
            :aria-label="effectiveAriaLabel"
            :disabled="disabled"
          />
          <span class="slider round"></span>
        </label>
      </template>
    </div>
    <div
      v-if="enforceable"
      class="toggle-row toggle-row--enforced border-radius"
      :class="{ 'toggle-row--disabled': enforcementDisabled }"
    >
      <span class="enforced-label">{{ enforcedLabelText }}</span>
      <label class="switch">
        <input
          :id="enforcedInputId"
          type="checkbox"
          :checked="enforced"
          :disabled="enforcementDisabled"
          :aria-label="enforcedLabelText"
          @change="updateEnforced"
        />
        <span class="slider round"></span>
      </label>
    </div>
  </div>
</template>

<script>
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";
import {
  hideInteractiveTooltip,
  showHoverTooltip,
} from "@/utils/tooltipHelp.js";

let enforcedIdCounter = 0;

export default {
  name: "ToggleSwitch",
  components: { HelpTooltipIcon },
  props: {
    modelValue: {
      type: Boolean,
      required: true,
    },
    name: {
      type: String,
      required: true,
    },
    description: {
      type: String,
      required: false,
      default: "",
    },
    ariaLabel: {
      type: String,
      required: false,
      default: "",
    },
    disabled: {
      type: Boolean,
      required: false,
      default: false,
    },
    enforcementDisabled: {
      type: Boolean,
      required: false,
      default: false,
    },
    enforceable: {
      type: Boolean,
      default: false,
    },
    enforced: {
      type: Boolean,
      default: false,
    },
    enforcementLocked: {
      type: Boolean,
      default: false,
    },
    valueTooltip: {
      type: String,
      default: "",
    },
    variant: {
      type: String,
      default: "primary",
      validator: (v) => ["primary", "neutral"].includes(v),
    },
    offIcon: {
      type: String,
      default: "",
    },
    onIcon: {
      type: String,
      default: "",
    },
    iconOutlined: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    enforcedIdCounter += 1;
    return {
      enforcedInputId: `toggle-enforced-${enforcedIdCounter}`,
    };
  },
  computed: {
    iconMode() {
      return Boolean(this.offIcon && this.onIcon);
    },
    iconClass() {
      return this.iconOutlined ? "material-symbols-outlined" : "material-symbols";
    },
    effectiveAriaLabel() {
      return this.ariaLabel || this.name;
    },
    enforcedLabelText() {
      return this.$t("general.enforce");
    },
  },
  methods: {
    updateValue(event) {
      this.$emit("update:modelValue", event.target.checked);
    },
    updateEnforced(event) {
      this.$emit("update:enforced", event.target.checked);
    },
    showValueRowTooltipIfNeeded(event) {
      if (!this.disabled) {
        return;
      }
      if (this.valueTooltip) {
        showHoverTooltip(this.valueTooltip, event);
        return;
      }
      if (this.enforcementLocked) {
        showHoverTooltip(this.$t("profileSettings.enforcedByAdmin"), event);
      }
    },
    hideTooltip() {
      hideInteractiveTooltip();
    },
  },
};
</script>

<style scoped>
.toggle-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 1rem;
}

.toggle-container--enforceable {
  flex-direction: column;
  align-items: stretch;
  gap: 0.35em;
  padding: 0.35em;
  border-radius: var(--borderRadius);
  transition: background-color 0.15s ease;
}

.toggle-container--enforceable:hover {
  background-color: var(--surfaceSecondary);
}

.toggle-container--enforceable .toggle-row {
  box-sizing: border-box;
  min-height: 3.25em;
  padding: 0.5em 1em;
}

.toggle-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.toggle-row--icon-mode {
  justify-content: center;
}

.toggle-icon-cluster {
  display: flex;
  align-items: center;
  justify-content: center;
}

.toggle-row--enforced .enforced-label {
  flex: 0 1 auto;
  min-width: 0;
  padding-right: 0.75em;
}

.enforced-label {
  user-select: none;
  font-size: 1rem;
}

.toggle-name-container {
  display: flex;
  align-items: center;
}

.tooltip-info-icon {
  font-size: 1.2rem;
  cursor: pointer;
}

.switch {
  position: relative;
  display: inline-block;
  padding-right: 4em;
  height: 34px;
}

.switch--icon-mode {
  width: 4em;
  height: 34px;
  padding-right: 0;
  flex-shrink: 0;
}

.switch--icon-mode .slider:before {
  content: none;
  display: none;
}

.switch--icon-mode .slider {
  overflow: hidden;
}

.switch--icon-mode .slider-icons {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  box-sizing: border-box;
  padding: 0 0.42em;
  pointer-events: none;
}

.switch--icon-mode .slider-icon--slot {
  flex: 1;
  font-size: 1.15rem;
  line-height: 1;
  text-align: center;
  color: var(--textSecondary);
  opacity: 0.55;
  user-select: none;
  transition: color 0.2s ease, opacity 0.2s ease;
}

.switch--icon-mode .slider-icon--active {
  color: var(--textPrimary);
  opacity: 1;
}

/* Same circular thumb as the default switch (26×26px). */
.switch--icon-mode .slider-knob {
  position: absolute;
  bottom: 4px;
  left: 6px;
  box-sizing: border-box;
  width: 26px;
  height: 26px;
  background-color: white;
  border-radius: 50%;
  transition: transform 0.4s;
  z-index: 2;
  transform: translateX(0);
}

.switch--icon-mode input:checked + .slider .slider-knob {
  transform: translateX(26px);
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
  outline: none;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  transition: 0.4s;
  background-color: gray;
}

.slider:before {
  position: absolute;
  content: "";
  height: 26px;
  width: 26px;
  left: 6px;
  bottom: 4px;
  background-color: white;
  transition: 0.4s;
}

input:checked + .slider {
  background-color: var(--primaryColor);
}

input:focus-visible + .slider {
  box-shadow: 0 0 0 2px var(--primaryColor);
}

input:checked + .slider:before {
  transform: translateX(26px);
}

.toggle-container--neutral .slider {
  background-color: var(--surfaceSecondary);
}

.toggle-container--neutral input:checked + .slider {
  background-color: var(--surfaceSecondary);
}

.toggle-container--neutral input:focus-visible + .slider {
  box-shadow: 0 0 0 2px var(--textSecondary);
}

.switch--icon-mode input:focus + .slider,
.switch--icon-mode input:focus-visible + .slider {
  box-shadow: none;
}

.toggle-container--neutral .slider:before,
.toggle-container--neutral .slider-knob {
  background-color: var(--surfacePrimary);
}

.slider.round {
  border-radius: 50px;
}

.slider.round:before {
  border-radius: 50%;
}

.toggle-row--disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.toggle-row--disabled .toggle-name {
  color: #999;
}

.toggle-row--disabled .slider {
  cursor: not-allowed;
}

.toggle-row--disabled input:disabled + .slider {
  cursor: not-allowed;
  background-color: #ccc;
}

.toggle-row--disabled input:disabled:checked + .slider {
  background-color: #999;
}

.toggle-container--neutral .toggle-row--disabled input:disabled:checked + .slider {
  background-color: #999;
}
</style>
