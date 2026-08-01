<template>
  <div
    class="toggle-container"
    :class="{
      'toggle-container--enforceable': enforceable,
    }"
  >
    <div
      class="toggle-row toggle-row--value"
      :class="{ 'toggle-row--disabled': disabled, 'border-radius': enforceable }"
      @mouseenter="showValueRowTooltipIfNeeded"
      @mouseleave="hideTooltip"
    >
      <div class="toggle-name-container">
        <span class="toggle-name">{{ name }}</span>
        <i
          v-if="description"
          class="material-symbols-outlined tooltip-info-icon"
          @mouseenter="showTooltip"
          @mouseleave="hideTooltip"
        >
          help
        </i>
      </div>
      <label class="switch">
        <input
          type="checkbox"
          :checked="modelValue"
          @change="updateValue"
          :aria-label="ariaLabel"
          :disabled="disabled"
        />
        <span class="slider round"></span>
      </label>
    </div>
    <div
      v-if="enforceable"
      class="toggle-row toggle-row--enforced border-radius"
      :class="{ 'toggle-row--disabled': enforcementDisabled }"
    >
      <label class="enforced-label" :for="enforcedInputId">{{ enforcedLabelText }}</label>
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
import { mutations } from "@/store";

let enforcedIdCounter = 0;

export default {
  name: "ToggleSwitch",
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
  },
  data() {
    enforcedIdCounter += 1;
    return {
      enforcedInputId: `toggle-enforced-${enforcedIdCounter}`,
    };
  },
  computed: {
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
    showTooltip(event) {
      if (this.description) {
        mutations.showTooltip({
          content: this.description,
          x: event.clientX,
          y: event.clientY,
        });
      }
    },
    showValueRowTooltipIfNeeded(event) {
      if (!this.disabled) {
        return;
      }
      if (this.valueTooltip) {
        mutations.showTooltip({
          content: this.valueTooltip,
          x: event.clientX,
          y: event.clientY,
        });
        return;
      }
      if (this.enforcementLocked) {
        mutations.showTooltip({
          content: this.$t("profileSettings.enforcedByAdmin"),
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
}

.toggle-container--enforceable .toggle-row {
  box-sizing: border-box;
  min-height: 3.25em;
  padding: 0.5em 1em;
  transition: background-color 0.15s ease;
}

.toggle-container--enforceable .toggle-row:hover {
  background-color: var(--surfaceSecondary);
}

.toggle-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.toggle-row--enforced .enforced-label {
  flex: 1;
  min-width: 0;
  padding-right: 0.75em;
}

.enforced-label {
  cursor: pointer;
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

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
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

input:focus + .slider {
  box-shadow: 0 0 1px var(--primaryColor);
}

input:checked + .slider:before {
  transform: translateX(26px);
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
</style>
