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
 * @property {boolean} [autoclose]
 * @property {ReturnType<typeof setTimeout> | null} [timeoutId]
 */

/** @type {Notification[]} */
let notifications = []

/** @type {((notifications: Notification[]) => void) | null} */
let updateCallback = null

/**
 * Set the callback function to be called when notifications change
 * @param {(notifications: Notification[]) => void} callback
 */
export function setUpdateCallback(callback) {
  updateCallback = callback
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
 * @param {boolean} [options.autoclose=true]
 * @param {string} [options.icon]
 * @param {NotificationButton[]} [options.buttons]
 */
export function showPopup(type, message, options = {}) {
  const {
    autoclose = type !== 'action',
    icon,
    buttons
  } = options

  const notificationId = generateId()
  const parsedMessage = parseMessage(message)

  /** @type {Notification} */
  const notification = {
    id: notificationId,
    type,
    message: parsedMessage,
    icon,
    buttons,
    autoclose,
    timeoutId: null
  }

  notifications.push(notification)
  notifyUpdate()

  // Handle special case for multiple selection
  if (parsedMessage === 'Multiple Selection Enabled' && state.multiple) {
    // This will be handled in closeNotification
  }

  // Set auto-close timeout if applicable
  if (autoclose && type !== 'action') {
    notification.timeoutId = setTimeout(() => {
      closeNotification(notificationId)
    }, 50000)
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

/** @param {unknown} message */
export function showSuccess(message, options = {}) {
  showPopup('success', message, options)
}

/** @param {unknown} message */
export function showError(message, options = {}) {
  showPopup('error', message, options)
  console.error(message)
}

export function showMultipleSelection() {
  showPopup('action', 'Multiple Selection Enabled')
}
