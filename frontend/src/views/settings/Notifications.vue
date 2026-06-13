<template>
  <div class="notifications-view">
    <div class="card-title">
      <h2>{{ $t("notifications.title") }}</h2>
    </div>
    <div class="card-content app-notifications-settings">
      <p v-if="!notificationsSupported" class="settings-hint">
        {{ $t("notifications.appUnsupported") }}
      </p>
      <template v-else>
        <ToggleSwitch
          class="item"
          v-model="appNotificationsEnabled"
          @change="onAppNotificationsChange"
          :name="$t('notifications.appEnabled')"
          :description="$t('notifications.appEnabledDescription')"
        />
      </template>
    </div>

    <div class="notifications-history">
      <p class="notifications-description">{{ $t("notifications.description") }}</p>
      <div v-if="sortedNotifications.length > 0" class="header-actions">
        <button
          type="button"
          @click="clearHistory"
          class="button button--flat button--grey clear-button"
        >
          <i class="material-symbols">delete_sweep</i>
          {{ $t("notifications.clearAll") }}
        </button>
        <span class="notification-count">
          {{ formatNotificationCount(sortedNotifications.length) }}
        </span>
      </div>

      <div v-if="sortedNotifications.length === 0" class="empty-state">
        <i class="material-symbols empty-icon">notifications_none</i>
        <p>{{ $t("notifications.empty") }}</p>
      </div>

      <div v-else class="notifications-scroll-container">
        <div class="notifications-list">
          <div v-for="notification in sortedNotifications" :key="notification.id" class="notification-wrapper">
            <div :class="['notification-item', notification.type]">
              <div class="notification-content-wrapper">
                <div class="notification-header">
                  <i v-if="notification.icon" class="material-symbols notification-icon">
                    {{ notification.icon }}
                  </i>
                  <div class="notification-message">{{ notification.message }}</div>
                </div>
                <div v-if="notification.buttons && notification.buttons.length > 0" class="notification-buttons">
                  <button
                    type="button"
                    v-for="(button, btnIndex) in notification.buttons"
                    :key="btnIndex"
                    :class="['button', button.className]"
                    @click="handleButtonClick(button)"
                    :aria-label="button.label"
                    :title="button.label"
                  >
                    {{ button.label }}
                  </button>
                </div>
              </div>
            </div>

            <div class="notification-metadata">
              <span class="notification-time">{{ formatTimestamp(notification.timestamp) }}</span>
              <span class="notification-type">{{ notification.type }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { state } from "@/store";
import { fromNow } from "@/utils/moment";
import { notify } from "@/notify";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import {
  formatNotificationCount,
  getNotificationPermission,
  isAppNotificationsEnabled,
  isNotificationSupported,
  requestNotificationPermission,
  setAppNotificationsEnabled,
} from "@/utils/appNotifications";
import { restoreNotificationButtonAction } from "@/utils/notificationActions";

export default {
  name: "Notifications",
  components: {
    ToggleSwitch,
  },
  data() {
    return {
      appNotificationsEnabled: isAppNotificationsEnabled(),
      notificationPermission: getNotificationPermission(),
    };
  },
  computed: {
    sortedNotifications() {
      const activeNotifications = notify.getNotifications();
      return [...state.notificationHistory]
        .map((entry) => ({
          ...entry,
          buttons: notify.resolveHistoryNotificationButtons(
            entry.buttons,
            activeNotifications.find((n) => n.id === entry.id)?.buttons
          ),
        }))
        .sort((a, b) => b.timestamp - a.timestamp);
    },
    notificationsSupported() {
      return isNotificationSupported();
    },
  },
  mounted() {
    this.appNotificationsEnabled = isAppNotificationsEnabled();
    this.notificationPermission = getNotificationPermission();
  },
  methods: {
    formatNotificationCount,
    formatTimestamp(timestamp) {
      return fromNow(timestamp, state.user.locale);
    },
    handleButtonClick(button) {
      const action = button._action || restoreNotificationButtonAction(button);
      if (typeof action === "function") {
        action();
      }
    },
    clearHistory() {
      state.notificationHistory = [];
      try {
        sessionStorage.removeItem("notificationHistory");
      } catch (error) {
        console.error("Failed to clear notification history:", error);
      }
    },
    async requestPermission() {
      this.notificationPermission = await requestNotificationPermission();
      if (this.notificationPermission === "granted") {
        notify.showSuccessToast(this.$t("notifications.permissionGranted"));
      } else if (this.notificationPermission === "denied") {
        notify.showError(this.$t("notifications.permissionDeniedHelp"));
      }
    },
    async onAppNotificationsChange() {
      if (this.appNotificationsEnabled && this.notificationPermission !== "granted") {
        await this.requestPermission();
        if (this.notificationPermission !== "granted") {
          this.appNotificationsEnabled = false;
        }
      }
      setAppNotificationsEnabled(this.appNotificationsEnabled);
    },
  },
};
</script>

<style scoped>
.notifications-view {
  max-width: 60em;
  margin: 0 auto;
  padding: 2em;
}

.card-title h2 {
  font-size: 1.5em;
  font-weight: 500;
  margin: 0;
}

.app-notifications-settings {
  margin-bottom: 1.5em;
}

.settings-hint {
  margin: 0;
  padding: 0 1em 0.5em;
  opacity: 0.85;
  font-size: 0.95em;
  color: var(--textSecondary);
}

.item {
  padding: 1em;
}

.notifications-history {
  margin-top: 0.5em;
}

.notifications-description {
  color: var(--textSecondary);
  margin: 0 0 1em 0;
}

.header-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.5em;
  flex-shrink: 0;
  margin-bottom: 1em;
}

.empty-state {
  text-align: center;
  padding: 4em 2em;
  color: var(--textSecondary);
}

.empty-icon {
  font-size: 4em;
  opacity: 0.5;
  margin-bottom: 0.5em;
}

.empty-state p {
  font-size: 1.1em;
}

.notifications-scroll-container {
  flex: 1;
  overflow-y: auto;
  max-height: calc(100vh - 20em);
  padding-right: 0.5em;
}

.notifications-list {
  display: flex;
  flex-direction: column;
  gap: 1em;
}

.notification-wrapper {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
  width: 100%;
}

.notification-item {
  border-radius: 1em;
  color: #fff;
  position: relative;
  width: 100%;
  height: auto;
  display: flex;
  padding: 0.5em;
  align-items: center;
  z-index: 7;
}

.notification-content-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.notification-header {
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.notification-message {
  color: white;
  padding: 1em;
  flex: 1;
  word-wrap: break-word;
  user-select: text;
  cursor: text;
}

.notification-icon {
  font-size: 1.5em;
  opacity: 0.9;
  flex-shrink: 0;
}

.notification-buttons {
  display: flex;
  gap: 0.5em;
  flex-wrap: wrap;
  margin-top: 0.25em;
}

.notification-buttons .button {
  color: white !important;
  border-color: rgba(255, 255, 255, 0.3);
}

.notification-buttons .button:hover {
  background: rgba(255, 255, 255, 0.1);
  border-color: rgba(255, 255, 255, 0.5);
}

.notification-item.success {
  background: var(--primaryColor);
}

.notification-item.error {
  background: var(--red);
}

.notification-item.action {
  background: var(--primaryColor);
}

.notification-metadata {
  display: flex;
  gap: 1em;
  align-items: center;
  padding: 0.5em 1em;
  font-size: 0.875em;
  color: var(--textSecondary);
  user-select: text;
  cursor: text;
}

.notification-time,
.notification-type {
  color: var(--textSecondary);
  user-select: text;
  cursor: text;
}

.notification-type {
  text-transform: uppercase;
  font-weight: 500;
}

.clear-button {
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.notification-count {
  color: var(--textSecondary);
  font-size: 0.875em;
}

@media (max-width: 768px) {
  .notifications-view {
    padding: 1em;
  }

  .header-actions {
    align-items: stretch;
  }

  .notification-item {
    padding: 0.75em;
  }

  .notification-icon {
    font-size: 1.25em;
  }

  .notification-metadata {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.25em;
  }

  .clear-button {
    width: 100%;
    justify-content: center;
  }

  .notification-count {
    text-align: center;
    width: 100%;
  }
}
</style>
