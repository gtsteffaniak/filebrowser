<template>
  <span 
    class="loader" 
    :class="[`loader--${size}`, mode !== 'spinner' ? `loader--${mode}` : '']"
    :style="{
      '--animation-duration': `${speed || (mode === 'spinner' ? 1 : 0.75)}s`,
      '--spinner-color': color
    }"
  >
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

<style scoped>
/* ============================================
   LOADING SPINNER - Simple CSS Spinner
   ============================================ */
.loader {
    width: 48px;
    height: 48px;
    border-radius: 50%;
    display: block;
    border-top: 3px solid var(--spinner-color, var(--primaryColor));
    border-right: 3px solid transparent;
    border-bottom: 3px solid transparent;
    box-sizing: border-box;
    animation: spinner-rotation var(--animation-duration, 1s) linear infinite;
    margin: 0 auto;
}

@keyframes spinner-rotation {
    0% {
        transform: rotate(0deg);
    }
    100% {
        transform: rotate(360deg);
    }
}

/* ============================================
   SIZE VARIANTS
   ============================================ */
.loader--xsmall {
    width: 12px;
    height: 12px;
    border-width: 1.5px;
}

.loader--small {
    width: 24px;
    height: 24px;
    border-width: 2px;
}

.loader--medium {
    width: 48px;
    height: 48px;
    border-width: 3px;
}

.loader--large {
    width: 96px;
    height: 96px;
    border-width: 4px;
}

/* ============================================
   PULSE MODE
   ============================================ */
.loader--pulse {
    border: none;
    background: var(--spinner-color, var(--primaryColor));
    animation: pulse-scale var(--animation-duration, 0.75s) ease infinite;
}

@keyframes pulse-scale {
    0%, 100% {
        transform: scale(1);
        opacity: 1;
    }
    50% {
        transform: scale(0.95);
        opacity: 0.8;
    }
}

/* ============================================
   BOUNCE MODE
   ============================================ */
.loader--bounce {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.3em;
    border: none;
    background: transparent;
    animation: none;
}

.loader--bounce::before,
.loader--bounce::after {
    content: '';
    width: 0.6em;
    height: 0.6em;
    background-color: var(--spinner-color, var(--primaryColor));
    border-radius: 50%;
    display: inline-block;
    animation: bounce-scale var(--animation-duration, 0.75s) infinite ease-in-out both;
}

.loader--bounce::before {
    animation-delay: -0.32s;
}

.loader--bounce::after {
    animation-delay: -0.16s;
}

.loader--bounce .bounce-dot {
    width: 0.6em;
    height: 0.6em;
    background-color: var(--spinner-color, var(--primaryColor));
    border-radius: 50%;
    display: inline-block;
    animation: bounce-scale var(--animation-duration, 0.75s) infinite ease-in-out both;
}

@keyframes bounce-scale {
    0%, 80%, 100% {
        transform: scale(0);
        opacity: 0.5;
    }
    40% {
        transform: scale(1);
        opacity: 1;
    }
}

/* Size variants for bounce mode */
.loader--bounce.loader--xsmall::before,
.loader--bounce.loader--xsmall::after,
.loader--bounce.loader--xsmall .bounce-dot {
    width: 0.3em;
    height: 0.3em;
}

.loader--bounce.loader--small::before,
.loader--bounce.loader--small::after,
.loader--bounce.loader--small .bounce-dot {
    width: 0.45em;
    height: 0.45em;
}

.loader--bounce.loader--medium::before,
.loader--bounce.loader--medium::after,
.loader--bounce.loader--medium .bounce-dot {
    width: 0.6em;
    height: 0.6em;
}

.loader--bounce.loader--large::before,
.loader--bounce.loader--large::after,
.loader--bounce.loader--large .bounce-dot {
    width: 0.9em;
    height: 0.9em;
}

/* ============================================
   PLACEHOLDER MODE
   ============================================ */
.loader--placeholder {
    display: flex;
    flex-direction: column;
    gap: 0.5em;
    border: none;
    animation: none;
    width: auto;
    height: auto;
    border-radius: 0;
}

.placeholder-item {
    background-color: rgba(128, 128, 128, 0.1);
    animation: placeholder-ripple 1.8s cubic-bezier(0.25, 0.46, 0.45, 0.94) infinite,
               placeholder-bg-fade 2s ease-in-out infinite;
    display: block;
    border-radius: 0.25em;
    margin: 0;
    padding: 0;
    transition: transform 0.4s cubic-bezier(0.25, 0.46, 0.45, 0.94),
                box-shadow 0.4s cubic-bezier(0.25, 0.46, 0.45, 0.94);
}

.placeholder-item-1 {
    animation-delay: 0s;
}

.placeholder-item-2 {
    animation-delay: 0.36s;
}

.placeholder-item-3 {
    animation-delay: 0.72s;
}

@keyframes placeholder-ripple {
    0% {
        transform: scale(1);
        box-shadow: none;
        opacity: 0.3;
    }
    25% {
        transform: scale(1.07);
        box-shadow: inset 0 -3em 3em rgba(172, 172, 172, 0.211),
                    0 0 0 2px var(--alt-background),
                    0 4px 12px rgba(0, 0, 0, 0.15);
        opacity: 0.6;
    }
    50% {
        transform: scale(0.97);
        box-shadow: none;
        opacity: 0.3;
    }
    100% {
        transform: scale(1);
        box-shadow: none;
        opacity: 0.3;
    }
}

@keyframes placeholder-bg-fade {
    0% {
        background-color: rgba(128, 128, 128, 0.1);
    }
    50% {
        background-color: rgba(128, 128, 128, 0.4);
    }
    100% {
        background-color: rgba(128, 128, 128, 0.1);
    }
}

/* Size variants for placeholder mode */
.loader--placeholder.loader--xsmall .placeholder-item {
    width: 30px;
    height: 7.5px;
    min-width: 30px;
    min-height: 7.5px;
}

.loader--placeholder.loader--small .placeholder-item {
    width: 150px;
    height: 12.5px;
    min-width: 150px;
    min-height: 12.5px;
}

.loader--placeholder.loader--medium .placeholder-item {
    width: 200px;
    height: 25px;
    min-width: 200px;
    min-height: 25px;
}

.loader--placeholder.loader--large .placeholder-item {
    width: 400px;
    height: 50px;
    min-width: 400px;
    min-height: 50px;
}
</style>
