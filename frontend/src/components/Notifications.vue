<template>
  <div id="notifications-container">
    <transition-group name="notification" tag="div" class="notifications-list">
      <div
        v-for="notification in notifications"
        :key="notification.id"
        :data-notification-id="notification.id"
        :class="['notification-item', 'border-radius', notification.type, { swiping: isSwiping(notification.id) }]"
        :style="swipeStyle(notification.id)"
        @mouseenter="pauseTimer(notification.id, 'hover')"
        @mouseleave="resumeTimer(notification.id, 'hover')"
        @pointerdown="onPointerDown($event, notification.id)"
        @pointermove="onPointerMove($event, notification.id)"
        @pointerup="onPointerUp($event, notification.id)"
        @pointercancel="onPointerUp($event, notification.id)"
      >
        <!-- Close button - always present on every notification, separate from optional buttons array -->
        <i class="material-symbols" @click="closeNotification(notification.id)">close</i>
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
  data: () => ({
    notifications: [],
    swipe: new Map(),
    swipeDistance: 180, // in px
    pause: new Map(),
    selectionId: null
  }),
  computed: {
    swipeCommitDistance() {
      return this.swipeDistance / 15;
    }
  },
  mounted() {
    // Initialize notifications
    this.notifications = notify.getNotifications();
    // Register callback to receive notification updates
    notify.setUpdateCallback((notifications) => {
      this.notifications = notifications;
    });
    window.addEventListener('pointerup', this.handleWindowPointerEnd);
    window.addEventListener('pointercancel', this.handleWindowPointerEnd);
    document.addEventListener('selectionchange', this.handleSelectionChange);
  },
  beforeUnmount() {
    window.removeEventListener('pointerup', this.handleWindowPointerEnd);
    window.removeEventListener('pointercancel', this.handleWindowPointerEnd);
    document.removeEventListener('selectionchange', this.handleSelectionChange);
  },
  methods: {
    closeNotification(notificationId) {
      notify.closeNotification(notificationId);
    },
    closePopUp() {
      return notify.closePopUp();
    },
    handleButtonClick(button) {
      if (typeof button.action === "function") {
        button.action();
      }
    },
    // Hover persistence methods
    pauseAutoClose(notificationId) {
      notify.pauseAutoClose(notificationId);
    },
    resumeAutoClose(notificationId) {
      notify.resumeAutoClose(notificationId);
    },
    // Notifications timer can be paused by hover, text selection and in-progress swipe
    pauseTimer(notificationId, reason) {
      if (!notificationId) return;
      if (!this.pause.has(notificationId)) {
        this.pause.set(notificationId, new Set());
      }
      this.pause.get(notificationId).add(reason);
      this.pauseAutoClose(notificationId);
    },
    resumeTimer(notificationId, reason) {
      if (!notificationId) return;
      const reasons = this.pause.get(notificationId);
      if (!reasons) return;
      reasons.delete(reason);
      if (reasons.size === 0) {
        this.pause.delete(notificationId);
        this.resumeAutoClose(notificationId);
      }
    },
    handleSelectionChange() {
      const selection = window.getSelection();
      const hasText = !!selection && selection.toString().length > 0;
      let activeId = null;

      if (hasText && selection.anchorNode) {
        const anchorEl = selection.anchorNode.nodeType === Node.TEXT_NODE
          ? selection.anchorNode.parentElement
          : selection.anchorNode;
        activeId = anchorEl?.closest?.('.notification-item')?.dataset.notificationId || null;
      }
      if (activeId === this.selectionId) return;
      if (this.selectionId) this.resumeTimer(this.selectionId, 'selection');
      if (activeId) this.pauseTimer(activeId, 'selection');
      this.selectionId = activeId;
    },
    // A swipe only commits if there's no active text selection and the swipe is (obviously) horizontal.
    isSwiping(notificationId) {
      return !!this.swipe.get(notificationId)?.dragging;
    },
    swipeStyle(notificationId) {
      const swipeState = this.swipe.get(notificationId);
      if (!swipeState || (!swipeState.dragging && !swipeState.deltaX)) return {};
      const distance = Math.abs(swipeState.deltaX);
      const fade = Math.max(0, 1 - distance / (swipeState.width || 240));
      return {
        transform: `translateX(${swipeState.deltaX}px)`,
        opacity: fade
      };
    },
    onPointerDown(event, notificationId) {
      if (event.button !== undefined && event.button !== 0) return;
      this.swipe.set(notificationId, {
        element: event.currentTarget,
        startX: event.clientX,
        startY: event.clientY,
        deltaX: 0,
        dragging: false,
        rejected: false,
        width: event.currentTarget.offsetWidth || 240,
        pointerId: event.pointerId
      });
    },
    onPointerMove(event, notificationId) {
      const swipeState = this.swipe.get(notificationId);
      if (!swipeState || swipeState.rejected) return;

      const deltaX = event.clientX - swipeState.startX;
      const deltaY = event.clientY - swipeState.startY;

      if (!swipeState.dragging) {
        // If the browser has already started highlighting text let it continue
        const hasSelection = (window.getSelection?.()?.toString() || '').length > 0;
        if (hasSelection) {
          swipeState.rejected = true;
          this.swipe.set(notificationId, swipeState);
          return;
        }
        const isHorizontal = Math.abs(deltaX) > Math.abs(deltaY) * 1.2;
        if (isHorizontal && Math.abs(deltaX) > this.swipeCommitDistance) {
          swipeState.element.setPointerCapture?.(swipeState.pointerId);
          this.pauseTimer(notificationId, 'swipe');
          swipeState.dragging = true;
        } else if (!isHorizontal && Math.abs(deltaY) > this.swipeCommitDistance) {
          swipeState.rejected = true;
          this.swipe.set(notificationId, swipeState);
          return;
        }
      }
      if (swipeState.dragging) {
        event.preventDefault();
        swipeState.deltaX = deltaX;
      }
      this.swipe.set(notificationId, swipeState);
    },
    onPointerUp(_event, notificationId) {
      const swipeState = this.swipe.get(notificationId);
      if (!swipeState || swipeState.closing) return;
      if (!swipeState.dragging) {
        this.swipe.delete(notificationId);
        return;
      }
      swipeState.element.releasePointerCapture?.(swipeState.pointerId);
      const threshold = Math.max(this.swipeDistance, swipeState.width * 0.55);
      if (Math.abs(swipeState.deltaX) > threshold) {
        const direction = swipeState.deltaX > 0 ? 1 : -1;
        swipeState.dragging = false;
        swipeState.closing = true;
        swipeState.deltaX = direction * swipeState.width * 1.2;
        this.swipe.set(notificationId, swipeState);
        this.pause.delete(notificationId);
        this.closeNotification(notificationId);
        setTimeout(() => {
          this.swipe.delete(notificationId);
        }, 250);
      } else {
        swipeState.dragging = false;
        swipeState.deltaX = 0;
        this.swipe.set(notificationId, swipeState);
        this.resumeTimer(notificationId, 'swipe');
      }
    },
    handleWindowPointerEnd(event) {
      for (const [notificationId, swipeState] of this.swipe) {
        if (swipeState.pointerId === event.pointerId) {
          this.onPointerUp(event, notificationId);
        }
      }
    }
  },
};
</script>

<style>
#notifications-container {
  position: fixed;
  bottom: 0;
  right: 0;
  z-index: 20;
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
  color: #fff;
  position: relative;
  max-width: 90vw;
  height: auto;
  bottom: 0;
  right: 0em;
  display: flex;
  padding: 0.5em;
  align-items: center;
  transition: right 1s ease, transform 0.35s cubic-bezier(0.22, 1, 0.36, 1), opacity 0.35s ease;
  z-index: 21;
  pointer-events: all;
  user-select: text;
  overflow: hidden;
  touch-action: pan-y;
}

.notification-item.swiping {
  transition: none;
  cursor: grabbing;
}

/* selection color for better visibility (since the selection color is the same as --primaryColor) */
.notification-item ::selection {
  background: rgba(255, 255, 255, 0.8);
  color: #000;
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

.notification-item > .material-symbols:first-child {
  cursor: pointer;
  font-size: 1.75em;
  opacity: 0.8;
}

.notification-item > .material-symbols:first-child:hover {
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
