<template>
  <div class="card-content view-defaults settings-items">
    <div class="setting-row item">
      <div class="label">
        <label id="default-view-mode-label">{{ $t('profileSettings.defaultViewMode') }}</label>
        <HelpTooltipIcon :text="$t('profileSettings.defaultViewModeDescription')" />
      </div>
      <ExpandDropdown
        v-model="localViewMode"
        :options="viewModeOptions"
        :aria-label="$t('profileSettings.defaultViewMode')"
        :disabled="viewModeDisabled"
      />
    </div>
    <div class="setting-row slider-row item">
      <div class="label">
        <label for="default-gallery-size">{{ $t('profileSettings.defaultGallerySize') }}</label>
        <HelpTooltipIcon :text="$t('profileSettings.defaultGallerySizeDescription')" />
      </div>
      <div class="setting-row">
        <input
          id="default-gallery-size"
          type="range"
          min="1"
          max="9"
          v-model.number="localGallerySize"
          :disabled="gallerySizeDisabled"
        />
        <span class="range-value">{{ localGallerySize }}</span>
      </div>
    </div>
  </div>
  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--grey"
      @click="cancel"
      :title="$t('general.cancel')"
      :aria-label="$t('general.cancel')"
    >
      {{ $t('general.cancel') }}
    </button>
    <button
      type="button"
      class="button button--flat"
      @click="save"
      :title="$t('general.save')"
      :aria-label="$t('general.save')"
    >
      {{ $t('general.save') }}
    </button>
  </div>
</template>

<script>
import { getters, mutations } from "@/store";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";

export default {
  name: "default-view-prefs",
  components: {
    ExpandDropdown,
    HelpTooltipIcon,
  },
  props: {
    viewMode: {
      type: String,
      default: "normal",
    },
    gallerySize: {
      type: Number,
      default: 3,
    },
    viewModeDisabled: {
      type: Boolean,
      default: false,
    },
    gallerySizeDisabled: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {
      localViewMode: this.viewMode,
      localGallerySize: this.gallerySize,
    };
  },
  computed: {
    viewModeOptions() {
      return [
        { value: "list", label: this.$t("buttons.listView") },
        { value: "normal", label: this.$t("buttons.normalView") },
        { value: "icons", label: this.$t("buttons.galleryView") },
      ];
    },
  },
  methods: {
    cancel() {
      mutations.closeTopPrompt();
    },
    save() {
      const confirm = getters.currentPrompt()?.confirm;
      const parsedSize = Number(this.localGallerySize);
      const size = Number.isFinite(parsedSize)
        ? Math.min(9, Math.max(1, parsedSize))
        : 3;
      if (typeof confirm === "function") {
        confirm({ viewMode: this.localViewMode, gallerySize: size });
      }
      mutations.closeTopPrompt();
    },
  },
};
</script>

<style scoped>
.view-defaults {
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 0 0.75em;
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1em;
}

.setting-row .expand-dropdown {
  max-width: 13em;
}

.label {
  display: flex;
  align-items: center;
  gap: 0.35em;
}

.slider-row.item {
  padding-top: 0.75em;
  padding-bottom: 0.75em;
}
</style>
