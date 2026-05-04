/**
 * 根据 Runner 返回的 requestSnapshot（method、url、headers、body）生成可复制的 curl 命令。
 */
export function buildCurlFromRequestSnapshot(snap) {
  if (!snap || typeof snap !== 'object') return '# 无请求信息'
  const method = String(snap.method || 'GET').toUpperCase()
  const url = snap.url
  if (!url) return '# 无请求信息'

  const quote = (str) => `'${String(str).replace(/'/g, "'\\''")}'`

  const parts = [`curl -sS -X ${method}`]

  const headers = snap.headers && typeof snap.headers === 'object' ? snap.headers : {}
  const keys = Object.keys(headers).sort((a, b) => a.localeCompare(b))
  for (const key of keys) {
    const raw = headers[key]
    const val = Array.isArray(raw) ? raw.join(', ') : String(raw)
    parts.push(`-H ${quote(`${key}: ${val}`)}`)
  }

  let bodyStr = ''
  if (snap.body != null && snap.body !== '') {
    if (typeof snap.body === 'string') {
      bodyStr = snap.body
    } else {
      try {
        bodyStr = JSON.stringify(snap.body)
      } catch {
        bodyStr = String(snap.body)
      }
    }
  }
  if (bodyStr.length > 0) {
    parts.push(`--data-binary ${quote(bodyStr)}`)
  }

  parts.push(quote(url))
  return parts.join(' \\\n  ')
}
