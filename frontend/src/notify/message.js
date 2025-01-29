import { mutations, state } from '@/store'

let active = false

export function showPopup (type, message, autoclose = true) {
  if (active) {
    closePopUp()
  }
  const [popup, popupContent] = getElements()
  if (popup === undefined) {
    return
  }
  // Get the spinner canvas element
  let spinner = document.querySelector('.notification-spinner')
  if (spinner) {
    spinner.classList.add('hidden')
  }

  popup.classList.remove('success', 'error') // Clear previous types
  popup.classList.add(type)
  active = true

  let apiMessage

  try {
    apiMessage = JSON.parse(message)
    // Check if 'apiMessage' has 'status' and 'message' properties
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

  popup.style.right = '1em'

  // don't hide for actions
  if (type === 'action') {
    popup.classList.add('success')
    return
  }
  if (!autoclose || !active) {
    return
  }
  setTimeout(() => {
    if (active) {
      closePopUp()
    }
  }, 5000)
}

export function closePopUp () {
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

function getElements () {
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

export function showSuccess (message) {
  showPopup('success', message)
}

export function showError (message) {
  showPopup('error', message)
  console.error(message)
}

export function showMultipleSelection () {
  showPopup('action', 'Multiple Selection Enabled')
}


export function startLoading (from, to) {
  if (from == to) {
    return
  }

  console.log('startLoading', from, to)
  // Get the spinner canvas element
  let spinner = document.querySelector('.notification-spinner')
  if (!spinner) {
    console.error('Spinner canvas element not found')
    return
  }
  spinner.classList.remove('hidden')

  // Get the 2D context of the canvas
  let ctx = spinner.getContext('2d')
  if (!ctx) {
    console.error('Could not get 2D context')
    return
  }

  // Set canvas dimensions
  let width = spinner.width
  let height = spinner.height

  // Initialize variables
  let degrees = from * 3.6 // Convert percentage to degrees
  let new_degrees = to * 3.6 // Convert percentage to degrees
  let difference = new_degrees - degrees
  let color = spinner.style.color || '#ddd'
  let bgcolor = '#222'
  let animation_loop

  // Clear any existing animation loop
  if (animation_loop !== undefined) clearInterval(animation_loop)

  // Calculate the increment per 10ms
  let duration = 300 // Duration of the animation in ms
  let increment = difference / (duration / 10)

  // Start the animation loop
  animation_loop = setInterval(function () {
    // Check if the animation should stop
    if (
      (increment > 0 && degrees >= new_degrees) ||
      (increment < 0 && degrees <= new_degrees)
    ) {
      clearInterval(animation_loop)
      return
    }

    // Update the degrees
    degrees += increment

    // Clear the canvas
    ctx.clearRect(0, 0, width, height)

    // Draw the background circle
    ctx.beginPath()
    ctx.strokeStyle = bgcolor
    ctx.lineWidth = 10
    ctx.arc(width / 2, height / 2, height / 3, 0, Math.PI * 2, false)
    ctx.stroke()

    // Draw the foreground circle
    let radians = (degrees * Math.PI) / 180
    ctx.beginPath()
    ctx.strokeStyle = color
    ctx.lineWidth = 10
    ctx.arc(
      width / 2,
      height / 2,
      height / 3,
      0 - (90 * Math.PI) / 180,
      radians - (90 * Math.PI) / 180,
      false
    )
    ctx.stroke()

    // Draw the text
    ctx.fillStyle = color
    ctx.font = '1.2em Roboto'
    let text = Math.floor((degrees / 360) * 100) + '%'
    let text_width = ctx.measureText(text).width
    ctx.fillText(text, width / 2 - text_width / 2, height / 2 + 8)
  }, 10) // Update every 10ms
}
