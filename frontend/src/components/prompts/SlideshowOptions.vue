<template>
  <div class="card-content slideshow-options-content settings-items">
    <div class="settings-number-input item">
      <div class="no-padding">
        <label for="slideshow-duration">{{ $t("slideshow.duration") }}</label>
        <HelpTooltipIcon :text="$t('slideshow.durationDescription')" />
      </div>
      <div class="no-padding">
        <input
          id="slideshow-duration"
          v-model.number="form.duration"
          class="sizeInput input"
          type="number"
          min="1"
          max="3600"
          :placeholder="$t('general.number')"
        />
      </div>
    </div>

    <ToggleSwitch class="item" v-model="form.fade" :name="$t('slideshow.fadeTransition')" />

    <ToggleSwitch class="item" v-model="form.fitToScreen" :name="$t('slideshow.fitToScreen')"
      :description="$t('slideshow.fitToScreenDescription')" />

    <ToggleSwitch class="item" v-model="form.enlargeToFill" :name="$t('slideshow.enlargeToFill')"
      :description="$t('slideshow.enlargeToFillDescription')" />

    <ToggleSwitch class="item" v-model="form.shuffle" :name="$t('slideshow.shuffle')" />
    <ToggleSwitch class="item" v-model="form.loop" :name="$t('slideshow.loop')" />
    <ToggleSwitch class="item" v-model="form.kenBurns" :name="$t('slideshow.kenBurns')" />
    <ToggleSwitch class="item" v-model="form.includeVideos" :name="$t('slideshow.includeVideos')"
      :description="$t('slideshow.includeVideosDescription')" />
    <ToggleSwitch class="item" v-model="form.playVideosToEnd" :name="$t('slideshow.playVideosToEnd')" :description="$t('slideshow.playVideosToEndDescription')" />
    <ToggleSwitch class="item" v-model="form.showFilename" :name="$t('slideshow.showFilename')" />
    <ToggleSwitch class="item" v-model="form.startFullscreen" :name="$t('slideshow.startFullscreen')" />

    <template v-if="!isShare">
      <ToggleSwitch
        class="item"
        v-model="form.includeSubfolders"
        :name="$t('slideshow.includeSubfolders')"
        :description="$t('slideshow.includeSubfoldersDescription')"
      />
      <div v-if="form.includeSubfolders" class="setting-row item">
        <div class="setting-label">
          <label for="slideshow-max-items">{{ $t("slideshow.maxItems") }}</label>
          <HelpTooltipIcon :text="$t('slideshow.maxItemsHint', { max: maxItemsCap })" />
        </div>
        <div class="setting-control">
          <input
            id="slideshow-max-items"
            type="number"
            class="input"
            v-model.number="form.maxItems"
            min="1"
            :max="maxItemsCap"
            @blur="clampMaxItems"
          />
        </div>
      </div>
    </template>

  </div>
  <div class="card-actions">
    <button type="button" class="button button--flat" @click="startSlideshow"
      :aria-label="$t('slideshow.startSlideshow')" :title="$t('slideshow.startSlideshow')">
      {{ $t("slideshow.startSlideshow") }}
    </button>
  </div>
</template>

<script>
import { getters, mutations, state } from "@/store";
import { router } from "@/router/index.ts";
import { removeLastDir } from "@/utils/url.js";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import HelpTooltipIcon from "@/components/HelpTooltipIcon.vue";
import {
  slideshowConfig,
  saveSlideshowConfig,
  SLIDESHOW_DEFAULTS,
  effectiveMaxItemsCap,
} from "@/utils/slideshowConfig.js";
import { globalVars } from "@/utils/constants";

export default {
  name: "SlideshowOptions",
  components: { ToggleSwitch, HelpTooltipIcon },
  props: {
    promptId: { type: [String, Number], default: undefined },
    // Name of the image the user opened, so the slideshow starts there.
    startName: { type: String, default: "" },
  },
  data() {
    return {
      form: { ...slideshowConfig },
    };
  },
  computed: {
    isShare() {
      return getters.isShare();
    },
    maxItemsCap() {
      return effectiveMaxItemsCap(globalVars.searchResultsLimit);
    },
  },
  methods: {
    clampMaxItems() {
      const value = Number(this.form.maxItems);
      if (!Number.isFinite(value) || value < 1) {
        this.form.maxItems = SLIDESHOW_DEFAULTS.maxItems;
      } else if (value > this.maxItemsCap) {
        this.form.maxItems = this.maxItemsCap;
      } else {
        this.form.maxItems = Math.floor(value);
      }
    },
    startSlideshow() {
      this.clampMaxItems();
      const duration = Number(this.form.duration);
      saveSlideshowConfig({
        ...this.form,
        duration: Number.isFinite(duration) ? Math.min(3600, Math.max(1, Math.round(duration))) : SLIDESHOW_DEFAULTS.duration,
        includeSubfolders: this.isShare ? false : this.form.includeSubfolders,
      });

      const query = { slideshow: "1" };
      if (this.startName) query.start = this.startName;

      mutations.closeTopPrompt(this.promptId ?? undefined);
      router.push({ path: removeLastDir(state.route.path), query });
    },
  },
};
</script>

<style scoped>
.slideshow-options-content {
  min-width: 18em;
}

/* Two-column number row, matching the settings-number-input layout in FileLoading.vue. */
.settings-number-input {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 1em;
}

.settings-number-input > div {
  display: flex;
  align-items: center;
  padding: 0.5em;
}

.no-padding {
  padding: 0 !important;
}

.sizeInput {
  width: 6em;
}
</style>
