<template>
  <div id="notifications-container">
    <transition-group name="notification" tag="div" class="notifications-list">
      <div
        v-for="notification in notifications"
        :key="notification.id"
        :class="['notification-item', notification.type]"
        @mousedown="startDrag($event, notification.id)"
        @touchstart="startDrag($event, notification.id)"
        @mousemove="handleDrag($event, notification.id)"
        @touchmove="handleDrag($event, notification.id)"
        @mouseup="endDrag(notification.id)"
        @touchend="endDrag(notification.id)"
        @mouseenter="pauseAutoClose(notification.id)"
        @mouseleave="handleMouseLeave(notification.id)"
        :style="{
          transform: `translateX(${notification.dragOffset || 0}px)`,
          opacity: notification.dragOpacity !== undefined ? notification.dragOpacity : 1
        }"
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
        <!-- Progress bar for the notifications timeout-->
        <div
          v-if="notification.autoclose && !notification.persistent"
          class="notification-progress-bar"
        >
          <div
            class="notification-progress-fill"
            :style="{
              width: `${notification.progress || 100}%`
            }"
          ></div>
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
      dragState: {
        isDragging: false,
        startX: 0,
        currentX: 0,
        notificationId: null
      }
    };
  },
  mounted() {
    // Initialize notifications with drag state
    this.notifications = notify.getNotifications().map(notification => ({
      ...notification,
      dragOffset: 0,
      dragOpacity: 1
    }));
    // Register callback to receive notification updates
    notify.setUpdateCallback((notifications) => {
      this.notifications = notifications.map(notification => {
        // Find existing notification to preserve drag state
        const existing = this.notifications.find(n => n.id === notification.id);
        return {
          ...notification,
          dragOffset: existing?.dragOffset,
          dragOpacity: existing?.dragOpacity !== undefined ? existing.dragOpacity : 1
        };
      });
    });
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
    startDrag(event, notificationId) {
      const clientX = event.type.includes('touch') ? event.touches[0].clientX : event.clientX;
      this.dragState = {
        isDragging: true,
        startX: clientX,
        currentX: clientX,
        notificationId: notificationId
      };
    },
    handleDrag(event, notificationId) {
      if (!this.dragState.isDragging || this.dragState.notificationId !== notificationId) {
        return;
      }
      event.preventDefault();
      const clientX = event.type.includes('touch') ? event.touches[0].clientX : event.clientX;
      this.dragState.currentX = clientX;
      const deltaX = clientX - this.dragState.startX;
      // Allow dragging in both directions (left and right)
      const dragOffset = deltaX;
      const absoluteDeltaX = Math.abs(deltaX);
      const dragOpacity = Math.max(0.4, 1 - (absoluteDeltaX / 150));
      // Update the notification position and opacity
      const notificationIndex = this.notifications.findIndex(n => n.id === notificationId);
      if (notificationIndex !== -1) {
        this.notifications[notificationIndex].dragOffset = dragOffset;
        this.notifications[notificationIndex].dragOpacity = dragOpacity;
      }
    },
    endDrag(notificationId) {
      if (!this.dragState.isDragging || this.dragState.notificationId !== notificationId) {
        return;
      }
      const deltaX = this.dragState.currentX - this.dragState.startX;
      const absoluteDeltaX = Math.abs(deltaX);
      const notificationIndex = this.notifications.findIndex(n => n.id === notificationId);
      if (notificationIndex !== -1) {
        // Check if drag distance is enought to close the notification (120px or 25% of screen width)
        const threshold = Math.min(120, window.innerWidth * 0.25);
        if (absoluteDeltaX > threshold) {
          // If the swipe is enought, close
          this.closeNotification(notificationId);
        } else {
          // If not, returnt it back to the original position
          this.notifications[notificationIndex].dragOffset = 0;
          this.notifications[notificationIndex].dragOpacity = 1;
        }
      }
      // Reset drag state
      this.dragState = {
        isDragging: false,
        startX: 0,
        currentX: 0,
        notificationId: null
      };
    },
    // Handle mouse leave for both drag and auto-close
    handleMouseLeave(notificationId) {
      this.endDrag(notificationId);
      this.resumeAutoClose(notificationId);
    },
    // Hover persistence methods
    pauseAutoClose(notificationId) {
      notify.pauseAutoClose(notificationId);
    },
    resumeAutoClose(notificationId) {
      notify.resumeAutoClose(notificationId);
    }
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
  user-select: none;
  overflow: hidden;
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
  transition: right 0.3s ease, max-height 0.2s ease, padding 0.2s ease, margin 0.2s ease, opacity 0.2s ease;
  overflow: hidden;
}

.notification-enter-from {
  right: -50em;
}

.notification-enter-to {
  right: 0em;
}

.notification-leave-from {
  right: 0em;
  max-height: 500px;
  opacity: 1;
}

.notification-leave-to {
  right: -50em;
  max-height: 0;
  padding-top: 0;
  padding-bottom: 0;
  margin: 0;
  opacity: 0;
}

.notification-move {
  transition: transform 0.3s ease;
}

/* Progress bar for the remaining timeout */
.notification-progress-bar {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: rgba(255, 255, 255, 0.25);
  overflow: hidden;
}

.notification-progress-fill {
  height: 100%;
  background: rgba(255, 255, 255, 0.7);
  transition: width 0.1s linear;
}
</style>
