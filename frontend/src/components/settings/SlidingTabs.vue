<template>
  <div
    class="sliding-tabs"
    :class="{ 'sliding-tabs--icon-only': iconOnly }"
    :role="iconOnly ? 'radiogroup' : 'tablist'"
    :style="{ '--tab-count': tabs.length, '--tab-index': activeIndex }"
  >
    <button
      v-for="tab in tabs"
      :key="tab.id"
      type="button"
      class="sliding-tab"
      :class="{ active: tab.id === modelValue }"
      :role="iconOnly ? 'radio' : 'tab'"
      :aria-selected="iconOnly ? undefined : tab.id === modelValue"
      :aria-checked="iconOnly ? tab.id === modelValue : undefined"
      :aria-label="tab.label ? undefined : tab.title"
      :title="tab.title ?? tab.label"
      @click="emit('update:modelValue', tab.id)"
    >
      <i v-if="tab.icon" class="material-symbols">{{ tab.icon }}</i>
      <span v-if="tab.label" class="sliding-tab-label">{{ tab.label }}</span>
      <span v-if="tab.suffix" class="sliding-tab-suffix">{{ tab.suffix }}</span>
    </button>
    <div v-if="activeIndex >= 0" class="sliding-tabs-indicator" aria-hidden="true"></div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";

export interface SlidingTab {
  id: string;
  /** Text of the tab. iconOnly tabs omit it and use only title */
  label?: string;
  icon?: string;
  suffix?: string;
  title?: string;
}

const props = withDefaults(
  defineProps<{
    tabs: SlidingTab[];
    modelValue: string;
    iconOnly?: boolean;
  }>(),
  { iconOnly: false }
);

const emit = defineEmits<{ "update:modelValue": [id: string] }>();
const activeIndex = computed(() => props.tabs.findIndex((tab) => tab.id === props.modelValue));
</script>

<style scoped>
.sliding-tabs {
  --tab-gap: 0.5em;
  --tab-count: 1; /* set inline, from the tabs */
  --tab-index: 0; /* set inline, from the active tab */

  position: relative;
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: minmax(0, 1fr);
  gap: var(--tab-gap);
}

.sliding-tabs--icon-only {
  --tab-gap: 0px;

  background: var(--surfaceSecondary);
  border-radius: 2em;
  overflow: hidden;
}

.sliding-tab {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.3em;
  min-width: 0;
  padding: 0.5em;
  border: none;
  border-radius: var(--borderRadius);
  background: transparent;
  color: var(--textSecondary);
  cursor: pointer;
  font-size: 0.9rem;
  white-space: nowrap;
  user-select: none;
  transition: color 0.2s ease, transform 0.2s ease;
}

.sliding-tab > i,
.sliding-tab-suffix {
  flex-shrink: 0;
}

.sliding-tabs--icon-only .sliding-tab {
  padding: 0.35rem 0.75rem;
  border-radius: 0;
}

.sliding-tabs--icon-only .sliding-tab > i {
  font-size: 1.5rem;
}

.sliding-tab.active {
  color: white;
}

.sliding-tab:hover:not(.active) {
  color: var(--primaryColor);
  transform: scale(1.02);
}

.sliding-tab-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sliding-tabs-indicator {
  position: absolute;
  inset: 0 auto 0 0;
  z-index: 0;
  width: calc((100% - (var(--tab-count) - 1) * var(--tab-gap)) / var(--tab-count));
  transform: translateX(calc(var(--tab-index) * (100% + var(--tab-gap))));
  border-radius: inherit;
  background: var(--primaryColor);
  pointer-events: none;
  transition: transform 0.35s cubic-bezier(0.25, 0.8, 0.25, 1),
              width 0.35s cubic-bezier(0.25, 0.8, 0.25, 1);
}

.sliding-tabs:not(.sliding-tabs--icon-only) .sliding-tabs-indicator {
  border-radius: var(--borderRadius);
}
</style>
