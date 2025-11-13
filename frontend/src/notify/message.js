import { mutations, state } from '@/store'

/**
 * @typedef {Object} NotificationButton
 * @property {string} label
 * @property {() => void} action
 * @property {boolean} [keepOpen]
 * @property {boolean} [primary]
 * @property {string} [className]
 */

/**
 * @typedef {Object} Notification
 * @property {string} id
 * @property {'success' | 'error' | 'action'} type
 * @property {string} message
 * @property {string} [icon]
 * @property {NotificationButton[]} [buttons]
 * @property {boolean} [autoclose] - For backward compatibility (inverse of persistent)
 * @property {ReturnType<typeof setTimeout> | null} [timeoutId]
 */

/**
 * @typedef {Object} Toast
 * @property {string} id
 * @property {'success' | 'error' | 'info' | 'warning'} type
 * @property {string} message
 * @property {string} [icon]
 * @property {ReturnType<typeof setTimeout> | null} [timeoutId]
 */

/** @type {Notification[]} */
let notifications = []

/** @type {Toast[]} */
let toasts = []

/** @type {((notifications: Notification[]) => void) | null} */
let updateCallback = null

/** @type {((toasts: Toast[]) => void) | null} */
let toastUpdateCallback = null

/**
 * Set the callback function to be called when notifications change
 * @param {(notifications: Notification[]) => void} callback
 */
export function setUpdateCallback(callback) {
  updateCallback = callback
}

/**
 * Set the callback function to be called when toasts change
 * @param {(toasts: Toast[]) => void} callback
 */
export function setToastUpdateCallback(callback) {
  toastUpdateCallback = callback
}

/**
 * Generate a unique ID for a notification
 * @returns {string}
 */
function generateId() {
  return `notification-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
}

/**
 * Notify listeners that notifications have changed
 */
function notifyUpdate() {
  if (updateCallback) {
    updateCallback([...notifications])
  }
}

/**
 * Notify listeners that toasts have changed
 */
function notifyToastUpdate() {
  if (toastUpdateCallback) {
    toastUpdateCallback([...toasts])
  }
}

/**
 * Parse message to extract display text
 * @param {unknown} message
 * @returns {string}
 */
function parseMessage(message) {
  try {
    // Normalize message to a string first to avoid calling string methods on non-strings
    const normalizedMessage =
      message instanceof Error
        ? message.toString() // e.g. "Error: {...}"
        : typeof message === 'string'
          ? message
          : JSON.stringify(message)

    let apiMessage
    // check if message starts with "Error: "
    if (normalizedMessage.startsWith('Error: ')) {
      const errorMessage = normalizedMessage.replace(/^Error:\s*/, '')
      apiMessage = JSON.parse(errorMessage)
    } else {
      apiMessage = JSON.parse(normalizedMessage)
    }
    if (
      typeof apiMessage === 'object' &&
      Object.prototype.hasOwnProperty.call(apiMessage, 'status') &&
      Object.prototype.hasOwnProperty.call(apiMessage, 'message')
    ) {
      return apiMessage.status + ': ' + apiMessage.message
    } else {
      // Fallback to showing the normalized message if it is not an API error shape
      return normalizedMessage
    }
  } catch (error) {
    // Fallback to a safe string representation
    const fallback =
      message instanceof Error
        ? message.message || message.toString()
        : typeof message === 'string'
          ? message
          : JSON.stringify(message)
    return fallback
  }
}

/**
 * Show a popup notification
 * @param {'success' | 'error' | 'action'} type
 * @param {unknown} message
 * @param {Object} [options]
 * @param {boolean} [options.persistent=false] - If true, notification won't auto-close
 * @param {boolean} [options.autoclose] - Deprecated, use persistent instead
 * @param {string} [options.icon]
 * @param {NotificationButton[]} [options.buttons]
 */
export function showPopup(type, message, options = {}) {
  const {
    persistent = false,
    autoclose,
    icon,
    buttons
  } = options

  // Determine if notification should auto-close
  // Priority: persistent option > autoclose option > default behavior
  let shouldAutoClose
  if (persistent) {
    shouldAutoClose = false
  } else if (autoclose !== undefined) {
    shouldAutoClose = autoclose
  } else {
    // Default: action types don't auto-close, others do
    shouldAutoClose = type !== 'action'
  }

  const notificationId = generateId()
  const parsedMessage = parseMessage(message)

  /** @type {Notification} */
  const notification = {
    id: notificationId,
    type,
    message: parsedMessage,
    icon,
    buttons,
    autoclose: !persistent, // Store the inverse for backward compatibility
    timeoutId: null
  }

  notifications.push(notification)
  notifyUpdate()

  // Save to notification history with timestamp
  // Serialize buttons to preserve action metadata
  const serializedButtons = buttons ? buttons.map(button => ({
    label: button.label,
    className: button.className,
    keepOpen: button.keepOpen,
    primary: button.primary,
    // Store action as metadata if it's a function
    actionType: button.actionType || (typeof button.action === 'function' ? 'function' : null),
    actionData: button.actionData || null,
    // Keep the action function in memory for immediate use
    _action: typeof button.action === 'function' ? button.action : null
  })) : null

  const historyEntry = {
    id: notificationId,
    type,
    message: parsedMessage,
    icon,
    buttons: serializedButtons,
    timestamp: Date.now(),
    persistent: persistent
  }
  state.notificationHistory.push(historyEntry)

  // Persist to sessionStorage (survives page refresh, cleared on tab close)
  // Note: Functions can't be serialized, so _action will be lost, but actionType/actionData are preserved
  try {
    const serializableHistory = state.notificationHistory.map(entry => ({
      ...entry,
      buttons: entry.buttons ? entry.buttons.map(btn => ({
        label: btn.label,
        className: btn.className,
        keepOpen: btn.keepOpen,
        primary: btn.primary,
        actionType: btn.actionType,
        actionData: btn.actionData
        // _action is intentionally omitted as it can't be serialized
      })) : null
    }))
    sessionStorage.setItem('notificationHistory', JSON.stringify(serializableHistory))
  } catch (error) {
    console.error('Failed to save notification history:', error)
  }

  // Set auto-close timeout if applicable
  if (shouldAutoClose) {
    notification.timeoutId = setTimeout(() => {
      closeNotification(notificationId)
    }, 5000)
  }
}

/**
 * Close a specific notification by ID
 * @param {string} notificationId
 */
export function closeNotification(notificationId) {
  const index = notifications.findIndex(n => n.id === notificationId)
  if (index === -1) {
    return
  }

  const notification = notifications[index]

  // Clear timeout if exists
  if (notification.timeoutId) {
    clearTimeout(notification.timeoutId)
  }

  // Handle special case for multiple selection
  if (
    notification.message === 'Multiple Selection Enabled' &&
    state.multiple
  ) {
    mutations.setMultiple(false)
  }

  // Remove from array
  notifications.splice(index, 1)
  notifyUpdate()
}

/**
 * Close all notifications
 */
export function closePopUp() {
  notifications.forEach(notification => {
    if (notification.timeoutId) {
      clearTimeout(notification.timeoutId)
    }
  })

  // Handle multiple selection special case
  if (state.multiple) {
    const multipleNotification = notifications.find(
      n => n.message === 'Multiple Selection Enabled'
    )
    if (multipleNotification) {
      mutations.setMultiple(false)
    }
  }
  notifications = []
  notifyUpdate()
}

/**
 * Get all active notifications
 * @returns {Notification[]}
 */
export function getNotifications() {
  return [...notifications]
}

/**
 * Show a success notification
 * @param {unknown} message
 * @param {Object} [options]
 * @param {boolean} [options.persistent=false] - If true, notification won't auto-close
 * @param {string} [options.icon]
 * @param {NotificationButton[]} [options.buttons]
 */
export function showSuccess(message, options = {}) {
  showPopup('success', message, options)
}

/**
 * Show an error notification
 * @param {unknown} message
 * @param {Object} [options]
 * @param {boolean} [options.persistent=false] - If true, notification won't auto-close
 * @param {string} [options.icon]
 * @param {NotificationButton[]} [options.buttons]
 */
export function showError(message, options = {}) {
  showPopup('error', message, options)
  console.error(message)
}

export function showMultipleSelection() {
  showPopup('success', 'Multiple Selection Enabled', { persistent: true })
}

// ============================================================================
// Toast Notifications
// ============================================================================
// Usage examples:
//   import { notify } from "@/notify";
//   
//   notify.showSuccessToast("File saved!");
//   notify.showErrorToast("Failed to save file");
//   notify.showInfoToast("Processing...");
//   notify.showWarningToast("Disk space is low");
//   
//   // With custom icon and duration:
//   notify.showSuccessToast("Done!", { icon: "check", duration: 3000 });
//   notify.showToast("info", "Custom message", { icon: "star", duration: 5000 });
// ============================================================================

/**
 * Get all active toasts
 * @returns {Toast[]}
 */
export function getToasts() {
  return [...toasts]
}

/**
 * Close a specific toast by ID
 * @param {string} toastId
 */
export function closeToast(toastId) {
  const index = toasts.findIndex(t => t.id === toastId)
  if (index === -1) {
    return
  }

  const toast = toasts[index]

  // Clear timeout if exists
  if (toast.timeoutId) {
    clearTimeout(toast.timeoutId)
  }

  // Remove from array
  toasts.splice(index, 1)
  notifyToastUpdate()
}

/**
 * Show a toast notification
 * @param {'success' | 'error' | 'info' | 'warning'} type
 * @param {string} message
 * @param {Object} [options]
 * @param {string} [options.icon] - Material icon name
 * @param {number} [options.duration=2000] - Duration in milliseconds before auto-close
 */
export function showToast(type, message, options = {}) {
  const {
    icon = getDefaultToastIcon(type),
    duration = 2000
  } = options

  const toastId = generateId()

  /** @type {Toast} */
  const toast = {
    id: toastId,
    type,
    message,
    icon,
    timeoutId: null
  }

  toasts.push(toast)
  notifyToastUpdate()

  // Set auto-close timeout
  if (duration > 0) {
    toast.timeoutId = setTimeout(() => {
      closeToast(toastId)
    }, duration)
  }
}

/**
 * Get default icon for toast type
 * @param {'success' | 'error' | 'info' | 'warning'} type
 * @returns {string}
 */
function getDefaultToastIcon(type) {
  const iconMap = {
    success: 'check_circle',
    error: 'error',
    info: 'info',
    warning: 'warning'
  }
  return iconMap[type] || 'info'
}

/**
 * Show a success toast
 * @param {string} message
 * @param {Object} [options]
 * @param {string} [options.icon]
 * @param {number} [options.duration=2000]
 */
export function showSuccessToast(message, options = {}) {
  showToast('success', message, options)
}

/**
 * Show an error toast
 * @param {string} message
 * @param {Object} [options]
 * @param {string} [options.icon]
 * @param {number} [options.duration=3000]
 */
export function showErrorToast(message, options = {}) {
  showToast('error', message, { duration: 3000, ...options })
}

/**
 * Show an info toast
 * @param {string} message
 * @param {Object} [options]
 * @param {string} [options.icon]
 * @param {number} [options.duration=2000]
 */
export function showInfoToast(message, options = {}) {
  showToast('info', message, options)
}

/**
 * Show a warning toast
 * @param {string} message
 * @param {Object} [options]
 * @param {string} [options.icon]
 * @param {number} [options.duration=2500]
 */
export function showWarningToast(message, options = {}) {
  showToast('warning', message, { duration: 2500, ...options })
}
