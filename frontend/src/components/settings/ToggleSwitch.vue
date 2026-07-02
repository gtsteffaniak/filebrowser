<template>
  <div class="toggle-container" :class="{ 'disabled': disabled }">
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
      <input type="checkbox" :checked="modelValue" @change="updateValue" :aria-label="ariaLabel" :disabled="disabled" />
      <span class="slider round"></span>
    </label>
  </div>
</template>

<script>
import { mutations } from "@/store";

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
  },
  methods: {
    updateValue(event) {
      this.$emit("update:modelValue", event.target.checked);
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
  width: 2.75em;
  height: 1.5em;
  flex-shrink: 0;
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
  transition: background-color 0.2s ease;
  background-color: var(--border-strong);
}

.slider:before {
  position: absolute;
  content: "";
  height: 1.1em;
  width: 1.1em;
  left: 0.2em;
  bottom: 0.2em;
  background-color: white;
  box-shadow: 0 1px 2px rgb(0 0 0 / 0.25);
  transition: transform 0.2s ease;
}

input:checked + .slider {
  background-color: var(--primaryColor);
}

input:focus-visible + .slider {
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primaryColor), transparent 75%);
}

input:checked + .slider:before {
  transform: translateX(1.25em);
}

.slider.round {
  border-radius: 999px;
}

.slider.round:before {
  border-radius: 50%;
}

.toggle-container.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.toggle-container.disabled .toggle-name {
  color: var(--textSecondary);
}

.toggle-container.disabled .slider {
  cursor: not-allowed;
}

input:disabled + .slider {
  cursor: not-allowed;
  background-color: var(--surfaceSecondary);
}

input:disabled:checked + .slider {
  background-color: color-mix(in srgb, var(--primaryColor), transparent 50%);
}
</style>
