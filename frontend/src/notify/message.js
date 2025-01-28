import { mutations, state } from '@/store'

export function showPopup (type, message) {
  const [popup, popupContent] = getElements()
  if (popup === undefined) {
    return
  }
  popup.classList.remove('success', 'error') // Clear previous types
  popup.classList.add(type)

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


}

export function closePopUp () {
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
  let spinner = document.querySelectorAll('.notification-spinner')[0]
  let ctx = spinner.getContext('2d')
  let width = spinner.width
  let height = spinner.height
  let degrees = 0
  let new_degrees = 0
  let difference = 0
  let color = '#7d0e9e'
  let bgcolor = '#222'
  let text
  let animation_loop

  if (animation_loop !== undefined) clearInterval(animation_loop)
  degrees = from * 3.6 // Convert percentage to degrees
  new_degrees = to * 3.6 // Convert percentage to degrees
  difference = new_degrees - degrees
  let duration = 300 // Duration of the animation in ms
  let increment = difference / (duration / 10) // Calculate increment per 10ms
  animation_loop = setInterval(function () {
    if (
      (increment > 0 && degrees >= new_degrees) ||
      (increment < 0 && degrees <= new_degrees)
    ) {
      clearInterval(animation_loop)
    } else {
      degrees += increment
      ctx.clearRect(0, 0, width, height)
      // Background circle
      ctx.beginPath()
      ctx.strokeStyle = bgcolor
      ctx.lineWidth = 30
      ctx.arc(width / 2, width / 2, 100, 0, Math.PI * 2, false)
      ctx.stroke()
      // Foreground circle
      let radians = (degrees * Math.PI) / 180
      ctx.beginPath()
      ctx.strokeStyle = color
      ctx.lineWidth = 30
      ctx.arc(
        width / 2,
        height / 2,
        100,
        0 - (90 * Math.PI) / 180,
        radians - (90 * Math.PI) / 180,
        false
      )
      ctx.stroke()
      // Text
      ctx.fillStyle = color
      ctx.font = '50px Arial'
      text = Math.floor((degrees / 360) * 100) + '%'
      let text_width = ctx.measureText(text).width
      ctx.fillText(text, width / 2 - text_width / 2, height / 2 + 15)
    }
  }, 10) // Update every 10ms
}
