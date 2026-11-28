<template>
  <div class="card-content">
    <div v-if="loading" class="loading-hint">
      <LoadingSpinner size="medium" />
    </div>
    <template v-else>
      <div v-if="hasImagePreview" class="settings-items">
        <h3>{{ t("media.imagePreview") }}</h3>
        <ToggleSwitch
          v-for="(value, key) in media.convert.imagePreview"
          :key="`img-${key}`"
          class="item"
          :model-value="value"
          @update:model-value="media.convert.imagePreview[key] = $event"
          :disabled="isLocked(`convert.imagePreview.${key}`)"
          :value-tooltip="fieldTooltip(`convert.imagePreview.${key}`)"
          :name="key"
        />
      </div>
      <div v-if="hasVideoPreview" class="settings-items">
        <h3>{{ t("media.videoPreview") }}</h3>
        <ToggleSwitch
          v-for="(value, key) in media.convert.videoPreview"
          :key="`vid-${key}`"
          class="item"
          :model-value="value"
          @update:model-value="media.convert.videoPreview[key] = $event"
          :disabled="isLocked(`convert.videoPreview.${key}`)"
          :value-tooltip="fieldTooltip(`convert.videoPreview.${key}`)"
          :name="key"
        />
      </div>
    </template>
  </div>

  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--grey"
      :disabled="saving"
      @click="close"
      :aria-label="t('general.cancel')"
      :title="t('general.cancel')"
    >
      {{ t("general.cancel") }}
    </button>
    <button
      type="button"
      class="button button--flat button--blue"
      :disabled="saving || loading"
      @click="save"
      :aria-label="t('general.save')"
      :title="t('general.save')"
    >
      {{ t("general.save") }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useI18n } from "@/i18n/index.ts";
import { notify } from "@/notify/index.ts";
import { mutations } from "@/store/index.ts";
import { eventBus } from "@/store/eventBus.ts";
import { getMediaSettings } from "@/api/settings.ts";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import type { MediaSettings } from "@/types/media.ts";
import {
  MEDIA_SETTINGS_CHANGED_EVENT,
  emptyMediaSettings,
  mergeMediaSettings,
  saveMediaSettingsPatch,
} from "@/utils/config/mediaSettings.ts";

const props = withDefaults(
  defineProps<{
    promptId?: string | number | null;
    /** Media settings pinned in the config file cannot be changed here. */
    lockedFromConfigPaths?: string[];
    lockMessage?: string;
  }>(),
  { promptId: null, lockedFromConfigPaths: () => [], lockMessage: "" },
);

const { t } = useI18n();

const media = ref<MediaSettings>(emptyMediaSettings());
const loading = ref(true);
const saving = ref(false);

const hasImagePreview = computed(() => Object.keys(media.value.convert.imagePreview).length > 0);
const hasVideoPreview = computed(() => Object.keys(media.value.convert.videoPreview).length > 0);

function isLocked(path: string): boolean {
  return props.lockedFromConfigPaths.includes(path);
}

function fieldTooltip(path: string): string {
  return isLocked(path) ? t("settings.lockedFromConfig") : "";
}

function close(): void {
  mutations.closeTopPrompt();
}

async function save(): Promise<void> {
  if (saving.value || loading.value) {
    return;
  }
  saving.value = true;
  try {
    const result = await saveMediaSettingsPatch({ convert: media.value.convert }, props.lockedFromConfigPaths);
    if (!result.skipped) {
      media.value = result.media;
    }
    void mutations.syncMediaSettings();
    notify.showSuccess(t("general.saved"));
    eventBus.emit(MEDIA_SETTINGS_CHANGED_EVENT);
    close();
  } catch (e) {
    notify.showError(e);
    console.error(e);
  } finally {
    saving.value = false;
  }
}

onMounted(async () => {
  try {
    media.value = mergeMediaSettings(await getMediaSettings());
  } catch (e) {
    notify.showError(e);
    console.error(e);
  } finally {
    loading.value = false;
  }
});
</script>

<style scoped>
.card-content h3 {
  text-align: center;
}

.loading-hint {
  display: flex;
  justify-content: center;
  padding: 2em;
}

/* Settings pinned in the config file cannot be edited here. */
input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
