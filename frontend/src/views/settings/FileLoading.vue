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

      <h3 class="section-title">{{ $t("desktopNotifications.title") }}</h3>
      <p v-if="!notificationsSupported" class="notification-hint">
        {{ $t("desktopNotifications.unsupported") }}
      </p>
      <template v-else>
        <p class="notification-hint">{{ $t("desktopNotifications.description") }}</p>
        <p class="notification-permission">
          {{ $t("desktopNotifications.permissionStatus", { status: permissionStatusLabel }) }}
        </p>
        <button
          v-if="showPermissionButton"
          type="button"
          class="button button--flat permission-button"
          @click="requestPermission"
        >
          {{ $t("desktopNotifications.requestPermission") }}
        </button>
        <ToggleSwitch
          class="item"
          v-model="localuser.desktopNotifications.enabled"
          :disabled="!notificationsSupported"
          @change="onNotificationsEnabledChange"
          :name="$t('desktopNotifications.enabled')"
          :description="$t('desktopNotifications.enabledDescription')"
        />
        <ToggleSwitch
          class="item"
          v-model="localuser.desktopNotifications.upload"
          :disabled="!localuser.desktopNotifications.enabled"
          @change="updateSettings"
          :name="$t('desktopNotifications.upload')"
          :description="$t('desktopNotifications.uploadDescription')"
        />
        <ToggleSwitch
          class="item"
          v-model="localuser.desktopNotifications.download"
          :disabled="!localuser.desktopNotifications.enabled"
          @change="updateSettings"
          :name="$t('desktopNotifications.download')"
          :description="$t('desktopNotifications.downloadDescription')"
        />
        <ToggleSwitch
          class="item"
          v-model="localuser.desktopNotifications.moveCopy"
          :disabled="!localuser.desktopNotifications.enabled"
          @change="updateSettings"
          :name="$t('desktopNotifications.moveCopy')"
          :description="$t('desktopNotifications.moveCopyDescription')"
        />
        <ToggleSwitch
          class="item"
          v-model="localuser.desktopNotifications.errors"
          :disabled="!localuser.desktopNotifications.enabled"
          @change="updateSettings"
          :name="$t('desktopNotifications.errors')"
          :description="$t('desktopNotifications.errorsDescription')"
        />
      </template>
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
import {
  defaultDesktopNotificationSettings,
  getNotificationPermission,
  isNotificationSupported,
  requestNotificationPermission,
} from "@/utils/desktopNotifications";

export default {
  name: "fileLoading",
  components: {
    ToggleSwitch,
  },

  data() {
    return {
      localuser: { fileLoading: {}, desktopNotifications: defaultDesktopNotificationSettings() },
      notificationPermission: getNotificationPermission(),
    };
  },
  computed: {
    user() {
      return state.user;
    },
    active() {
      return state.activeSettingsView === "fileLoading-main";
    },
    notificationsSupported() {
      return isNotificationSupported();
    },
    permissionStatusLabel() {
      switch (this.notificationPermission) {
        case "granted":
          return this.$t("desktopNotifications.permissionGranted");
        case "denied":
          return this.$t("desktopNotifications.permissionDenied");
        default:
          return this.$t("desktopNotifications.permissionDefault");
      }
    },
    showPermissionButton() {
      return this.notificationPermission !== "granted" && this.notificationPermission !== "unsupported";
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
    this.localuser.desktopNotifications = {
      ...defaultDesktopNotificationSettings(),
      ...(this.localuser.desktopNotifications || {}),
    };
    this.notificationPermission = getNotificationPermission();
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
    async requestPermission() {
      this.notificationPermission = await requestNotificationPermission();
      if (this.notificationPermission === "granted") {
        notify.showSuccessToast(this.$t("desktopNotifications.permissionGranted"));
      } else if (this.notificationPermission === "denied") {
        notify.showError(this.$t("desktopNotifications.permissionDeniedHelp"));
      }
    },
    async onNotificationsEnabledChange() {
      if (this.localuser.desktopNotifications.enabled && this.notificationPermission !== "granted") {
        await this.requestPermission();
        if (this.notificationPermission !== "granted") {
          this.localuser.desktopNotifications.enabled = false;
        }
      }
      await this.updateSettings();
    },
    async updateSettings(event) {
      if (event !== undefined) {
        event.preventDefault();
      }
      try {
        const data = this.localuser;
        mutations.updateCurrentUser(data);
        await usersApi.update(data, ["fileLoading", "desktopNotifications"]);
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

.section-title {
  width: 100%;
  margin: 0.5em 0 0;
  padding: 0 1em;
  text-align: left;
}

.notification-hint,
.notification-permission {
  width: 100%;
  margin: 0;
  padding: 0 1em 0.5em;
  opacity: 0.85;
  font-size: 0.95em;
}

.permission-button {
  margin: 0 1em 0.5em;
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

.apply-btn {
  margin-left: 1em;
  padding: 0.5em 1em;
  border: none;
  background-color: var(--primaryColor);
  color: white;
  border-radius: 5px;
  cursor: pointer;
  font-size: 0.9em;
}

.apply-btn:hover {
  opacity: 0.9;
}

.item {
  padding: 1em;
}
</style>
