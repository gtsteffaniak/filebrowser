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
  let color = spinner.style.color || '#fff'
  let bgcolor = '#666'
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
    ctx.fillStyle = '#fff'
    ctx.font = '1.2em Roboto'
    let text = Math.floor((degrees / 360) * 100) + '%'
    let text_width = ctx.measureText(text).width
    ctx.fillText(text, width / 2 - text_width / 2, height / 2 + 8)
  }, 10) // Update every 10ms
}
