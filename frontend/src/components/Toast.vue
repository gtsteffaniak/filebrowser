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
import { getIconClass } from "@/utils/material-symbols";

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
  padding: 0.85em 1em;
  display: flex;
  align-items: center;
  gap: 0.5em;
  border-radius: 0.75rem;
  box-shadow: 0 4px 16px rgb(0 0 0 / 0.18);
  font-weight: 500;
  background: var(--surfacePrimary);
  color: var(--textPrimary);
  min-width: 250px;
  max-width: 500px;
  border: 1px solid var(--divider);
  border-left: 4px solid var(--primaryColor);
  transition: .1s ease all; /* Reuse button transition */
}

/* Toast Types - severity accent bar + icon color */
.toast--success {
  border-left-color: var(--icon-green);
}

.toast--success i {
  color: var(--icon-green);
}

.toast--error {
  border-left-color: var(--red);
}

.toast--error i {
  color: var(--red);
}

.toast--info {
  border-left-color: var(--primaryColor);
}

.toast--info i {
  color: var(--primaryColor);
}

.toast--warning {
  border-left-color: var(--icon-orange);
}

.toast--warning i {
  color: var(--icon-orange);
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

