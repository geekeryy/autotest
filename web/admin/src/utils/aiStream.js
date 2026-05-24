// AI assistant SSE client.
// ========================
//
// This module turns the backend's SSE protocol into an async-friendly
// callback API for the Vue store. We intentionally do not use
// `EventSource` because:
//
//   - EventSource only supports GET, but our chat/confirm endpoints are
//     POST (the body carries the user message / decision).
//   - EventSource does not let us set the `Authorization` header.
//
// Instead we use `fetch()` with `ReadableStream`. The SSE wire format we
// expect mirrors what the backend writes from `aiprovider/assistant_handler.go`:
//
//   event: <kind>\n
//   data: <json>\n
//   \n
//
// Heartbeat frames start with ":" and are silently dropped by browsers
// even when we forward them, but we still skip them explicitly so the
// downstream handler does not see them as events.

import { apiBaseURL } from './apiBase'
import { getToken } from './storage'

/**
 * @typedef {Object} StreamEvent
 * @property {string} kind
 * @property {string} [text]
 * @property {Object} [toolCall]
 * @property {Object} [toolResult]
 * @property {Object} [message]
 * @property {string} [model]
 * @property {string} [finish]
 * @property {string} [error]
 */

/**
 * Open a streaming POST request and dispatch parsed SSE events to the
 * provided callback. The returned promise resolves once the server has
 * closed the stream (either gracefully via `done` event or because the
 * connection dropped).
 *
 * @param {string} url Absolute or relative URL (relative is joined to /api/v1)
 * @param {Object} body JSON body to send
 * @param {(event: StreamEvent) => void} onEvent
 * @param {{ signal?: AbortSignal }} [options]
 */
export async function streamPostSSE(url, body, onEvent, options = {}) {
  const target = absolute(url)
  const token = getToken()
  const headers = {
    'Content-Type': 'application/json',
    Accept: 'text/event-stream',
  }
  if (token) headers.Authorization = `Bearer ${token}`

  const response = await fetch(target, {
    method: 'POST',
    headers,
    body: JSON.stringify(body || {}),
    signal: options.signal,
    // Always omit credentials cross-origin: the dev proxy strips cookies anyway
    // and we want the bearer token to be the only auth surface.
    credentials: 'same-origin',
  })

  if (!response.ok || !response.body) {
    // Try to surface backend error JSON when present; fall back to text.
    let payload = ''
    try {
      payload = await response.text()
    } catch (err) {
      payload = String(err)
    }
    throw new Error(`SSE 请求失败 (${response.status}): ${truncate(payload, 280)}`)
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder('utf-8')
  let buffer = ''

  try {
    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const events = drainFrames(() => {
        const idx = buffer.indexOf('\n\n')
        if (idx === -1) return null
        const frame = buffer.slice(0, idx)
        buffer = buffer.slice(idx + 2)
        return frame
      })
      for (const event of events) {
        if (event) onEvent(event)
      }
    }
  } catch (err) {
    // AbortError is the caller cancelling; surface a soft event so the UI
    // can clean up state without lighting up an error banner.
    if (err?.name === 'AbortError') {
      onEvent({ kind: 'done', finish: 'aborted' })
      return
    }
    throw err
  }
}

// drainFrames repeatedly calls take() until it returns null, parsing each
// returned frame string into a StreamEvent object. Returning an array lets
// the caller iterate predictably and short-circuits on parse failure.
function drainFrames(take) {
  const out = []
  let frame
  while ((frame = take()) !== null) {
    if (!frame) continue
    if (frame.startsWith(':')) continue // heartbeat
    out.push(parseFrame(frame))
  }
  return out
}

function parseFrame(frame) {
  let eventName = ''
  const dataParts = []
  for (const line of frame.split('\n')) {
    if (!line) continue
    if (line.startsWith(':')) continue
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
  } catch (err) {
    return { kind: eventName || 'unknown', error: `parse: ${err.message}`, raw }
  }
}

function absolute(url) {
  if (/^https?:\/\//i.test(url)) return url
  const base = apiBaseURL()
  if (url.startsWith('/')) return base + url
  return `${base}/${url}`
}

function truncate(str, max) {
  if (!str) return ''
  return str.length <= max ? str : `${str.slice(0, max)}…`
}
