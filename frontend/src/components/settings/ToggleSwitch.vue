<template>
  <div class="toggle-container">
    <div class="toggle-name-container">
      <span class="toggle-name">{{ name }}</span>
      <i
        v-if="description"
        class="no-select material-symbols-outlined tooltip-info-icon"
        @mouseenter="showTooltip"
        @mouseleave="hideTooltip"
      >
        help
      </i>
    </div>
    <label class="switch">
      <input type="checkbox" :checked="modelValue" @change="updateValue" :aria-label="ariaLabel" />
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
</style>
