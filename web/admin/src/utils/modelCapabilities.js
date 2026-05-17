/** Modality tags aligned with backend `client` constants. */
export const MODALITY_TEXT = 'text'
export const MODALITY_IMAGE = 'image'
export const MODALITY_AUDIO = 'audio'
export const MODALITY_VIDEO = 'video'

const LABELS = {
  [MODALITY_TEXT]: '文本',
  [MODALITY_IMAGE]: '图片',
  [MODALITY_AUDIO]: '音频',
  [MODALITY_VIDEO]: '视频',
}

export function modalityLabel(modality) {
  return LABELS[modality] || modality
}

export function formatModelCapabilities(capabilities) {
  const list = Array.isArray(capabilities) ? capabilities : []
  if (!list.length) return '文本（推断）'
  return list.map((c) => modalityLabel(c)).join(' · ')
}

export function modelHasCapability(model, modality) {
  const caps = Array.isArray(model?.capabilities) ? model.capabilities : []
  if (!caps.length && modality === MODALITY_TEXT) return true
  return caps.includes(modality)
}

export function filterModelsByCapability(models, modality) {
  const list = Array.isArray(models) ? models : []
  return list.filter((m) => modelHasCapability(m, modality))
}
