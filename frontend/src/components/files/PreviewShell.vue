<template>
  <div id="previewer">
    <div v-if="isTransitioning" class="transition-loading">
      <LoadingSpinner size="medium" />
    </div>
    <div class="preview" :class="[{ transitioning: isTransitioning }, previewClass]">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import { state } from "@/store/index.ts";

defineProps<{ previewClass?: Record<string, boolean> }>();
defineSlots<{ default(): unknown }>();

const isTransitioning = computed(() => state.navigation.isTransitioning);
</script>

<style scoped>
.transition-loading {
  position: fixed;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: var(--background);
  z-index: 10000;
  transition: opacity 0.1s ease;
}

.preview.transitioning {
  opacity: 0.3;
  pointer-events: none;
}
</style>
