/** 将各上游 usage 字段归并为统一的缓存统计项（不区分厂商）。 */

function sumPositive(...values) {
  let total = 0
  let any = false
  for (const value of values) {
    const n = Number(value)
    if (Number.isFinite(n) && n > 0) {
      total += n
      any = true
    }
  }
  return any ? total : 0
}

/** 缓存命中：从缓存读取的输入 Token（cached_tokens / prompt_cache_hit / cache_read 等）。 */
export function cacheHitTokens(usage) {
  if (!usage) return 0
  return sumPositive(
    usage.cachedPromptTokens,
    usage.promptCacheHitTokens,
    usage.cacheReadInputTokens,
  )
}

/** 缓存未命中：未走缓存的提示 Token（prompt_cache_miss_tokens 等）。 */
export function cacheMissTokens(usage) {
  const n = Number(usage?.promptCacheMissTokens)
  return Number.isFinite(n) && n > 0 ? n : 0
}

/** 缓存写入：写入服务端缓存的 Token（cache_creation_input_tokens 等）。 */
export function cacheWriteTokens(usage) {
  const n = Number(usage?.cacheCreationInputTokens)
  return Number.isFinite(n) && n > 0 ? n : 0
}

export function hasCacheUsage(usage) {
  return cacheHitTokens(usage) > 0 || cacheMissTokens(usage) > 0 || cacheWriteTokens(usage) > 0
}

/**
 * 输入 Token 分解：总量、缓存命中及占比（用于进度条展示）。
 * @param {object} usage
 * @returns {{ total: number, hit: number, nonHit: number, hitPercent: number, showBreakdown: boolean }}
 */
export function inputTokenCacheBreakdown(usage) {
  const total = Number(usage?.promptTokens)
  const hit = cacheHitTokens(usage)
  const miss = cacheMissTokens(usage)
  const safeTotal = Number.isFinite(total) && total > 0 ? total : 0
  const safeHit = hit > 0 ? hit : 0
  const denom =
    safeTotal > 0 ? safeTotal : miss > 0 ? safeHit + miss : safeHit
  const nonHit =
    safeTotal > 0
      ? Math.max(0, safeTotal - safeHit)
      : miss > 0
        ? miss
        : Math.max(0, denom - safeHit)
  const hitPercent = denom > 0 && safeHit > 0 ? Math.min(100, (safeHit / denom) * 100) : 0
  return {
    total: safeTotal || denom,
    hit: safeHit,
    nonHit,
    hitPercent,
    showBreakdown: safeHit > 0 && denom > 0,
  }
}

/**
 * @param {object} usage
 * @param {(n: number) => string} format
 * @param {{ omitHit?: boolean, omitMiss?: boolean }} [opts]
 * @returns {{ label: string, value: string }[]}
 */
export function buildCacheStatItems(usage, format, opts = {}) {
  const fmt = typeof format === 'function' ? format : (n) => String(n)
  const items = []
  const hit = cacheHitTokens(usage)
  const miss = cacheMissTokens(usage)
  const write = cacheWriteTokens(usage)
  if (hit > 0 && !opts.omitHit) items.push({ label: '缓存命中', value: fmt(hit) })
  if (miss > 0 && !opts.omitMiss) items.push({ label: '缓存未命中', value: fmt(miss) })
  if (write > 0) items.push({ label: '缓存写入', value: fmt(write) })
  return items
}
