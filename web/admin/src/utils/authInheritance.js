// Shared helpers that derive "inherited" Headers / Query rows from a service environment's
// `auth` JSON. Used by both 场景编排 (ScenarioEditor) 和运行控制台 (CaseRun) so both surfaces
// pick the same auth profile (security → path rule → defaultProfile fallback) and render the
// same inherited rows (with a `authInherited` marker) until the user explicitly overrides them.

const PATH_VAR_NAME = /^[A-Za-z_][A-Za-z0-9_-]*$/

export function normalizeAuthObject(value) {
  return value && typeof value === 'object' && !Array.isArray(value) ? value : {}
}

export function parseEnvironmentAuth(raw) {
  if (!raw) return {}
  if (typeof raw === 'object' && !Array.isArray(raw)) return raw
  if (typeof raw === 'string') {
    try {
      return normalizeAuthObject(JSON.parse(raw))
    } catch {
      return {}
    }
  }
  return {}
}

function isPathVariableName(name) {
  return PATH_VAR_NAME.test(String(name || '').trim())
}

export function normalizeAuthPathTemplate(path) {
  return String(path || '')
    .replace(/\{+\s*(\$[^{}]+?)\s*\}+/g, (_, expr) => `{{${expr.trim()}}}`)
    .replace(/\{\{+([^{}]+)\}\}+/g, (match, name) => {
      const trimmed = String(name || '').trim()
      return isPathVariableName(trimmed) ? `{${trimmed}}` : match
    })
}

function normalizeAuthPath(path) {
  const trimmed = String(path || '').trim()
  if (!trimmed) return ''
  return trimmed.startsWith('/') ? trimmed : `/${trimmed}`
}

function pathMatchesAuthPrefix(path, prefix) {
  if (!path || !prefix) return false
  return path === prefix || path.startsWith(`${prefix.replace(/\/+$/, '')}/`)
}

function hasSecurityRequirements(security) {
  return Array.isArray(security) && security.some((requirement) => (
    requirement && typeof requirement === 'object' && Object.keys(requirement).length > 0
  ))
}

function profileNameForSecurity(authConfig, profiles, security) {
  const securityList = Array.isArray(security) ? security : []
  const securitySchemes = normalizeAuthObject(authConfig.securitySchemes)
  for (const requirement of securityList) {
    if (!requirement || typeof requirement !== 'object') continue
    for (const schemeName of Object.keys(requirement)) {
      const scheme = String(schemeName || '').trim()
      if (!scheme) continue
      const mapped = String(securitySchemes[scheme] || '').trim()
      if (mapped && profiles[mapped]) return mapped
      if (profiles[scheme]) return scheme
    }
  }
  return ''
}

function profileNameForPath(authConfig, profiles, paths) {
  const rules = Array.isArray(authConfig.pathRules) ? authConfig.pathRules : []
  let best = ''
  let bestLen = -1
  for (const rule of rules) {
    const profile = String(rule?.profile || '').trim()
    if (!profile || !profiles[profile]) continue
    for (const rawPrefix of [rule.prefix, rule.path]) {
      const prefix = normalizeAuthPath(rawPrefix)
      if (!prefix) continue
      for (const path of paths) {
        if (pathMatchesAuthPrefix(normalizeAuthPath(path), prefix) && prefix.length > bestLen) {
          best = profile
          bestLen = prefix.length
        }
      }
    }
  }
  return best
}

// Returns the resolved auth profile object (e.g. `{ type: 'bearer', token: '...' }`) given the
// raw environment record, request `security` and one or more candidate paths. Mirrors the
// scenario editor's selection rules: profiles are picked by `security`, then by `pathRules`,
// then by `defaultProfile`. Legacy single-profile auth (no `profiles` map) only applies when
// the request declared `security`, so manually crafted requests don't accidentally inherit auth.
export function selectEnvironmentAuthDefinition({ environment, security, paths }) {
  const rawAuth = parseEnvironmentAuth(environment?.auth)
  if (!rawAuth || !Object.keys(rawAuth).length) return null

  const profiles = normalizeAuthObject(rawAuth.profiles)
  if (!Object.keys(profiles).length) {
    return hasSecurityRequirements(security) ? rawAuth : null
  }

  const bySecurity = profileNameForSecurity(rawAuth, profiles, security)
  if (bySecurity) return normalizeAuthObject(profiles[bySecurity])

  const candidatePaths = (Array.isArray(paths) ? paths : [paths])
    .filter((path) => path != null)
    .map((path) => normalizeAuthPathTemplate(path))
  const byPath = profileNameForPath(rawAuth, profiles, candidatePaths)
  if (byPath) return normalizeAuthObject(profiles[byPath])

  const fallback = String(rawAuth.defaultProfile || '').trim()
  return fallback && profiles[fallback] ? normalizeAuthObject(profiles[fallback]) : null
}

// Translates a resolved auth profile into the flat list of `{ key, value }` rows that should be
// projected into the request's Headers / Query tables. Supports the same profile types as the
// runner: bearer/jwt/oauth2 (Authorization header), basic (Basic <user>:<pass>), api_key /
// apikey / api-key (header or query depending on `in`), explicit `header` / `query` types,
// and the legacy form where `headers` and `query` maps are spelled out directly.
export function buildInheritedAuthRows(authDefinition) {
  if (!authDefinition) return { headers: [], query: [] }
  const headers = []
  const query = []
  const headerKeys = new Set()
  const queryKeys = new Set()
  const addHeader = (key, value) => {
    const name = String(key || '').trim()
    const normalized = name.toLowerCase()
    if (name && !headerKeys.has(normalized)) {
      headerKeys.add(normalized)
      headers.push({ key: name, value: String(value ?? '') })
    }
  }
  const addQuery = (key, value) => {
    const name = String(key || '').trim()
    if (name && !queryKeys.has(name)) {
      queryKeys.add(name)
      query.push({ key: name, value: String(value ?? '') })
    }
  }

  for (const [key, value] of Object.entries(normalizeAuthObject(authDefinition.headers))) {
    addHeader(key, value)
  }
  for (const [key, value] of Object.entries(normalizeAuthObject(authDefinition.query))) {
    addQuery(key, value)
  }

  const type = String(authDefinition.type || '').trim().toLowerCase()
  if (['bearer', 'jwt', 'oauth2'].includes(type)) {
    const tokenTemplate = authDefinition.token || authDefinition.value || ''
    const token = String(tokenTemplate).trim()
    if (token) addHeader('Authorization', token.toLowerCase().startsWith('bearer ') ? token : `Bearer ${token}`)
  } else if (type === 'basic') {
    const username = authDefinition.username || '{{username}}'
    const password = authDefinition.password || '{{password}}'
    addHeader('Authorization', `Basic ${username}:${password}`)
  } else if (['api_key', 'apikey', 'api-key'].includes(type)) {
    const name = authDefinition.name || ''
    const value = authDefinition.value || authDefinition.token || ''
    if (String(authDefinition.in || '').toLowerCase() === 'query') addQuery(name, value)
    else addHeader(name, value)
  } else if (type === 'header') {
    addHeader(authDefinition.name, authDefinition.value)
  } else if (type === 'query') {
    addQuery(authDefinition.name, authDefinition.value)
  }
  return { headers, query }
}

// Convenience wrapper: resolve and project in a single call.
export function computeInheritedAuthRows(context) {
  return buildInheritedAuthRows(selectEnvironmentAuthDefinition(context))
}

// Merges inherited rows back into a list of user-edited rows. Inherited rows that the user has
// already promoted via `markAuthRowOverridden` (or otherwise touched) are kept as-is; pure
// inherited rows get re-derived from `inheritedRows`. When a user row already supplies the
// same key (case-insensitive for headers), the inherited row is skipped — user value wins.
export function mergeInheritedAuthRows(currentRows, inheritedRows, options = {}) {
  const headerMode = options.headerMode === true
  const manualRows = (currentRows || []).filter((row) => !row.authInherited || row.authOverridden)
  const exists = (key) => {
    const target = headerMode ? String(key).toLowerCase() : String(key)
    return manualRows.some((row) => {
      if (row.enabled === false) return false
      const rowKey = headerMode ? String(row.key || '').toLowerCase() : String(row.key || '')
      return rowKey === target
    })
  }
  const additions = (inheritedRows || [])
    .filter((row) => row.key && !exists(row.key))
    .map((row) => ({
      enabled: true,
      required: false,
      ...row,
      authInherited: true,
      authOverridden: false
    }))
  return [...manualRows, ...additions]
}

// Marker called from input/checkbox handlers in the row to convert an inherited row into a
// user override (so subsequent saves persist it and the row is no longer auto-refreshed).
export function markAuthRowOverridden(row) {
  if (row && row.authInherited) {
    row.authOverridden = true
  }
}

export const AUTH_INHERITED_ROW_COMMENT = '继承自当前环境认证；修改后会保存为请求覆盖，支持模板变量。'
