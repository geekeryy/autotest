/** Preferred Xiaomi upstream id for image turns (server applies the same routing). */
export const XIAOMI_VISION_MODEL_PREFERRED = 'mimo-v2.5'

const VISION_CANDIDATES = [
  'mimo-v2.5',
  'mimo-v2-5',
  'mimo-v2.5-preview',
  'mimo-v2-omni',
]

export function isXiaomiVisionModel(model) {
  const id = String(model || '').toLowerCase()
  if (!id) return false
  if (id.includes('omni')) return true
  return id.includes('2.5') || id.includes('v2-5')
}

/** @deprecated Use server-side routing; kept for UI hints only. */
export function modelSupportsVision(providerType, model) {
  if (String(providerType || '').toLowerCase() !== 'xiaomi') return false
  return isXiaomiVisionModel(model)
}

export function findVisionModel(providerType, modelIds) {
  if (String(providerType || '').toLowerCase() !== 'xiaomi') return ''
  const list = Array.isArray(modelIds) ? modelIds : []
  for (const want of VISION_CANDIDATES) {
    const hit = list.find((id) => String(id).toLowerCase() === want)
    if (hit) return hit
  }
  const fuzzy = list.find((id) => isXiaomiVisionModel(id) && String(id).toLowerCase().includes('2.5'))
  if (fuzzy) return fuzzy
  const omni = list.find((id) => String(id).toLowerCase().includes('omni'))
  return omni || XIAOMI_VISION_MODEL_PREFERRED
}
