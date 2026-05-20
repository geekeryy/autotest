// Notification SSE client (GET /notifications/stream).
// Uses fetch + ReadableStream so we can send Authorization; reconnects on drop.

import { getToken } from './storage'

const DEFAULT_LIMIT = 20
const MIN_RECONNECT_MS = 1000
const MAX_RECONNECT_MS = 30_000

/**
 * @typedef {Object} NotificationStreamEvent
 * @property {'snapshot'|'notification'} kind
 * @property {number} [unreadCount]
 * @property {Array<Object>} [items]
 * @property {Object} [notification]
 */

/**
 * Subscribe to notification SSE. Returns a stop() function.
 *
 * @param {(event: NotificationStreamEvent) => void} onEvent
 * @param {{ signal?: AbortSignal, limit?: number }} [options]
 */
export function subscribeNotificationStream(onEvent, options = {}) {
  const limit = options.limit ?? DEFAULT_LIMIT
  let stopped = false
  let reconnectMs = MIN_RECONNECT_MS
  let reconnectTimer = null
  let activeAbort = null

  const stop = () => {
    stopped = true
    if (reconnectTimer) {
      window.clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    activeAbort?.abort()
    activeAbort = null
  }

  if (options.signal) {
    options.signal.addEventListener('abort', stop, { once: true })
  }

  const scheduleReconnect = () => {
    if (stopped) return
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null
      connect()
    }, reconnectMs)
    reconnectMs = Math.min(reconnectMs * 2, MAX_RECONNECT_MS)
  }

  const connect = async () => {
    if (stopped) return
    activeAbort?.abort()
    const controller = new AbortController()
    activeAbort = controller
    if (options.signal) {
      options.signal.addEventListener('abort', () => controller.abort(), { once: true })
    }

    const target = absolute(`/notifications/stream?limit=${limit}`)
    const token = getToken()
    const headers = { Accept: 'text/event-stream' }
    if (token) headers.Authorization = `Bearer ${token}`

    try {
      const response = await fetch(target, {
        method: 'GET',
        headers,
        signal: controller.signal,
        credentials: 'same-origin',
      })
      if (!response.ok || !response.body) {
        scheduleReconnect()
        return
      }
      reconnectMs = MIN_RECONNECT_MS

      const reader = response.body.getReader()
      const decoder = new TextDecoder('utf-8')
      let buffer = ''
      while (!stopped) {
        const { value, done } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        for (const event of drainFrames(() => {
          const idx = buffer.indexOf('\n\n')
          if (idx === -1) return null
          const frame = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)
          return frame
        })) {
          if (event) onEvent(event)
        }
      }
      if (!stopped) scheduleReconnect()
    } catch (err) {
      if (err?.name === 'AbortError' || stopped) return
      scheduleReconnect()
    }
  }

  connect()
  return stop
}

function drainFrames(take) {
  const out = []
  let frame
  while ((frame = take()) !== null) {
    if (!frame || frame.startsWith(':')) continue
    out.push(parseFrame(frame))
  }
  return out
}

function parseFrame(frame) {
  let eventName = ''
  const dataParts = []
  for (const line of frame.split('\n')) {
    if (!line || line.startsWith(':')) continue
    if (line.startsWith('event:')) {
      eventName = line.slice(6).trim()
    } else if (line.startsWith('data:')) {
      dataParts.push(line.slice(5).trimStart())
    }
  }
  if (dataParts.length === 0) {
    return { kind: eventName || 'unknown' }
  }
  const raw = dataParts.join('\n')
  try {
    const parsed = JSON.parse(raw)
    if (!parsed.kind && eventName) parsed.kind = eventName
    return parsed
  } catch {
    return { kind: eventName || 'unknown' }
  }
}

function absolute(url) {
  if (/^https?:\/\//i.test(url)) return url
  const base = import.meta.env.VITE_API_BASE_URL || '/api/v1'
  if (url.startsWith('/')) return base + url
  return `${base}/${url}`
}
