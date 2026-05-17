export function tryParseJSONText(value) {
  const text = String(value).trim()
  if (!text || !['{', '['].includes(text[0])) return undefined
  try {
    return JSON.parse(text)
  } catch {
    return undefined
  }
}

export function normalizeSnapshotForViewer(value) {
  if (value == null || value === '') return value
  if (typeof value === 'string') {
    const parsed = tryParseJSONText(value)
    return parsed === undefined ? value : normalizeSnapshotForViewer(parsed)
  }
  if (Array.isArray(value)) {
    return value.map((item) => normalizeSnapshotForViewer(item))
  }
  if (typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value).map(([key, item]) => [key, normalizeSnapshotForViewer(item)])
    )
  }
  return value
}

export function collectJsonLines(value, keyName, path, depth, isLast, collapsed, lines) {
  const id = `${path}-${lines.length}`
  if (value === null) {
    lines.push({
      id,
      type: 'primitive',
      key: keyName,
      depth,
      valueType: 'null',
      displayValue: 'null',
      hasComma: !isLast
    })
    return
  }
  if (typeof value !== 'object') {
    lines.push({
      id,
      type: 'primitive',
      key: keyName,
      depth,
      valueType: typeof value,
      displayValue: JSON.stringify(value),
      hasComma: !isLast
    })
    return
  }
  if (Array.isArray(value)) {
    if (value.length === 0) {
      lines.push({ id, type: 'empty-array', key: keyName, depth, hasComma: !isLast })
      return
    }
    if (collapsed[path]) {
      lines.push({
        id,
        type: 'collapsed',
        key: keyName,
        depth,
        open: '[',
        close: ']',
        count: value.length,
        hasComma: !isLast
      })
      return
    }
    lines.push({ id, type: 'open', key: keyName, depth, bracket: '[', path, hasComma: false })
    value.forEach((item, i) => {
      collectJsonLines(item, null, `${path}[${i}]`, depth + 1, i === value.length - 1, collapsed, lines)
    })
    lines.push({ id: `${id}-close`, type: 'close', depth, bracket: ']', hasComma: !isLast })
    return
  }
  const keys = Object.keys(value)
  if (keys.length === 0) {
    lines.push({ id, type: 'empty-object', key: keyName, depth, hasComma: !isLast })
    return
  }
  if (collapsed[path]) {
    lines.push({
      id,
      type: 'collapsed',
      key: keyName,
      depth,
      open: '{',
      close: '}',
      count: keys.length,
      hasComma: !isLast
    })
    return
  }
  lines.push({ id, type: 'open', key: keyName, depth, bracket: '{', path, hasComma: false })
  keys.forEach((k, i) => {
    collectJsonLines(value[k], k, `${path}.${k}`, depth + 1, i === keys.length - 1, collapsed, lines)
  })
  lines.push({ id: `${id}-close`, type: 'close', depth, bracket: '}', hasComma: !isLast })
}

export function jsonLinesForSnapshot(snapshot, collapseKey, collapsedMap) {
  const value = normalizeSnapshotForViewer(snapshot)
  if (value == null || value === '') return []
  const lines = []
  const collapsed = collapsedMap[collapseKey] || {}
  collectJsonLines(value, null, '$', 0, true, collapsed, lines)
  return lines
}
