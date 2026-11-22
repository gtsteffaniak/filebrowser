<template>
  <div class="info">
    <div class="title">
      <i class="material-symbols">{{ icon }}</i>
      {{ title }}
    </div>
    <slot />
    <div v-if="downloadUrl || $slots.actions" class="preview-buttons">
      <slot name="actions" />
      <a
        v-if="downloadUrl"
        target="_blank"
        :href="downloadUrl"
        class="button button--flat preview-action--icon-only"
        :aria-label="t('general.download', { suffix: '' })"
      >
        <i class="material-symbols">file_download</i>
      </a>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from "@/i18n/index.ts";

defineProps<{
  icon: string;
  title: string;
  downloadUrl?: string;
}>();

defineSlots<{ default?(): unknown; actions?(): unknown }>();

const { t } = useI18n();
</script>

<style scoped>
.preview-buttons {
  display: flex;
  flex-direction: row;
  gap: 1em;
}
</style>
