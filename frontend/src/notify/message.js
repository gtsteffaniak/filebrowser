import { mutations, state } from '@/store'

let active = false
let closeTimeout // Store timeout ID

export function showPopup(type, message, autoclose = true) {
  if (active) {
    clearTimeout(closeTimeout) // Clear the existing timeout
  }

  const [popup, popupContent] = getElements()
  if (popup === undefined) {
    return
  }
  popup.classList.remove('success', 'error') // Clear previous types
  popup.classList.add(type)
  active = true

  let apiMessage

  try {
    apiMessage = JSON.parse(message)
    if (
      apiMessage &&
      Object.prototype.hasOwnProperty.call(apiMessage, 'status') &&
      Object.prototype.hasOwnProperty.call(apiMessage, 'message')
    ) {
      popupContent.textContent =
        'Error ' + apiMessage.status + ': ' + apiMessage.message
    }
  } catch (error) {
    popupContent.textContent = message
  }

  popup.style.right = '0em'

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
  if (popupContent == undefined) {
    return
  }
  if (
    popupContent.textContent == 'Multiple Selection Enabled' &&
    state.multiple
  ) {
    mutations.setMultiple(false)
  }
  popup.style.right = '-50em' // Slide out
  popupContent.textContent = 'no content'
}

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

export function showSuccess(message) {
  showPopup('success', message)
}

export function showError(message) {
  showPopup('error', message)
  console.error(message)
}

export function showMultipleSelection() {
  showPopup('action', 'Multiple Selection Enabled')
}
