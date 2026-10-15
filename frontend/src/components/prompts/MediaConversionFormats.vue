<template>
  <div class="card-content">
    <div v-if="loading" class="loading-hint">
      <LoadingSpinner size="medium" />
    </div>
    <template v-else>
      <div v-if="hasImagePreview" class="settings-items">
        <h3>{{ $t("settings.imagePreview") }}</h3>
        <ToggleSwitch
          v-for="(value, key) in media.convert.imagePreview"
          :key="`img-${key}`"
          class="item"
          v-model="media.convert.imagePreview[key]"
          :disabled="isLocked(`convert.imagePreview.${key}`)"
          :value-tooltip="fieldTooltip(`convert.imagePreview.${key}`)"
          :name="String(key)"
        />
      </div>
      <div v-if="hasVideoPreview" class="settings-items">
        <h3>{{ $t("settings.videoPreview") }}</h3>
        <ToggleSwitch
          v-for="(value, key) in media.convert.videoPreview"
          :key="`vid-${key}`"
          class="item"
          v-model="media.convert.videoPreview[key]"
          :disabled="isLocked(`convert.videoPreview.${key}`)"
          :value-tooltip="fieldTooltip(`convert.videoPreview.${key}`)"
          :name="String(key)"
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
      :aria-label="$t('general.cancel')"
      :title="$t('general.cancel')"
    >
      {{ $t("general.cancel") }}
    </button>
    <button
      type="button"
      class="button button--flat button--blue"
      :disabled="saving || loading"
      @click="save"
      :aria-label="$t('general.save')"
      :title="$t('general.save')"
    >
      {{ $t("general.save") }}
    </button>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { mutations } from "@/store";
import { eventBus } from "@/store/eventBus";
import { getMediaSettings, patchMediaSettings } from "@/api/settings";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import LoadingSpinner from "@/components/LoadingSpinner.vue";
import {
  MEDIA_SETTINGS_CHANGED_EVENT,
  emptyMediaSettings,
  mergeMediaSettings,
} from "@/utils/mediaSettings";

export default {
  name: "MediaConversionFormats",
  components: {
    ToggleSwitch,
    LoadingSpinner,
  },
  props: {
    promptId: {
      type: [String, Number],
      default: null,
    },
    /** Media settings pinned in the config file cannot be changed here. */
    lockedFromConfigPaths: {
      type: Array,
      default: () => [],
    },
    lockMessage: {
      type: String,
      default: "",
    },
  },
  data() {
    return {
      media: emptyMediaSettings(),
      loading: true,
      saving: false,
    };
  },
  computed: {
    hasImagePreview() {
      return Object.keys(this.media.convert.imagePreview || {}).length > 0;
    },
    hasVideoPreview() {
      return Object.keys(this.media.convert.videoPreview || {}).length > 0;
    },
  },
  async mounted() {
    try {
      this.media = mergeMediaSettings(await getMediaSettings());
    } catch (e) {
      notify.showError(e);
      console.error(e);
    } finally {
      this.loading = false;
    }
  },
  methods: {
    isLocked(path) {
      return this.lockedFromConfigPaths.includes(path);
    },
    fieldTooltip(path) {
      return this.isLocked(path) ? this.$t("settings.mediaFieldLockedFromConfig") : "";
    },
    close() {
      mutations.closeTopPrompt();
    },
    async save() {
      if (this.saving || this.loading) {
        return;
      }
      this.saving = true;
      try {
        await patchMediaSettings({ convert: this.media.convert });
        notify.showSuccess(this.$t("general.saved"));
        eventBus.emit(MEDIA_SETTINGS_CHANGED_EVENT);
        this.close();
      } catch (e) {
        notify.showError(e);
        console.error(e);
      } finally {
        this.saving = false;
      }
    },
  },
};
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
