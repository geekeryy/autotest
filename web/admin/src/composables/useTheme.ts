/**
 * 主题管理 Composable
 * 替代 utils/appearance.js，使用 Composition API 重构
 */
import { ref, computed } from 'vue'

const APPEARANCE_KEY = 'autotest_admin_appearance'

// ── 常量 ──────────────────────────────────────────────────────────
export interface FontSizeOption {
  value: string
  label: string
  base: number
}

export interface PaletteOption {
  value: string
  label: string
  primary: string
  secondary: string
  accent: string
  sidebarBg: string
  sidebarText: string
  sidebarActive: string
  pageBg: string
  cardBg: string
  text: string
  muted: string
  border: string
  codeBg: string
}

export const FONT_SIZE_OPTIONS: FontSizeOption[] = [
  { value: 'compact', label: '紧凑', base: 13 },
  { value: 'default', label: '标准', base: 14 },
  { value: 'comfortable', label: '宽松', base: 16 },
  { value: 'large', label: '大号', base: 18 },
  { value: 'xlarge', label: '特大', base: 20 },
  { value: 'xxlarge', label: '超大', base: 22 },
]

export const PALETTE_OPTIONS: PaletteOption[] = [
  {
    value: 'blue', label: '现代科技蓝',
    primary: '#2563eb', secondary: '#14b8a6', accent: '#f97316',
    sidebarBg: '#0f172a', sidebarText: '#cbd5e1', sidebarActive: '#ffffff',
    pageBg: '#f8fafc', cardBg: '#ffffff', text: '#0f172a', muted: '#64748b', border: '#e2e8f0', codeBg: '#f1f5f9',
  },
  {
    value: 'purple', label: '柔和紫粉',
    primary: '#7c3aed', secondary: '#ec4899', accent: '#06b6d4',
    sidebarBg: '#271a45', sidebarText: '#ddd6fe', sidebarActive: '#ffffff',
    pageBg: '#faf5ff', cardBg: '#ffffff', text: '#1f2937', muted: '#6b7280', border: '#e9d5ff', codeBg: '#f5f3ff',
  },
  {
    value: 'apple-blue', label: '苹果浅蓝灰',
    primary: '#007aff', secondary: '#5ac8fa', accent: '#34c759',
    sidebarBg: '#1d1d1f', sidebarText: '#d2d2d7', sidebarActive: '#ffffff',
    pageBg: '#f5f5f7', cardBg: '#ffffff', text: '#1d1d1f', muted: '#86868b', border: '#d2d2d7', codeBg: '#f2f2f7',
  },
  {
    value: 'notion-gray', label: 'Notion 黑白灰',
    primary: '#2f3437', secondary: '#787774', accent: '#0f766e',
    sidebarBg: '#2f3437', sidebarText: '#e9e9e7', sidebarActive: '#ffffff',
    pageBg: '#fbfbfa', cardBg: '#ffffff', text: '#37352f', muted: '#9b9a97', border: '#e9e9e7', codeBg: '#f7f6f3',
  },
  {
    value: 'stripe-purple', label: '科技紫蓝',
    primary: '#635bff', secondary: '#00d4ff', accent: '#ff80b5',
    sidebarBg: '#0a2540', sidebarText: '#d9e2ec', sidebarActive: '#ffffff',
    pageBg: '#f6f9fc', cardBg: '#ffffff', text: '#0a2540', muted: '#425466', border: '#d9e2ec', codeBg: '#edf2f7',
  },
  {
    value: 'finance', label: '专业金融蓝绿',
    primary: '#1d4ed8', secondary: '#059669', accent: '#f59e0b',
    sidebarBg: '#0f172a', sidebarText: '#cbd5e1', sidebarActive: '#ffffff',
    pageBg: '#f8fafc', cardBg: '#ffffff', text: '#0f172a', muted: '#475569', border: '#cbd5e1', codeBg: '#f1f5f9',
  },
]

export interface Appearance {
  fontSize: string
  palette: string
  primaryColor: string
}

const DEFAULT_APPEARANCE: Appearance = {
  fontSize: 'default',
  palette: 'blue',
  primaryColor: PALETTE_OPTIONS[0].primary,
}

// ── 辅助函数 ──────────────────────────────────────────────────────
function hexToRgb(hex: string) {
  const n = hex.replace('#', '')
  return {
    r: parseInt(n.slice(0, 2), 16),
    g: parseInt(n.slice(2, 4), 16),
    b: parseInt(n.slice(4, 6), 16),
  }
}

function rgbToHex({ r, g, b }: { r: number; g: number; b: number }) {
  return `#${[r, g, b].map(v => v.toString(16).padStart(2, '0')).join('')}`
}

function mix(hex: string, target: string, weight: number) {
  const c = hexToRgb(hex)
  const t = hexToRgb(target)
  return rgbToHex({
    r: Math.round(c.r * (1 - weight) + t.r * weight),
    g: Math.round(c.g * (1 - weight) + t.g * weight),
    b: Math.round(c.b * (1 - weight) + t.b * weight),
  })
}

function setVar(name: string, value: string) {
  document.documentElement.style.setProperty(name, value)
}

function clampColor(value: string | undefined): string {
  if (!value || typeof value !== 'string') return DEFAULT_APPEARANCE.primaryColor
  const color = value.trim()
  return /^#[0-9a-fA-F]{6}$/.test(color) ? color : DEFAULT_APPEARANCE.primaryColor
}

function findFontSize(value: string): FontSizeOption {
  return FONT_SIZE_OPTIONS.find(o => o.value === value) || FONT_SIZE_OPTIONS[1]
}

function findPalette(value: string): PaletteOption {
  return PALETTE_OPTIONS.find(o => o.value === value) || PALETTE_OPTIONS[0]
}

function normalizeAppearance(value: unknown): Appearance {
  const stored = value && typeof value === 'object' ? value as Record<string, unknown> : {}
  const fontSize = findFontSize(stored.fontSize as string).value
  const palette = stored.palette === 'custom' ? 'custom' : findPalette(stored.palette as string).value
  const primaryColor = palette === 'custom' ? clampColor(stored.primaryColor as string) : findPalette(palette).primary
  return { fontSize, palette, primaryColor }
}

function getResolvedPalette(appearance: Appearance): PaletteOption {
  const normalized = normalizeAppearance(appearance)
  if (normalized.palette !== 'custom') return findPalette(normalized.palette)

  const primary = normalized.primaryColor
  return {
    value: 'custom', label: '自定义', primary,
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
    codeBg: mix(primary, '#f8fafc', 0.94),
  }
}

// ── Composable ────────────────────────────────────────────────────
export function useTheme() {
  const appearance = ref<Appearance>(loadAppearance())

  function loadAppearance(): Appearance {
    try {
      const raw = localStorage.getItem(APPEARANCE_KEY)
      return normalizeAppearance(raw ? JSON.parse(raw) : DEFAULT_APPEARANCE)
    } catch {
      return { ...DEFAULT_APPEARANCE }
    }
  }

  function save(next: Appearance) {
    const normalized = normalizeAppearance(next)
    localStorage.setItem(APPEARANCE_KEY, JSON.stringify(normalized))
    appearance.value = normalized
  }

  function apply(appearance: Appearance): Appearance {
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
    setVar('--app-sidebar-menu-hover-bg', mix(palette.sidebarBg, '#ffffff', 0.1))
    setVar('--app-sidebar-menu-active-bg', mix(palette.primary, palette.sidebarBg, 0.82))
    setVar('--app-sidebar-menu-active-accent', palette.primary)
    setVar('--app-page-bg', palette.pageBg)
    setVar('--app-card-bg', palette.cardBg)
    setVar('--app-text-color', palette.text)
    setVar('--app-secondary-text', palette.muted)
    setVar('--app-border-color', palette.border)
    setVar('--app-code-bg', palette.codeBg)

    // Element Plus token 覆盖
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
    setVar('--el-menu-hover-bg-color', mix(palette.sidebarBg, '#ffffff', 0.1))
    setVar('--el-menu-active-color', palette.sidebarActive)

    return normalized
  }

  function reset() {
    localStorage.removeItem(APPEARANCE_KEY)
    appearance.value = { ...DEFAULT_APPEARANCE }
    apply(DEFAULT_APPEARANCE)
  }

  function initialize() {
    apply(appearance.value)
  }

  return {
    appearance,
    fontSize: computed(() => findFontSize(appearance.value.fontSize)),
    palette: computed(() => getResolvedPalette(appearance.value)),
    save,
    apply,
    reset,
    initialize,
  }
}
