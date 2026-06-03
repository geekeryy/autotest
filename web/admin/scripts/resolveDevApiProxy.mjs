import os from 'node:os'

const DEFAULT_PORT = '8080'

/**
 * Dev API proxy target for Vite. Cursor (and similar tools) often bind
 * localhost:8080, while the Go API listens on *:8080 — loopback requests then
 * get an empty reply. Prefer a non-loopback IPv4 when VITE_DEV_API_PROXY is unset.
 */
export function resolveDevApiProxyTarget(envValue) {
  const trimmed = String(envValue ?? '').trim()
  if (trimmed) return trimmed.replace(/\/$/, '')

  const nets = os.networkInterfaces()
  for (const name of Object.keys(nets)) {
    for (const net of nets[name] ?? []) {
      const family = net.family
      const isIPv4 = family === 'IPv4' || family === 4
      if (isIPv4 && !net.internal) {
        return `http://${net.address}:${DEFAULT_PORT}`
      }
    }
  }
  return `http://127.0.0.1:${DEFAULT_PORT}`
}
