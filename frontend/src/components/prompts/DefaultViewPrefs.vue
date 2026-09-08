<template>
  <div class="card-content view-defaults settings-items">
    <p>{{ $t('profileSettings.defaultViewModeDescription') }}</p>
    <div class="setting-row item">
      <label id="default-view-mode-label">{{ $t('profileSettings.defaultViewMode') }}</label>
      <ExpandDropdown
        v-model="localViewMode"
        :options="viewModeOptions"
        :aria-label="$t('profileSettings.defaultViewMode')"
      />
    </div>
    <div class="setting-row slider-row item">
      <label for="default-gallery-size">{{ $t('general.size') }}</label>
      <div class="setting-row">
        <input id="default-gallery-size" type="range" min="1" max="9" v-model.number="localGallerySize" />
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

export default {
  name: "default-view-prefs",
  components: {
    ExpandDropdown,
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

.slider-row.item {
  padding-top: 0.75em;
  padding-bottom: 0.75em;
}
</style>
