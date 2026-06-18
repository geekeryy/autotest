/**
 * Cursor MCP one-click install deeplinks.
 * @see https://cursor.com/docs/mcp/install-links
 */

const CURSOR_MCP_INSTALL = 'cursor://anysphere.cursor-deeplink/mcp/install'

/**
 * @param {string} serverName
 * @param {Record<string, unknown>} config transport config (url/headers or command/env)
 * @returns {string}
 */
export function buildCursorInstallLink(serverName, config) {
  const json = JSON.stringify(config)
  const bytes = new TextEncoder().encode(json)
  let binary = ''
  bytes.forEach((b) => {
    binary += String.fromCharCode(b)
  })
  const encoded = btoa(binary)
  const params = new URLSearchParams({ name: serverName, config: encoded })
  return `${CURSOR_MCP_INSTALL}?${params.toString()}`
}

const API_KEY_PLACEHOLDER = 'at-your-api-key'

/**
 * @param {string} link backend-generated deeplink (may contain placeholder key)
 * @param {string} apiKey user-provided at-... key
 * @returns {string}
 */
export function applyApiKeyToCursorInstallLink(link, apiKey) {
  const key = (apiKey || '').trim()
  if (!key || !link || !key.startsWith('at-')) {
    return link
  }
  try {
    const u = new URL(link)
    const raw = atob(u.searchParams.get('config') || '')
    if (!raw.includes(API_KEY_PLACEHOLDER)) {
      return link
    }
    const updated = raw.replaceAll(API_KEY_PLACEHOLDER, key)
    const bytes = new TextEncoder().encode(updated)
    let binary = ''
    bytes.forEach((b) => {
      binary += String.fromCharCode(b)
    })
    u.searchParams.set('config', btoa(binary))
    return u.toString()
  } catch {
    return link
  }
}

/**
 * @param {string} link
 */
export function openCursorInstallLink(link) {
  if (!link) return
  window.location.href = link
}
