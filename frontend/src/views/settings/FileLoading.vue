<template>
  <div class="card-title">
    <h2>{{ $t("fileLoading.title") }}</h2>
  </div>
  <div class="card-content">
    <div class="settings-items">
      <div class="settings-number-input item">
        <div class="no-padding">
          <label for="maxConcurrentUpload">{{ $t("fileLoading.maxConcurrentUpload") }}</label>
          <i class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('fileLoading.maxConcurrentUploadHelp'))" @mouseleave="hideTooltip">
            help <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          </i>
        </div>
        <div>
          <input v-model.number="localuser.fileLoading.maxConcurrentUpload" type="range" min="1" max="10"
            :placeholder="$t('general.number')" />
          <span class="range-value">{{ localuser.fileLoading.maxConcurrentUpload }}</span>
        </div>
      </div>
      <div class="settings-number-input item">
        <div class="no-padding">
          <label for="uploadChunkSizeMb">{{ $t("fileLoading.uploadChunkSizeMb") }}</label>
          <i class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('fileLoading.uploadChunkSizeMbHelp'))" @mouseleave="hideTooltip">
            help <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          </i>
        </div>
        <div class="no-padding">
          <input class="sizeInput input" v-model.number="localuser.fileLoading.uploadChunkSizeMb" type="number" min="0"
            :placeholder="$t('general.number')" />
        </div>
      </div>
      <div class="settings-number-input item">
        <div class="no-padding">
          <label for="downloadChunkSizeMb">{{ $t("fileLoading.downloadChunkSizeMb") }}</label>
          <i class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('fileLoading.downloadChunkSizeMbHelp'))" @mouseleave="hideTooltip">
            help <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
          </i>
        </div>
        <div class="no-padding">
          <input class="sizeInput input" v-model.number="localuser.fileLoading.downloadChunkSizeMb" type="number" min="0"
            :placeholder="$t('general.number')" />
        </div>
      </div>
      <ToggleSwitch class="item" v-model="localuser.fileLoading.clearAll" @change="updateSettings"
        :name="$t('fileLoading.clearAll')"
        :description="$t('fileLoading.clearAllDescription')" />
    </div>
    <div class="card-actions">
      <button
        type="button"
        class="button button--flat"
        @click="updateSettings"
      >
        {{ $t("general.save") }}
      </button>
    </div>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { state, mutations } from "@/store";
import { usersApi } from "@/api";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";

export default {
  name: "fileLoading",
  components: {
    ToggleSwitch,
  },

  data() {
    return {
      localuser: { fileLoading: {} },
    };
  },
  computed: {
    user() {
      return state.user;
    },
    active() {
      return state.activeSettingsView === "fileLoading-main";
    },
  },
  mounted() {
    this.localuser = JSON.parse(JSON.stringify(state.user));
    if (!this.localuser.fileLoading) {
      this.localuser.fileLoading = {};
    }
    if (this.localuser.fileLoading.downloadChunkSizeMb === undefined || this.localuser.fileLoading.downloadChunkSizeMb === null) {
      this.localuser.fileLoading.downloadChunkSizeMb = 0;
    }
  },
  methods: {
    showTooltip(event, text) {
      mutations.showTooltip({
        content: text,
        x: event.clientX,
        y: event.clientY,
      });
    },
    hideTooltip() {
      mutations.hideTooltip();
    },
    async updateSettings(event) {
      if (event !== undefined) {
        event.preventDefault();
      }
      try {
        const data = this.localuser;
        await mutations.updateCurrentUser(data);
        await usersApi.update(data, ["fileLoading"]);
        notify.showSuccessToast(this.$t("settings.settingsUpdated"));
      } catch (e) {
        console.error(e);
      }
    },
  },
};
</script>

<style scoped>
.card-content h3 {
  text-align: center;
}

.settings-number-input {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 1em;
}

.settings-number-input div {
  display: flex;
  padding: 0.5em;
  align-items: center;
}

.range-value {
  margin-left: 1em;
  min-width: 2ch;
  text-align: center;
}

.item {
  padding: 1em;
}
</style>
