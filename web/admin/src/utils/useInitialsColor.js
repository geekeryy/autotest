/**
 * Derive display initials and a stable HSL background from a user identifier.
 */

export function getInitials(name, fallback = '用') {
  const n = String(name || fallback).trim()
  if (!n) return fallback.slice(0, 2)
  if (/[\u4e00-\u9fff]/.test(n)) {
    return n.length >= 2 ? n.slice(0, 2) : n.slice(0, 1)
  }
  const parts = n.split(/[\s._-]+/).filter(Boolean)
  if (parts.length >= 2) {
    return (parts[0][0] + parts[1][0]).toUpperCase()
  }
  return n.slice(0, 2).toUpperCase()
}

export function getInitialsHueBackground(seed) {
  const raw = String(seed || 'x')
  let h = 0
  for (let i = 0; i < raw.length; i++) {
    h = raw.charCodeAt(i) + ((h << 5) - h)
  }
  const hue = Math.abs(h) % 360
  return `hsl(${hue}, 42%, 44%)`
}

export function getAvatarBgStyle(seed, hasImageUrl) {
  if (hasImageUrl) return {}
  return { backgroundColor: getInitialsHueBackground(seed) }
}
