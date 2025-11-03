<template>
  <div id="notifications-container">
    <transition-group name="notification" tag="div" class="notifications-list">
      <div
        v-for="notification in notifications"
        :key="notification.id"
        :class="['notification-item', notification.type]"
      >
        <!-- Close button - always present on every notification, separate from optional buttons array -->
        <i class="material-icons" @click="closeNotification(notification.id)">close</i>
        <div class="notification-content-wrapper">
          <div class="notification-header">
            <i v-if="notification.icon" class="material-icons notification-icon">
              {{ notification.icon }}
            </i>
            <div class="notification-message">{{ notification.message }}</div>
          </div>
          <div v-if="notification.buttons && notification.buttons.length > 0" class="notification-buttons">
            <button
              v-for="(button, btnIndex) in notification.buttons"
              :key="btnIndex"
              :class="['button', button.className]"
              @click="handleButtonClick(button, notification.id)"
              :aria-label="button.label"
              :title="button.label"
            >
              {{ button.label }}
            </button>
          </div>
        </div>
      </div>
    </transition-group>
  </div>
</template>

<script>
import { notify } from "@/notify";

export default {
  name: "notifications",
  data: function () {
    return {
      notifications: [],
    };
  },
  mounted() {
    // Register callback to receive notification updates
    notify.setUpdateCallback((notifications) => {
      this.notifications = notifications;
    });
    // Initialize with current notifications
    this.notifications = notify.getNotifications();
  },
  methods: {
    closeNotification(notificationId) {
      notify.closeNotification(notificationId);
    },
    closePopUp() {
      return notify.closePopUp();
    },
    handleButtonClick(button, notificationId) {
      if (typeof button.action === "function") {
        button.action();
      }
      if (button.keepOpen !== true) {
        this.closeNotification(notificationId);
      }
    },
  },
};
</script>

<style>
#notifications-container {
  position: fixed;
  bottom: 0;
  right: 0;
  z-index: 6;
  pointer-events: none;
  margin: 1em;
}

.notifications-list {
  display: flex;
  flex-direction: column-reverse;
  align-items: flex-end;
  gap: 0.5em;
}

.notification-item {
  border-radius: 1em;
  color: #fff;
  position: relative;
  max-width: 90vw;
  height: auto;
  bottom: 0;
  right: 0em;
  display: flex;
  padding: 0.5em;
  align-items: center;
  transition: right 1s ease;
  z-index: 5;
  pointer-events: all;
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

.notification-item > .material-icons:first-child {
  cursor: pointer;
  font-size: 1.75em;
  margin-right: 0.5em;
  opacity: 0.8;
}

.notification-item > .material-icons:first-child:hover {
  opacity: 1;
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

canvas.notification-spinner {
  color: #fff;
}

/* Slide-in animation */
.notification-enter-active {
  transition: right 1s ease;
}

.notification-leave-active {
  transition: right 1s ease;
}

.notification-enter-from {
  right: -50em;
}

.notification-enter-to {
  right: 0em;
}

.notification-leave-from {
  right: 0em;
}

.notification-leave-to {
  right: -50em;
}

.notification-move {
  transition: transform 0.3s ease;
}
</style>
