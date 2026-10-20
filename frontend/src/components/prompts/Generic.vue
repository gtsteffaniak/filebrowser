<template>
  <div class="card-content">{{ body }}</div>
  <div class="card-actions">
    <button
      type="button"
      v-for="(button, index) in displayButtons"
      :key="index"
      :class="getButtonClass(button)"
      @click="handleButtonClick(button)"
      :aria-label="button.label"
      :title="button.label"
    >
      {{ button.label }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
// use this component for any prompt to display text message and custom buttons
// body is rendered as plain text, anything with markup should get its own component.

export interface PromptButton {
  label: string;
  action?: () => void;
  className?: string;
}

const props = withDefaults(
  defineProps<{
    promptId?: string | number | null;
    title: string;
    body: string;
    buttons?: PromptButton[];
  }>(),
  {
    promptId: null,
    buttons: () => [],
  },
);

const displayButtons = computed(() => props.buttons);

function handleButtonClick(button: PromptButton) {
  if (typeof button.action === "function") {
    try {
      button.action();
    } catch (error) {
      console.error("Error executing button action:", error);
    }
  }
}

function getButtonClass(button: PromptButton) {
  let classes = "button button--flat";
  if (button.className) {
    classes += ` ${button.className}`;
  }
  return classes;
}
</script>
