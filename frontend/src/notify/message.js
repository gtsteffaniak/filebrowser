import { mutations, state } from '@/store'

let active = false
/** @type {ReturnType<typeof setTimeout> | null} */
let closeTimeout // Store timeout ID

/**
 * Show a popup notification
 * @param {'success' | 'error' | 'action'} type
 * @param {unknown} message
 * @param {boolean} [autoclose=true]
 */
export function showPopup(type, message, autoclose = true) {
  if (active) {
    if (closeTimeout) clearTimeout(closeTimeout) // Clear the existing timeout
  }

  const [popup, popupContent] = getElements()
  if (popup == null || popupContent == null) {
    return
  }
  /** @type {HTMLElement} */
  // @ts-ignore - narrow Element to HTMLElement after null check
  const popupEl = popup
  /** @type {HTMLElement} */
  // @ts-ignore - narrow Element to HTMLElement after null check
  const popupContentEl = popupContent

  popupEl.classList.remove('success', 'error') // Clear previous types
  popupEl.classList.add(type)
  active = true

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
      popupContentEl.textContent =
        apiMessage.status + ': ' + apiMessage.message
    } else {
      // Fallback to showing the normalized message if it is not an API error shape
      popupContentEl.textContent = normalizedMessage
    }
  } catch (error) {
    // Fallback to a safe string representation
    const fallback =
      message instanceof Error
        ? message.message || message.toString()
        : typeof message === 'string'
          ? message
          : JSON.stringify(message)
    popupContentEl.textContent = fallback
  }

  popupEl.style.right = '0em'

  // Don't auto-hide for 'action' type popups
  if (type === 'action') {
    popup.classList.add('success')
    return
  }

  if (!autoclose || !active) {
    return
  }

  // Set a new timeout for closing
  closeTimeout = setTimeout(() => {
    if (active) {
      closePopUp()
    }
  }, 5000)
}

export function closePopUp() {
  active = false
  const [popup, popupContent] = getElements()
  if (popup == null || popupContent == null) {
    return
  }
  /** @type {HTMLElement} */
  // @ts-ignore - narrow Element to HTMLElement after null check
  const popupEl = popup
  /** @type {HTMLElement} */
  // @ts-ignore - narrow Element to HTMLElement after null check
  const popupContentEl = popupContent
  if (
    popupContentEl.textContent == 'Multiple Selection Enabled' &&
    state.multiple
  ) {
    mutations.setMultiple(false)
  }
  popupEl.style.right = '-50em' // Slide out
  popupContentEl.textContent = 'no content'
}

/**
 * @returns {[Element | null, Element | null]}
 */
function getElements() {
  const popup = document.getElementById('popup-notification')
  if (!popup) {
    return [null, null]
  }

  const popupContent = popup.querySelector('#popup-notification-content')
  if (!popupContent) {
    return [null, null]
  }

  return [popup, popupContent]
}

/** @param {unknown} message */
export function showSuccess(message) {
  showPopup('success', message)
}

/** @param {unknown} message */
export function showError(message) {
  showPopup('error', message)
  console.error(message)
}

export function showMultipleSelection() {
  showPopup('action', 'Multiple Selection Enabled')
}
