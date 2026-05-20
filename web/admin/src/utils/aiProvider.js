/** Provider/model helpers shared by AI assistant UI. */

export function formatProviderLabel(provider) {
  if (!provider) return ''
  const suffix = provider.isDefault ? ' · 默认' : ''
  return `${provider.name || provider.providerType}${suffix}`
}

export function providerMeta(provider, providerTypes = []) {
  if (!provider) return null
  return providerTypes.find((t) => t.type === provider.providerType) || null
}

export function providerDefaultModel(provider, providerTypes = []) {
  if (!provider) return ''
  return provider.defaultModel || providerMeta(provider, providerTypes)?.defaultModel || ''
}

export function providerSupportsThinking(provider) {
  const t = String(provider?.providerType || '').toLowerCase()
  return t === 'deepseek' || t === 'xiaomi'
}

export function providerSupportsWebSearch(provider) {
  return String(provider?.providerType || '').toLowerCase() === 'xiaomi'
}

/** True when provider has a configured image model (modalityModels.image). */
export function providerHasImageModel(provider) {
  return !!String(provider?.modalityModels?.image || '').trim()
}

export function collectModelOptionIds({ remoteModelList = [], selectedModel = '', defaultModel = '' } = {}) {
  const values = new Set()
  for (const model of remoteModelList) {
    if (String(model || '').trim()) values.add(String(model).trim())
  }
  if (defaultModel) values.add(String(defaultModel).trim())
  if (selectedModel) values.add(String(selectedModel).trim())
  return Array.from(values).sort()
}

export function imageUploadTooltip(provider) {
  if (!providerHasImageModel(provider)) return ''
  const model = String(provider?.modalityModels?.image || '').trim()
  return `添加图片（JPEG/PNG/GIF/WebP，单张≤5MB）；含图时自动切换到提供商配置的图片模型${model ? `（${model}）` : ''}`
}
