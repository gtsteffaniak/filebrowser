<template>
  <div class="preview-notice">
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
.preview-notice {
  display: flex;
  height: 100%;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  box-sizing: border-box;
  padding: 2.5em;
  font-size: 1.5em;
  color: #fff;
  text-align: center;
}

.title {
  margin-bottom: 1em;
  max-width: 100%;
}

.title i {
  display: block;
  margin-bottom: 0.1em;
  font-size: 4em;
}

.preview-buttons {
  display: flex;
  flex-direction: row;
  gap: 1em;
}

.preview-buttons :deep(.button--flat) {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.preview-buttons :deep(.button--flat:hover) {
  background-color: rgb(255 255 255 / 20%);
}

.preview-buttons :deep(.button--flat i) {
  display: block;
  margin-bottom: 4px;
  font-size: 1.3em;
}

.preview-buttons :deep(.button--flat.preview-action--icon-only i) {
  margin-bottom: 0;
}
</style>
