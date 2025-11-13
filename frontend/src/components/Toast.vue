<template>
  <transition-group name="toast" tag="div" class="toast-container">
    <div
      v-for="toast in toasts"
      :key="toast.id"
      :class="['toast', `toast--${toast.type}`]"
    >
      <i v-if="toast.icon" :class="getIconClass(toast.icon)">{{ toast.icon }}</i>
      <span class="toast-message">{{ toast.message }}</span>
    </div>
  </transition-group>
</template>

<script>
import { getIconClass } from "@/utils/material-icons";

export default {
  name: "Toast",
  props: {
    toasts: {
      type: Array,
      required: true,
      default: () => [],
    },
  },
  methods: {
    getIconClass,
  },
};
</script>

<style scoped>
.toast-container {
  position: fixed;
  bottom: 2em;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10000;
  display: flex;
  flex-direction: column;
  gap: 0.75em;
  pointer-events: none;
}

.toast {
  padding: 1em;
  display: flex;
  align-items: center;
  gap: 0.5em;
  border-radius: 1em; /* Reuse button border-radius */
  box-shadow: 0 0 5px rgba(0, 0, 0, 0.05), 0 4px 12px rgba(0, 0, 0, 0.2);
  font-weight: 500;
  color: white;
  min-width: 250px;
  max-width: 500px;
  border: 1px solid rgba(0, 0, 0, 0.05);
  transition: .1s ease all; /* Reuse button transition */
}

/* Toast Types - using existing color variables */
.toast--success {
  background: var(--successColor, #4caf50);
}

.toast--error {
  background: var(--errorColor, #f44336);
}

.toast--info {
  background: var(--primaryColor);
}

.toast--warning {
  background: var(--warningColor, #ff9800);
}

.toast i {
  font-size: 1.2em;
  flex-shrink: 0;
}

.toast-message {
  flex: 1;
}

/* Animations */
.toast-enter-active {
  animation: toast-in 0.3s ease-out;
}

.toast-leave-active {
  animation: toast-out 0.3s ease-in;
}

@keyframes toast-in {
  from {
    opacity: 0;
    transform: translateY(1em);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes toast-out {
  from {
    opacity: 1;
    transform: translateY(0);
  }
  to {
    opacity: 0;
    transform: translateY(1em);
  }
}

/* Responsive */
@media (max-width: 768px) {
  .toast-container {
    bottom: 1em;
    left: 1em;
    right: 1em;
    transform: none;
  }

  .toast {
    width: 100%;
    max-width: none;
  }
}
</style>

