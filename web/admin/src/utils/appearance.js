const APPEARANCE_KEY = 'autotest_admin_appearance'

export const FONT_SIZE_OPTIONS = [
  { value: 'compact', label: '紧凑', base: 13 },
  { value: 'default', label: '标准', base: 14 },
  { value: 'comfortable', label: '宽松', base: 16 },
  { value: 'large', label: '大号', base: 18 },
  { value: 'xlarge', label: '特大', base: 20 },
  { value: 'xxlarge', label: '超大', base: 22 }
]

export const PALETTE_OPTIONS = [
  {
    value: 'blue',
    label: '现代科技蓝',
    primary: '#2563eb',
    secondary: '#14b8a6',
    accent: '#f97316',
    sidebarBg: '#0f172a',
    sidebarText: '#cbd5e1',
    sidebarActive: '#ffffff',
    pageBg: '#f8fafc',
    cardBg: '#ffffff',
    text: '#0f172a',
    muted: '#64748b',
    border: '#e2e8f0',
    codeBg: '#f1f5f9'
  },
  {
    value: 'black-gold',
    label: '高级黑金',
    primary: '#d4af37',
    secondary: '#8b5cf6',
    accent: '#f59e0b',
    sidebarBg: '#050816',
    sidebarText: '#d1d5db',
    sidebarActive: '#ffffff',
    pageBg: '#0b0f19',
    cardBg: '#111827',
    text: '#f9fafb',
    muted: '#9ca3af',
    border: '#374151',
    codeBg: '#020617',
    isDark: true
  },
  {
    value: 'green',
    label: '清新自然绿',
    primary: '#16a34a',
    secondary: '#0ea5e9',
    accent: '#facc15',
    sidebarBg: '#14532d',
    sidebarText: '#dcfce7',
    sidebarActive: '#ffffff',
    pageBg: '#f7fdf9',
    cardBg: '#ffffff',
    text: '#14532d',
    muted: '#6b7280',
    border: '#d1fae5',
    codeBg: '#ecfdf5'
  },
  {
    value: 'purple',
    label: '柔和紫粉',
    primary: '#7c3aed',
    secondary: '#ec4899',
    accent: '#06b6d4',
    sidebarBg: '#271a45',
    sidebarText: '#ddd6fe',
    sidebarActive: '#ffffff',
    pageBg: '#faf5ff',
    cardBg: '#ffffff',
    text: '#1f2937',
    muted: '#6b7280',
    border: '#e9d5ff',
    codeBg: '#f5f3ff'
  },
  {
    value: 'neutral',
    label: '极简中性色',
    primary: '#111827',
    secondary: '#4f46e5',
    accent: '#10b981',
    sidebarBg: '#111827',
    sidebarText: '#d1d5db',
    sidebarActive: '#ffffff',
    pageBg: '#f9fafb',
    cardBg: '#ffffff',
    text: '#111827',
    muted: '#6b7280',
    border: '#e5e7eb',
    codeBg: '#f3f4f6'
  },
  {
    value: 'orange',
    label: '温暖橙棕',
    primary: '#ea580c',
    secondary: '#92400e',
    accent: '#fbbf24',
    sidebarBg: '#3a2414',
    sidebarText: '#fed7aa',
    sidebarActive: '#ffffff',
    pageBg: '#fff7ed',
    cardBg: '#ffffff',
    text: '#431407',
    muted: '#78716c',
    border: '#fed7aa',
    codeBg: '#ffedd5'
  },
  {
    value: 'apple-blue',
    label: '苹果浅蓝灰',
    primary: '#007aff',
    secondary: '#5ac8fa',
    accent: '#34c759',
    sidebarBg: '#1d1d1f',
    sidebarText: '#d2d2d7',
    sidebarActive: '#ffffff',
    pageBg: '#f5f5f7',
    cardBg: '#ffffff',
    text: '#1d1d1f',
    muted: '#86868b',
    border: '#d2d2d7',
    codeBg: '#f2f2f7'
  },
  {
    value: 'notion-gray',
    label: 'Notion 黑白灰',
    primary: '#2f3437',
    secondary: '#787774',
    accent: '#0f766e',
    sidebarBg: '#2f3437',
    sidebarText: '#e9e9e7',
    sidebarActive: '#ffffff',
    pageBg: '#fbfbfa',
    cardBg: '#ffffff',
    text: '#37352f',
    muted: '#9b9a97',
    border: '#e9e9e7',
    codeBg: '#f7f6f3'
  },
  {
    value: 'stripe-purple',
    label: '科技紫蓝',
    primary: '#635bff',
    secondary: '#00d4ff',
    accent: '#ff80b5',
    sidebarBg: '#0a2540',
    sidebarText: '#d9e2ec',
    sidebarActive: '#ffffff',
    pageBg: '#f6f9fc',
    cardBg: '#ffffff',
    text: '#0a2540',
    muted: '#425466',
    border: '#d9e2ec',
    codeBg: '#edf2f7'
  },
  {
    value: 'linear-dark',
    label: '深色蓝紫',
    primary: '#5e6ad2',
    secondary: '#8b5cf6',
    accent: '#22d3ee',
    sidebarBg: '#08090a',
    sidebarText: '#a1a1aa',
    sidebarActive: '#f4f4f5',
    pageBg: '#08090a',
    cardBg: '#111113',
    text: '#f4f4f5',
    muted: '#a1a1aa',
    border: '#27272a',
    codeBg: '#18181b',
    isDark: true
  },
  {
    value: 'cream',
    label: '温和奶油色',
    primary: '#d97706',
    secondary: '#65a30d',
    accent: '#e11d48',
    sidebarBg: '#422006',
    sidebarText: '#fde68a',
    sidebarActive: '#ffffff',
    pageBg: '#fffbeb',
    cardBg: '#ffffff',
    text: '#422006',
    muted: '#78716c',
    border: '#fde68a',
    codeBg: '#fef3c7'
  },
  {
    value: 'finance',
    label: '专业金融蓝绿',
    primary: '#1d4ed8',
    secondary: '#059669',
    accent: '#f59e0b',
    sidebarBg: '#0f172a',
    sidebarText: '#cbd5e1',
    sidebarActive: '#ffffff',
    pageBg: '#f8fafc',
    cardBg: '#ffffff',
    text: '#0f172a',
    muted: '#475569',
    border: '#cbd5e1',
    codeBg: '#f1f5f9'
  }
]

const DEFAULT_APPEARANCE = {
  fontSize: 'default',
  palette: 'blue',
  primaryColor: PALETTE_OPTIONS[0].primary
}

function findFontSize(value) {
  return FONT_SIZE_OPTIONS.find((item) => item.value === value) || FONT_SIZE_OPTIONS[1]
}

function findPalette(value) {
  return PALETTE_OPTIONS.find((item) => item.value === value) || PALETTE_OPTIONS[0]
}

function clampColor(value) {
  if (!value || typeof value !== 'string') return DEFAULT_APPEARANCE.primaryColor
  const color = value.trim()
  return /^#[0-9a-fA-F]{6}$/.test(color) ? color : DEFAULT_APPEARANCE.primaryColor
}

function hexToRgb(hex) {
  const normalized = hex.replace('#', '')
  return {
    r: parseInt(normalized.slice(0, 2), 16),
    g: parseInt(normalized.slice(2, 4), 16),
    b: parseInt(normalized.slice(4, 6), 16)
  }
}

function rgbToHex({ r, g, b }) {
  return `#${[r, g, b].map((value) => value.toString(16).padStart(2, '0')).join('')}`
}

function mix(hex, target, weight) {
  const color = hexToRgb(hex)
  const targetColor = hexToRgb(target)
  return rgbToHex({
    r: Math.round(color.r * (1 - weight) + targetColor.r * weight),
    g: Math.round(color.g * (1 - weight) + targetColor.g * weight),
    b: Math.round(color.b * (1 - weight) + targetColor.b * weight)
  })
}

function setVar(name, value) {
  document.documentElement.style.setProperty(name, value)
}

function normalizeAppearance(value) {
  const stored = value && typeof value === 'object' ? value : {}
  const fontSize = findFontSize(stored.fontSize).value
  const palette = stored.palette === 'custom' ? 'custom' : findPalette(stored.palette).value
  const primaryColor = palette === 'custom' ? clampColor(stored.primaryColor) : findPalette(palette).primary

  return { fontSize, palette, primaryColor }
}

export function getStoredAppearance() {
  try {
    const raw = window.localStorage.getItem(APPEARANCE_KEY)
    return normalizeAppearance(raw ? JSON.parse(raw) : DEFAULT_APPEARANCE)
  } catch {
    return { ...DEFAULT_APPEARANCE }
  }
}

export function saveAppearance(appearance) {
  const normalized = normalizeAppearance(appearance)
  window.localStorage.setItem(APPEARANCE_KEY, JSON.stringify(normalized))
  return normalized
}

export function resetAppearance() {
  window.localStorage.removeItem(APPEARANCE_KEY)
  applyAppearance(DEFAULT_APPEARANCE)
  return { ...DEFAULT_APPEARANCE }
}

export function getResolvedPalette(appearance) {
  const normalized = normalizeAppearance(appearance)
  if (normalized.palette !== 'custom') return findPalette(normalized.palette)

  const primary = normalized.primaryColor
  return {
    value: 'custom',
    label: '自定义',
    primary,
    secondary: mix(primary, '#14b8a6', 0.65),
    accent: mix(primary, '#f59e0b', 0.65),
    sidebarBg: mix(primary, '#111827', 0.7),
    sidebarText: mix(primary, '#ffffff', 0.78),
    sidebarActive: '#ffffff',
    pageBg: mix(primary, '#ffffff', 0.94),
    cardBg: '#ffffff',
    text: '#0f172a',
    muted: '#64748b',
    border: mix(primary, '#e2e8f0', 0.82),
    codeBg: mix(primary, '#f8fafc', 0.94)
  }
}

export function applyAppearance(appearance) {
  const normalized = normalizeAppearance(appearance)
  const fontSize = findFontSize(normalized.fontSize)
  const palette = getResolvedPalette(normalized)

  setVar('--app-font-size-base', `${fontSize.base}px`)
  setVar('--app-font-size-small', `${fontSize.base - 2}px`)
  setVar('--app-font-size-title', `${fontSize.base + 6}px`)
  setVar('--app-font-size-metric', `${fontSize.base + 18}px`)
  setVar('--app-component-size', `${fontSize.base + 18}px`)
  setVar('--app-component-size-small', `${fontSize.base + 10}px`)
  setVar('--app-component-size-large', `${fontSize.base + 26}px`)
  setVar('--app-line-height-base', '1.5')
  setVar('--app-primary-color', palette.primary)
  setVar('--app-primary-hover', mix(palette.primary, '#ffffff', 0.2))
  setVar('--app-secondary-color', palette.secondary)
  setVar('--app-accent-color', palette.accent)
  setVar('--app-sidebar-bg', palette.sidebarBg)
  setVar('--app-sidebar-text', palette.sidebarText)
  setVar('--app-sidebar-active', palette.sidebarActive)
  setVar('--app-page-bg', palette.pageBg)
  setVar('--app-card-bg', palette.cardBg)
  setVar('--app-text-color', palette.text)
  setVar('--app-secondary-text', palette.muted)
  setVar('--app-border-color', palette.border)
  setVar('--app-code-bg', palette.codeBg)

  setVar('--el-font-size-extra-small', `${fontSize.base - 2}px`)
  setVar('--el-font-size-small', `${fontSize.base - 1}px`)
  setVar('--el-font-size-base', `${fontSize.base}px`)
  setVar('--el-font-size-medium', `${fontSize.base + 2}px`)
  setVar('--el-font-size-large', `${fontSize.base + 4}px`)
  setVar('--el-font-size-extra-large', `${fontSize.base + 6}px`)
  setVar('--el-component-size', `${fontSize.base + 18}px`)
  setVar('--el-component-size-small', `${fontSize.base + 10}px`)
  setVar('--el-component-size-large', `${fontSize.base + 26}px`)
  setVar('--el-color-primary', palette.primary)
  setVar('--el-color-primary-dark-2', mix(palette.primary, '#000000', 0.18))
  setVar('--el-color-primary-light-3', mix(palette.primary, '#ffffff', 0.3))
  setVar('--el-color-primary-light-5', mix(palette.primary, '#ffffff', 0.5))
  setVar('--el-color-primary-light-7', mix(palette.primary, '#ffffff', 0.7))
  setVar('--el-color-primary-light-8', mix(palette.primary, '#ffffff', 0.8))
  setVar('--el-color-primary-light-9', mix(palette.primary, '#ffffff', 0.9))
  setVar('--el-bg-color', palette.cardBg)
  setVar('--el-bg-color-page', palette.pageBg)
  setVar('--el-bg-color-overlay', palette.cardBg)
  setVar('--el-text-color-primary', palette.text)
  setVar('--el-text-color-regular', palette.text)
  setVar('--el-text-color-secondary', palette.muted)
  setVar('--el-text-color-placeholder', palette.muted)
  setVar('--el-border-color', palette.border)
  setVar('--el-border-color-light', mix(palette.border, palette.cardBg, 0.35))
  setVar('--el-border-color-lighter', mix(palette.border, palette.cardBg, 0.55))
  setVar('--el-fill-color-blank', palette.cardBg)
  setVar('--el-fill-color-light', palette.codeBg)
  setVar('--el-fill-color-lighter', palette.codeBg)
  setVar('--el-fill-color-extra-light', palette.pageBg)
  setVar('--el-menu-hover-bg-color', palette.isDark ? mix(palette.sidebarBg, '#ffffff', 0.08) : mix(palette.sidebarBg, '#ffffff', 0.16))

  return normalized
}

export function initializeAppearance() {
  return applyAppearance(getStoredAppearance())
}
