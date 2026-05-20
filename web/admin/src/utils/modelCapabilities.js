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
  if (!list.length) return ''
  return list.map((c) => modalityLabel(c)).join(' · ')
}
