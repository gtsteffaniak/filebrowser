import { mutations, state } from '@/store'
import { notify } from '@/notify'
import { globalVars } from '@/utils/constants'
import { filesApi } from '@/api'

let eventSrc = null
let reconnectTimeout = null
let isManuallyClosed = false
let authenticationFailed = false

async function updateSourceInfo () {
  try {
    const sourceinfo = await filesApi.sources()
    mutations.updateSourceInfo(sourceinfo)
  } catch (err) {
    mutations.updateSourceInfo('error')
  }
}

function cleanup () {
  if (eventSrc) {
    isManuallyClosed = true
    eventSrc.close()
    eventSrc = null
  }
}

function scheduleReconnect () {
  // Don't reconnect if authentication has failed
  if (authenticationFailed) {
    console.log('🚫 Not reconnecting due to authentication failure')
    return
  }

  reconnectTimeout = setTimeout(() => {
    console.log('🔁 Attempting SSE reconnect...')
    setupSSE()
  }, 5000)
}

function clearReconnect () {
  if (reconnectTimeout) {
    clearTimeout(reconnectTimeout)
    reconnectTimeout = null
  }
}

// Test the events endpoint to check for authentication before setting up EventSource
async function testEventsEndpoint() {
  const url = `${globalVars.baseURL}api/events?sessionId=${state.sessionId}`
  try {
    const response = await fetch(url, {
      method: 'GET',
      credentials: 'same-origin', // Ensure cookies are sent for SSE authentication
      headers: {
        'Accept': 'text/event-stream',
        'Cache-Control': 'no-cache'
      }
    })

    if (response.status === 401) {
      console.log('🚫 Events endpoint returned 401, authentication failed')
      authenticationFailed = true
      return false
    }

    // Close the test connection immediately
    response.body?.cancel()
    return true
  } catch (error) {
    // For network errors (like ERR_CONNECTION_REFUSED), we'll try the EventSource anyway
    // Only actual 401 responses should stop reconnection
    return true
  }
}

async function setupSSE () {
  // Only test authentication if we haven't already failed
  if (!authenticationFailed) {
    const isAuthenticated = await testEventsEndpoint()
    if (!isAuthenticated) {
      console.log('🚫 Authentication failed, not setting up EventSource')
      authenticationFailed = true
      notify.showError('Authentication failed. Please refresh the page to log in again.')
      return
    }
  }

  const url = `${globalVars.baseURL}api/events?sessionId=${state.sessionId}`
  eventSrc = new EventSource(url)
  isManuallyClosed = false

  eventSrc.onopen = () => {
    if (!state.realtimeActive) {
      console.log('✅ SSE connected')
    }
    if (state.realtimeDownCount > 1) {
      notify.showSuccess('Reconnected to server.')
    }
    clearReconnect()
    mutations.setRealtimeActive(true)
    updateSourceInfo()
    // Reset authentication failure flag on successful connection
    authenticationFailed = false
  }

  eventSrc.onmessage = event => {
    try {
      const msg = JSON.parse(event.data)
      eventRouter(msg.eventType, msg.message)
    } catch (err) {
      console.error('Error parsing SSE:', err, event.data)
    }
  }

  eventSrc.onerror = e => {
    console.warn('❌ SSE connection error', e)
    cleanup()
    mutations.setRealtimeActive(false)
    mutations.updateSourceInfo('error')

    // Don't reconnect if authentication has failed
    if (authenticationFailed) {
      console.log('🚫 Not reconnecting due to authentication failure')
      return
    }

    // Original notification logic - only show error after multiple failures
    if (state.realtimeDownCount == 2 && !isManuallyClosed) {
      notify.showError('The connection to server was lost. Trying to reconnect...')
    }
    scheduleReconnect()
  }
}

export function startSSE () {
  // Reset authentication failure flag when starting SSE
  authenticationFailed = false
  setupSSE()
}

export function startOnlyOfficeSSE () {
  // Reset authentication failure flag when starting SSE
  authenticationFailed = false
  setupSSE()
}

export function stopSSE () {
  cleanup()
}

async function eventRouter (eventType, message) {
  switch (eventType) {
    case 'notification':
      if (message === 'the server is shutting down') {
        notify.showError('Server is shutting down. Reconnecting...')
        mutations.setRealtimeActive(false)
        cleanup()
        scheduleReconnect()
      }
      break

    case 'watchDirChange':
      mutations.setWatchDirChangeAvailable(message)
      break

    case 'sourceUpdate':
      mutations.updateSourceInfo(message)
      break

    case 'acknowledge':
      if (!state.realtimeActive) {
        notify.showSuccess('Reconnected to server.')
      }
      mutations.setRealtimeActive(true)
      break

    case 'onlyOfficeLog':
      // Dispatch custom event for OnlyOffice logs
      try {
        // message is already a parsed object, not a JSON string
        const logData = message
        window.dispatchEvent(new CustomEvent('onlyOfficeLogEvent', { detail: logData }))
      } catch (error) {
        console.error('Error dispatching OnlyOffice log event:', error)
      }
      break

    default:
      console.log('Unknown SSE event:', eventType, message)
  }
}

