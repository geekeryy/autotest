/** Mock Server 对外访问地址：使用当前页面域名，便于远程或局域网访问。 */
export function formatMockServerAccessURL(port) {
  const n = Number(port)
  if (!n) return '-'
  const hostname =
    typeof window !== 'undefined' && window.location.hostname ? window.location.hostname : 'localhost'
  return `http://${hostname}:${n}`
}
