<template>
  <span 
    class="loader" 
    :class="[`loader--${size}`, mode !== 'spinner' ? `loader--${mode}` : '']"
    :style="{
      '--animation-duration': `${speed || (mode === 'spinner' ? 1 : 0.75)}s`,
      '--spinner-color': color
    }"
  >
    <span
      v-if="mode === 'spinner'"
      class="loader__comet"
      aria-hidden="true"
    ></span>
    <span v-if="mode === 'bounce'" class="bounce-dot"></span>
    <span v-if="mode === 'bounce'" class="bounce-dot"></span>
    <div v-if="mode === 'placeholder'" class="placeholder-item placeholder-item-1"></div>
    <div v-if="mode === 'placeholder'" class="placeholder-item placeholder-item-2"></div>
    <div v-if="mode === 'placeholder'" class="placeholder-item placeholder-item-3"></div>
  </span>
</template>

<script>
export default {
  name: "LoadingSpinner",
  props: {
    mode: {
      type: String,
      default: "spinner",
      validator: (value) => ["spinner", "pulse", "bounce", "placeholder"].includes(value),
    },
    size: {
      type: String,
      default: "medium",
      validator: (value) => ["xsmall", "small", "medium", "large"].includes(value),
    },
    speed: {
      type: Number,
      default: null,
      validator: (value) => value === null || value > 0,
    },
    color: {
      type: String,
      default: "var(--primaryColor)",
    },
  },
};
</script>
