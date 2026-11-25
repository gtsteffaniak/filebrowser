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
#previewer {
  background-color: rgb(0 0 0 / 90%);
  width: 100%;
  height: 100%;
  z-index: 9999;
}

.preview {
  text-align: center;
  max-height: 100%;
  max-width: 100%;
  height: 100%;
}

.preview :deep(img),
.preview :deep(video) {
  max-height: 100%;
  margin: 0;
}

.preview :deep(video) {
  height: 100%;
}

.preview :deep(h2.message) {
  color: rgb(255 255 255 / 50%);
}

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
