const STORAGE_KEY = 'autotest.dashboardShortcutClicks'

/** @returns {Record<string, number>} */
export function readShortcutClickCounts() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    return typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed) ? parsed : {}
  } catch {
    return {}
  }
}

export function incrementShortcutClick(key) {
  if (!key) return
  const counts = readShortcutClickCounts()
  counts[key] = (Number(counts[key]) || 0) + 1
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(counts))
  } catch {
    // 存储失败不影响跳转
  }
}

/**
 * @param {{ key: string }[]} items 已按权限过滤后的快捷项
 * @param {string[]} defaultOrderKeys 全量 key 顺序，用于点击次数相同时的稳定排序
 */
export function rankShortcutsByClicks(items, defaultOrderKeys) {
  const counts = readShortcutClickCounts()
  const orderIndex = new Map(defaultOrderKeys.map((k, i) => [k, i]))
  return [...items].sort((a, b) => {
    const ca = Number(counts[a.key]) || 0
    const cb = Number(counts[b.key]) || 0
    if (cb !== ca) return cb - ca
    return (orderIndex.get(a.key) ?? 999) - (orderIndex.get(b.key) ?? 999)
  })
}
