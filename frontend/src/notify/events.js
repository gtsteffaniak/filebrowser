import { mutations, state } from '@/store'
import { notify } from '@/notify'
import { baseURL } from '@/utils/constants'
import { filesApi } from '@/api'

let eventSrc = null
let reconnectTimeout = null
let isManuallyClosed = false

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

function setupSSE () {
  const url = `${baseURL}api/events?sessionId=${state.sessionId}`
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

    if (state.realtimeDownCount === 2) {
      notify.showError(
        'The connection to server was lost. Trying to reconnect...'
      )
    }

    scheduleReconnect()
  }
}

export function startSSE () {
  setupSSE()
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

    default:
      console.log('Unknown SSE event:', eventType, message)
  }
}
