<template>
  <div class="notifications-view">
    <div class="card-title">
      <h1>{{ $t("notifications.title") }}</h1>
      <p class="notifications-description">{{ $t("notifications.description") }}</p>
      <div v-if="sortedNotifications.length > 0" class="header-actions">
        <button @click="clearHistory" class="button button--flat button--grey clear-button">
          <i class="material-icons">delete_sweep</i>
          {{ $t("notifications.clearAll") }}
        </button>
        <span class="notification-count">
          {{ $t("notifications.total", { count: sortedNotifications.length }) }}
        </span>
      </div>
    </div>

    <div v-if="sortedNotifications.length === 0" class="empty-state">
      <i class="material-icons empty-icon">notifications_none</i>
      <p>{{ $t("notifications.empty") }}</p>
    </div>

    <div v-else class="notifications-scroll-container">
      <div class="notifications-list">
        <div v-for="notification in sortedNotifications" :key="notification.id" class="notification-wrapper">
          <!-- Reuse the exact notification item structure from popup component -->
          <div :class="['notification-item', notification.type]">
            <div class="notification-content-wrapper">
              <div class="notification-header">
                <i v-if="notification.icon" class="material-icons notification-icon">
                  {{ notification.icon }}
                </i>
                <div class="notification-message">{{ notification.message }}</div>
              </div>
              <div v-if="notification.buttons && notification.buttons.length > 0" class="notification-buttons">
                <button v-for="(button, btnIndex) in notification.buttons" :key="btnIndex"
                  :class="['button', button.className]" @click="handleButtonClick(button)" :aria-label="button.label"
                  :title="button.label">
                  {{ button.label }}
                </button>
              </div>
            </div>
          </div>

          <!-- Metadata wrapper below each notification -->
          <div class="notification-metadata">
            <span class="notification-time">{{ formatTimestamp(notification.timestamp) }}</span>
            <span class="notification-type">{{ notification.type }}</span>
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

export default {
  name: "Notifications",
  computed: {
    sortedNotifications() {
      // Return notifications sorted by timestamp (newest first)
      const history = [...state.notificationHistory];

      // Try to restore button actions from active notifications if they still exist
      // This works for notifications that are still in memory (same session)
      const activeNotifications = notify.getNotifications();
      history.forEach(historyEntry => {
        if (historyEntry.buttons) {
          // Try to find matching active notification
          const activeNotification = activeNotifications.find(n => n.id === historyEntry.id);
          if (activeNotification && activeNotification.buttons) {
            // Restore actions from active notification
            historyEntry.buttons.forEach((historyButton, index) => {
              const activeButton = activeNotification.buttons[index];
              if (activeButton && typeof activeButton.action === 'function') {
                historyButton._action = activeButton.action;
              }
            });
          }
        }
      });

      return history.sort((a, b) => b.timestamp - a.timestamp);
    },
  },
  methods: {
    formatTimestamp(timestamp) {
      return fromNow(timestamp, state.user.locale);
    },
    handleButtonClick(button) {
      // Try to use the action function if available
      // _action is stored in memory for current session
      // action is the original property name
      const action = button._action || button.action;
      if (typeof action === "function") {
        action();
      } else if (button.actionType && button.actionData) {
        // Try to recreate action based on stored metadata
        // This is a fallback for actions that need to be recreated
        console.warn('Button action cannot be restored from history. Action type:', button.actionType);
      }
    },
    clearHistory() {
      state.notificationHistory = [];
      // Also clear from sessionStorage
      try {
        sessionStorage.removeItem('notificationHistory');
      } catch (error) {
        console.error('Failed to clear notification history:', error);
      }
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

.notifications-header {
  margin-bottom: 1.5em;
}

.header-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 1em;
}

.notifications-header h1 {
  font-size: 2em;
  font-weight: 500;
  color: var(--textPrimary);
  margin: 0 0 0.5em 0;
}

.notifications-description {
  color: var(--textSecondary);
  margin: 0;
}

.header-actions {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.5em;
  flex-shrink: 0;
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

/* Scrollable container for notifications */
.notifications-scroll-container {
  flex: 1;
  overflow-y: auto;
  max-height: calc(100vh - 12em);
  padding-right: 0.5em;
}

.notifications-list {
  display: flex;
  flex-direction: column;
  gap: 1em;
}

/* Wrapper for each notification with its metadata */
.notification-wrapper {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
  width: 100%;
}

/* Reuse exact styles from popup notification component */
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

/* Override button colors for notifications - buttons should be white on colored backgrounds */
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

/* Metadata wrapper below notification */
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

.notification-time {
  color: var(--textSecondary);
  user-select: text;
  cursor: text;
}

.notification-type {
  text-transform: uppercase;
  font-weight: 500;
  color: var(--textSecondary);
  user-select: text;
  cursor: text;
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

/* Responsive */
@media (max-width: 768px) {
  .notifications-view {
    padding: 1em;
  }

  .header-top {
    flex-direction: column;
    align-items: stretch;
  }

  .header-actions {
    align-items: stretch;
    margin-top: 1em;
  }

  .notifications-header h1 {
    font-size: 1.5em;
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
